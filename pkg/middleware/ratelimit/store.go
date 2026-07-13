package ratelimit

import "time"

// Decision is the result of a rate limit check.
type Decision struct {
	Allowed      bool
	Limit        int
	Remaining    int
	RetryAfter   time.Duration
	ResetAt      time.Time
	CurrentCount int
}

// Limiter enforces sliding-window rate limits.
type Limiter interface {
	Allow(key string, limit int, window time.Duration) Decision
}

// LimiterStore persists request timestamps for sliding-window counting.
type LimiterStore interface {
	Allow(key string, now time.Time, window time.Duration, limit int) (allowed bool, count int, oldest time.Time)
	Cleanup(olderThan time.Time)
}
