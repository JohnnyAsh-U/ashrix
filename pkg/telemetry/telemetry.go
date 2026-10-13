// Package telemetry bootstraps OpenTelemetry for the Ashrix Gateway.
//
// It wires up three signals:
//   - Traces   → OTLP gRPC → collector
//   - Metrics  → OTLP gRPC → collector (periodic, default 15 s)
//   - Logs     → OTLP gRPC → collector (via log bridge; slog records are bridged)
//
// Configuration (all read from environment / Viper):
//
//	OTEL_EXPORTER_OTLP_ENDPOINT   gRPC endpoint of the collector, e.g. "localhost:4317"
//	OTEL_EXPORTER_OTLP_INSECURE   "true" to use plaintext gRPC (default: false → TLS)
//	OTEL_SERVICE_NAME              service name reported in every signal
//	OTEL_SERVICE_VERSION           binary version reported in resource attributes
//	OTEL_METRICS_INTERVAL          export interval for metrics, e.g. "15s" (default 15s)
//	OTEL_ENABLED                   "false" to disable telemetry entirely (default: true)
//
// Usage:
//
//	tp, err := telemetry.Bootstrap(ctx, cfg)
//	if err != nil { ... }
//	defer tp.Shutdown(ctx)
//
// The returned *Provider carries the TracerProvider, MeterProvider, and LoggerProvider.
// Call telemetry.OtelSlogHandler() to get an slog.Handler that forwards records to the
// OTLP log exporter.
package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds the telemetry configuration loaded from env / Viper.
type Config struct {
	// Endpoint is the OTLP gRPC collector address, e.g. "localhost:4317".
	// Required when Enabled is true.
	Endpoint string

	// Insecure disables TLS for the gRPC connection to the collector.
	// Set to true in development against a local collector.
	Insecure bool

	// ServiceName is the logical name of this service, e.g. "ashrix-gateway".
	ServiceName string

	// ServiceVersion is the binary version, e.g. "v1.1.0".
	ServiceVersion string

	// MetricsInterval is how often metrics are pushed to the collector.
	// Defaults to 15 seconds.
	MetricsInterval time.Duration

	// GatewayID is attached as a resource attribute on all signals.
	GatewayID string

	// Enabled controls whether OTel is initialised at all.
	// If false, no-op providers are installed and Bootstrap returns immediately.
	Enabled bool
}

// Provider holds the three OTel SDK providers and their shared gRPC connection.
// Call Shutdown when the process exits to flush pending telemetry.
type Provider struct {
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	LoggerProvider *sdklog.LoggerProvider

	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
	conn           *grpc.ClientConn
}

// Tracer returns a named tracer from the global TracerProvider.
func (p *Provider) Tracer(name string) trace.Tracer {
	return p.TracerProvider.Tracer(name)
}

// Meter returns a named meter from the global MeterProvider.
func (p *Provider) Meter(name string) metric.Meter {
	return p.MeterProvider.Meter(name)
}

// Shutdown flushes and shuts down all three providers in reverse order, then
// closes the shared gRPC connection.  Pass the process shutdown context.
func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error

	if p.loggerProvider != nil {
		if err := p.loggerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("log provider shutdown: %w", err))
		}
	}
	if p.meterProvider != nil {
		if err := p.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("metric provider shutdown: %w", err))
		}
	}
	if p.tracerProvider != nil {
		if err := p.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("trace provider shutdown: %w", err))
		}
	}
	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("grpc conn close: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("telemetry shutdown errors: %v", errs)
	}
	return nil
}

// NewSlogHandler bridges slog records to the configured OTLP log provider.
func NewSlogHandler(provider *sdklog.LoggerProvider) slog.Handler {
	if provider == nil {
		return nil
	}
	return &slogHandler{logger: provider.Logger("ashrix-gateway")}
}

type slogHandler struct {
	logger otellog.Logger
	attrs  []attribute.KeyValue
	groups []string
}

func (h *slogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	severity := slogSeverity(level)
	return h.logger.Enabled(ctx, otellog.EnabledParameters{Severity: severity})
}

func (h *slogHandler) Handle(ctx context.Context, record slog.Record) error {
	logRecord := otellog.Record{}
	logRecord.SetTimestamp(record.Time)
	logRecord.SetObservedTimestamp(time.Now())
	logRecord.SetSeverity(slogSeverity(record.Level))
	logRecord.SetSeverityText(record.Level.String())
	logRecord.SetBody(attribute.StringValue(record.Message))
	logRecord.AddAttributes(h.attrs...)
	record.Attrs(func(attr slog.Attr) bool {
		appendSlogAttr(&logRecord, attr, h.groups)
		return true
	})
	h.logger.Emit(ctx, logRecord)
	return nil
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := h.clone()
	for _, attr := range attrs {
		appendSlogAttrToSlice(&clone.attrs, attr, clone.groups)
	}
	return clone
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := h.clone()
	clone.groups = append(clone.groups, name)
	return clone
}

func (h *slogHandler) clone() *slogHandler {
	return &slogHandler{
		logger: h.logger,
		attrs:  append([]attribute.KeyValue(nil), h.attrs...),
		groups: append([]string(nil), h.groups...),
	}
}

func appendSlogAttr(record *otellog.Record, attr slog.Attr, groups []string) {
	var attrs []attribute.KeyValue
	appendSlogAttrToSlice(&attrs, attr, groups)
	record.AddAttributes(attrs...)
}

