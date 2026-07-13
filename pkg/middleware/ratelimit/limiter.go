package ratelimit

import (
	"time"
)

// SlidingWindowLimiter enforces per-key sliding-window limits.
type SlidingWindowLimiter struct {
	store LimiterStore
	clock func() time.Time
}

// NewSlidingWindowLimiter creates a limiter backed by the given store.
func NewSlidingWindowLimiter(store LimiterStore) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		store: store,
		clock: time.Now,
	}
}

func (l *SlidingWindowLimiter) Allow(key string, limit int, window time.Duration) Decision {
	now := l.clock().UTC()
	allowed, count, oldest := l.store.Allow(key, now, window, limit)

	decision := Decision{
		Allowed:      allowed,
		Limit:        limit,
		CurrentCount: count,
	}

	if allowed {
		decision.Remaining = limit - count
		if decision.Remaining < 0 {
			decision.Remaining = 0
		}
		decision.ResetAt = now.Add(window)
		return decision
	}

	decision.Remaining = 0
	if !oldest.IsZero() {
		resetAt := oldest.Add(window)
		decision.ResetAt = resetAt
		retryAfter := resetAt.Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}
		decision.RetryAfter = retryAfter
	}

	return decision
}
