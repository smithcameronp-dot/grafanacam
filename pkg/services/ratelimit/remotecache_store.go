package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/grafana/grafana/pkg/infra/remotecache"
)

const remoteCacheKeyPrefix = "ratelimit:"

// remoteCacheStore is a sliding-window rate limiter backed by the remote cache.
// Because limits are stored centrally they are shared across all Grafana
// replicas, unlike the in-process store.
type remoteCacheStore struct {
	cache remotecache.CacheStorage
	nowFn func() time.Time
}

func newRemoteCacheStore(cache remotecache.CacheStorage) *remoteCacheStore {
	return &remoteCacheStore{cache: cache, nowFn: time.Now}
}

func (s *remoteCacheStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (Result, error) {
	if limit <= 0 || window <= 0 {
		return Result{Allowed: true, Limit: limit}, nil
	}

	now := s.nowFn()
	windowNanos := window.Nanoseconds()
	idx := now.UnixNano() / windowNanos
	elapsed := time.Duration(now.UnixNano() % windowNanos)

	cur, err := s.get(ctx, key, idx)
	if err != nil {
		return Result{}, err
	}
	prev, err := s.get(ctx, key, idx-1)
	if err != nil {
		return Result{}, err
	}

	res := decide(prev, cur, limit, window, elapsed)
	if res.Allowed {
		// Best-effort increment: the remote cache exposes no atomic increment,
		// so a small amount of under-counting is possible under heavy
		// concurrency. This is acceptable for coarse-grained rate limiting. The
		// TTL spans two windows so stale counters expire on their own.
		if err := s.set(ctx, key, idx, cur+1, 2*window); err != nil {
			return Result{}, err
		}
	}
	return res, nil
}

func (s *remoteCacheStore) cacheKey(key string, idx int64) string {
	return fmt.Sprintf("%s%s:%d", remoteCacheKeyPrefix, key, idx)
}

func (s *remoteCacheStore) get(ctx context.Context, key string, idx int64) (int, error) {
	b, err := s.cache.Get(ctx, s.cacheKey(key, idx))
	if err != nil {
		if errors.Is(err, remotecache.ErrCacheItemNotFound) {
			return 0, nil
		}
		return 0, err
	}

	n, err := strconv.Atoi(string(b))
	if err != nil {
		// Treat a corrupt value as an empty window rather than failing closed.
		return 0, nil
	}
	return n, nil
}

func (s *remoteCacheStore) set(ctx context.Context, key string, idx int64, val int, ttl time.Duration) error {
	return s.cache.Set(ctx, s.cacheKey(key, idx), []byte(strconv.Itoa(val)), ttl)
}
