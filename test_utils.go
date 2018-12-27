package tracer

import (
	"sync"
	"sync/atomic"
)

// InMemorySpanRecorder is a simple thread-safe implementation of
// SpanRecorder that stores all reported spans in memory, accessible
// via reporter.GetSpans(). It is primarily intended for testing purposes.
type InMemorySpanRecorder struct {
	sync.RWMutex
	spans []RawSpan
}

// NewInMemoryRecorder creates new InMemorySpanRecorder
func NewInMemoryRecorder() *InMemorySpanRecorder {
	return new(InMemorySpanRecorder)
}

// RecordSpan implements the respective method of SpanRecorder.
func (r *InMemorySpanRecorder) RecordSpan(span RawSpan) {
	r.Lock()
	defer r.Unlock()
	r.spans = append(r.spans, span)
}

// GetSpans returns a copy of the array of spans accumulated so far.
func (r *InMemorySpanRecorder) GetSpans() []RawSpan {
	r.RLock()
	defer r.RUnlock()
	spans := make([]RawSpan, len(r.spans))
	copy(spans, r.spans)
	return spans
}

// GetSampledSpans returns a slice of spans accumulated so far which were sampled.
func (r *InMemorySpanRecorder) GetSampledSpans() []RawSpan {
	r.RLock()
	defer r.RUnlock()
	spans := make([]RawSpan, 0, len(r.spans))
	for _, span := range r.spans {
		if span.Context.Sampled {
			spans = append(spans, span)
		}
	}
	return spans
}

// Reset clears the internal array of spans.
func (r *InMemorySpanRecorder) Reset() {
	r.Lock()
	defer r.Unlock()
	r.spans = nil
}

// CountingRecorder it is primarily intended for testing purposes.
type CountingRecorder int32

// RecordSpan implements the respective method of SpanRecorder.
func (c *CountingRecorder) RecordSpan(r RawSpan) {
	atomic.AddInt32((*int32)(c), 1)
}
