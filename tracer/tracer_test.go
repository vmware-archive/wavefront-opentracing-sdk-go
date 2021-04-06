package tracer

import (
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithW3CGenerator(t *testing.T) {
	reporter := NewInMemoryReporter()
	tracer := New(reporter, WithW3CGenerator())

	span := tracer.StartSpan("test-op")
	span.Finish()

	require.Len(t, reporter.spans, 1)
	rawSpan := reporter.spans[0]
	assert.Equal(t, "test-op", rawSpan.Operation)
	assert.Len(t, FromUUID(rawSpan.Context.TraceID), 32)
	assert.Len(t, FromUUID(rawSpan.Context.SpanID), 16)
}

func TestWithW3CPropagator(t *testing.T) {
	reporter := NewInMemoryReporter()
	tracer := New(reporter, WithW3CPropagator())

	carrier := opentracing.TextMapCarrier{}
	carrier["traceparent"] = "00-11111111111111111111111111111111-2222222222222222-01"
	spanCtx, err := tracer.Extract(opentracing.TextMap, carrier)
	assert.NoError(t, err)
	assert.NotNil(t, spanCtx)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", spanCtx.(SpanContext).TraceID)
	assert.Equal(t, "00000000-0000-0000-2222-222222222222", spanCtx.(SpanContext).SpanID)

	span := tracer.StartSpan("test-op", opentracing.FollowsFrom(spanCtx))
	spanCtx = span.Context()
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", spanCtx.(SpanContext).TraceID)
	assert.Len(t, FromUUID(spanCtx.(SpanContext).SpanID), 16)

	span.Finish()
	require.Len(t, reporter.spans, 1)
	spanID := FromUUID(reporter.spans[0].Context.SpanID)
	assert.Len(t, spanID, 16)

	err = tracer.Inject(spanCtx, opentracing.TextMap, carrier)
	assert.NoError(t, err)
	assert.Equal(t, "00-11111111111111111111111111111111-"+spanID+"-01", carrier["traceparent"])
}
