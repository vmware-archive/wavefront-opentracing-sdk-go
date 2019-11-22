package tracer

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
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

func NewJaegerWfPropagator(traceIdHeader string, baggagePrefix string) *JaegerWavefrontPropagator {
	return &JaegerWavefrontPropagator{
		traceIdHeader: traceIdHeader,
		baggagePrefix: baggagePrefix,
		tracer:        nil,
	}
}

func (p *JaegerWavefrontPropagator) Inject(spanContext jaeger.SpanContext,
	opaqueCarrier interface{}) error {
	carrier, ok := opaqueCarrier.(opentracing.TextMapWriter)
	if !ok {
		log.Println("inject break because of ctx type")
		return opentracing.ErrInvalidCarrier
	}
	log.Println("-------------ContextToTraceIdHeader-------------: ", contextToTraceIdHeader(spanContext))
	carrier.Set(p.traceIdHeader, contextToTraceIdHeader(spanContext))
	log.Println("-------------SC baggage-------------: ")
	spanContext.ForeachBaggageItem(func(k, v string) bool {
		carrier.Set(p.baggagePrefix+k, v)
		log.Println(p.baggagePrefix+k, v)
		return true
	})
	if spanContext.IsSampled() {
		carrier.Set(SAMPLING_DECISION_KEY, strconv.FormatBool(spanContext.IsSampled()))
	}
	log.Println("-------------Carrier After Injection-------------: ", carrier)
	return nil
}

func (p *JaegerWavefrontPropagator) Extract(opaqueCarrier interface{}) (jaeger.SpanContext,
	error) {
	carrier, ok := opaqueCarrier.(opentracing.TextMapReader)
	if !ok {
		return jaeger.SpanContext{}, opentracing.ErrInvalidCarrier
	}

	var traceID uint64
	var spanID uint64
	var parentID uint64
	sampled := false
	var baggage map[string]string
	log.Println("-------------Extract Carrier-------------: jaeger!!!!!!")
	log.Println("-------------Extract jaeger traceIdHeader-------------: ", p.traceIdHeader)

	err := carrier.ForeachKey(func(k, v string) error {
		log.Println("Key Value in Extracted Carrier: ", k, v)
		lowercaseK := strings.ToLower(k)
		if lowercaseK == p.traceIdHeader {
			traceData := p.ContextFromTraceIdHeader(v)
			log.Println("-------------Extract Data: ", traceData)
			if traceData != nil {
				traceIdStr, err := ToUUID(traceData[0])
				if err != nil {
					return opentracing.ErrSpanContextCorrupted
				}
				traceID, err = strconv.ParseUint(traceIdStr, 16, 64)
				log.Println("-------------Extract traceId: ", traceID)

				spanIdStr, err := ToUUID(traceData[1])
				if err != nil {
					return opentracing.ErrSpanContextCorrupted
				}
				spanID, err = strconv.ParseUint(spanIdStr, 16, 64)
				log.Println("-------------Extract spanId: ", spanID)

				parentID =spanID

				decision, err := strconv.ParseBool(traceData[3])
				if err != nil {
					return opentracing.ErrSpanContextCorrupted
				}
				log.Println("-------------Extract decision: ", decision)
			} else {
				return opentracing.ErrSpanContextCorrupted
			}
		} else if strings.HasPrefix(lowercaseK, strings.ToLower(p.baggagePrefix)) {
			log.Println("-------------Extract other baggage: ", strings.TrimPrefix(lowercaseK,
				p.baggagePrefix), v)
			baggage[strings.TrimPrefix(lowercaseK, p.baggagePrefix)] = v
		}
		return nil
	})
	if err != nil {
		log.Println("here1")
		return jaeger.SpanContext{}, err
	}
	if traceID == 0 && spanID == 0 {
		log.Println("here2")
		return jaeger.SpanContext{}, opentracing.ErrSpanContextNotFound
	}
	log.Println("-------------Extract Result-------------: ", traceID, spanID,
		sampled, baggage)
	return jaeger.NewSpanContext(
		jaeger.TraceID{Low: traceID},
		jaeger.SpanID(spanID),
		jaeger.SpanID(parentID),
		sampled, baggage), nil
}

func contextToTraceIdHeader(spanContext jaeger.SpanContext) string {
	var b bytes.Buffer
	b.WriteString(convertUUID(spanContext.TraceID().String()))
	b.WriteString(":")
	b.WriteString(convertUUID(spanContext.SpanID().String()))
	b.WriteString(":")
	b.WriteString(spanContext.ParentID().String())
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
