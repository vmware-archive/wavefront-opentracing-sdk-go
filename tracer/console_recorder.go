package tracer

import (
	"fmt"
	"log"
	"strconv"
	"sync"

	wf "github.com/wavefronthq/wavefront-sdk-go/senders"
)

// ConsoleSpanRecorder send spand to STDOUT.
type ConsoleSpanRecorder struct {
	mux sync.Mutex
}

// NewConsoleSpanRecorder returns a ConsoleSpanRecorder.
func NewConsoleSpanRecorder() SpanRecorder {
	return &ConsoleSpanRecorder{}
}

// RecordSpan complies with the SpanRecorder interface.
func (r *ConsoleSpanRecorder) RecordSpan(span RawSpan) {
	allTags := make(map[string]string)

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

	r.mux.Lock()
	defer r.mux.Unlock()

	log.Printf("-- Operation: %v\n", span.Operation)
	log.Printf("\t- TraceID: %v\n", span.Context.TraceID)
	log.Printf("\t- SpanID: %v\n", span.Context.SpanID)
	log.Printf("\t- parents: %v\n", span.ParentSpanID)
	log.Printf("\t- start: %v (%d)\n", span.Start.UnixNano(), len(strconv.FormatInt(span.Start.UnixNano(), 10)))
	log.Printf("\t- Duration: %v\n", span.Duration.Nanoseconds())
	log.Printf("\t- tags: %v\n", tags)
	log.Printf("\t- allTags: %v\n", allTags)
}
