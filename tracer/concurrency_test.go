package tracer

import (
	"sync"
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
)

const op = "test"

func TestConcurrentUsage(t *testing.T) {
	var cr CountingReporter
	tracer := New(&cr)
	var wg sync.WaitGroup
	const num = 100
	wg.Add(num)
	for i := 0; i < num; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < num; j++ {
				sp := tracer.StartSpan(op)
				sp.LogEvent("test event")
				sp.SetTag("foo", "bar")
				sp.SetBaggageItem("boo", "far")
				sp.SetOperationName("x")
				csp := tracer.StartSpan(
					"csp",
					opentracing.ChildOf(sp.Context()))
				csp.Finish()
				defer sp.Finish()
			}
		}()
	}
	wg.Wait()
}

func TestDisableSpanPool(t *testing.T) {
	var cr CountingReporter
	tracer := New(&cr)

	parent := tracer.StartSpan("parent")
	parent.Finish()
	// This shouldn't panic.
	child := tracer.StartSpan(
		"child",
		opentracing.ChildOf(parent.Context()))
	child.Finish()
}