func appendSlogAttrToSlice(attrs *[]attribute.KeyValue, attr slog.Attr, groups []string) {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return
	}
	if attr.Value.Kind() == slog.KindGroup {
		nestedGroups := append(append([]string(nil), groups...), attr.Key)
		for _, nested := range attr.Value.Group() {
			appendSlogAttrToSlice(attrs, nested, nestedGroups)
		}
		return
	}
	keyParts := append([]string(nil), groups...)
	if attr.Key != "" {
		keyParts = append(keyParts, attr.Key)
	}
	key := strings.Join(keyParts, ".")
	if key == "" {
		return
	}
	*attrs = append(*attrs, attribute.KeyValue{Key: attribute.Key(key), Value: slogAttributeValue(attr.Value)})
}

func slogAttributeValue(value slog.Value) attribute.Value {
	switch value.Kind() {
	case slog.KindBool:
		return attribute.BoolValue(value.Bool())
	case slog.KindDuration:
		return attribute.Int64Value(int64(value.Duration()))
	case slog.KindFloat64:
		return attribute.Float64Value(value.Float64())
	case slog.KindInt64:
		return attribute.Int64Value(value.Int64())
	case slog.KindString:
		return attribute.StringValue(value.String())
	case slog.KindTime:
		return attribute.StringValue(value.Time().Format(time.RFC3339Nano))
	default:
		return attribute.StringValue(value.String())
	}
}

func slogSeverity(level slog.Level) otellog.Severity {
	switch {
	case level >= slog.LevelError:
		return otellog.SeverityError
	case level >= slog.LevelWarn:
		return otellog.SeverityWarn
	case level >= slog.LevelInfo:
		return otellog.SeverityInfo
	default:
		return otellog.SeverityDebug
	}
}

// HTTPMiddleware records server spans and request metrics for the gateway ingress.
func HTTPMiddleware(gatewayID string) func(http.Handler) http.Handler {
	meter := otel.GetMeterProvider().Meter("ashrix-gateway")
	requestCount, countErr := meter.Int64Counter("http.server.request.count",
		metric.WithDescription("Number of requests handled by the gateway"),
	)
	requestDuration, durationErr := meter.Float64Histogram("http.server.request.duration",
		metric.WithUnit("ms"),
		metric.WithDescription("Gateway request duration"),
	)
	tracer := otel.Tracer("ashrix-gateway")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parentCtx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, span := tracer.Start(parentCtx, "HTTP "+r.Method,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.request.method", r.Method),
					attribute.String("network.protocol.name", "http"),
					attribute.String("ashrix.gateway.id", gatewayID),
				),
			)
			start := time.Now()
			next.ServeHTTP(w, r.WithContext(ctx))
			elapsed := float64(time.Since(start)) / float64(time.Millisecond)
			attrs := []attribute.KeyValue{
				attribute.String("http.request.method", r.Method),
				attribute.String("ashrix.gateway.id", gatewayID),
			}
			if countErr == nil {
				requestCount.Add(ctx, 1, metric.WithAttributes(attrs...))
			}
			if durationErr == nil {
				requestDuration.Record(ctx, elapsed, metric.WithAttributes(attrs...))
			}
			span.End()
		})
	}
}

// Bootstrap initialises all three OTel signals and sets the global providers.
// If cfg.Enabled is false it installs no-op providers and returns immediately.
func Bootstrap(ctx context.Context, cfg Config) (*Provider, error) {
	if !cfg.Enabled {
		// Install no-op providers so instrumented code does not panic.
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		otel.SetMeterProvider(noop.NewMeterProvider())
		global.SetLoggerProvider(sdklog.NewLoggerProvider())
		return &Provider{
			TracerProvider: otel.GetTracerProvider(),
			MeterProvider:  otel.GetMeterProvider(),
		}, nil
	}

	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("telemetry: OTEL_EXPORTER_OTLP_ENDPOINT is required when OTEL_ENABLED=true")
	}

	if cfg.MetricsInterval == 0 {
		cfg.MetricsInterval = 15 * time.Second
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "ashrix-gateway"
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// ── Shared gRPC connection ─────────────────────────────────────────────────
	dialOpts := []grpc.DialOption{grpc.WithBlock()}
	if cfg.Insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(nil)))
	}

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, cfg.Endpoint, dialOpts...) //nolint:staticcheck
	if err != nil {
		return nil, fmt.Errorf("telemetry: dial collector %q: %w", cfg.Endpoint, err)
	}

	// ── Resource ──────────────────────────────────────────────────────────────
	resourceAttrs := []attribute.KeyValue{
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		semconv.DeploymentEnvironment("production"),
	}
	if cfg.GatewayID != "" {
		resourceAttrs = append(resourceAttrs, attribute.String("ashrix.gateway.id", cfg.GatewayID))
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(resourceAttrs...),
		resource.WithFromEnv(), // picks up OTEL_RESOURCE_ATTRIBUTES
		resource.WithProcess(), // adds process.pid, process.executable etc.
		resource.WithHost(),    // adds host.name
	)
	if err != nil {
		res = resource.Default()
	}

	p := &Provider{conn: conn}

	// ── Trace provider ────────────────────────────────────────────────────────
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithGRPCConn(conn),
	)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("telemetry: trace exporter: %w", err)
	}

	p.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(p.tracerProvider)
	p.TracerProvider = p.tracerProvider

	// ── Metric provider ───────────────────────────────────────────────────────
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithGRPCConn(conn),
	)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("telemetry: metric exporter: %w", err)
	}

	p.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(metricExporter,
				sdkmetric.WithInterval(cfg.MetricsInterval),
			),
		),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(p.meterProvider)
	p.MeterProvider = p.meterProvider

	// ── Log provider ──────────────────────────────────────────────────────────
	logExporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithGRPCConn(conn),
	)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("telemetry: log exporter: %w", err)
	}

	p.loggerProvider = sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(p.loggerProvider)
	p.LoggerProvider = p.loggerProvider

	return p, nil
}
