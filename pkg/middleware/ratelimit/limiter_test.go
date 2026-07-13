package ratelimit

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSlidingWindowLimiter_AllowsWithinLimit(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	limiter := NewSlidingWindowLimiter(NewMemoryStore())
	limiter.clock = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		decision := limiter.Allow("client", 3, time.Minute)
		require.True(t, decision.Allowed, "request %d", i+1)
		require.Equal(t, 3-i-1, decision.Remaining)
	}

	decision := limiter.Allow("client", 3, time.Minute)
	require.False(t, decision.Allowed)
	require.Equal(t, 0, decision.Remaining)
	require.Greater(t, decision.RetryAfter, time.Duration(0))
}

func TestSlidingWindowLimiter_SlidesWindow(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	current := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	limiter := NewSlidingWindowLimiter(store)
	limiter.clock = func() time.Time { return current }

	for i := 0; i < 2; i++ {
		require.True(t, limiter.Allow("client", 2, time.Second).Allowed)
	}

	require.False(t, limiter.Allow("client", 2, time.Second).Allowed)

	current = current.Add(1100 * time.Millisecond)
	require.True(t, limiter.Allow("client", 2, time.Second).Allowed)
}

func TestSlidingWindowLimiter_IndependentKeys(t *testing.T) {
	t.Parallel()

	limiter := NewSlidingWindowLimiter(NewMemoryStore())
	require.True(t, limiter.Allow("a", 1, time.Minute).Allowed)
	require.True(t, limiter.Allow("b", 1, time.Minute).Allowed)
	require.False(t, limiter.Allow("a", 1, time.Minute).Allowed)
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	now := time.Now().UTC()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Allow("key", now, time.Minute, 100)
		}()
	}

	wg.Wait()
}
