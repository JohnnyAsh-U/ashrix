package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestUnaryClientInterceptorInjectsChildSpanContext(t *testing.T) {
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})

	parentCtx, parentSpan := provider.Tracer("test").Start(context.Background(), "parent")
	parentSpanID := parentSpan.SpanContext().SpanID()
	parentSpan.End()

	interceptor := UnaryClientInterceptor("test")
	err := interceptor(parentCtx, "/test.Service/Call", nil, nil, nil,
		func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			childSpanContext := trace.SpanContextFromContext(ctx)
			if !childSpanContext.IsValid() || childSpanContext.SpanID() == parentSpanID {
				t.Fatalf("expected a valid child span context, got %v", childSpanContext)
			}

			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				t.Fatal("outgoing trace metadata was not added")
			}
			propagatedContext := propagation.TraceContext{}.Extract(context.Background(), metadataCarrier(md))
			propagatedSpanContext := trace.SpanContextFromContext(propagatedContext)
			if propagatedSpanContext.SpanID() != childSpanContext.SpanID() {
				t.Fatalf("propagated span %s does not match child span %s", propagatedSpanContext.SpanID(), childSpanContext.SpanID())
			}
			return nil
		})
	if err != nil {
		t.Fatalf("interceptor returned an error: %v", err)
	}
}
