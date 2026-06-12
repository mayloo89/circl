package ratelimit

import "sync"

// ConcurrentLimiter caps the number of simultaneous holders per key. It is
// in-memory by design: WebSocket connections live and die with this process,
// so a process-local count is always exact and cannot leak across restarts
// the way a Redis counter would if the process died between INCR and DECR.
type ConcurrentLimiter struct {
	mu     sync.Mutex
	counts map[string]int
	max    int
}

// NewConcurrentLimiter returns a limiter allowing up to max concurrent
// acquisitions per key.
func NewConcurrentLimiter(max int) *ConcurrentLimiter {
	return &ConcurrentLimiter{counts: make(map[string]int), max: max}
}

// Acquire reserves a slot for key. It returns false when the key already
// holds max slots; the caller must not call Release in that case.
func (l *ConcurrentLimiter) Acquire(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.counts[key] >= l.max {
		return false
	}
	l.counts[key]++
	return true
}

// Release frees a slot previously reserved with Acquire.
func (l *ConcurrentLimiter) Release(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.counts[key] <= 1 {
		delete(l.counts, key)
		return
	}
	l.counts[key]--
}
