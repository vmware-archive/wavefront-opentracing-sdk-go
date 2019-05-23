// Package reporter provides functionality for reporting spans to Wavefront.
package reporter

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go/ext"
	"github.com/rcrowley/go-metrics"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
	"github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"
	"github.com/wavefronthq/wavefront-sdk-go/application"
	"github.com/wavefronthq/wavefront-sdk-go/senders"
)

// WavefrontSpanReporter implements the wavefront.Reporter interface.
type WavefrontSpanReporter interface {
	tracer.SpanReporter
	Flush()
}

type reporter struct {
	source           string
	sender           senders.Sender
	application      application.Tags
	heartbeater      application.HeartbeatService
	spansCh          chan tracer.RawSpan
	done             chan bool
	mtx              sync.Mutex
	derivedReporter  reporting.WavefrontMetricsReporter
	internalReporter reporting.WavefrontMetricsReporter

	queueSize      metrics.Gauge
	remCapacity    metrics.Gauge
	errorsCount    metrics.Counter
	spansReceived  metrics.Counter
	spansDropped   metrics.Counter
	spansDiscarded metrics.Counter
}

// Option allow WavefrontSpanReporter customization
type Option func(*reporter)

// Source tag for the spans
func Source(source string) Option {
	return func(args *reporter) {
		args.source = source
	}
}

// Buffer size for the in-memory buffer. Incoming spans are dropped if buffer is full.
// Defaults to 50,000.
func BufferSize(size int) Option {
	return func(args *reporter) {
		args.spansCh = make(chan tracer.RawSpan, size)
	}
}

// New returns a WavefrontSpanReporter for the given `sender`.
func New(sender senders.Sender, app application.Tags, setters ...Option) WavefrontSpanReporter {
	r := &reporter{
		sender:      sender,
		source:      hostname(),
		application: app,
	}

	for _, setter := range setters {
		setter(r)
	}

	if r.spansCh == nil {
		r.spansCh = make(chan tracer.RawSpan, 50000)
	}

	r.derivedReporter = reporting.NewReporter(
		sender,
		r.application,
		reporting.Interval(time.Second*60),
		reporting.Source(r.source),
		reporting.Prefix("tracing.derived"),
	)

	r.internalReporter = reporting.NewReporter(
		sender,
		r.application,
		reporting.Interval(time.Second*60),
		reporting.Source(r.source),
		reporting.Prefix("~sdk.go.opentracing.reporter"),
		reporting.CustomRegistry(metrics.NewRegistry()),
	)

	r.spansReceived = r.internalReporter.GetOrRegisterMetric("spans.received.count", metrics.NewCounter(), nil).(metrics.Counter)
	r.spansDropped = r.internalReporter.GetOrRegisterMetric("spans.dropped.count", metrics.NewCounter(), nil).(metrics.Counter)
	r.spansDiscarded = r.internalReporter.GetOrRegisterMetric("spans.discarded.count", metrics.NewCounter(), nil).(metrics.Counter)
	r.errorsCount = r.internalReporter.GetOrRegisterMetric("errors.count", metrics.NewCounter(), nil).(metrics.Counter)

	r.queueSize = r.internalReporter.GetOrRegisterMetric("queue.size", metrics.NewFunctionalGauge(func() int64 {
		return int64(len(r.spansCh))
	}), nil).(metrics.Gauge)
	r.remCapacity = r.internalReporter.GetOrRegisterMetric("queue.remaining_capacity", metrics.NewFunctionalGauge(func() int64 {
		return int64(cap(r.spansCh) - len(r.spansCh))
	}), nil).(metrics.Gauge)

	r.heartbeater = application.StartHeartbeatService(
		sender,
		r.application,
		r.source,
		"go",
		"opentracing",
	)

	// kick off async span processing
	go r.process()

	return r
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		name = "wavefront-tracer-go"
	}
	return name
}

func (t *reporter) process() {
	for {
		select {
		case span, more := <-t.spansCh:
			if !more {
				t.done <- true
				return
			}
			t.reportInternal(span)
		}
	}
}

// ReportSpan complies with the tracer.SpanReporter interface.
func (t *reporter) ReportSpan(span tracer.RawSpan) {
	t.reportDerivedMetrics(span)
	if span.Context.IsSampled() && !*span.Context.SamplingDecision() {
		t.spansDiscarded.Inc(1)
		return
	}

	t.spansReceived.Inc(1)
	select {
	case t.spansCh <- span:
		return
	default:
		t.spansDropped.Inc(1)
	}
}

func (t *reporter) Close() error {
	close(t.spansCh)
	select {
	case <-t.done:
		log.Println("closed wavefront reporter")
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timed out closing wavefront reporter")
	}
	t.derivedReporter.Close()
	t.internalReporter.Close()
	return nil
}

func (t *reporter) reportInternal(span tracer.RawSpan) {
	tags := prepareTags(span)
	parents, followsFrom := prepareReferences(span)

	for k, v := range t.application.Map() {
		// do not append if tag is already present on the span
		if value, found := getAppTag(k, v, span.Tags); !found {
			tags = append(tags, senders.SpanTag{Key: k, Value: value})
		}
	}
	logs := prepareLogs(span)

	err := t.sender.SendSpan(span.Operation, span.Start.UnixNano()/1000000, span.Duration.Nanoseconds()/1000000,
		t.source, span.Context.TraceID, span.Context.SpanID, parents, followsFrom, tags, logs)
	if err != nil {
		t.errorsCount.Inc(1)
	}
}

func (t *reporter) reportDerivedMetrics(span tracer.RawSpan) {
	// override application and service name if tag present
	appName, appFound := getAppTag("application", t.application.Application, span.Tags)
	serviceName, svcFound := getAppTag("service", t.application.Service, span.Tags)

	metricName := fmt.Sprintf("%s.%s.%s", appName, serviceName, span.Operation)
	metricName = strings.Replace(metricName, " ", "-", -1)
	metricName = strings.Replace(metricName, "\"", "\\\"", -1)

	tags := t.application.Map()
	tags["operationName"] = span.Operation
	tags["component"] = span.Component
	replaceTag(tags, "application", appName, appFound)
	replaceTag(tags, "service", serviceName, svcFound)

	t.getHistogram(metricName+".duration.micros", tags).Update(span.Duration.Nanoseconds() / 1000)
	t.getCounter(metricName+".total_time.millis", tags).Inc(span.Duration.Nanoseconds() / 1000000)
	t.getCounter(metricName+".invocation", tags).Inc(1)
	errors := t.getCounter(metricName+".error", tags)
	if span.Tags[string(ext.Error)] == true {
		errors.Inc(1)
	}
}

func (t *reporter) getHistogram(name string, tags map[string]string) reporting.Histogram {
	h := t.derivedReporter.GetMetric(name, tags)
	if h == nil {
		t.mtx.Lock()
		h = t.derivedReporter.GetOrRegisterMetric(name, reporting.NewHistogram(), tags)
		t.mtx.Unlock()
	}
	return h.(reporting.Histogram)
}

func (t *reporter) getCounter(name string, tags map[string]string) metrics.Counter {
	c := t.derivedReporter.GetMetric(name, tags)
	if c == nil {
		t.mtx.Lock()
		c = t.derivedReporter.GetOrRegisterMetric(name, metrics.NewCounter(), tags)
		t.mtx.Unlock()
	}
	return c.(metrics.Counter)
}

func (t *reporter) Flush() {
	t.derivedReporter.Report()
}
