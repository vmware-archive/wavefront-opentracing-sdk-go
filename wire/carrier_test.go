package wire_test

import (
	"testing"

	"github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"
	"github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer/wire"
)

func TestProtobufCarrier(t *testing.T) {
	var carrier tracer.DelegatingCarrier = &wire.ProtobufCarrier{}

	var traceID, spanID string = "1", "2"
	sampled := true
	baggageKey, expVal := "key1", "val1"

	carrier.SetState(traceID, spanID, sampled)
	carrier.SetBaggageItem(baggageKey, expVal)
	gotTraceID, gotSpanID, gotSampled := carrier.State()
	if traceID != gotTraceID || spanID != gotSpanID || sampled != gotSampled {
		t.Errorf("Wanted state %s %s %t, got %s %s %t", spanID, traceID, sampled,
			gotTraceID, gotSpanID, gotSampled)
	}

	gotBaggage := map[string]string{}
	f := func(k, v string) {
		gotBaggage[k] = v
	}

	carrier.GetBaggage(f)
	value, ok := gotBaggage[baggageKey]
	if !ok {
		t.Errorf("Expected baggage item %s to exist", baggageKey)
	}
	if value != expVal {
		t.Errorf("Expected key %s to be %s, got %s", baggageKey, expVal, value)
	}
}
