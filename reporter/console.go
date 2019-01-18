package reporter

import (
	"log"
	"strconv"
	"sync"

	"github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"
)

// ConsoleSpanReporter send spand to STDOUT.
type ConsoleSpanReporter struct {
	sync.Mutex
}

// NewConsoleSpanReporter returns a ConsoleSpanReporter.
func NewConsoleSpanReporter() tracer.SpanReporter {
	return &ConsoleSpanReporter{}
}

// ReportSpan complies with the SpanReporter interface.
func (r *ConsoleSpanReporter) ReportSpan(span tracer.RawSpan) {
	tags := prepareTags(span)
	parents, followsFrom := prepareReferences(span)

	r.Lock()
	defer r.Unlock()

	log.Printf("-- Operation: %v\n", span.Operation)
	log.Printf("\t- TraceID: %v\n", span.Context.TraceID)
	log.Printf("\t- SpanID: %v\n", span.Context.SpanID)
	log.Printf("\t- parents: %v\n", parents)
	log.Printf("\t- followsFrom: %v\n", followsFrom)
	log.Printf("\t- start: %v (%d)\n", span.Start.UnixNano(), len(strconv.FormatInt(span.Start.UnixNano(), 10)))
	log.Printf("\t- Duration: %v\n", span.Duration.Nanoseconds())
	log.Printf("\t- tags: %v\n", tags)
}
