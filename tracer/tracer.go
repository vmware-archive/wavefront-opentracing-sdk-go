// Package tracer provides an OpenTracing compliant Tracer.
package tracer

import (
	"io"
	"time"

	"github.com/opentracing/opentracing-go"
)

const (
	defaultComponent = "none"
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

// SpanContextPropagator implements SpanContext propagation to/from another processes.
type SpanContextPropagator interface {
	Inject(spanContext opentracing.SpanContext, carrier interface{}) error
	Extract(carrier interface{}) (opentracing.SpanContext, error)
}

// WavefrontTracer implements the OpenTracing `Tracer` interface.
type WavefrontTracer struct {
	textPropagator            SpanContextPropagator
	binaryPropagator          SpanContextPropagator
	accessorPropagator        SpanContextPropagator
	jaegerWavefrontPropagator *JaegerWavefrontPropagator
	zipkinWavefrontPropagator *ZipkinWavefrontPropagator

	earlySamplers []Sampler
	lateSamplers  []Sampler
	reporter      SpanReporter

	generator Generator
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

// WithGenerator configures Tracer to use a custom trace id generator implementation.
func WithGenerator(generator Generator) Option {
	return func(t *WavefrontTracer) {
		t.generator = generator
	}
}

// WithJaegerPropagator configures Tracer to use Jaeger trace context propagation.
func WithJaegerPropagator(traceId, baggagePrefix string) Option {
	return func(args *WavefrontTracer) {
		var options []JaegerOption
		if traceId != "" {
			options = append(options, WithTraceIdHeader(traceId))
		}
		if baggagePrefix != "" {
			options = append(options, WithBaggagePrefix(baggagePrefix))
		}
		args.jaegerWavefrontPropagator = NewJaegerWavefrontPropagator(args, options)
	}
}

// WithZipkinPropagator configures Tracer to use Zipkin trace context propagation.
func WithZipkinPropagator(zipkinOptions ...ZipkinOption) Option {
	return func(args *WavefrontTracer) {
		args.zipkinWavefrontPropagator = NewZipkinWavefrontPropagator(zipkinOptions...)
	}
}

// WithW3CGenerator configures Tracer to generate Trace and Span IDs according to W3C spec.
func WithW3CGenerator() Option {
	return WithGenerator(NewGeneratorW3C())
}

// WithW3CPropagator configures Tracer to use trace context propagation according to W3C spec.
func WithW3CPropagator() Option {
	return func(t *WavefrontTracer) {
		// implies W3C Generator. Custom ID generators should use WithGenerator after this option.
		t.generator = NewGeneratorW3C()
		t.textPropagator = NewPropagatorW3C()
	}
}

// New creates and returns a WavefrontTracer which defers completed Spans to the given `reporter`.
func New(reporter SpanReporter, options ...Option) opentracing.Tracer {
	tracer := &WavefrontTracer{
		reporter:  reporter,
		generator: NewGeneratorUUID(),
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
		sp.raw.Context.SpanID = t.generator.SpanID()
		sp.raw.Context.Sampled = refCtx.Sampled
		sp.raw.ParentSpanID = refCtx.SpanID

	} else {
		// indicates a root span and that no decision has been inherited from a parent span.
		// allocate new trace and span ids and perform sampling.
		sp.raw.Context.TraceID = t.generator.TraceID()
		sp.raw.Context.SpanID = t.generator.SpanID()
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
	if _, ok := format.(JaegerWavefrontPropagator); ok {
		if t.jaegerWavefrontPropagator == nil {
			return opentracing.ErrUnsupportedFormat
		}
		return t.jaegerWavefrontPropagator.Inject(sc, carrier)
	}

	if _, ok := format.(ZipkinWavefrontPropagator); ok {
		if t.zipkinWavefrontPropagator == nil {
			return opentracing.ErrUnsupportedFormat
		}
		return t.zipkinWavefrontPropagator.Inject(sc, carrier)
	}

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
	if _, ok := format.(JaegerWavefrontPropagator); ok {
		if t.jaegerWavefrontPropagator == nil {
			return nil, opentracing.ErrUnsupportedFormat
		}
		return t.jaegerWavefrontPropagator.Extract(carrier)
	}

	if _, ok := format.(ZipkinWavefrontPropagator); ok {
		if t.zipkinWavefrontPropagator == nil {
			return nil, opentracing.ErrUnsupportedFormat
		}
		return t.zipkinWavefrontPropagator.Extract(carrier)
	}

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
