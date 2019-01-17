package tracer

// CompositeSpanRecorder record spans to multiple SpanRecorder.
type CompositeSpanRecorder struct {
	recorders []SpanRecorder
}

// NewCompositeSpanRecorder returns a SpanRecorder with multiple sub recorders.
func NewCompositeSpanRecorder(recorders ...SpanRecorder) SpanRecorder {
	return CompositeSpanRecorder{recorders: recorders}
}

// RecordSpan complies with the tracer.Recorder interface.
func (c CompositeSpanRecorder) RecordSpan(span RawSpan) {
	for _, recorder := range c.recorders {
		recorder.RecordSpan(span)
	}
}
