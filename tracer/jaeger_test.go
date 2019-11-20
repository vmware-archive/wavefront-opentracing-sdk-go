package tracer

import (
	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"github.com/uber/jaeger-client-go"
	"net/http"
	"testing"
)

func TestJaegerWavefrontPropagator_Extract(t *testing.T) {
	traceIdHeader, baggagePrefix := "uber-trace-id", "uberctx-"
	jaegerWfPropagator := NewJaegerWavefrontPropagator(WithTraceIdHeader(traceIdHeader),
		WithBaggagePrefix(baggagePrefix))

	val := "3871de7e09c53ae8:7499dd16d98ab60e:3771de7e09c55ae8:1"
	carrier := opentracing.HTTPHeadersCarrier(http.Header{})
	carrier[traceIdHeader] = []string{val}
	ctx, _ := jaegerWfPropagator.Extract(carrier)
	assert.NotNil(t, ctx)
	assert.Equal(t, "00000000-0000-0000-3871-de7e09c53ae8", ctx.(SpanContext).TraceID)
	assert.Equal(t, "00000000-0000-0000-7499-dd16d98ab60e", ctx.(SpanContext).SpanID)
	assert.Equal(t, "00000000-0000-0000-7499-dd16d98ab60e", ctx.(SpanContext).Baggage["parent-id"])
	assert.True(t, ctx.(SpanContext).IsSampled())

	invalidVal := ":7499dd16d98ab60e:3771de7e09c55ae8:1"
	invalidCarrier := opentracing.HTTPHeadersCarrier(http.Header{})
	invalidCarrier[traceIdHeader] = []string{invalidVal}
	invalidCtx, _ := jaegerWfPropagator.Extract(invalidCarrier)
	assert.Nil(t, invalidCtx)
}

func TestJaegerWavefrontPropagator_Inject(t *testing.T) {
	traceIdHeader, baggagePrefix := "Uber-Trace-Id", "Uberctx-"
	tmc := opentracing.HTTPHeadersCarrier(http.Header{})
	jaegerWfPropagator := NewJaegerWavefrontPropagator(WithBaggagePrefix(baggagePrefix),
		WithTraceIdHeader(traceIdHeader))
	jaegerSC := jaeger.NewSpanContext(jaeger.TraceID{Low: 1}, 1, 1, true, nil)
	jaegerSC = jaegerSC.WithBaggageItem("x", "y")
	if err := jaegerWfPropagator.Inject(jaegerSC, tmc); err != nil {
		t.Fatalf("%d: %v", 0, err)
	}
	_, ok := tmc[traceIdHeader]
	assert.True(t, ok)
	assert.Equal(t, "0000000000000001:0000000000000001:0000000000000001:1", tmc[traceIdHeader][0])
}


func TestToUUID(t *testing.T) {
	id := "ef27b4b9f6e946f5ab2b47bbb24746c5"
	out, _ := ToUUID(id)
	assert.Equal(t, "ef27b4b9-f6e9-46f5-ab2b-47bbb24746c5", out)
}
