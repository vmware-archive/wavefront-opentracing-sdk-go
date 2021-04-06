package tracer

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/opentracing/opentracing-go"
)

const (
	traceparent = "traceparent"
	tracestate  = "tracestate"
)

// PropagatorW3C implements trace context propagation according to W3C definition https://www.w3.org/TR/trace-context/
type PropagatorW3C struct {
}

// NewPropagatorW3C creates PropagatorW3C instance.
func NewPropagatorW3C() SpanContextPropagator {
	return &PropagatorW3C{}
}

func (p *PropagatorW3C) Inject(spanContext opentracing.SpanContext, opaqueCarrier interface{}) error {
	sc, ok := spanContext.(SpanContext)
	if !ok {
		return opentracing.ErrInvalidSpanContext
	}
	carrier, ok := opaqueCarrier.(opentracing.TextMapWriter)
	if !ok {
		return opentracing.ErrInvalidCarrier
	}

	tp := traceparentString(sc.TraceID, sc.SpanID, sc.Sampled)
	if len(tp) != 55 {
		return opentracing.ErrInvalidSpanContext
	}
	carrier.Set(traceparent, tp)

	if len(sc.Baggage) > 0 {
		states := make([]string, 0, len(sc.Baggage))
		for k, v := range sc.Baggage {
			if strings.ContainsAny(k, "=,") || strings.ContainsAny(v, "=,") {
				return opentracing.ErrInvalidSpanContext
			}
			states = append(states, fmt.Sprintf("%s=%s", k, v))
		}
		carrier.Set(tracestate, strings.Join(states, ","))
	}
	return nil
}

func (p *PropagatorW3C) Extract(opaqueCarrier interface{}) (opentracing.SpanContext, error) {
	carrier, ok := opaqueCarrier.(opentracing.TextMapReader)
	if !ok {
		return nil, opentracing.ErrInvalidCarrier
	}

	sc := SpanContext{Baggage: make(map[string]string)}
	err := carrier.ForeachKey(func(k, v string) error {
		switch strings.ToLower(k) {
		case traceparent:
			return traceparentParse(v, &sc)
		case tracestate:
			return tracestateParse(v, &sc)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(sc.TraceID) == 0 {
		return nil, opentracing.ErrSpanContextNotFound
	}
	return sc, nil
}

func traceparentString(traceID, spanID string, sampled *bool) string {
	flags := "00"
	if sampled != nil && *sampled {
		flags = "01"
	}
	return fmt.Sprintf("00-%s-%s-%s", FromUUID(traceID), FromUUID(spanID), flags)
}

func traceparentParse(s string, sc *SpanContext) (err error) {
	if len(s) != 55 {
		return opentracing.ErrSpanContextCorrupted
	}

	lengths := []int{2, 32, 16, 2}
	parts := strings.Split(s, "-")
	if len(parts) != len(lengths) {
		return opentracing.ErrSpanContextCorrupted
	}
	for i, l := range lengths {
		if len(parts[i]) != l {
			return opentracing.ErrSpanContextCorrupted
		}
		if _, err := hex.DecodeString(parts[i]); err != nil {
			return opentracing.ErrSpanContextCorrupted
		}
	}
	if parts[0] != "00" {
		return opentracing.ErrSpanContextCorrupted
	}

	sc.TraceID, _ = ToUUID(parts[1])
	sc.SpanID, _ = ToUUID(parts[2])
	sampled := parts[3] == "01"
	sc.Sampled = &sampled
	return nil
}

func tracestateParse(s string, sc *SpanContext) error {
	for _, state := range strings.Split(s, ",") {
		parts := strings.Split(state, "=")
		if len(parts) == 2 {
			sc.Baggage[parts[0]] = parts[1]
		}
	}
	return nil
}

func FromUUID(id string) string {
	const zeros = "0000000000000000"
	s := strings.Join(strings.Split(id, "-"), "")
	if strings.HasPrefix(s, zeros) {
		return s[len(zeros):]
	}
	return s
}
