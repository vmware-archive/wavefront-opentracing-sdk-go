package tracer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/opentracing/opentracing-go"
)

const (
	zipkinPrefix = "X-B3-"

	ZipkinTraceIdKey      = zipkinPrefix + "TraceId"
	ZipkinSpanIdKey       = zipkinPrefix + "SpanId"
	ZipkinParentSpanIdKey = zipkinPrefix + "ParentSpanId"
	ZipkinSampledKey      = zipkinPrefix + "Sampled"
	ZipkinFlagsKey        = zipkinPrefix + "Flags"
)

var (
	ErrInvalidZipkinTraceId = fmt.Errorf("%w. Invalid %s found", opentracing.ErrSpanContextCorrupted, ZipkinTraceIdKey)
	ErrInvalidZipkinSpanId  = fmt.Errorf("%w. Invalid %s found", opentracing.ErrSpanContextCorrupted, ZipkinSpanIdKey)
	ErrInvalidZipkinSampled = errors.New(fmt.Sprintf("invalid %s found", ZipkinSampledKey))
	emptySpanCtx            SpanContext
)

type ZipkinWavefrontPropagator struct {
	overrideSampled  bool
	samplingDecision bool // sampling accept(true) or deny(false) regardless the value coming from X-B3-Sampled header
}

type ZipkinOption func(*ZipkinWavefrontPropagator)

// WithOverrideSamplingDecision configures ZipkinWavefrontPropagator to override sampling (X-B3-Sampled) with following samplingDecision:
//
// true: Accept
//
// false: Deny
func WithOverrideSamplingDecision(samplingDecision bool) ZipkinOption {
	return func(args *ZipkinWavefrontPropagator) {
		args.overrideSampled = true
		args.samplingDecision = samplingDecision
	}
}

func NewZipkinWavefrontPropagator(opts ...ZipkinOption) *ZipkinWavefrontPropagator {
	z := &ZipkinWavefrontPropagator{
		overrideSampled: false,
	}

	for _, opt := range opts {
		opt(z)
	}
	return z
}

func (z *ZipkinWavefrontPropagator) Inject(spanContext opentracing.SpanContext, opaqueCarrier interface{}) error {
	carrier, ok := opaqueCarrier.(opentracing.TextMapWriter)
	if !ok {
		return opentracing.ErrInvalidCarrier
	}

	sc, ok := spanContext.(SpanContext)
	if !ok {
		return opentracing.ErrInvalidSpanContext
	}

	if sc.TraceID != "" && sc.SpanID != "" {
		carrier.Set(ZipkinTraceIdKey, convertUUID(sc.TraceID))
		carrier.Set(ZipkinSpanIdKey, convertUUID(sc.SpanID))
	}

	var flags string
	sc.ForeachBaggageItem(func(k, v string) bool {
		key := strings.ToLower(k)
		if key == strings.ToLower(ZipkinParentSpanIdKey) {
			parentSpanId := strings.TrimSpace(v)
			if parentSpanId != "" {
				carrier.Set(ZipkinParentSpanIdKey, convertUUID(parentSpanId))
			}
		} else if key == strings.ToLower(ZipkinFlagsKey) {
			flags = strings.TrimSpace(v)
		} else {
			carrier.Set(k, v)
		}
		return true
	})

	// If sampling decision is not overridden, then set from sc. Otherwise, use overriding decision.
	if !z.overrideSampled {
		// Debug ("1") implies an accept decision, so don't set X-B3-Sampled too
		if flags == "1" {
			carrier.Set(ZipkinFlagsKey, flags)
		} else if sc.IsSampled() {
			carrier.Set(ZipkinSampledKey, convertSampled(*sc.Sampled))
		}
	} else {
		carrier.Set(ZipkinSampledKey, convertSampled(z.samplingDecision))
	}

	return nil
}

func (z *ZipkinWavefrontPropagator) Extract(opaqueCarrier interface{}) (SpanContext, error) {
	carrier, ok := opaqueCarrier.(opentracing.TextMapReader)
	if !ok {
		return emptySpanCtx, opentracing.ErrInvalidCarrier
	}

	var (
		traceId      string
		spanId       string
		parentSpanId string
		sampled      string
		flags        string
	)

	baggage := make(map[string]string)
	_ = carrier.ForeachKey(func(k, v string) error {
		key := strings.ToLower(k)
		val := strings.TrimSpace(v)
		if key == strings.ToLower(ZipkinTraceIdKey) {
			traceId = val
		} else if key == strings.ToLower(ZipkinSpanIdKey) {
			spanId = val
		} else if key == strings.ToLower(ZipkinParentSpanIdKey) {
			parentSpanId = val
		} else if key == strings.ToLower(ZipkinSampledKey) {
			sampled = val
		} else if key == strings.ToLower(ZipkinFlagsKey) {
			flags = val
		} else {
			baggage[k] = v
		}
		return nil
	})

	sc, err := z.contextFromZipkinHeaders(traceId, spanId, parentSpanId, sampled, flags)
	if err != nil {
		return emptySpanCtx, err
	}

	for k, v := range baggage {
		sc = sc.WithBaggageItem(k, v)
	}

	return sc, nil
}

func (z *ZipkinWavefrontPropagator) contextFromZipkinHeaders(traceId string, spanId string, parentSpanId string, sampled string, flags string) (SpanContext, error) {
	var (
		sc  SpanContext
		err error
	)

	if traceId != "" && spanId != "" {
		sc.TraceID, err = ToUUID(traceId)
		if err != nil {
			return emptySpanCtx, ErrInvalidZipkinTraceId
		}

		sc.SpanID, err = ToUUID(spanId)
		if err != nil {
			return emptySpanCtx, ErrInvalidZipkinSpanId
		}
	}

	// Wavefront SpanContext does not have support for ParentSpanId and Flags. Therefore, adding it to baggage.
	if parentSpanId != "" {
		parSpanId, err := ToUUID(parentSpanId)
		if err == nil {
			sc = sc.WithBaggageItem(ZipkinParentSpanIdKey, parSpanId)
		}
	}

	// If sampling decision is not overridden, then use from headers. Otherwise, use from overriding decision.
	if !z.overrideSampled {
		switch strings.ToLower(sampled) {
		case "0":
			sampledVal := false
			sc.Sampled = &sampledVal
		case "1":
			sampledVal := true
			sc.Sampled = &sampledVal
		case "":
			// spanCtx.Sampled = nil
		default:
			return emptySpanCtx, ErrInvalidZipkinSampled
		}

		// Wavefront SpanContext does not have support for Flags. Therefore, adding it to baggage.
		// Debug implies accept decision, so don't send X-B3-Sampled too.
		if flags == "1" {
			sc.Sampled = nil
			sc = sc.WithBaggageItem(ZipkinFlagsKey, flags)
		}
	} else {
		sc.Sampled = &z.samplingDecision
	}

	return sc, nil
}

func convertSampled(sampled bool) string {
	if sampled {
		return "1"
	}
	return "0"
}
