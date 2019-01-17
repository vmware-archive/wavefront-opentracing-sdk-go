package tracer

import (
	"time"

	opentracing "github.com/opentracing/opentracing-go"
)

// SpanReporter record completed Spans
type SpanReporter interface {
	ReportSpan(span RawSpan)
}

// Sampler control if a span shold be sampled
type Sampler interface {
	ShouldSample(span RawSpan) bool
}

// WavefrontTracer implements the `Tracer` interface.
type WavefrontTracer struct {
	textPropagator     *textMapPropagator
	binaryPropagator   *binaryPropagator
	accessorPropagator *accessorPropagator

	sampler        Sampler
	reporter       SpanReporter
	enableSpanPool bool
}

// Option allow WavefrontTracer customization
type Option func(*WavefrontTracer)

// WithSampler define a Sampler
func WithSampler(sampler Sampler) Option {
	return func(args *WavefrontTracer) {
		args.sampler = sampler
	}
}

// DisableSpanPool disable the span pool
func DisableSpanPool() Option {
	return func(args *WavefrontTracer) {
		args.enableSpanPool = false
	}
}

// New creates and returns a WavefrontTracer which defers completed Spans to
// `reporter`.
func New(reporter SpanReporter, options ...Option) opentracing.Tracer {
	tracer := &WavefrontTracer{
		reporter:       reporter,
		enableSpanPool: false,
		sampler:        &AllwaysSample{},
	}

	tracer.textPropagator = &textMapPropagator{tracer}
	tracer.binaryPropagator = &binaryPropagator{tracer}
	tracer.accessorPropagator = &accessorPropagator{tracer}

	for _, option := range options {
		option(tracer)
	}
	return tracer
}

func (t *WavefrontTracer) StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	options := opentracing.StartSpanOptions{}
	for _, o := range opts {
		o.Apply(&options)
	}

	// Start time.
	startTime := options.StartTime
	if startTime.IsZero() {
		startTime = time.Now()
	}

	// Tags.
	tags := options.Tags

	// Build the new span. This is the only allocation: We'll return this as
	// an opentracing.Span.
	sp := t.getSpan()
	// Look for a parent in the list of References.
	//
	// TODO: would be nice if tracer did something with all
	// References, not just the first one.
ReferencesLoop:
	for _, ref := range options.References {
		switch ref.Type {
		case opentracing.ChildOfRef,
			opentracing.FollowsFromRef:

			refCtx := ref.ReferencedContext.(SpanContext)
			sp.raw.Context.TraceID = refCtx.TraceID
			sp.raw.Context.SpanID = randomID()
			sp.raw.Context.Sampled = refCtx.Sampled
			sp.raw.ParentSpanID = refCtx.SpanID

			if l := len(refCtx.Baggage); l > 0 {
				sp.raw.Context.Baggage = make(map[string]string, l)
				for k, v := range refCtx.Baggage {
					sp.raw.Context.Baggage[k] = v
				}
			}
			break ReferencesLoop
		}
	}
	if len(sp.raw.Context.TraceID) == 0 {
		// No parent Span found; allocate new trace and span ids and determine
		// the Sampled status.
		sp.raw.Context.TraceID, sp.raw.Context.SpanID = randomID2()
		sp.raw.Context.Sampled = t.sampler.ShouldSample(sp.raw)
	}

	sp.tracer = t
	sp.raw.Operation = operationName
	sp.raw.Start = startTime
	sp.raw.Duration = -1
	sp.raw.Tags = tags

	return sp
}

func (t *WavefrontTracer) getSpan() *spanImpl {
	if t.enableSpanPool {
		sp := spanPool.Get().(*spanImpl)
		sp.reset()
		return sp
	}
	return &spanImpl{}
}

type delegatorType struct{}

// Delegator is the format to use for DelegatingCarrier.
var Delegator delegatorType

func (t *WavefrontTracer) Inject(sc opentracing.SpanContext, format interface{}, carrier interface{}) error {
	switch format {
	case opentracing.TextMap, opentracing.HTTPHeaders:
		return t.textPropagator.Inject(sc, carrier)
	case opentracing.Binary:
		return t.binaryPropagator.Inject(sc, carrier)
	}
	if _, ok := format.(delegatorType); ok {
		return t.accessorPropagator.Inject(sc, carrier)
	}
	return opentracing.ErrUnsupportedFormat
}

func (t *WavefrontTracer) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
	switch format {
	case opentracing.TextMap, opentracing.HTTPHeaders:
		return t.textPropagator.Extract(carrier)
	case opentracing.Binary:
		return t.binaryPropagator.Extract(carrier)
	}
	if _, ok := format.(delegatorType); ok {
		return t.accessorPropagator.Extract(carrier)
	}
	return nil, opentracing.ErrUnsupportedFormat
}
