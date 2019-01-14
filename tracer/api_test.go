package tracer

import (
	"testing"
	"time"

	ot "github.com/opentracing/opentracing-go"
	// "github.com/opentracing/opentracing-go/harness"
	"github.com/stretchr/testify/assert"
)

// newTracer creates a new tracer for each test, and returns a nil cleanup function.
func newTracer() (tracer ot.Tracer, closer func()) {
	tracer = NewWithOptions(Options{
		Recorder:     NewInMemoryRecorder(),
		ShouldSample: func(traceID string) bool { return true }, // always sample
	})
	return tracer, nil
}

func TestInMemoryRecorderSpans(t *testing.T) {
	recorder := NewInMemoryRecorder()
	var apiRecorder SpanRecorder = recorder
	span := RawSpan{
		Context:   SpanContext{},
		Operation: "test-span",
		Start:     time.Now(),
		Duration:  -1,
	}
	apiRecorder.RecordSpan(span)
	assert.Equal(t, []RawSpan{span}, recorder.GetSpans())
	assert.Equal(t, []RawSpan{}, recorder.GetSampledSpans())
}

// TODO: Un-comment when the "github.com/opentracing/opentracing-go/harness"
// package is available on a release
//
// func TestAPICheck(t *testing.T) {
// 	harness.RunAPIChecks(t,
// 		newTracer,
// 		harness.CheckEverything(),
// 		harness.UseProbe(apiCheckProbe{}),
// 	)
// }

// implements harness.APICheckProbe
type apiCheckProbe struct{}

// SameTrace helps tests assert that this tracer's spans are from the same trace.
func (apiCheckProbe) SameTrace(first, second ot.Span) bool {
	span1, ok := first.(*spanImpl)
	if !ok { // some other tracer's span
		return false
	}
	span2, ok := second.(*spanImpl)
	if !ok { // some other tracer's span
		return false
	}
	return span1.raw.Context.TraceID == span2.raw.Context.TraceID
}

// SameSpanContext helps tests assert that a span and a context are from the same trace and span.
func (apiCheckProbe) SameSpanContext(span ot.Span, spanContext ot.SpanContext) bool {
	sp, ok := span.(*spanImpl)
	if !ok {
		return false
	}
	ctx, ok := spanContext.(SpanContext)
	if !ok {
		return false
	}
	return sp.raw.Context.TraceID == ctx.TraceID && sp.raw.Context.SpanID == ctx.SpanID
}
