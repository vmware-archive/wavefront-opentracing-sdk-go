package reporter

import (
	"fmt"
	"os"
	"time"

	"github.com/opentracing/opentracing-go/ext"
	metrics "github.com/rcrowley/go-metrics"
	wfMetrics "github.com/wavefronthq/go-metrics-wavefront"
	"github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"
	"github.com/wavefronthq/wavefront-sdk-go/application"
	wf "github.com/wavefronthq/wavefront-sdk-go/senders"
)

// WavefrontSpanReporter implements the wavefront.Reporter interface.
type WavefrontSpanReporter struct {
	source      string
	sender      wf.Sender
	application application.Tags
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

	hostTags := map[string]string{
		"source": r.source,
	}
	go wfMetrics.WavefrontDirect(metrics.DefaultRegistry, 5*time.Second, hostTags,
		"tracing.derived", "https://virunga.wavefront.com", "be4d70eb-c03a-4b93-b5d6-b8fd4b29629b")

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

func (t WavefrontSpanReporter) reportDerivedMetrics(span tracer.RawSpan) {
	metricName := fmt.Sprintf("%s.%s.%s", t.application.Application, t.application.Service, span.Operation)
	tags := t.application.Map()
	tags["operationName"] = span.Operation

	wfMetrics.GetOrRegisterMetric(metricName+".invocation", metrics.NewCounter(), tags).(metrics.Counter).Inc(1)
	fmt.Printf("--- %v ---\n", span.Tags[string(ext.Error)])
	if span.Tags[string(ext.Error)] == true {
		wfMetrics.GetOrRegisterMetric(metricName+".error", metrics.NewCounter(), tags).(metrics.Counter).Inc(1)
	}
}
