package identifier

import (
	"regexp"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestUUIDv7VersionVariantAndMonotonicity(t *testing.T) {
	g := NewUUIDv7Generator()
	g.now = func() time.Time { return time.UnixMilli(1700000000000) }
	got := make([]string, 1000)
	seen := make(map[string]struct{}, len(got))
	for i := range got {
		var err error
		got[i], err = g.New()
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(got[i]) {
			t.Fatalf("invalid UUIDv7: %s", got[i])
		}
		if _, exists := seen[got[i]]; exists {
			t.Fatalf("duplicate: %s", got[i])
		}
		seen[got[i]] = struct{}{}
	}
	sorted := append([]string(nil), got...)
	sort.Strings(sorted)
	for i := range got {
		if got[i] != sorted[i] {
			t.Fatalf("not monotonic at %d", i)
		}
	}
}

func TestUUIDv7ConcurrentUniqueness(t *testing.T) {
	g := NewUUIDv7Generator()
	const count = 1000
	ids := make(chan string, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := g.New()
			if err != nil {
				t.Error(err)
				return
			}
			ids <- id
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate: %s", id)
		}
		seen[id] = true
	}
	if len(seen) != count {
		t.Fatalf("got %d", len(seen))
	}
}
