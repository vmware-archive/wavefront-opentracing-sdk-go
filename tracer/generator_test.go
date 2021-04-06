package tracer_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"
)

func TestGeneratorUUID(t *testing.T) {
	g := tracer.NewGeneratorUUID()

	assertID(t, g.TraceID(), false)
	assertID(t, g.SpanID(), false)
}

func TestGeneratorW3C(t *testing.T) {
	g := tracer.NewGeneratorW3C()

	assertID(t, g.TraceID(), false)
	assertID(t, g.SpanID(), true)
}

func assertID(t *testing.T, id string, short bool) {
	assert.Len(t, id, 36)
	_, err := uuid.Parse(id)
	assert.NoError(t, err)
	zeros := strings.HasPrefix(id, "00000000-0000-0000-")
	assert.True(t, zeros == short)
}
