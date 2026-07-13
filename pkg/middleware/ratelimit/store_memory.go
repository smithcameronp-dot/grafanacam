package ratelimit

import (
	"sync"
	"time"
)

const defaultMaxKeys = 100_000

type memoryStore struct {
	mu      sync.Mutex
	entries map[string][]time.Time
	maxKeys int
}

// NewMemoryStore creates an in-memory sliding-window store.
func NewMemoryStore() LimiterStore {
	return &memoryStore{
		entries: make(map[string][]time.Time),
		maxKeys: defaultMaxKeys,
	}
}

func (s *memoryStore) Allow(key string, now time.Time, window time.Duration, limit int) (bool, int, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := now.Add(-window)
	timestamps := s.entries[key]

	filtered := timestamps[:0]
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}

	if len(filtered) >= limit {
		oldest := filtered[0]
		s.entries[key] = filtered
		return false, len(filtered), oldest
	}

	if len(filtered) == 0 {
		delete(s.entries, key)
	}

	if _, exists := s.entries[key]; !exists && len(s.entries) >= s.maxKeys {
		s.evictOneKey()
	}

	filtered = append(filtered, now)
	s.entries[key] = filtered
	return true, len(filtered), time.Time{}
}

func (s *memoryStore) Cleanup(olderThan time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, timestamps := range s.entries {
		filtered := timestamps[:0]
		for _, ts := range timestamps {
			if ts.After(olderThan) {
				filtered = append(filtered, ts)
			}
		}
		if len(filtered) == 0 {
			delete(s.entries, key)
			continue
		}
		s.entries[key] = filtered
	}
}

func (s *memoryStore) evictOneKey() {
	for key := range s.entries {
		delete(s.entries, key)
		return
	}
}
