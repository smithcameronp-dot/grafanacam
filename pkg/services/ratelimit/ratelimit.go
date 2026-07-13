// Package ratelimit provides per-key request rate limiting using a sliding
// window counter. It offers an in-process store and a distributed store backed
// by Grafana's remote cache so limits can be shared across replicas.
package ratelimit

import (
	"context"
	"math"
	"time"

	"github.com/grafana/grafana/pkg/infra/remotecache"
	"github.com/grafana/grafana/pkg/setting"
)

// Result is the outcome of a rate limit check for a single request.
type Result struct {
	// Allowed reports whether the request may proceed.
	Allowed bool
	// Limit is the maximum number of requests permitted within the window.
	Limit int
	// Remaining is the approximate number of requests still permitted in the
	// current window after accounting for this request.
	Remaining int
	// RetryAfter is how long the caller should wait before retrying. It is only
	// meaningful when Allowed is false.
	RetryAfter time.Duration
	// Reset is the time until the current fixed window rolls over.
	Reset time.Duration
}

// Store tracks request counts per key and decides whether a request is allowed.
// Implementations must be safe for concurrent use.
type Store interface {
	// Allow records a request for the given key and reports whether it is within
	// the limit for the provided window. The current request is included in the
	// decision. A limit or window of zero (or less) disables limiting and always
	// allows the request.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (Result, error)
}

// ProvideService returns a rate limit Store. When Grafana is configured with a
// distributed remote cache (redis or memcached) the limiter is backed by that
// cache so limits are shared across replicas; otherwise it falls back to an
// in-process store which limits per instance.
func ProvideService(cfg *setting.Cfg, remoteCache *remotecache.RemoteCache) Store {
	if cfg.RemoteCacheOptions != nil {
		switch cfg.RemoteCacheOptions.Name {
		case "redis", "memcached":
			return newRemoteCacheStore(remoteCache)
		}
	}
	return newMemoryStore()
}

// decide computes the sliding-window decision from the request counts in the
// current and previous fixed windows. estimated is the weighted count of
// requests already recorded (excluding the current request): the previous
// window is weighted by the fraction of it still overlapping the sliding
// window, and the current window is counted in full.
func decide(prev, cur, limit int, window, elapsed time.Duration) Result {
	ratio := 0.0
	if window > 0 {
		ratio = float64(window-elapsed) / float64(window)
		if ratio < 0 {
			ratio = 0
		}
	}

	estimated := float64(prev)*ratio + float64(cur)
	res := Result{
		Allowed: estimated < float64(limit),
		Limit:   limit,
		Reset:   window - elapsed,
	}

	if res.Allowed {
		// Account for the current request when reporting the remaining budget.
		remaining := limit - int(math.Floor(estimated)) - 1
		if remaining < 0 {
			remaining = 0
		}
		res.Remaining = remaining
	} else {
		res.RetryAfter = window - elapsed
		if res.RetryAfter <= 0 {
			res.RetryAfter = window
		}
	}

	return res
}
