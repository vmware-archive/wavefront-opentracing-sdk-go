package tracer

import (
	"sync"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
)

// Implements the `Span` interface. Created via tracerImpl (see
// `wavefront.New()`).
type spanImpl struct {
	tracer     *WavefrontTracer
	sync.Mutex // protects the fields below
	raw        RawSpan
}

// RawSpan holds the span information
type RawSpan struct {
	// Those recording the RawSpan should also record the contents of its
	// SpanContext.
	Context SpanContext

	// The SpanID of this SpanContext's first intra-trace reference (i.e.,
	// "parent"), or 0 if there is no parent.
	ParentSpanID string

	References []opentracing.SpanReference

	// The name of the "operation" this span is an instance of. (Called a "span
	// name" in some implementations)
	Operation string

	// We store <start, duration> rather than <start, end> so that only
	// one of the timestamps has global clock uncertainty issues.
	Start    time.Time
	Duration time.Duration

	// Essentially an extension mechanism. Can be used for many purposes,
	// not to be enumerated here.
	Tags opentracing.Tags

	// The span's "microlog".
	Logs []opentracing.LogRecord
}

func (s *spanImpl) reset() {
	s.tracer = nil
	s.raw = RawSpan{
		Context: SpanContext{},
	}
}

func (s *spanImpl) SetOperationName(operationName string) opentracing.Span {
	s.Lock()
	defer s.Unlock()
	s.raw.Operation = operationName
	return s
}

func (s *spanImpl) SetTag(key string, value interface{}) opentracing.Span {
	s.Lock()
	defer s.Unlock()
	if key == string(ext.SamplingPriority) {
		if v, ok := value.(uint16); ok {
			s.raw.Context.Sampled = v != 0
			return s
		}
	}

	if s.raw.Tags == nil {
		s.raw.Tags = opentracing.Tags{}
	}
	s.raw.Tags[key] = value
	return s
}

func (s *spanImpl) LogKV(keyValues ...interface{}) {
}

func (s *spanImpl) LogFields(fields ...log.Field) {
}

func (s *spanImpl) LogEvent(event string) {
}

func (s *spanImpl) LogEventWithPayload(event string, payload interface{}) {
}

func (s *spanImpl) Log(ld opentracing.LogData) {
}

func (s *spanImpl) Finish() {
	s.FinishWithOptions(opentracing.FinishOptions{})
}

func (s *spanImpl) FinishWithOptions(opts opentracing.FinishOptions) {
	finishTime := opts.FinishTime
	if finishTime.IsZero() {
		finishTime = time.Now()
	}
	duration := finishTime.Sub(s.raw.Start)

	s.Lock()
	defer s.Unlock()

	s.raw.Duration = duration

	if s.raw.Context.Sampled {
		if len(s.tracer.lateSamplers) > 0 {
			s.raw.Context.Sampled = false
			for _, sampler := range s.tracer.lateSamplers {
				if !sampler.IsEarly() {
					if !s.raw.Context.Sampled {
						s.raw.Context.Sampled = sampler.ShouldSample(s.raw)
					}
				}
			}
		}
	}

	if !s.raw.Context.Sampled {
		s.raw.Context.Sampled = (s.raw.Tags["error"] == true)
	}

	s.tracer.reporter.ReportSpan(s.raw)
}

func (s *spanImpl) Tracer() opentracing.Tracer {
	return s.tracer
}

func (s *spanImpl) Context() opentracing.SpanContext {
	return s.raw.Context
}

func (s *spanImpl) SetBaggageItem(key, val string) opentracing.Span {
	s.Lock()
	defer s.Unlock()
	s.raw.Context = s.raw.Context.WithBaggageItem(key, val)
	return s
}

func (s *spanImpl) BaggageItem(key string) string {
	s.Lock()
	defer s.Unlock()
	return s.raw.Context.Baggage[key]
}
