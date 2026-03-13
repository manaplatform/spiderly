// internal/core/dedup.go
package core

import "sync"

// URLDedup is a thread-safe set for URL deduplication.
type URLDedup struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

// NewURLDedup creates a new URL deduplication set.
func NewURLDedup() *URLDedup {
	return &URLDedup{
		seen: make(map[string]struct{}),
	}
}

// Add returns true if the URL was not already seen (and adds it).
// Returns false if it's a duplicate.
func (d *URLDedup) Add(url string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.seen[url]; exists {
		return false
	}
	d.seen[url] = struct{}{}
	return true
}
