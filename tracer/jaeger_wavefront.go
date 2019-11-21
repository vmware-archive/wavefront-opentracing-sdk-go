package tracer

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"log"
	"strconv"
	"strings"
)

const (
	BAGGAGE_PREFIX = "baggage-"
	TRACE_ID_KEY   = "trace-id"
	PARENT_ID_KEY         = "parent-id"
	SAMPLING_DECISION_KEY = "sampling-decision"
)

type JaegerWavefrontPropagator struct {
	traceIdHeader string
	baggagePrefix string
	tracer        *WavefrontTracer
}

type Setter func(*JaegerWavefrontPropagator)

func (j *JaegerWavefrontPropagator) WithBaggagePrefix(baggagePrefix string) {
	j.baggagePrefix = baggagePrefix
}

func (j *JaegerWavefrontPropagator) WithTraceIdHeader(traceIdHeader string) {
	j.traceIdHeader = traceIdHeader
}

func NewJaegerWavefrontPropagator(tracer *WavefrontTracer) *JaegerWavefrontPropagator {
	j := &JaegerWavefrontPropagator{
		traceIdHeader: TRACE_ID_KEY,
		baggagePrefix: BAGGAGE_PREFIX,
		tracer: tracer,
	}
	return j
}

func (p *JaegerWavefrontPropagator) Inject(spanContext opentracing.SpanContext, opaqueCarrier interface{}) error {
	carrier, ok := opaqueCarrier.(opentracing.TextMapWriter)
	if !ok {
		return opentracing.ErrInvalidCarrier
	}
	sc, ok := spanContext.(SpanContext)
	if !ok {
		return opentracing.ErrInvalidSpanContext
	}
	log.Println("-------------ContextToTraceIdHeader-------------: ", contextToTraceIdHeader(sc))
	carrier.Set(p.traceIdHeader, contextToTraceIdHeader(sc))
	log.Println("-------------SC baggage-------------: ")
	sc.ForeachBaggageItem(func(k, v string) bool {
		carrier.Set(p.baggagePrefix+k, v)
		log.Println(p.baggagePrefix+k, v)
		return true
	})
	if sc.IsSampled() {
		carrier.Set(SAMPLING_DECISION_KEY, strconv.FormatBool(sc.IsSampled()))
	}
	log.Println("-------------Carrier After Injection-------------: ", carrier)
	return nil
}

func (p *JaegerWavefrontPropagator) Extract(opaqueCarrier interface{}) (opentracing.SpanContext,
	error) {
	carrier, ok := opaqueCarrier.(opentracing.TextMapReader)
	if !ok {
		return nil, opentracing.ErrInvalidCarrier
	}
	result := SpanContext{Baggage: make(map[string]string)}
	var err error
	var parentId string
	log.Println("-------------Extract Carrier-------------: jaeger!!!!!!")
	err = carrier.ForeachKey(func(k, v string) error {
		log.Println("Key Value in Extracted Carrier: ", k, v)
		lowercaseK := strings.ToLower(k)
		if lowercaseK == p.traceIdHeader {
			traceData := p.ContextFromTraceIdHeader(v)
			log.Println("-------------Extract Data: ", traceData)
			if traceData != nil {
				traceId, err := ToUUID(traceData[0])
				log.Println("-------------Extract traceId: ", traceId)
				if err != nil {
					return opentracing.ErrSpanContextCorrupted
				}
				result.TraceID = traceId
				spanID, err := ToUUID(traceData[1])
				log.Println("-------------Extract spanId: ", spanID)
				if err != nil {
					return opentracing.ErrSpanContextCorrupted
				}
				result.SpanID = spanID
				parentId = result.SpanID
				decision, err := strconv.ParseBool(traceData[3])
				log.Println("-------------Extract decision: ", decision)
				if err != nil {
					return opentracing.ErrSpanContextCorrupted
				}
				result.Sampled = &decision
			} else {
				return opentracing.ErrSpanContextCorrupted
			}
		} else if strings.HasPrefix(lowercaseK, strings.ToLower(p.baggagePrefix)) {
			log.Println("-------------Extract other baggage: ", strings.TrimPrefix(lowercaseK,
				p.baggagePrefix), v)
			result.Baggage[strings.TrimPrefix(lowercaseK, p.baggagePrefix)] = v
		}
		return nil
	})
	if err != nil {
		log.Println("here1")
		return nil, err
	}
	if len(result.SpanID) == 0 && len(result.TraceID) == 0 {
		log.Println("here2")
		return nil, opentracing.ErrSpanContextNotFound
	}
	if parentId != "" {
		log.Println("-------------Extract has parentId-------------: ")
		result.Baggage[PARENT_ID_KEY] = parentId
	}
	log.Println("-------------Extract Result-------------: ", result.TraceID, result.SpanID,
		result.IsSampled(),
		result.Baggage)
	return result, nil
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

func (p *JaegerWavefrontPropagator) ContextFromTraceIdHeader(value string) []string {
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
