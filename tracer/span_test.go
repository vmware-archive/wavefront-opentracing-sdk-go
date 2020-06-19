package tracer

import (
	"github.com/opentracing/opentracing-go/log"
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

func TestBaggageItems(t *testing.T) {
	// Create parentCtx with baggage items
	reporter := NewInMemoryReporter()
	tracer := New(reporter)
	baggage := map[string]string{
		"foo":  "bar",
		"user": "name",
	}
	parent_ctx := SpanContext{
		TraceID: "traceId",
		SpanID:  "spanId",
		Sampled: nil,
		Baggage: baggage,
	}
	child := tracer.StartSpan("test", opentracing.ChildOf(parent_ctx))
	assert.Equal(t, "bar", child.BaggageItem("foo"))
	assert.Equal(t, "name", child.BaggageItem("user"))

	// parent and follows
	items := map[string]string{
		"tracker": "id",
		"db.name": "name",
	}
	follows_ctx := SpanContext{
		TraceID: "traceId",
		SpanID:  "spanId",
		Sampled: nil,
		Baggage: items,
	}
	follower := tracer.StartSpan("follow", opentracing.ChildOf(parent_ctx),
		opentracing.FollowsFrom(follows_ctx))
	assert.Equal(t, "bar", follower.BaggageItem("foo"))
	assert.Equal(t, "id", follower.BaggageItem("tracker"))
	assert.Equal(t, "id", follower.BaggageItem("tracker"))
	assert.Equal(t, "name", follower.BaggageItem("db.name"))

	// validate root span
	reporter.Reset()
	span := tracer.StartSpan("x")
	span.Finish()
	spans := reporter.getSpans()
	assert.Equal(t, 1, len(spans))
	assert.NotNil(t, spans[0].Context.Baggage)
	assert.Empty(t, spans[0].Context.Baggage)
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

func TestSamplingDebug(t *testing.T) {
	reporter := NewInMemoryReporter()
	tracer := New(reporter, WithSampler(NeverSample{}))

	reporter.Reset()
	span := tracer.StartSpan("x")
	span.SetTag("debug", true);
	span.Finish()
	assert.Equal(t, 1, len(reporter.getSampledSpans()), "debug tag should turn on sampling")
}

func TestEmptySpanTag(t *testing.T) {
	span := spanImpl{}
	span.SetTag("key", "")
	found := getAppTag("key", span.raw.Tags)
	assert.Equal(t, found, false)
}

func TestSpanImpl_LogKV(t *testing.T) {
	reporter := NewInMemoryReporter()
	tracer := New(reporter)
	sp := tracer.StartSpan("foobar")
	sp.LogKV("foo", "bar")
	sp.Finish()
	assert.Equal(t, 1, len(reporter.spans), "should contain exactly one span")
	assert.Equal(t, 1, len(reporter.spans[0].Logs), "should contain exactly one spanlog")
	assert.Equal(t, 1, len(reporter.spans[0].Logs[0].Fields), "should contain exactly one spanlog field")
	assert.Equal(t, "foo", reporter.spans[0].Logs[0].Fields[0].Key(), "wrong spanlog key")
	assert.Equal(t, "bar", reporter.spans[0].Logs[0].Fields[0].Value(), "wrong spanlog value")
}

func TestSpanImpl_FinishWithOptions(t *testing.T) {
	reporter := NewInMemoryReporter()
	tracer := New(reporter)
	sp := tracer.StartSpan("foobar")
	sp.FinishWithOptions(opentracing.FinishOptions{
		LogRecords: []opentracing.LogRecord{
			{
				Timestamp: time.Now(),
				Fields: []log.Field{
					log.String("foo", "bar"),
				},
			},
		},
	})
	assert.Equal(t, 1, len(reporter.spans), "should contain exactly one span")
	assert.Equal(t, 1, len(reporter.spans[0].Logs), "should contain exactly one spanlog")
	assert.Equal(t, 1, len(reporter.spans[0].Logs[0].Fields), "should contain exactly one spanlog field")
	assert.Equal(t, "foo", reporter.spans[0].Logs[0].Fields[0].Key(), "wrong spanlog key")
	assert.Equal(t, "bar", reporter.spans[0].Logs[0].Fields[0].Value(), "wrong spanlog value")
}

func getAppTag(key string, tags map[string]interface{}) bool {
	if len(tags) > 0 {
		if _, found := tags[key]; found {
			return true
		}
	}
	return false
}
