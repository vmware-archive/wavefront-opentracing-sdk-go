package reporter

import (
	"fmt"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"
	wf "github.com/wavefronthq/wavefront-sdk-go/senders"
)

func prepareReferences(span tracer.RawSpan) ([]string, []string) {
	parents := make([]string, 0)
	followsFrom := make([]string, 0)

	for _, ref := range span.References {
		refCtx := ref.ReferencedContext.(tracer.SpanContext)
		switch ref.Type {
		case opentracing.ChildOfRef:
			parents = append(parents, refCtx.SpanID)
		case opentracing.FollowsFromRef:
			followsFrom = append(followsFrom, refCtx.SpanID)
		}
	}
	return parents, followsFrom
}

func prepareTags(span tracer.RawSpan) []wf.SpanTag {
	tags := make([]wf.SpanTag, 0)

	for k, v := range span.Context.Baggage {
		tags = append(tags, wf.SpanTag{Key: k, Value: v})
	}

	for k, v := range span.Tags {
		tags = append(tags, wf.SpanTag{Key: k, Value: fmt.Sprintf("%v", v)})
	}

	return tags
}
