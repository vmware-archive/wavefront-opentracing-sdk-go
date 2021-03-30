package tracer_test

import (
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"

	"github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"
)

type TestContext struct {
}

func (t *TestContext) ForeachBaggageItem(_ func(k, v string) bool) {
}

func TestW3CPropagator_Inject(t *testing.T) {
	p := tracer.NewW3CPropagator()

	// unknown context
	assert.Error(t, p.Inject(&TestContext{}, nil))

	// invalid carrier
	assert.Error(t, p.Inject(tracer.SpanContext{}, nil))
	assert.Error(t, p.Inject(tracer.SpanContext{}, ""))

	// invalid IDs
	sc := tracer.SpanContext{TraceID: "tid", SpanID: "sid"}
	c := opentracing.TextMapCarrier{}
	assert.Error(t, p.Inject(sc, c))
	sc = tracer.SpanContext{TraceID: "8104fb39-455c-c5d4-831a-95b54b8c9af9", SpanID: "8104fb39-455c-c5d4-831a-95b54b8c9af9"}
	assert.Error(t, p.Inject(sc, c))

	// valid IDs
	sampled := true
	sc = tracer.SpanContext{TraceID: "8104fb39-455c-c5d4-831a-95b54b8c9af9", SpanID: "00000000-0000-0000-831a-95b54b8c9af9", Sampled: &sampled}
	assert.NoError(t, p.Inject(sc, c))
	assert.Equal(t, "00-8104fb39455cc5d4831a95b54b8c9af9-831a95b54b8c9af9-01", c["traceparent"])

	// invalid baggage
	sc.Baggage = map[string]string{",": "v"}
	assert.Error(t, p.Inject(sc, c))
	sc.Baggage = map[string]string{"k": "="}
	assert.Error(t, p.Inject(sc, c))

	// valid baggage
	sc.Baggage = map[string]string{"k1": "v1", "k2": "v2"}
	assert.NoError(t, p.Inject(sc, c))
	assert.True(t, c["tracestate"] == "k1=v1,k2=v2" || c["tracestate"] == "k2=v2,k1=v1")
}

func TestW3CPropagator_Extract(t *testing.T) {
	p := tracer.NewW3CPropagator()

	// invalid carrier
	sc, err := p.Extract(nil)
	assert.Error(t, err)

	// empty carrier
	c := opentracing.TextMapCarrier{}
	_, err = p.Extract(c)
	assert.Error(t, err)

	// missing traceparent
	c["foo"] = "bar"
	_, err = p.Extract(c)
	assert.Error(t, err)

	// invalid traceparent
	c["traceparent"] = "invalid"
	_, err = p.Extract(c)
	assert.Error(t, err)
	c["traceparent"] = "12*12345678901234567890123456789012-1234567890123456-12"
	_, err = p.Extract(c)
	assert.Error(t, err)
	c["traceparent"] = "121-2345678901234567890123456789012-1234567890123456-12"
	_, err = p.Extract(c)
	assert.Error(t, err)
	c["traceparent"] = "zz-12345678901234567890123456789012-1234567890123456-12"
	_, err = p.Extract(c)
	assert.Error(t, err)
	c["traceparent"] = "12-12345678901234567890123456789012-1234567890123456-12"
	_, err = p.Extract(c)
	assert.Error(t, err)

	// valid traceparent
	c["traceparent"] = "00-12345678901234567890123456789012-1234567890123456-01"
	sc, err = p.Extract(c)
	assert.NoError(t, err)
	assert.Equal(t, "12345678-9012-3456-7890-123456789012", sc.(tracer.SpanContext).TraceID)
	assert.Equal(t, "00000000-0000-0000-1234-567890123456", sc.(tracer.SpanContext).SpanID)
	assert.True(t, *sc.(tracer.SpanContext).Sampled)

	// empty tracestate
	c["tracestate"] = ""
	sc, err = p.Extract(c)
	assert.NoError(t, err)
	assert.Empty(t, sc.(tracer.SpanContext).Baggage)

	// invalid tracestate
	c["tracestate"] = "k,v"
	sc, err = p.Extract(c)
	assert.NoError(t, err)
	assert.Empty(t, sc.(tracer.SpanContext).Baggage)

	// valid tracestate
	c["tracestate"] = "k1=v1,k2=v2"
	sc, err = p.Extract(c)
	assert.NoError(t, err)
	assert.Len(t, sc.(tracer.SpanContext).Baggage, 2)
	assert.Equal(t, "v1", sc.(tracer.SpanContext).Baggage["k1"])
	assert.Equal(t, "v2", sc.(tracer.SpanContext).Baggage["k2"])
}
