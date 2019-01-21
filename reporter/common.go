package reporter

import (
	"fmt"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"
	wf "github.com/wavefronthq/wavefront-sdk-go/senders"
)

func prepareReferences(span tracer.RawSpan) ([]string, []string) {
	var parents []string
	var followsFrom []string

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
	if len(span.Tags) == 0 {
		return nil
	}
	tags := make([]wf.SpanTag, len(span.Tags))
	i := 0
	for k, v := range span.Tags {
		tags[i] = wf.SpanTag{Key: k, Value: fmt.Sprintf("%v", v)}
		i++
	}
	return tags
}
