package identifier

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// RFC 9562 準拠の単調UUIDv7生成器
type UUIDv7Generator struct {
	mu     sync.Mutex
	now    func() time.Time
	lastMS int64
	seq    uint16
}

// UUIDv7生成器の生成
func NewUUIDv7Generator() *UUIDv7Generator {
	return &UUIDv7Generator{now: time.Now}
}

// 単調なUUIDv7の生成
func (g *UUIDv7Generator) New() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	// 時計逆行時のミリ秒固定とシーケンス繰り上げ
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
		// 新ミリ秒の乱数シーケンス初期化
		var seed [2]byte
		if _, err := rand.Read(seed[:]); err != nil {
			return "", fmt.Errorf("seed uuidv7 sequence: %w", err)
		}
		g.seq = (uint16(seed[0])<<8 | uint16(seed[1])) & 0x0fff
	}
	g.lastMS = ms
	// タイムスタンプと版・バリアントの埋め込み
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

// 16バイトのハイフン付き16進文字列化
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
