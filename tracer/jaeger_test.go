package tracer

import (
	"net/http"
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJaegerWavefrontPropagator_Extract(t *testing.T) {
	traceIdHeader, baggagePrefix := "uber-trace-id", "uberctx-"
	tracer := New(NewInMemoryReporter(), WithJaegerPropagator(traceIdHeader, baggagePrefix))

	carrier := opentracing.HTTPHeadersCarrier(http.Header{})
	carrier.Set(traceIdHeader, "3871de7e09c53ae8:7499dd16d98ab60e:3771de7e09c55ae8:1")
	carrier.Set(baggagePrefix+"key-1", "val-1")
	carrier.Set(baggagePrefix+"key-2", "val-2")

	ctx, _ := tracer.Extract(JaegerWavefrontPropagator{}, carrier)
	require.IsType(t, SpanContext{}, ctx)
	assert.Equal(t, "00000000-0000-0000-3871-de7e09c53ae8", ctx.(SpanContext).TraceID)
	assert.Equal(t, "00000000-0000-0000-7499-dd16d98ab60e", ctx.(SpanContext).SpanID)
	assert.Equal(t, "00000000-0000-0000-7499-dd16d98ab60e", ctx.(SpanContext).Baggage["parent-id"])
	assert.True(t, ctx.(SpanContext).IsSampled())
	assert.True(t, *ctx.(SpanContext).SamplingDecision())
	assert.Equal(t, "val-1", ctx.(SpanContext).Baggage["Key-1"])
	assert.Equal(t, "val-2", ctx.(SpanContext).Baggage["Key-2"])

	invalidVal := ":7499dd16d98ab60e:3771de7e09c55ae8:1"
	invalidCarrier := opentracing.HTTPHeadersCarrier(http.Header{})
	invalidCarrier[traceIdHeader] = []string{invalidVal}
	invalidCtx, _ := tracer.Extract(JaegerWavefrontPropagator{}, invalidCarrier)
	assert.Equal(t, "", invalidCtx.(SpanContext).TraceID)
}

func TestJaegerWavefrontPropagator_Inject(t *testing.T) {
	traceIdHeader, baggagePrefix := "Uber-Trace-Id", "Uberctx-"
	tmc := opentracing.HTTPHeadersCarrier(http.Header{})
	tracer := New(NewInMemoryReporter(), WithJaegerPropagator(traceIdHeader, baggagePrefix))
	sampled := false
	spanContext := SpanContext{
		TraceID: "3871de7e09c53ae8",
		SpanID:  "7499dd16d98ab60e",
		Sampled: &sampled,
		Baggage: nil,
	}
	spanContext = spanContext.WithBaggageItem("x", "y")
	if err := tracer.Inject(spanContext, JaegerWavefrontPropagator{}, tmc); err != nil {
		t.Fatalf("%d: %v", 0, err)
	}
	_, ok := tmc[traceIdHeader]
	assert.True(t, ok)
	assert.Equal(t, "3871de7e09c53ae8:7499dd16d98ab60e::0", tmc[traceIdHeader][0])
	assert.Equal(t, "false", tmc["Sampling-Decision"][0])
}

func TestToUUID(t *testing.T) {
	id := "ef27b4b9f6e946f5ab2b47bbb24746c5"
	out, _ := ToUUID(id)
	assert.Equal(t, "ef27b4b9-f6e9-46f5-ab2b-47bbb24746c5", out)
}
