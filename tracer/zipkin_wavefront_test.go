package tracer

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
)

const (
	longTraceId     = "463ac35c9f6413ad48485a3953bb6124"
	longTraceIdUuid = "463ac35c-9f64-13ad-4848-5a3953bb6124"

	shortTraceId     = "e6705969875dd888"
	shortTraceIdUuid = "00000000-0000-0000-e670-5969875dd888"

	spanId     = "a2fb4a1d1a96d312"
	spanIdUuid = "00000000-0000-0000-a2fb-4a1d1a96d312"

	parentSpanId     = "a5e3ac9a4f6e3b90"
	parentSpanIdUuid = "00000000-0000-0000-a5e3-ac9a4f6e3b90"

	baggageKey1   = "Test-Key-1"
	baggageValue1 = "test-value-1"
	baggageKey2   = "Test-Key-2"
	baggageValue2 = "test-value-2"
)

func TestZipkinWavefrontPropagator_Extract(t *testing.T) {
	t.Run("with_valid_headers", func(t *testing.T) {
		tracer := New(NewInMemoryReporter(), WithZipkinPropagator())
		carrier := opentracing.HTTPHeadersCarrier(http.Header{})
		carrier.Set(ZipkinTraceIdKey, longTraceId)
		carrier.Set(ZipkinSpanIdKey, spanId)
		carrier.Set(ZipkinParentSpanIdKey, parentSpanId)
		carrier.Set(ZipkinSampledKey, "1")
		carrier.Set(baggageKey1, baggageValue1)
		carrier.Set(baggageKey2, baggageValue2)

		ctx, err := tracer.Extract(ZipkinWavefrontPropagator{}, carrier)
		assert.Nil(t, err)
		require.IsType(t, SpanContext{}, ctx)

		spanCtx := ctx.(SpanContext)
		assert.Equal(t, longTraceIdUuid, spanCtx.TraceID)
		assert.Equal(t, spanIdUuid, spanCtx.SpanID)
		assert.Equal(t, parentSpanIdUuid, spanCtx.Baggage[ZipkinParentSpanIdKey])
		assert.True(t, spanCtx.IsSampled())
		assert.True(t, *spanCtx.SamplingDecision())
		assert.Equal(t, baggageValue1, spanCtx.Baggage[baggageKey1])
		assert.Equal(t, baggageValue2, spanCtx.Baggage[baggageKey2])
	})

	t.Run("without_parent_span", func(t *testing.T) {
		tracer := New(NewInMemoryReporter(), WithZipkinPropagator())
		carrier := opentracing.HTTPHeadersCarrier(http.Header{})
		carrier.Set(ZipkinTraceIdKey, shortTraceId)
		carrier.Set(ZipkinSpanIdKey, spanId)
		carrier.Set(ZipkinSampledKey, "1")
		carrier.Set(baggageKey1, baggageValue1)

		ctx, err := tracer.Extract(ZipkinWavefrontPropagator{}, carrier)
		assert.Nil(t, err)
		require.IsType(t, SpanContext{}, ctx)

		spanCtx := ctx.(SpanContext)
		assert.Equal(t, shortTraceIdUuid, spanCtx.TraceID)
		assert.Equal(t, spanIdUuid, spanCtx.SpanID)
		assert.Empty(t, spanCtx.Baggage[ZipkinParentSpanIdKey])
		assert.True(t, spanCtx.IsSampled())
		assert.True(t, *spanCtx.SamplingDecision())
		assert.Equal(t, baggageValue1, spanCtx.Baggage[baggageKey1])
		assert.Empty(t, spanCtx.Baggage[baggageKey2])
	})

	t.Run("without_trace_id", func(t *testing.T) {
		tracer := New(NewInMemoryReporter(), WithZipkinPropagator())
		carrier := opentracing.HTTPHeadersCarrier(http.Header{})
		carrier.Set(ZipkinSampledKey, "1")
		carrier.Set(baggageKey1, baggageValue1)

		ctx, err := tracer.Extract(ZipkinWavefrontPropagator{}, carrier)
		assert.Nil(t, err)
		require.IsType(t, SpanContext{}, ctx)

		spanCtx := ctx.(SpanContext)
		assert.True(t, spanCtx.IsSampled())
		assert.True(t, *spanCtx.SamplingDecision())
	})

	t.Run("with_flags_sampled_deny", func(t *testing.T) {
		tracer := New(NewInMemoryReporter(), WithZipkinPropagator())
		carrier := opentracing.HTTPHeadersCarrier(http.Header{})
		carrier.Set(ZipkinTraceIdKey, longTraceId)
		carrier.Set(ZipkinSpanIdKey, spanId)
		carrier.Set(ZipkinSampledKey, "0") // This should be ignored when Flags set to "1"
		carrier.Set(ZipkinFlagsKey, "1")

		ctx, err := tracer.Extract(ZipkinWavefrontPropagator{}, carrier)
		assert.Nil(t, err)
		require.IsType(t, SpanContext{}, ctx)

		spanCtx := ctx.(SpanContext)
		assert.Equal(t, longTraceIdUuid, spanCtx.TraceID)
		assert.Equal(t, spanIdUuid, spanCtx.SpanID)
		assert.False(t, spanCtx.IsSampled())

		flags, ok := fetchBaggageItem(spanCtx, ZipkinFlagsKey)
		assert.True(t, ok)
		assert.Equal(t, "1", flags)
	})

	t.Run("with_override_sampling_deny", func(t *testing.T) {
		tracer := New(NewInMemoryReporter(), WithZipkinPropagator(WithOverrideSamplingDecision(false)))
		carrier := opentracing.HTTPHeadersCarrier(http.Header{})
		carrier.Set(ZipkinTraceIdKey, longTraceId)
		carrier.Set(ZipkinSpanIdKey, spanId)
		carrier.Set(ZipkinSampledKey, "1") // This should be ignored with override sampling

		ctx, err := tracer.Extract(ZipkinWavefrontPropagator{}, carrier)
		assert.Nil(t, err)
		require.IsType(t, SpanContext{}, ctx)

		spanCtx := ctx.(SpanContext)
		assert.Equal(t, longTraceIdUuid, spanCtx.TraceID)
		assert.Equal(t, spanIdUuid, spanCtx.SpanID)
		assert.True(t, spanCtx.IsSampled())
		assert.False(t, *spanCtx.SamplingDecision())
	})

	t.Run("with_override_sampling_accept", func(t *testing.T) {
		tracer := New(NewInMemoryReporter(), WithZipkinPropagator(WithOverrideSamplingDecision(true)))
		carrier := opentracing.HTTPHeadersCarrier(http.Header{})
		carrier.Set(ZipkinTraceIdKey, longTraceId)
		carrier.Set(ZipkinSpanIdKey, spanId)
		carrier.Set(ZipkinSampledKey, "0") // This should be ignored with override sampling

		ctx, err := tracer.Extract(ZipkinWavefrontPropagator{}, carrier)
		assert.Nil(t, err)
		require.IsType(t, SpanContext{}, ctx)

		spanCtx := ctx.(SpanContext)
		assert.Equal(t, longTraceIdUuid, spanCtx.TraceID)
		assert.Equal(t, spanIdUuid, spanCtx.SpanID)
		assert.True(t, spanCtx.IsSampled())
		assert.True(t, *spanCtx.SamplingDecision())
	})

	t.Run("with_override_sampling_flags", func(t *testing.T) {
		tracer := New(NewInMemoryReporter(), WithZipkinPropagator(WithOverrideSamplingDecision(false)))
		carrier := opentracing.HTTPHeadersCarrier(http.Header{})
		carrier.Set(ZipkinTraceIdKey, longTraceId)
		carrier.Set(ZipkinSpanIdKey, spanId)
		carrier.Set(ZipkinFlagsKey, "1") // This should be ignored with override sampling

		ctx, err := tracer.Extract(ZipkinWavefrontPropagator{}, carrier)
		assert.Nil(t, err)
		require.IsType(t, SpanContext{}, ctx)

		spanCtx := ctx.(SpanContext)
		assert.Equal(t, longTraceIdUuid, spanCtx.TraceID)
		assert.Equal(t, spanIdUuid, spanCtx.SpanID)
		assert.True(t, spanCtx.IsSampled())
		assert.False(t, *spanCtx.SamplingDecision())

		_, ok := fetchBaggageItem(spanCtx, ZipkinFlagsKey)
		assert.False(t, ok)
	})
}

