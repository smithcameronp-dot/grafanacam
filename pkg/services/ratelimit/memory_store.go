package ratelimit

import (
	"context"
	"sync"
	"time"
)

// memoryCleanupEvery controls how frequently (in Allow calls) the store sweeps
// out entries that can no longer influence a decision. Doing it inline avoids a
// background goroutine while keeping the map bounded for churny key spaces such
// as per-IP keys.
const memoryCleanupEvery = 1024

type memoryEntry struct {
	// window is the fixed-window index (unix nanos / window size) that cur counts.
	window int64
	prev   int
	cur    int
}

// memoryStore is an in-process sliding-window rate limiter. Limits are enforced
// per Grafana instance, so in a multi-replica deployment the effective limit is
// roughly the configured limit multiplied by the number of replicas.
type memoryStore struct {
	mu      sync.Mutex
	entries map[string]*memoryEntry
	ops     int
	nowFn   func() time.Time
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		entries: make(map[string]*memoryEntry),
		nowFn:   time.Now,
	}
}

func (s *memoryStore) Allow(_ context.Context, key string, limit int, window time.Duration) (Result, error) {
	if limit <= 0 || window <= 0 {
		return Result{Allowed: true, Limit: limit}, nil
	}

	now := s.nowFn()
	windowNanos := window.Nanoseconds()
	idx := now.UnixNano() / windowNanos
	elapsed := time.Duration(now.UnixNano() % windowNanos)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ops++
	if s.ops >= memoryCleanupEvery {
		s.evictStale(idx)
		s.ops = 0
	}

	e, ok := s.entries[key]
	if !ok {
		e = &memoryEntry{window: idx}
		s.entries[key] = e
	}

	switch e.window {
	case idx:
		// Same window, counts still valid.
	case idx - 1:
		// Advanced by one window: the current becomes the previous.
		e.prev = e.cur
		e.cur = 0
		e.window = idx
	default:
		// Gap of two or more windows: previous counts no longer overlap.
		e.prev = 0
		e.cur = 0
		e.window = idx
	}

	res := decide(e.prev, e.cur, limit, window, elapsed)
	if res.Allowed {
		e.cur++
	}
	return res, nil
}

// evictStale removes entries whose windows are too old to affect any future
// decision. The caller must hold s.mu.
func (s *memoryStore) evictStale(currentIdx int64) {
	for k, e := range s.entries {
		if e.window < currentIdx-1 {
			delete(s.entries, k)
		}
	}
}
