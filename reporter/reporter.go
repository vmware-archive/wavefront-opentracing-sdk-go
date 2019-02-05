package reporter

import (
	"fmt"
	"os"
	"strings"

	"github.com/opentracing/opentracing-go/ext"
	metrics "github.com/rcrowley/go-metrics"
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
	source      string
	sender      senders.Sender
	application application.Tags
	metrics     reporting.WavefrontMetricsReporter
	heartbeater application.HeartbeatService
}

// Option allow WavefrontSpanReporter customization
type Option func(*reporter)

// Source tag for the spans
func Source(source string) Option {
	return func(args *reporter) {
		args.source = source
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

	r.metrics = reporting.NewReporter(
		sender,
		r.application,
		reporting.Source(r.source),
		reporting.Prefix("tracing.derived"),
	)

	r.heartbeater = application.StartHeartbeatService(
		sender,
		r.application,
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
func (t *reporter) ReportSpan(span tracer.RawSpan) {
	t.reportDerivedMetrics(span)
	if !span.Context.Sampled {
		return
	}

	tags := prepareTags(span)
	parents, followsFrom := prepareReferences(span)

	for k, v := range t.application.Map() {
		tags = append(tags, senders.SpanTag{Key: k, Value: v})
	}

	t.sender.SendSpan(span.Operation, span.Start.UnixNano()/1000000, span.Duration.Nanoseconds()/1000000, t.source,
		span.Context.TraceID, span.Context.SpanID, parents, followsFrom, tags, nil)
}

func (t *reporter) reportDerivedMetrics(span tracer.RawSpan) {
	metricName := fmt.Sprintf("%s.%s.%s", t.application.Application, t.application.Service, span.Operation)
	metricName = strings.Replace(metricName, " ", "-", -1)
	metricName = strings.Replace(metricName, "\"", "\\\"", -1)

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

func (t *reporter) getHistogram(name string, tags map[string]string) reporting.Histogram {
	h := reporting.GetMetric(name, tags)
	if h == nil {
		h = reporting.NewHistogram()
		err := reporting.RegisterMetric(name, h, tags)
		if err != nil {
			panic(err)
		}
	}
	return h.(reporting.Histogram)
}

func (t *reporter) getCounter(name string, tags map[string]string) metrics.Counter {
	c := reporting.GetMetric(name, tags)
	if c == nil {
		c = metrics.NewCounter()
		err := reporting.RegisterMetric(name, c, tags)
		if err != nil {
			panic(err)
		}
	}
	return c.(metrics.Counter)
}

func (t *reporter) Flush() {
	t.metrics.Report()
}