func TestZipkinWavefrontPropagator_Inject(t *testing.T) {
	t.Run("with_valid_context", func(t *testing.T) {
		carrier := opentracing.HTTPHeadersCarrier(http.Header{})
		tracer := New(NewInMemoryReporter(), WithZipkinPropagator())

		sampled := true
		baggage := map[string]string{
			baggageKey1: baggageValue1,
		}
		spanContext := SpanContext{
			TraceID: longTraceIdUuid,
			SpanID:  spanIdUuid,
			Sampled: &sampled,
			Baggage: baggage,
		}
		spanContext = spanContext.WithBaggageItem(baggageKey2, baggageValue2)
		if err := tracer.Inject(spanContext, ZipkinWavefrontPropagator{}, carrier); err != nil {
			t.Fatalf("%d: %v", 0, err)
		}

		traceId, ok := readFromCarrier(carrier, ZipkinTraceIdKey)
		assert.True(t, ok)
		assert.Equal(t, longTraceId, traceId)

		sampledVal, _ := readFromCarrier(carrier, ZipkinSampledKey)
		assert.Equal(t, "1", sampledVal)

		bag1, _ := readFromCarrier(carrier, baggageKey1)
		assert.Equal(t, baggageValue1, bag1)

		bag2, _ := readFromCarrier(carrier, baggageKey2)
		assert.Equal(t, baggageValue2, bag2)
	})

	t.Run("with_parent_span", func(t *testing.T) {
		carrier := opentracing.HTTPHeadersCarrier(http.Header{})
		tracer := New(NewInMemoryReporter(), WithZipkinPropagator())

		sampled := false
		baggage := map[string]string{
			baggageKey1: baggageValue1,
		}
		spanContext := SpanContext{
			TraceID: longTraceIdUuid,
			SpanID:  spanIdUuid,
			Sampled: &sampled,
			Baggage: baggage,
		}
		spanContext = spanContext.WithBaggageItem(ZipkinParentSpanIdKey, parentSpanIdUuid)
		if err := tracer.Inject(spanContext, ZipkinWavefrontPropagator{}, carrier); err != nil {
			t.Fatalf("%d: %v", 0, err)
		}

		traceId, ok := readFromCarrier(carrier, ZipkinTraceIdKey)
		assert.True(t, ok)
		assert.Equal(t, longTraceId, traceId)

		parSpanId, ok := readFromCarrier(carrier, ZipkinParentSpanIdKey)
		assert.True(t, ok)
		assert.Equal(t, parentSpanId, parSpanId)

		sampledVal, _ := readFromCarrier(carrier, ZipkinSampledKey)
		assert.Equal(t, "0", sampledVal)

		bag1, _ := readFromCarrier(carrier, baggageKey1)
		assert.Equal(t, baggageValue1, bag1)

		_, ok = readFromCarrier(carrier, baggageKey2)
		assert.False(t, ok)
	})

	t.Run("with_flags", func(t *testing.T) {
		carrier := opentracing.HTTPHeadersCarrier(http.Header{})
		tracer := New(NewInMemoryReporter(), WithZipkinPropagator())

		spanContext := SpanContext{
			TraceID: shortTraceIdUuid,
			SpanID:  spanIdUuid,
			Baggage: nil,
		}
		spanContext = spanContext.WithBaggageItem(ZipkinFlagsKey, "1")
		if err := tracer.Inject(spanContext, ZipkinWavefrontPropagator{}, carrier); err != nil {
			t.Fatalf("%d: %v", 0, err)
		}

		traceId, ok := readFromCarrier(carrier, ZipkinTraceIdKey)
		assert.True(t, ok)
		assert.Equal(t, shortTraceId, traceId)

		_, ok = readFromCarrier(carrier, ZipkinSampledKey)
		assert.False(t, ok)

		flags, ok := readFromCarrier(carrier, ZipkinFlagsKey)
		assert.True(t, ok)
		assert.Equal(t, "1", flags)
	})

	t.Run("with_flags_sampled", func(t *testing.T) {
		carrier := opentracing.HTTPHeadersCarrier(http.Header{})
		tracer := New(NewInMemoryReporter(), WithZipkinPropagator())

		sampled := true
		spanContext := SpanContext{
			TraceID: shortTraceIdUuid,
			SpanID:  spanIdUuid,
			Sampled: &sampled,
			Baggage: nil,
		}
		spanContext = spanContext.WithBaggageItem(ZipkinFlagsKey, "1")
		if err := tracer.Inject(spanContext, ZipkinWavefrontPropagator{}, carrier); err != nil {
			t.Fatalf("%d: %v", 0, err)
		}

		traceId, ok := readFromCarrier(carrier, ZipkinTraceIdKey)
		assert.True(t, ok)
		assert.Equal(t, shortTraceId, traceId)

		_, ok = readFromCarrier(carrier, ZipkinSampledKey)
		assert.False(t, ok)

		flags, ok := readFromCarrier(carrier, ZipkinFlagsKey)
		assert.True(t, ok)
		assert.Equal(t, "1", flags)
	})

	t.Run("with_sampled_override", func(t *testing.T) {
		carrier := opentracing.HTTPHeadersCarrier(http.Header{})
		tracer := New(NewInMemoryReporter(), WithZipkinPropagator(WithOverrideSamplingDecision(false)))

		sampled := true
		spanContext := SpanContext{
			TraceID: shortTraceIdUuid,
			SpanID:  spanIdUuid,
			Sampled: &sampled, // This should be overridden
			Baggage: nil,
		}
		if err := tracer.Inject(spanContext, ZipkinWavefrontPropagator{}, carrier); err != nil {
			t.Fatalf("%d: %v", 0, err)
		}

		traceId, ok := readFromCarrier(carrier, ZipkinTraceIdKey)
		assert.True(t, ok)
		assert.Equal(t, shortTraceId, traceId)

		sampledVal, ok := readFromCarrier(carrier, ZipkinSampledKey)
		assert.True(t, ok)
		assert.Equal(t, "0", sampledVal)
	})
}

func fetchBaggageItem(sc SpanContext, baggageKey string) (string, bool) {
	var (
		val   string
		found bool
	)

	sc.ForeachBaggageItem(func(k, v string) bool {
		if strings.ToLower(k) == strings.ToLower(baggageKey) {
			val = v
			found = true
		}
		return true
	})
	return val, found
}

func readFromCarrier(carrier opentracing.HTTPHeadersCarrier, key string) (string, bool) {
	var (
		val   string
		found bool
	)

	_ = carrier.ForeachKey(func(k, v string) error {
		if strings.ToLower(k) == strings.ToLower(key) {
			val = v
			found = true
		}
		return nil
	})
	return val, found
}
