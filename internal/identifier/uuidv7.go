package identifier

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// UUIDv7Generator produces time-ordered RFC 9562 UUIDv7 identifiers. Calls on
// one generator are strictly monotonic, including when the clock stalls or moves
// backwards.
type UUIDv7Generator struct {
	mu     sync.Mutex
	now    func() time.Time
	lastMS int64
	seq    uint16
}

func NewUUIDv7Generator() *UUIDv7Generator {
	return &UUIDv7Generator{now: time.Now}
}

func (g *UUIDv7Generator) New() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	ms := g.now().UnixMilli()
	if ms < g.lastMS {
		ms = g.lastMS
	}
	if ms == g.lastMS {
		g.seq++
		if g.seq > 0x0fff {
			ms++
			g.seq = 0
		}
	} else {
		var seed [2]byte
		if _, err := rand.Read(seed[:]); err != nil {
			return "", fmt.Errorf("seed uuidv7 sequence: %w", err)
		}
		g.seq = (uint16(seed[0])<<8 | uint16(seed[1])) & 0x0fff
	}
	g.lastMS = ms
	var id [16]byte
	for i := 5; i >= 0; i-- {
		id[i] = byte(ms)
		ms >>= 8
	}
	id[6] = 0x70 | byte(g.seq>>8)
	id[7] = byte(g.seq)
	if _, err := rand.Read(id[8:]); err != nil {
		return "", fmt.Errorf("randomize uuidv7: %w", err)
	}
	id[8] = (id[8] & 0x3f) | 0x80
	return format(id), nil
}

func format(id [16]byte) string {
	var dst [36]byte
	hex.Encode(dst[0:8], id[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], id[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], id[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], id[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], id[10:16])
	return string(dst[:])
}
