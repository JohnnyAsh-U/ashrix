package telemetry

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type rpcRecorder struct {
	tracer   trace.Tracer
	requests metric.Int64Counter
	duration metric.Float64Histogram
}

func newRPCRecorder(serviceName, direction string) rpcRecorder {
	meter := otel.GetMeterProvider().Meter(serviceName)
	requests, _ := meter.Int64Counter("rpc." + direction + ".request.count")
	duration, _ := meter.Float64Histogram("rpc."+direction+".duration", metric.WithUnit("s"))
	return rpcRecorder{tracer: otel.Tracer(serviceName), requests: requests, duration: duration}
}

func (r rpcRecorder) start(ctx context.Context, method string, kind trace.SpanKind) (context.Context, func(error)) {
	ctx, span := r.tracer.Start(ctx, method,
		trace.WithSpanKind(kind),
		trace.WithAttributes(
			attribute.String("rpc.system.name", "grpc"),
			attribute.String("rpc.method", method),
		),
	)
	started := time.Now()
	return ctx, func(err error) {
		grpcStatus := codes.OK
		if err != nil && !errors.Is(err, io.EOF) {
			grpcStatus = status.Code(err)
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
		}
		attrs := metric.WithAttributes(
			attribute.String("rpc.method", method),
			attribute.Int("rpc.grpc.status_code", int(grpcStatus)),
		)
		r.requests.Add(ctx, 1, attrs)
		r.duration.Record(ctx, time.Since(started).Seconds(), attrs)
		span.End()
	}
}

// UnaryServerInterceptor traces and measures incoming unary gRPC calls.
func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	recorder := newRPCRecorder(serviceName, "server")
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = extractTraceContext(ctx)
		ctx, finish := recorder.start(ctx, info.FullMethod, trace.SpanKindServer)
		response, err := handler(ctx, request)
		finish(err)
		return response, err
	}
}

// StreamServerInterceptor traces and measures incoming streaming gRPC calls.
func StreamServerInterceptor(serviceName string) grpc.StreamServerInterceptor {
	recorder := newRPCRecorder(serviceName, "server")
	return func(server any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := extractTraceContext(stream.Context())
		ctx, finish := recorder.start(ctx, info.FullMethod, trace.SpanKindServer)
		err := handler(server, &serverStreamWithContext{ServerStream: stream, ctx: ctx})
		finish(err)
		return err
	}
}

// UnaryClientInterceptor traces and measures outgoing unary gRPC calls.
func UnaryClientInterceptor(serviceName string) grpc.UnaryClientInterceptor {
	recorder := newRPCRecorder(serviceName, "client")
	return func(ctx context.Context, method string, request, reply any, connection *grpc.ClientConn, invoker grpc.UnaryInvoker, options ...grpc.CallOption) error {
		ctx, finish := recorder.start(ctx, method, trace.SpanKindClient)
		ctx = injectTraceContext(ctx)
		err := invoker(ctx, method, request, reply, connection, options...)
		finish(err)
		return err
	}
}

// StreamClientInterceptor traces and measures outgoing streaming gRPC calls.
func StreamClientInterceptor(serviceName string) grpc.StreamClientInterceptor {
	recorder := newRPCRecorder(serviceName, "client")
	return func(ctx context.Context, descriptor *grpc.StreamDesc, connection *grpc.ClientConn, method string, streamer grpc.Streamer, options ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx, finish := recorder.start(ctx, method, trace.SpanKindClient)
		ctx = injectTraceContext(ctx)
		stream, err := streamer(ctx, descriptor, connection, method, options...)
		if err != nil {
			finish(err)
			return nil, err
		}
		return &clientStreamWithFinish{ClientStream: stream, finish: finish}, nil
	}
}

type serverStreamWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *serverStreamWithContext) Context() context.Context { return s.ctx }

type clientStreamWithFinish struct {
	grpc.ClientStream
	finishOnce sync.Once
	finish     func(error)
}

func (s *clientStreamWithFinish) finishWith(err error) {
	s.finishOnce.Do(func() { s.finish(err) })
}

func (s *clientStreamWithFinish) SendMsg(message any) error {
	err := s.ClientStream.SendMsg(message)
	if err != nil {
		s.finishWith(err)
	}
	return err
}

func (s *clientStreamWithFinish) RecvMsg(message any) error {
	err := s.ClientStream.RecvMsg(message)
	if err != nil {
		s.finishWith(err)
	}
	return err
}

func extractTraceContext(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, metadataCarrier(md))
}

func injectTraceContext(ctx context.Context) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	if md == nil {
		md = metadata.MD{}
	}
	otel.GetTextMapPropagator().Inject(ctx, metadataCarrier(md))
	return metadata.NewOutgoingContext(ctx, md)
}

type metadataCarrier metadata.MD

func (m metadataCarrier) Get(key string) string {
	values := metadata.MD(m).Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (m metadataCarrier) Set(key, value string) {
	metadata.MD(m).Set(key, value)
}

func (m metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

var _ propagation.TextMapCarrier = metadataCarrier{}
