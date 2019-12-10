package tracer

import (
	"bytes"
	"errors"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"strconv"
	"strings"
)

const (
	BAGGAGE_PREFIX        = "baggage-"
	TRACE_ID_KEY          = "trace-id"
	PARENT_ID_KEY         = "parent-id"
	SAMPLING_DECISION_KEY = "sampling-decision"
)

type JaegerWavefrontPropagator struct {
	traceIdHeader string
	baggagePrefix string
	tracer        *WavefrontTracer
}

type JaegerOption func(*JaegerWavefrontPropagator)

func WithBaggagePrefix(baggagePrefix string) JaegerOption {
	return func(args *JaegerWavefrontPropagator) {
		args.baggagePrefix = baggagePrefix
	}
}

func WithTraceIdHeader(traceIdHeader string) JaegerOption {
	return func(args *JaegerWavefrontPropagator) {
		args.traceIdHeader = traceIdHeader
	}
}

func NewJaegerWavefrontPropagator(tracer *WavefrontTracer,
	options []JaegerOption) *JaegerWavefrontPropagator {
	j := &JaegerWavefrontPropagator{
		traceIdHeader: TRACE_ID_KEY,
		baggagePrefix: BAGGAGE_PREFIX,
		tracer:        tracer,
	}
	for _, option := range options {
		option(j)
	}
	return j
}

func (p *JaegerWavefrontPropagator) Inject(spanContext opentracing.SpanContext,
	opaqueCarrier interface{}) error {
	carrier, ok := opaqueCarrier.(opentracing.TextMapWriter)
	if !ok {
		return opentracing.ErrInvalidCarrier
	}
	sc, ok := spanContext.(SpanContext)
	if !ok {
		return opentracing.ErrInvalidSpanContext
	}
	carrier.Set(p.traceIdHeader, contextToTraceIdHeader(sc)) // p.traceIdHeader would be canonical
	sc.ForeachBaggageItem(func(k, v string) bool {
		carrier.Set(p.baggagePrefix+k, v)
		return true
	})
	if sc.IsSampled() {
		carrier.Set(SAMPLING_DECISION_KEY, strconv.FormatBool(sc.IsSampled()))
	}
	return nil
}

func (p *JaegerWavefrontPropagator) Extract(opaqueCarrier interface{}) (SpanContext,
	error) {
	carrier, ok := opaqueCarrier.(opentracing.TextMapReader)
	if !ok {
		return SpanContext{}, opentracing.ErrInvalidCarrier
	}

	var spanCtx SpanContext

	err := carrier.ForeachKey(func(k, v string) error {
		lowercaseK := strings.ToLower(k)
		if lowercaseK == strings.ToLower(p.traceIdHeader) {
			err := errors.New("")
			spanCtx, err = contextFromString(v)
			if err != nil {
				return opentracing.ErrSpanContextCorrupted
			}
		} else if strings.HasPrefix(lowercaseK, strings.ToLower(p.baggagePrefix)) {
			spanCtx.WithBaggageItem(strings.TrimPrefix(lowercaseK, p.baggagePrefix), v)
		}
		return nil
	})
	if err != nil {
		return SpanContext{}, err
	}
	if spanCtx.SpanID == "" || spanCtx.TraceID == "" {
		return SpanContext{}, opentracing.ErrSpanContextNotFound
	}
	return spanCtx, nil
}

func contextFromString(value string) (SpanContext, error) {
	var context SpanContext
	if value == "" {
		return context, opentracing.ErrSpanContextNotFound
	}
	parts := contextFromTraceIdHeader(value)
	if parts != nil {
		var err error
		context.TraceID, err = ToUUID(parts[0])
		if err != nil {
			return context, opentracing.ErrSpanContextCorrupted
		}

		context.SpanID, err = ToUUID(parts[1])
		if err != nil {
			return context, opentracing.ErrSpanContextCorrupted
		}

		context = context.WithBaggageItem(PARENT_ID_KEY, context.SpanID)

		sampled, err := strconv.ParseBool(parts[3])
		context.Sampled = &sampled
		if err != nil {
			return context, opentracing.ErrSpanContextCorrupted
		}
	} else {
		return context, opentracing.ErrSpanContextCorrupted
	}
	return context, nil
}

func contextToTraceIdHeader(spanContext SpanContext) string {
	var b bytes.Buffer
	b.WriteString(convertUUID(spanContext.TraceID))
	b.WriteString(":")
	b.WriteString(convertUUID(spanContext.SpanID))
	b.WriteString(":")
	b.WriteString(spanContext.Baggage[PARENT_ID_KEY])
	b.WriteString(":")
	samplingDecision := "0"
	if spanContext.IsSampled() {
		samplingDecision = "1"
	}
	b.WriteString(samplingDecision)
	return b.String()
}

func contextFromTraceIdHeader(value string) []string {
	if value == "" {
		return nil
	}
	header := strings.Split(value, ":")
	if len(header) != 4 || header[0] == "" {
		return nil
	}
	return header
}

func convertUUID(id string) string {
	if id == "" {
		return ""
	}
	str := strings.Join(strings.Split(id, "-"), "")
	start := 0
	for i, ch := range str {
		if ch != '0' {
			start = i
			break
		}
	}
	return str[start:]
}

func ToUUID(id string) (string, error) {
	if len(id) <= 32 {
		uuidString := strings.Repeat("0", 32-len(id)) + id
		resUUID, err := uuid.Parse(uuidString)
		if err != nil {
			return "", err
		}
		return resUUID.String(), nil
	}
	return "", opentracing.ErrSpanContextCorrupted
}
