// internal/crawler/queue.go
package crawler

import (
	"sync"

	"spiderly/internal/models"
)

// URLQueue is a thread-safe FIFO queue for discovered links.
// Backed by a ring buffer that grows as needed — no slice leak.
type URLQueue struct {
	mu   sync.Mutex
	buf  []models.DiscoveredLink
	head int
	tail int
	size int
	seen map[string]bool
}

const defaultQueueCap = 256

// NewURLQueue creates an empty queue.
func NewURLQueue() *URLQueue {
	return &URLQueue{
		buf:  make([]models.DiscoveredLink, defaultQueueCap),
		seen: make(map[string]bool),
	}
}

// Push adds a link to the back of the queue.
// Duplicate URLs (already pushed before) are silently ignored.
func (q *URLQueue) Push(link models.DiscoveredLink) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.seen[link.URL] {
		return
	}
	q.seen[link.URL] = true

	if q.size == len(q.buf) {
		q.grow()
	}

	q.buf[q.tail] = link
	q.tail = (q.tail + 1) % len(q.buf)
	q.size++
}

// Pop removes and returns the front element.
// Returns false if the queue is empty.
func (q *URLQueue) Pop() (models.DiscoveredLink, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.size == 0 {
		return models.DiscoveredLink{}, false
	}

	link := q.buf[q.head]
	q.buf[q.head] = models.DiscoveredLink{} // zero out for GC
	q.head = (q.head + 1) % len(q.buf)
	q.size--

	return link, true
}

// Len returns the current number of items in the queue.
func (q *URLQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size
}

// Has reports whether a URL has ever been pushed into the queue.
func (q *URLQueue) Has(rawURL string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.seen[rawURL]
}

// grow doubles the ring buffer capacity. Must be called with lock held.
func (q *URLQueue) grow() {
	newCap := len(q.buf) * 2
	newBuf := make([]models.DiscoveredLink, newCap)

	// Linearize the ring into the new buffer
	if q.head < q.tail {
		copy(newBuf, q.buf[q.head:q.tail])
	} else {
		n := copy(newBuf, q.buf[q.head:])
		copy(newBuf[n:], q.buf[:q.tail])
	}

	q.buf = newBuf
	q.head = 0
	q.tail = q.size
}

// Reset clears the queue and the seen set entirely.
func (q *URLQueue) Reset() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.buf = make([]models.DiscoveredLink, defaultQueueCap)
	q.head = 0
	q.tail = 0
	q.size = 0
	q.seen = make(map[string]bool)
}
