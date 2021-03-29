package tracer

import (
	crypto "crypto/rand"
	"encoding/binary"
	"math/rand"
	"sync"

	"github.com/google/uuid"
)

type Generator interface {
	TraceID() string
	SpanID() string
}

type GeneratorUUID struct {
	random *random
}

func NewGeneratorUUID() Generator {
	return &GeneratorUUID{
		random: newRandom(),
	}
}

func (g *GeneratorUUID) TraceID() string {
	return g.random.uuid(false)
}

func (g *GeneratorUUID) SpanID() string {
	return g.random.uuid(false)
}

type GeneratorW3C struct {
	random *random
}

func NewGeneratorW3C() Generator {
	return &GeneratorW3C{
		random: newRandom(),
	}
}

func (g *GeneratorW3C) TraceID() string {
	return g.random.uuid(false)
}

func (g *GeneratorW3C) SpanID() string {
	return g.random.uuid(true)
}

type random struct {
	sync.Mutex
	rng *rand.Rand
}

func newRandom() *random {
	var seed int64
	_ = binary.Read(crypto.Reader, binary.LittleEndian, &seed)
	return &random{
		Mutex: sync.Mutex{},
		rng:   rand.New(rand.NewSource(seed)),
	}
}

func (r *random) read(p []byte) {
	r.Lock()
	defer r.Unlock()

	r.rng.Read(p)
}

func (r *random) uuid(short bool) string {
	data := make([]byte, 16)
	if short {
		r.read(data[8:])
	} else {
		r.read(data)
	}
	id, _ := uuid.FromBytes(data)
	return id.String()
}
