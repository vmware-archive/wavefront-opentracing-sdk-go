package tracer

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"strconv"
	"strings"
)

const (
	baggagePrefix       = "baggage-"
	parentIdKey         = "parent-id"
	samplingDecisionKey = "sampling-decision"
)

type jaegerWavefrontPropagator struct {
	traceIdHeader string
	baggagePrefix string
}

func (p *jaegerWavefrontPropagator) Inject(spanContext opentracing.SpanContext, opaqueCarrier interface{}) error {
	sc, ok := spanContext.(SpanContext)
	if !ok {
		return opentracing.ErrInvalidSpanContext
	}
	carrier, ok := opaqueCarrier.(opentracing.TextMapWriter)
	if !ok {
		return opentracing.ErrInvalidCarrier
	}
	carrier.Set(p.traceIdHeader, p.ContextToTraceIdHeader(sc))
	for k, v := range sc.Baggage {
		carrier.Set(baggagePrefix+k, v)
	}
	if sc.IsSampled() {
		carrier.Set(samplingDecisionKey, strconv.FormatBool(*sc.SamplingDecision()))
	}
	return nil
}

func (p *jaegerWavefrontPropagator) Extract(opaqueCarrier interface{}) (opentracing.SpanContext,
	error) {
	carrier, ok := opaqueCarrier.(opentracing.TextMapReader)
	if !ok {
		return nil, opentracing.ErrInvalidCarrier
	}
	result := SpanContext{Baggage: make(map[string]string)}
	var err error
	var parentId string
	err = carrier.ForeachKey(func(k, v string) error {
		lowercaseK := strings.ToLower(k)
		if lowercaseK == p.traceIdHeader {
			traceData := p.ContextFromTraceIdHeader(v)
			if traceData != nil {
				traceId, err := ToUUID(traceData[0])
				if err != nil {
					return opentracing.ErrSpanContextCorrupted
				}
				result.TraceID = traceId
				spanID, err := ToUUID(traceData[1])
				if err != nil {
					return opentracing.ErrSpanContextCorrupted
				}
				result.SpanID = spanID
				parentId = result.SpanID
				decision, err := strconv.ParseBool(traceData[3])
				if err != nil {
					return opentracing.ErrSpanContextCorrupted
				}
				result.Sampled = &decision
			} else {
				return opentracing.ErrSpanContextCorrupted
			}
		} else if strings.HasPrefix(lowercaseK, strings.ToLower(baggagePrefix)) {
			result.Baggage[strings.TrimPrefix(lowercaseK, baggagePrefix)] = v
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(result.SpanID) == 0 && len(result.TraceID) == 0 {
		return nil, opentracing.ErrSpanContextNotFound
	}
	if parentId != "" {
		result.Baggage[parentIdKey] = parentId
	}
	return result, nil
}

func (p *jaegerWavefrontPropagator) ContextToTraceIdHeader(spanContext SpanContext) string {
	var b bytes.Buffer
	b.WriteString(ConvertUUID(spanContext.TraceID))
	b.WriteString(":")
	b.WriteString(ConvertUUID(spanContext.SpanID))
	b.WriteString(":")
	b.WriteString(spanContext.Baggage[parentIdKey])
	b.WriteString(":")
	samplingDecision := "0"
	if spanContext.IsSampled() {
		samplingDecision = "1"
	}
	b.WriteString(samplingDecision)
	return b.String()
}

func (p *jaegerWavefrontPropagator) ContextFromTraceIdHeader(value string) []string {
	if value == "" {
		return nil
	}
	header := strings.Split(value, ":")
	if len(header) != 4 || header[0] == "" {
		return nil
	}
	return header
}

func ConvertUUID(id string) string {
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
