package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMemoryStore_AllowWithinLimit(t *testing.T) {
	s := newMemoryStore()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		res, err := s.Allow(ctx, "ip", 5, time.Minute)
		require.NoError(t, err)
		require.True(t, res.Allowed, "request %d should be allowed", i)
		require.Equal(t, 5, res.Limit)
	}

	res, err := s.Allow(ctx, "ip", 5, time.Minute)
	require.NoError(t, err)
	require.False(t, res.Allowed)
	require.Equal(t, 0, res.Remaining)
	require.Greater(t, res.RetryAfter, time.Duration(0))
}

func TestMemoryStore_KeysAreIndependent(t *testing.T) {
	s := newMemoryStore()
	ctx := context.Background()

	res, err := s.Allow(ctx, "a", 1, time.Minute)
	require.NoError(t, err)
	require.True(t, res.Allowed)

	// Different key has its own budget.
	res, err = s.Allow(ctx, "b", 1, time.Minute)
	require.NoError(t, err)
	require.True(t, res.Allowed)

	res, err = s.Allow(ctx, "a", 1, time.Minute)
	require.NoError(t, err)
	require.False(t, res.Allowed)
}

func TestMemoryStore_WindowRollover(t *testing.T) {
	now := time.Unix(0, 0)
	s := newMemoryStore()
	s.nowFn = func() time.Time { return now }
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		res, err := s.Allow(ctx, "ip", 3, time.Minute)
		require.NoError(t, err)
		require.True(t, res.Allowed)
	}
	res, err := s.Allow(ctx, "ip", 3, time.Minute)
	require.NoError(t, err)
	require.False(t, res.Allowed)

	// Advance two full windows: previous counts no longer overlap.
	now = now.Add(2 * time.Minute)
	res, err = s.Allow(ctx, "ip", 3, time.Minute)
	require.NoError(t, err)
	require.True(t, res.Allowed)
}

func TestMemoryStore_SlidingBoundaryWeighting(t *testing.T) {
	now := time.Unix(0, 0)
	s := newMemoryStore()
	s.nowFn = func() time.Time { return now }
	ctx := context.Background()
	window := time.Minute

	// Fill the first window completely.
	for i := 0; i < 10; i++ {
		res, err := s.Allow(ctx, "ip", 10, window)
		require.NoError(t, err)
		require.True(t, res.Allowed)
	}

	// Move 30s into the next window. The previous window is weighted 0.5, so
	// its 10 requests count as 5, leaving room for exactly 5 more.
	now = time.Unix(90, 0)
	allowed := 0
	for i := 0; i < 10; i++ {
		res, err := s.Allow(ctx, "ip", 10, window)
		require.NoError(t, err)
		if res.Allowed {
			allowed++
		}
	}
	require.Equal(t, 5, allowed)
}

func TestMemoryStore_DisabledWhenNonPositive(t *testing.T) {
	s := newMemoryStore()
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		res, err := s.Allow(ctx, "ip", 0, time.Minute)
		require.NoError(t, err)
		require.True(t, res.Allowed)

		res, err = s.Allow(ctx, "ip", 10, 0)
		require.NoError(t, err)
		require.True(t, res.Allowed)
	}
}

func TestMemoryStore_Concurrency(t *testing.T) {
	s := newMemoryStore()
	ctx := context.Background()

	const limit = 100
	const workers = 1000

	var mu sync.Mutex
	allowed := 0
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// A one hour window ensures no rollover during the test.
			res, err := s.Allow(ctx, "ip", limit, time.Hour)
			require.NoError(t, err)
			if res.Allowed {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	require.Equal(t, limit, allowed)
}

func TestMemoryStore_EvictStale(t *testing.T) {
	s := newMemoryStore()

	s.entries["old"] = &memoryEntry{window: 0}
	s.entries["recent"] = &memoryEntry{window: 9}
	s.entries["current"] = &memoryEntry{window: 10}

	s.evictStale(10)

	_, hasOld := s.entries["old"]
	_, hasRecent := s.entries["recent"]
	_, hasCurrent := s.entries["current"]
	require.False(t, hasOld)
	require.True(t, hasRecent)
	require.True(t, hasCurrent)
}
