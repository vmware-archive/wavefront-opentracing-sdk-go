package tracer

import (
	"fmt"
	"os"
	"strconv"

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

// WavefrontSpanReporter implements the wavefront.Recorder interface.
type WavefrontSpanReporter struct {
	Source      string
	sender      wf.Sender
	application ApplicationTags
}

// NewSpanReporter returns a WavefrontSpanReporter for the given `sender`.
func NewSpanReporter(sender wf.Sender, application ApplicationTags) *WavefrontSpanReporter {
	return &WavefrontSpanReporter{
		sender:      sender,
		Source:      hostname(),
		application: application,
	}
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		name = "localhost"
	}
	return name
}

// RecordSpan complies with the tracer.Recorder interface.
func (t *WavefrontSpanReporter) RecordSpan(span RawSpan) {
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
	t.sender.SendSpan(span.Operation, span.Start.UnixNano(), span.Duration.Nanoseconds(), t.Source,
		span.Context.TraceID, span.Context.SpanID, parents,
		nil, tags, nil)

	// just for DEBUG
	fmt.Printf("-- Operation: %v\n", span.Operation)
	fmt.Printf("\t- TraceID: %v\n", span.Context.TraceID)
	fmt.Printf("\t- SpanID: %v\n", span.Context.SpanID)
	fmt.Printf("\t- parents: %v\n", span.ParentSpanID)
	fmt.Printf("\t- start: %v (%d)\n", span.Start.UnixNano(), len(strconv.FormatInt(span.Start.UnixNano(), 10)))
	fmt.Printf("\t- Duration: %v\n", span.Duration.Nanoseconds())
	fmt.Printf("\t- tags: %v\n", tags)
	fmt.Printf("\t- allTags: %v\n", allTags)
}
