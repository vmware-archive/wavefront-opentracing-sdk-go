package tracer

import (
	"sync"
	"sync/atomic"
)

// InMemorySpanReporter is a simple thread-safe implementation of
// SpanReporter that stores all reported spans in memory, accessible
// via reporter.getSpans(). It is primarily intended for testing purposes.
type InMemorySpanReporter struct {
	sync.RWMutex
	spans []rawSpan
}

// NewInMemoryReporter creates new InMemorySpanReporter
func NewInMemoryReporter() *InMemorySpanReporter {
	return new(InMemorySpanReporter)
}

// ReportSpan implements the respective method of SpanReporter.
func (r *InMemorySpanReporter) ReportSpan(span rawSpan) {
	r.Lock()
	defer r.Unlock()
	r.spans = append(r.spans, span)
}

// getSpans returns a copy of the array of spans accumulated so far.
func (r *InMemorySpanReporter) getSpans() []rawSpan {
	r.RLock()
	defer r.RUnlock()
	spans := make([]rawSpan, len(r.spans))
	copy(spans, r.spans)
	return spans
}

// getSampledSpans returns a slice of spans accumulated so far which were sampled.
func (r *InMemorySpanReporter) getSampledSpans() []rawSpan {
	r.RLock()
	defer r.RUnlock()
	spans := make([]rawSpan, 0, len(r.spans))
	for _, span := range r.spans {
		if span.Context.Sampled {
			spans = append(spans, span)
		}
	}
	return spans
}

// Reset clears the internal array of spans.
func (r *InMemorySpanReporter) Reset() {
	r.Lock()
	defer r.Unlock()
	r.spans = nil
}

// CountingReporter it is primarily intended for testing purposes.
type CountingReporter int32

// ReportSpan implements the respective method of SpanReporter.
func (c *CountingReporter) ReportSpan(r rawSpan) {
	atomic.AddInt32((*int32)(c), 1)
}
