package tracer

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/stretchr/testify/assert"
)

func TestSpan_Baggage(t *testing.T) {
	reporter := NewInMemoryReporter()
	tracer := New(reporter)
	span := tracer.StartSpan("x")
	span.SetBaggageItem("x", "y")
	assert.Equal(t, "y", span.BaggageItem("x"))
	span.Finish()
	spans := reporter.getSpans()
	assert.Equal(t, 1, len(spans))
	assert.Equal(t, map[string]string{"x": "y"}, spans[0].Context.Baggage)

	reporter.Reset()
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
	spans = reporter.getSpans()
	assert.Equal(t, 1, len(spans))
	assert.Equal(t, 2, len(spans[0].Context.Baggage))
}

func TestSampling(t *testing.T) {
	reporter := NewInMemoryReporter()
	tracer := New(reporter)

	span := tracer.StartSpan("x")
	span.Finish()
	assert.Equal(t, 1, len(reporter.getSampledSpans()), "without sampler span should be sampled")
}

func TestSamplingChild(t *testing.T) {
	reporter := NewInMemoryReporter()
	tracer := New(reporter)
	span := tracer.StartSpan("x")
	spanb := tracer.StartSpan("x", opentracing.ChildOf(span.Context()))
	spanb.Finish()
	span.Finish()
	assert.Equal(t, 2, len(reporter.getSampledSpans()), "without sampler span should be sampled")
}

func TestSamplingPriority(t *testing.T) {
	reporter := NewInMemoryReporter()
	tracer := New(reporter, WithSampler(NeverSample{}))

	reporter.Reset()
	span := tracer.StartSpan("x")
	span.Finish()
	assert.Equal(t, 0, len(reporter.getSampledSpans()), "span should not be sampled")

	reporter.Reset()
	span = tracer.StartSpan("x")
	ext.SamplingPriority.Set(span, 1)
	span.Finish()
	assert.Equal(t, 1, len(reporter.getSampledSpans()), "SamplingPriority=1 should turn on sampling")

	tracer = New(reporter) // no sampler, will sampler always

	reporter.Reset()
	span = tracer.StartSpan("x")
	ext.SamplingPriority.Set(span, 0)
	span.Finish()
	assert.Equal(t, 0, len(reporter.getSampledSpans()), "SamplingPriority=0 should turn off sampling")
}

func TestSamplingRate(t *testing.T) {
	reporter := NewInMemoryReporter()
	tracer := New(reporter, WithSampler(RateSampler{Rate: 0}))

	reporter.Reset()
	span := tracer.StartSpan("x")
	span.Finish()
	assert.Equal(t, 0, len(reporter.getSampledSpans()), "Rate 0 will never report")

	tracer = New(reporter, WithSampler(RateSampler{Rate: 100}))

	reporter.Reset()
	span = tracer.StartSpan("x")
	span.Finish()
	assert.Equal(t, 1, len(reporter.getSampledSpans()), "Rate 100 will always report")
}

func TestSamplingDuration(t *testing.T) {
	reporter := NewInMemoryReporter()
	tracer := New(reporter, WithSampler(DurationSampler{Duration: time.Millisecond * 5}))

	reporter.Reset()
	span := tracer.StartSpan("x")
	span.Finish()
	assert.Equal(t, 0, len(reporter.getSampledSpans()), "DurationSampler not working")

	reporter.Reset()
	span = tracer.StartSpan("x")
	time.Sleep(time.Millisecond * 10)
	span.Finish()
	assert.Equal(t, 1, len(reporter.getSampledSpans()), "DurationSampler not working")
}

func TestSamplingChain(t *testing.T) {
	reporter := NewInMemoryReporter()
	tracer := New(reporter,
		WithSampler(NeverSample{}),
		WithSampler(RateSampler{Rate: 100}),
		WithSampler(DurationSampler{Duration: time.Millisecond * 5}),
	)

	span := tracer.StartSpan("x")
	span.Finish()
	assert.Equal(t, 1, len(reporter.getSampledSpans()), "sampled by rate")

	tracer = New(reporter,
		WithSampler(NeverSample{}),
		WithSampler(RateSampler{Rate: 0}),
		WithSampler(DurationSampler{Duration: time.Millisecond * 5}),
	)

	reporter.Reset()
	span = tracer.StartSpan("x")
	time.Sleep(time.Millisecond * 10)
	span.Finish()
	assert.Equal(t, 1, len(reporter.getSampledSpans()), "sampled by duration")

	reporter.Reset()
	span = tracer.StartSpan("x")
	span.Finish()
	assert.Equal(t, 0, len(reporter.getSampledSpans()), "not sampled")
}

func TestSamplingError(t *testing.T) {
	reporter := NewInMemoryReporter()
	tracer := New(reporter, WithSampler(NeverSample{}))

	reporter.Reset()
	span := tracer.StartSpan("x")
	ext.Error.Set(span, true)
	span.Finish()
	assert.Equal(t, 1, len(reporter.getSampledSpans()), "error tag should turn on sampling")
}

func TestEmptySpanTag(t *testing.T) {
	rawSpan := RawSpan{}
	mutex := sync.Mutex{}
	span := spanImpl{nil, mutex, rawSpan}
	span.SetTag("key", "")
	_, ok := getAppTag("key", "true", span.raw.Tags);
	assert.Equal(t, ok, false)
}

func getAppTag(key, defaultVal string, tags map[string]interface{}) (string, bool) {
	if len(tags) > 0 {
		if v, found := tags[key]; found {
			return fmt.Sprint(v), true
		}
	}
	return defaultVal, false
}