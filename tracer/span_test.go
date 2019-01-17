package tracer

import (
	"testing"

	"github.com/opentracing/opentracing-go/ext"
	"github.com/stretchr/testify/assert"
)

func TestSpan_Baggage(t *testing.T) {
	recorder := NewInMemoryRecorder()
	tracer := New(recorder)
	span := tracer.StartSpan("x")
	span.SetBaggageItem("x", "y")
	assert.Equal(t, "y", span.BaggageItem("x"))
	span.Finish()
	spans := recorder.GetSpans()
	assert.Equal(t, 1, len(spans))
	assert.Equal(t, map[string]string{"x": "y"}, spans[0].Context.Baggage)

	recorder.Reset()
	span = tracer.StartSpan("x")
	span.SetBaggageItem("x", "y")
	baggage := make(map[string]string)
	span.Context().ForeachBaggageItem(func(k, v string) bool {
		baggage[k] = v
		return true
	})
	assert.Equal(t, map[string]string{"x": "y"}, baggage)

	span.SetBaggageItem("a", "b")
	baggage = make(map[string]string)
	span.Context().ForeachBaggageItem(func(k, v string) bool {
		baggage[k] = v
		return false // exit early
	})
	assert.Equal(t, 1, len(baggage))
	span.Finish()
	spans = recorder.GetSpans()
	assert.Equal(t, 1, len(spans))
	assert.Equal(t, 2, len(spans[0].Context.Baggage))
}

func TestSpan_Sampling(t *testing.T) {
	recorder := NewInMemoryRecorder()
	tracer := New(recorder, WithSampler(AllwaysSample{}))
	span := tracer.StartSpan("x")
	span.Finish()
	assert.Equal(t, 1, len(recorder.GetSampledSpans()), "by default span should be sampled")

	recorder.Reset()
	span = tracer.StartSpan("x")
	ext.SamplingPriority.Set(span, 0)
	span.Finish()
	assert.Equal(t, 0, len(recorder.GetSampledSpans()), "SamplingPriority=0 should turn off sampling")

	tracer = New(recorder, WithSampler(NeverSample{}))

	recorder.Reset()
	span = tracer.StartSpan("x")
	span.Finish()
	assert.Equal(t, 0, len(recorder.GetSampledSpans()), "by default span should not be sampled")

	recorder.Reset()
	span = tracer.StartSpan("x")
	ext.SamplingPriority.Set(span, 1)
	span.Finish()
	assert.Equal(t, 1, len(recorder.GetSampledSpans()), "SamplingPriority=1 should turn on sampling")
}
