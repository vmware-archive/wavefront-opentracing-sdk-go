// Package tracer provides an OpenTracing compliant Tracer.
package tracer

import (
	"io"
	"log"
	"time"

	"github.com/opentracing/opentracing-go"
)

// SpanReporter reports completed Spans
type SpanReporter interface {
	io.Closer
	ReportSpan(span RawSpan)
}

// Sampler controls whether a span should be sampled/reported
type Sampler interface {
	ShouldSample(span RawSpan) bool
	IsEarly() bool
}

// WavefrontTracer implements the OpenTracing `Tracer` interface.
type WavefrontTracer struct {
	textPropagator            *textMapPropagator
	binaryPropagator          *binaryPropagator
	accessorPropagator        *accessorPropagator
	jaegerWavefrontPropagator *JaegerWavefrontPropagator

	earlySamplers []Sampler
	lateSamplers  []Sampler
	reporter      SpanReporter
}

// Option allows customizing the WavefrontTracer.
type Option func(*WavefrontTracer)

// WithSampler defines a Sampler
func WithSampler(sampler Sampler) Option {
	return func(args *WavefrontTracer) {
		if sampler.IsEarly() {
			args.earlySamplers = append(args.earlySamplers, sampler)
		} else {
			args.lateSamplers = append(args.lateSamplers, sampler)
		}
	}
}

// New creates and returns a WavefrontTracer which defers completed Spans to the given `reporter`.
func New(reporter SpanReporter, options ...Option) opentracing.Tracer {
	tracer := &WavefrontTracer{
		reporter: reporter,
	}

	tracer.textPropagator = &textMapPropagator{tracer}
	tracer.binaryPropagator = &binaryPropagator{tracer}
	tracer.accessorPropagator = &accessorPropagator{tracer}
	tracer.jaegerWavefrontPropagator = NewJaegerWavefrontPropagator(WithTracer(tracer))

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

	// Build the new span. This is the only allocation: We'll return this as an opentracing.Span.
	sp := t.getSpan()

	// Look for a parent in the list of References.
	var firstChildOfRef SpanContext
	var firstFollowsFromRef SpanContext
	var refCtx SpanContext
	l := len(options.References)
	sp.raw.Context.Baggage = make(map[string]string, l)

	for _, ref := range options.References {
		for k, v := range ref.ReferencedContext.(SpanContext).Baggage {
			sp.raw.Context.Baggage[k] = v
		}
		switch ref.Type {
		case opentracing.ChildOfRef:
			if len(firstChildOfRef.TraceID) == 0 {
				firstChildOfRef = ref.ReferencedContext.(SpanContext)
			}
		case opentracing.FollowsFromRef:
			if len(firstChildOfRef.TraceID) == 0 {
				firstFollowsFromRef = ref.ReferencedContext.(SpanContext)
			}
		}
	}

	if len(firstChildOfRef.TraceID) != 0 {
		refCtx = firstChildOfRef
	} else {
		refCtx = firstFollowsFromRef
	}

	if len(refCtx.TraceID) != 0 {
		sp.raw.Context.TraceID = refCtx.TraceID
		sp.raw.Context.SpanID = randomID()
		sp.raw.Context.Sampled = refCtx.Sampled
		sp.raw.ParentSpanID = refCtx.SpanID

	} else {
		// indicates a root span and that no decision has been inherited from a parent span.
		// allocate new trace and span ids and perform sampling.
		sp.raw.Context.TraceID, sp.raw.Context.SpanID = randomID2()
		decision := t.earlySample(sp.raw)
		sp.raw.Context.Sampled = &decision
	}

	sp.tracer = t
	sp.raw.Operation = operationName
	sp.raw.Start = startTime
	sp.raw.Duration = -1
	sp.raw.References = options.References
	sp.raw.Component = defaultComponent

	for k, v := range tags {
		sp.SetTag(k, v)
	}
	return sp
}

func (t *WavefrontTracer) earlySample(raw RawSpan) bool {
	if len(t.earlySamplers) == 0 && len(t.lateSamplers) == 0 {
		return true
	}
	for _, sampler := range t.earlySamplers {
		if sampler.ShouldSample(raw) {
			return true
		}
	}
	return false
}

func (t *WavefrontTracer) lateSample(raw RawSpan) bool {
	for _, sampler := range t.lateSamplers {
		if sampler.ShouldSample(raw) {
			return true
		}
	}
	return false
}

func (t *WavefrontTracer) getSpan() *spanImpl {
	return &spanImpl{}
}

type delegatorType struct{}

// Delegator is the format to use for DelegatingCarrier.
var Delegator delegatorType

func (t *WavefrontTracer) Inject(sc opentracing.SpanContext, format interface{}, carrier interface{}) error {
	switch format {
	case opentracing.TextMap, opentracing.HTTPHeaders:
		log.Println("----------------Inject Format---------------: ", 1)
		return t.textPropagator.Inject(sc, carrier)
	case opentracing.Binary:
		log.Println("----------------Inject Format---------------: ", 2)
		return t.binaryPropagator.Inject(sc, carrier)
	}
	if _, ok := format.(delegatorType); ok {
		log.Println("----------------Inject Format---------------: ", 3)
		return t.accessorPropagator.Inject(sc, carrier)
	}
	//if _, ok := format.(JaegerWavefrontPropagator); ok {
	//	log.Println("----------------Inject Format---------------: JAEGER!")
	//	return t.jaegerWavefrontPropagator.Inject(sc, carrier)
	//}
	return opentracing.ErrUnsupportedFormat
}

func (t *WavefrontTracer) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
	switch format {
	case opentracing.TextMap, opentracing.HTTPHeaders:
		log.Println("-------------Extract Format-----------", 1)
		return t.textPropagator.Extract(carrier)
	case opentracing.Binary:
		log.Println("-------------Extract Format-----------", 2)
		return t.binaryPropagator.Extract(carrier)
	}
	if _, ok := format.(delegatorType); ok {
		log.Println("-------------Extract Format-----------", 3)
		return t.accessorPropagator.Extract(carrier)
	}
	if _, ok := format.(JaegerWavefrontPropagator); ok {
		log.Println("-------------Extract Format-----------: JAEGER!")
		return t.jaegerWavefrontPropagator.Extract(carrier)
	}
	return nil, opentracing.ErrUnsupportedFormat
}
