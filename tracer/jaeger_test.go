package tracer

import (
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestJaegerWavefrontPropagator_Extract(t *testing.T) {
	traceIdHeader, baggagePrefix := "uber-trace-id", "uberctx-"
	jaegerWfPropagator := JaegerWavefrontPropagator{
		traceIdHeader: traceIdHeader,
		baggagePrefix: baggagePrefix,
	}

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
	traceId := "00000000-0000-0000-3871-de7e09c53ae8"
	spanId := "00000000-0000-0000-7499-dd16d98ab60e"
	tmc := opentracing.HTTPHeadersCarrier(http.Header{})
	jaegerWfPropagator := JaegerWavefrontPropagator{
		traceIdHeader: traceIdHeader,
		baggagePrefix: baggagePrefix,
	}
	spanContext := SpanContext{
		TraceID: traceId,
		SpanID:  spanId,
		Sampled: nil,
		Baggage: nil,
	}
	if err := jaegerWfPropagator.Inject(spanContext, tmc); err != nil {
		t.Fatalf("%d: %v", 0, err)
	}
	_, ok := tmc[traceIdHeader]
	assert.True(t, ok)
	assert.Equal(t, "3871de7e09c53ae8:7499dd16d98ab60e::0", tmc[traceIdHeader][0])
}

func TestConvertUUID(t *testing.T) {
	assert.Equal(t, "", ConvertUUID(""))
	Id := "00000000-0000-0000-3871-de7e09c53ae8"
	assert.Equal(t, "3871de7e09c53ae8", ConvertUUID(Id))
}

func TestToUUID(t *testing.T) {
	id := "ef27b4b9f6e946f5ab2b47bbb24746c5"
	out, _ := ToUUID(id)
	assert.Equal(t, "ef27b4b9-f6e9-46f5-ab2b-47bbb24746c5", out)
}

func TestWavefrontUuidToJaegerIdConversion(t *testing.T) {
	in := uuid.New().String()
	out, _ := ToUUID(ConvertUUID(in))
	assert.Equal(t, in, out)
}
