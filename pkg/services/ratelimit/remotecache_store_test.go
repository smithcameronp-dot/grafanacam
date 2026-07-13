package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/infra/remotecache"
)

type fakeCache struct {
	mu sync.Mutex
	m  map[string][]byte
}

func newFakeCache() *fakeCache {
	return &fakeCache{m: map[string][]byte{}}
}

func (f *fakeCache) Get(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.m[key]
	if !ok {
		return nil, remotecache.ErrCacheItemNotFound
	}
	return v, nil
}

func (f *fakeCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[key] = value
	return nil
}

func (f *fakeCache) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, key)
	return nil
}

func TestRemoteCacheStore_AllowWithinLimit(t *testing.T) {
	s := newRemoteCacheStore(newFakeCache())
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		res, err := s.Allow(ctx, "ip", 3, time.Minute)
		require.NoError(t, err)
		require.True(t, res.Allowed, "request %d should be allowed", i)
	}

	res, err := s.Allow(ctx, "ip", 3, time.Minute)
	require.NoError(t, err)
	require.False(t, res.Allowed)
	require.Greater(t, res.RetryAfter, time.Duration(0))
}

func TestRemoteCacheStore_WindowRollover(t *testing.T) {
	now := time.Unix(0, 0)
	s := newRemoteCacheStore(newFakeCache())
	s.nowFn = func() time.Time { return now }
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		res, err := s.Allow(ctx, "ip", 2, time.Minute)
		require.NoError(t, err)
		require.True(t, res.Allowed)
	}
	res, err := s.Allow(ctx, "ip", 2, time.Minute)
	require.NoError(t, err)
	require.False(t, res.Allowed)

	// Two windows later the earlier counts no longer apply.
	now = now.Add(2 * time.Minute)
	res, err = s.Allow(ctx, "ip", 2, time.Minute)
	require.NoError(t, err)
	require.True(t, res.Allowed)
}

func TestRemoteCacheStore_SlidingBoundaryWeighting(t *testing.T) {
	now := time.Unix(0, 0)
	s := newRemoteCacheStore(newFakeCache())
	s.nowFn = func() time.Time { return now }
	ctx := context.Background()
	window := time.Minute

	for i := 0; i < 10; i++ {
		res, err := s.Allow(ctx, "ip", 10, window)
		require.NoError(t, err)
		require.True(t, res.Allowed)
	}

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

func TestRemoteCacheStore_DisabledWhenNonPositive(t *testing.T) {
	s := newRemoteCacheStore(newFakeCache())
	ctx := context.Background()

	res, err := s.Allow(ctx, "ip", 0, time.Minute)
	require.NoError(t, err)
	require.True(t, res.Allowed)

	res, err = s.Allow(ctx, "ip", 10, 0)
	require.NoError(t, err)
	require.True(t, res.Allowed)
}

func TestRemoteCacheStore_CorruptValueTreatedAsEmpty(t *testing.T) {
	cache := newFakeCache()
	s := newRemoteCacheStore(cache)
	ctx := context.Background()

	// Seed the current window counter with a non-numeric value.
	idx := s.nowFn().UnixNano() / time.Minute.Nanoseconds()
	require.NoError(t, cache.Set(ctx, s.cacheKey("ip", idx), []byte("not-a-number"), time.Minute))

	res, err := s.Allow(ctx, "ip", 1, time.Minute)
	require.NoError(t, err)
	require.True(t, res.Allowed)
}
