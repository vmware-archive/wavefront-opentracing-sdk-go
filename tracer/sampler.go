package tracer

import (
	"strconv"
	"time"
)

// AlwaysSample basic sampler to sample all Spans
type AlwaysSample struct{}

// ShouldSample allways true
func (t AlwaysSample) ShouldSample(span RawSpan) bool {
	return true
}

// NeverSample basic sampler to not sample any Spans
type NeverSample struct{}

// ShouldSample allways false
func (t NeverSample) ShouldSample(span RawSpan) bool {
	return false
}

// DurationSampler allows spans above a given duration in milliseconds to be reported.
type DurationSampler struct {
	Duration time.Duration
}

// ShouldSample is span duration is bigger than Duration
func (t DurationSampler) ShouldSample(span RawSpan) bool {
	return span.Duration > t.Duration
}

// RateSampler allows spans based on a rate
type RateSampler struct {
	Rate uint64
}

func (t RateSampler) ShouldSample(span RawSpan) bool {
	traceID := span.Context.TraceID[:8]
	id, _ := strconv.ParseUint(traceID, 16, 32)
	return (id % 100) <= t.Rate
}
