package tracer

// TODO: change package

import (
	"time"

	opentracing "github.com/opentracing/opentracing-go"
)

// Tracer extends the opentracing.Tracer interface with methods to
// probe implementation state, for use by tracer consumers.
type Tracer interface {
	opentracing.Tracer
	Options() Options
}

type SpanRecorder interface {
	RecordSpan(span RawSpan)
}

type Options struct {
	ShouldSample   func(traceID string) bool
	Recorder       SpanRecorder
	EnableSpanPool bool
}

// TODO: review ShouldSample
func DefaultOptions() Options {
	return Options{
		ShouldSample: func(traceID string) bool { return true }, //{ return traceID%64 == 0 },
	}
}

// NewWithOptions creates a customized Tracer.
func NewWithOptions(opts Options) opentracing.Tracer {
	rval := &tracerImpl{options: opts}
	rval.textPropagator = &textMapPropagator{rval}
	rval.binaryPropagator = &binaryPropagator{rval}
	rval.accessorPropagator = &accessorPropagator{rval}
	return rval
}

// New creates and returns a standard Tracer which defers completed Spans to
// `recorder`.
// Spans created by this Tracer support the ext.SamplingPriority tag: Setting
// ext.SamplingPriority causes the Span to be Sampled from that point on.
func New(recorder SpanRecorder) opentracing.Tracer {
	opts := DefaultOptions()
	opts.Recorder = recorder
	return NewWithOptions(opts)
}

// Implements the `Tracer` interface.
type tracerImpl struct {
	options            Options
	textPropagator     *textMapPropagator
	binaryPropagator   *binaryPropagator
	accessorPropagator *accessorPropagator
}

func (t *tracerImpl) StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	sso := opentracing.StartSpanOptions{}
	for _, o := range opts {
		o.Apply(&sso)
	}
	return t.StartSpanWithOptions(operationName, sso)
}

func (t *tracerImpl) getSpan() *spanImpl {
	if t.options.EnableSpanPool {
		sp := spanPool.Get().(*spanImpl)
		sp.reset()
		return sp
	}
	return &spanImpl{}
}

func (t *tracerImpl) StartSpanWithOptions(operationName string, opts opentracing.StartSpanOptions) opentracing.Span {
	// Start time.
	startTime := opts.StartTime
	if startTime.IsZero() {
		startTime = time.Now()
	}

	// Tags.
	tags := opts.Tags

	// Build the new span. This is the only allocation: We'll return this as
	// an opentracing.Span.
	sp := t.getSpan()
	// Look for a parent in the list of References.
	//
	// TODO: would be nice if tracer did something with all
	// References, not just the first one.
ReferencesLoop:
	for _, ref := range opts.References {
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
		sp.raw.Context.Sampled = t.options.ShouldSample(sp.raw.Context.TraceID)
	}

	sp.tracer = t
	sp.raw.Operation = operationName
	sp.raw.Start = startTime
	sp.raw.Duration = -1
	sp.raw.Tags = tags

	return sp
}

type delegatorType struct{}

// Delegator is the format to use for DelegatingCarrier.
var Delegator delegatorType

func (t *tracerImpl) Inject(sc opentracing.SpanContext, format interface{}, carrier interface{}) error {
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

func (t *tracerImpl) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
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

func (t *tracerImpl) Options() Options {
	return t.options
}
