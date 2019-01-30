package reporter

import (
	"fmt"
	"os"

	"github.com/wavefronthq/wavefront-sdk-go/heartbeater"

	"github.com/opentracing/opentracing-go/ext"
	metrics "github.com/rcrowley/go-metrics"
	metricsReporter "github.com/wavefronthq/go-metrics-wavefront/reporter"
	"github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"
	"github.com/wavefronthq/wavefront-sdk-go/application"
	wf "github.com/wavefronthq/wavefront-sdk-go/senders"
)

// WavefrontSpanReporter implements the wavefront.Reporter interface.
type WavefrontSpanReporter struct {
	source      string
	sender      wf.Sender
	application application.Tags
	metrics     metricsReporter.WavefrontMetricsReporter
	heartbeater heartbeater.Service
}

// Option allow WavefrontSpanReporter customization
type Option func(*WavefrontSpanReporter)

// Source tag for the spans
func Source(source string) Option {
	return func(args *WavefrontSpanReporter) {
		args.source = source
	}
}

// New returns a WavefrontSpanReporter for the given `sender`.
func New(sender wf.Sender, application application.Tags, setters ...Option) *WavefrontSpanReporter {
	r := &WavefrontSpanReporter{
		sender:      sender,
		source:      hostname(),
		application: application,
	}

	for _, setter := range setters {
		setter(r)
	}

	r.metrics = metricsReporter.New(
		sender,
		application,
		metricsReporter.Source(r.source),
		metricsReporter.Prefix("tracing.derived"),
	)

	r.heartbeater = heartbeater.Start(
		sender,
		application,
		r.source,
	)

	return r
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		name = "wavefront-tracer-go"
	}
	return name
}

// ReportSpan complies with the tracer.Reporter interface.
func (t *WavefrontSpanReporter) ReportSpan(span tracer.RawSpan) {
	t.reportDerivedMetrics(span)
	if !span.Context.Sampled {
		return
	}

	tags := prepareTags(span)
	parents, followsFrom := prepareReferences(span)

	for k, v := range t.application.Map() {
		tags = append(tags, wf.SpanTag{Key: k, Value: v})
	}

	t.sender.SendSpan(span.Operation, span.Start.UnixNano()/1000000, span.Duration.Nanoseconds()/1000000, t.source,
		span.Context.TraceID, span.Context.SpanID, parents, followsFrom, tags, nil)
}

func (t *WavefrontSpanReporter) reportDerivedMetrics(span tracer.RawSpan) {
	metricName := fmt.Sprintf("%s.%s.%s", t.application.Application, t.application.Service, span.Operation)
	tags := t.application.Map()
	tags["operationName"] = span.Operation

	t.getHistogram(metricName+".duration.micros", tags).Update(span.Duration.Nanoseconds() / 1000)
	t.getCounter(metricName+".total_time.millis", tags).Inc(span.Duration.Nanoseconds() / 1000000)
	t.getCounter(metricName+".invocation", tags).Inc(1)
	errors := t.getCounter(metricName+".error", tags)
	if span.Tags[string(ext.Error)] == true {
		errors.Inc(1)
	}
}

func (t *WavefrontSpanReporter) getHistogram(name string, tags map[string]string) metricsReporter.Histogram {
	h := metricsReporter.GetMetric(name, tags)
	if h == nil {
		h = metricsReporter.NewHistogram()
		metricsReporter.RegisterMetric(name, h, tags)
	}
	return h.(metricsReporter.Histogram)
}

func (t *WavefrontSpanReporter) getCounter(name string, tags map[string]string) metrics.Counter {
	c := metricsReporter.GetMetric(name, tags)
	if c == nil {
		c = metrics.NewCounter()
		metricsReporter.RegisterMetric(name, c, tags)
	}
	return c.(metrics.Counter)
}
