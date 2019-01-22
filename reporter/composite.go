package reporter

import "github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"

// CompositeSpanReporter record spans to multiple SpanReporter.
type CompositeSpanReporter struct {
	reporters []tracer.SpanReporter
}

// NewCompositeSpanReporter returns a SpanReporter with multiple sub reporter.
func NewCompositeSpanReporter(reporters ...tracer.SpanReporter) tracer.SpanReporter {
	return CompositeSpanReporter{reporters: reporters}
}

// ReportSpan complies with the tracer.SpanReporter interface.
func (c CompositeSpanReporter) ReportSpan(span tracer.RawSpan) {
	for _, reporter := range c.reporters {
		reporter.ReportSpan(span)
	}
}
