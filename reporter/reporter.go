// Package reporter provides functionality for reporting spans to Wavefront.
package reporter

import (
	"fmt"
	"log"
	"math/rand"
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
	bufferSize       int
	spansCh          chan tracer.RawSpan
	done             chan bool
	logPercent       float32
	mtx              sync.Mutex
	derivedReporter  reporting.WavefrontMetricsReporter
	internalReporter reporting.WavefrontMetricsReporter

	queueSize               metrics.Gauge
	remCapacity             metrics.Gauge
	errorsCount             metrics.Counter
	spansReceived           metrics.Counter
	spansDropped            metrics.Counter
	spansDiscarded          metrics.Counter
	redMetricsCustomTagKeys map[string]struct{}
}

var (
	exists = struct{}{}
)

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
		args.bufferSize = size
	}
}

// Percent of log messages to be logged by this reporter. Between 0.0 and 1.0. Defaults to 0.1 or 10%.
func LogPercent(percent float32) Option {
	return func(args *reporter) {
		if percent < 0.0 {
			percent = 0.0
		} else if percent > 1.0 {
			percent = 1.0
		}
		args.logPercent = percent
	}
}

// Custom RED metrics tags
func RedMetricsCustomTagKeys(redMetricsCustomTagKeys []string) Option {
	return func(args *reporter) {
		for _, key := range redMetricsCustomTagKeys {
			args.redMetricsCustomTagKeys[key] = exists
		}
	}
}

// New returns a WavefrontSpanReporter for the given `sender`.
func New(sender senders.Sender, app application.Tags, setters ...Option) WavefrontSpanReporter {
	r := &reporter{
		sender:                  sender,
		source:                  hostname(),
		application:             app,
		logPercent:              0.1,
		bufferSize:              50000,
		redMetricsCustomTagKeys: make(map[string]struct{}),
	}

	for _, setter := range setters {
		setter(r)
	}

	r.spansCh = make(chan tracer.RawSpan, r.bufferSize)

	// init rand for logging
	rand.Seed(time.Now().UnixNano())

	r.derivedReporter = reporting.NewReporter(
		sender,
		r.application,
		reporting.Interval(time.Second*60),
		reporting.Source(r.source),
		reporting.Prefix("tracing.derived"),
		reporting.CustomRegistry(metrics.NewRegistry()),
	)

	r.internalReporter = reporting.NewReporter(
		sender,
		r.application,
		reporting.Interval(time.Second*60),
		reporting.Source(r.source),
		reporting.Prefix("~sdk.go.opentracing.reporter"),
		reporting.CustomRegistry(metrics.NewRegistry()),
	)

	r.spansReceived = r.internalReporter.GetOrRegisterMetric(reporting.DeltaCounterName("spans.received"), metrics.NewCounter(), nil).(metrics.Counter)
	r.spansDropped = r.internalReporter.GetOrRegisterMetric(reporting.DeltaCounterName("spans.dropped"), metrics.NewCounter(), nil).(metrics.Counter)
	r.spansDiscarded = r.internalReporter.GetOrRegisterMetric(reporting.DeltaCounterName("spans.discarded"), metrics.NewCounter(), nil).(metrics.Counter)
	r.errorsCount = r.internalReporter.GetOrRegisterMetric(reporting.DeltaCounterName("errors"), metrics.NewCounter(), nil).(metrics.Counter)

	r.queueSize = r.internalReporter.GetOrRegisterMetric("queue.size", metrics.NewFunctionalGauge(func() int64 {
		return int64(len(r.spansCh))
	}), nil).(metrics.Gauge)
	r.remCapacity = r.internalReporter.GetOrRegisterMetric("queue.remaining_capacity", metrics.NewFunctionalGauge(func() int64 {
		return int64(r.bufferSize - len(r.spansCh))
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
		if t.loggingAllowed() {
			log.Printf("buffer full, dropping span: %s\n", span.Operation)
		}
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
		if t.loggingAllowed() {
			log.Printf("error reporting span: %s error: %v", span.Operation, err)
		}
	}
}

func (t *reporter) copyTags(oriTags map[string]string) map[string]string {
	newTags := make(map[string]string, len(oriTags)+1)
	for key, value := range oriTags {
		newTags[key] = value
	}
	return newTags
}

func (t *reporter) reportDerivedMetrics(span tracer.RawSpan) {
	// override application and service name if tag present
	appName, appFound := getAppTag("application", t.application.Application, span.Tags)
	serviceName, svcFound := getAppTag("service", t.application.Service, span.Tags)

	metricName := fmt.Sprintf("%s.%s.%s", appName, serviceName, span.Operation)
	metricName = strings.Replace(metricName, " ", "-", -1)
	metricName = strings.Replace(metricName, "\"", "\\\"", -1)

	tags := t.application.Map()
	tags["component"] = span.Component
	replaceTag(tags, "application", appName, appFound)
	replaceTag(tags, "service", serviceName, svcFound)

	for key := range t.redMetricsCustomTagKeys {
		if value, found := getAppTag(key, "", span.Tags); found {
			tags[key] = value
		}
	}
	err, _ := getAppTag(string(ext.Error), "false", span.Tags)
	isError := err == "true"
	// add http status if span has error
	if value, found := getAppTag(string(ext.HTTPStatusCode), "", span.Tags); found {
		tags[string(ext.HTTPStatusCode)] = value
	}
	// propagate span kind tag by default
	tags[string(ext.SpanKind)], _ = getAppTag(string(ext.SpanKind), "none", span.Tags)
	t.heartbeater.AddCustomTags(tags)

	// add operation tag after setting heartbeat tag
	tags["operationName"] = span.Operation

	errors := t.getCounter(reporting.DeltaCounterName(metricName+".error"), tags)
	if isError {
		errors.Inc(1)
	}

	t.getCounter(reporting.DeltaCounterName(metricName+".total_time.millis"), tags).Inc(span.Duration.Nanoseconds() / 1000000)
	t.getCounter(reporting.DeltaCounterName(metricName+".invocation"), tags).Inc(1)
	if isError {
		tagsError := t.copyTags(tags)
		tagsError["error"] = "true"
		t.getHistogram(metricName+".duration.micros", tagsError).Update(span.Duration.Nanoseconds() / 1000)
	} else {
		t.getHistogram(metricName+".duration.micros", tags).Update(span.Duration.Nanoseconds() / 1000)
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

func (t *reporter) loggingAllowed() bool {
	return rand.Float32() <= t.logPercent
}

func (t *reporter) Flush() {
	t.derivedReporter.Report()
}
