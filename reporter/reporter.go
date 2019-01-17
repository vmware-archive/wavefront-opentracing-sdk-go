package reporter

import (
	"fmt"
	"os"

	"github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"
	wf "github.com/wavefronthq/wavefront-sdk-go/senders"
)

type ApplicationTags struct {
	application string
	service     string
	Cluster     string
	Shard       string
	tags        map[string]string
}

func NewApplicationTags(app, serv string) ApplicationTags {
	return ApplicationTags{
		application: app,
		service:     serv,
		tags:        make(map[string]string, 0),
	}
}

// WavefrontSpanReporter implements the wavefront.Reporter interface.
type WavefrontSpanReporter struct {
	source      string
	sender      wf.Sender
	application ApplicationTags
}

// Option allow WavefrontSpanReporter customization
type Option func(*WavefrontSpanReporter)

// Source tag for the spans
func Source(source string) Option {
	return func(args *WavefrontSpanReporter) {
		args.source = source
	}
}

// Nwe returns a WavefrontSpanReporter for the given `sender`.
func New(sender wf.Sender, application ApplicationTags, setters ...Option) *WavefrontSpanReporter {
	r := &WavefrontSpanReporter{
		sender:      sender,
		source:      hostname(),
		application: application,
	}
	for _, setter := range setters {
		setter(r)
	}
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
	allTags := make(map[string]string)

	allTags["application"] = t.application.application
	allTags["service"] = t.application.service

	for k, v := range t.application.tags {
		allTags[k] = fmt.Sprintf("%v", v)
	}

	for k, v := range span.Context.Baggage {
		allTags[k] = fmt.Sprintf("%v", v)
	}

	for k, v := range span.Tags {
		allTags[k] = fmt.Sprintf("%v", v)
	}

	tags := make([]wf.SpanTag, 0)
	for k, v := range allTags {
		tags = append(tags, wf.SpanTag{Key: k, Value: fmt.Sprintf("%v", v)})
	}

	var parents []string
	if len(span.ParentSpanID) > 0 {
		parents = []string{span.ParentSpanID}
	}
	t.sender.SendSpan(span.Operation, span.Start.UnixNano(), span.Duration.Nanoseconds(), t.source,
		span.Context.TraceID, span.Context.SpanID, parents,
		nil, tags, nil)
}
