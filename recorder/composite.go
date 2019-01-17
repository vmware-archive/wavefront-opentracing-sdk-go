package recorder

import "github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"

// CompositeSpanRecorder record spans to multiple SpanRecorder.
type CompositeSpanRecorder struct {
	recorders []tracer.SpanRecorder
}

// NewCompositeSpanRecorder returns a SpanRecorder with multiple sub recorders.
func NewCompositeSpanRecorder(recorders ...tracer.SpanRecorder) tracer.SpanRecorder {
	return CompositeSpanRecorder{recorders: recorders}
}

// RecordSpan complies with the tracer.Recorder interface.
func (c CompositeSpanRecorder) RecordSpan(span tracer.RawSpan) {
	for _, recorder := range c.recorders {
		recorder.RecordSpan(span)
	}
}
