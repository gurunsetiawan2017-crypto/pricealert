package idgen

import (
	"crypto/rand"
	"io"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

type ULIDGenerator struct {
	mu      sync.Mutex
	entropy io.Reader
}

func NewULIDGenerator() *ULIDGenerator {
	return &ULIDGenerator{entropy: rand.Reader}
}

func (g *ULIDGenerator) Next() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	return ulid.MustNew(ulid.Timestamp(time.Now().UTC()), g.entropy).String()
}
