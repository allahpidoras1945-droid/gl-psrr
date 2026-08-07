package filter

import (
	"strings"
	"sync"
)

type Deduplicator struct {
	mu   sync.RWMutex
	seen map[string]struct{}
}

func NewDeduplicator() *Deduplicator { return &Deduplicator{seen: make(map[string]struct{}, 20000)} }

// IsDuplicate returns true after the first observation of an identifier.
func (d *Deduplicator) IsDuplicate(identifier string) bool {
	normalized := strings.ToLower(strings.TrimSpace(identifier))
	if normalized == "" {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.seen == nil {
		d.seen = make(map[string]struct{}, 20000)
	}
	if _, exists := d.seen[normalized]; exists {
		return true
	}
	d.seen[normalized] = struct{}{}
	return false
}

func (d *Deduplicator) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen = make(map[string]struct{}, 20000)
}
