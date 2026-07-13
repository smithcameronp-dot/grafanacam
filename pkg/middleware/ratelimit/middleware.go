package ratelimit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/web"
)

// Service wires together rate limiting dependencies.
type Service struct {
	matcher *Matcher
	limiter Limiter
	metrics *Metrics
}

// ProvideService creates a rate limiting service from configuration.
func ProvideService(cfg *setting.Cfg, registerer prometheus.Registerer) *Service {
	return &Service{
		matcher: NewMatcher(cfg),
		limiter: NewSlidingWindowLimiter(NewMemoryStore()),
		metrics: NewMetrics(registerer),
	}
}

// Middleware returns HTTP middleware that enforces per-IP sliding-window limits.
func (s *Service) Middleware() web.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !s.matcher.Enabled() {
				next.ServeHTTP(w, r)
				return
			}

			path, method := ScopeFromRequest(r)
			if s.matcher.ShouldBypass(path) {
				next.ServeHTTP(w, r)
				return
			}

			if !s.matcher.InScope(path) {
				next.ServeHTTP(w, r)
				return
			}

			rule := s.matcher.Resolve(path, method)
			clientIP := web.RemoteAddr(r)
			key := fmt.Sprintf("%s:%s:%s", clientIP, rule.Name, rule.Path)

			decision := s.limiter.Allow(key, rule.Limit, rule.Window)
			s.metrics.Observe(rule.Name, decision.Allowed)

			if decision.Allowed {
				setRateLimitHeaders(w, decision)
				next.ServeHTTP(w, r)
				return
			}

			writeRateLimitResponse(w, decision)
		})
	}
}

func setRateLimitHeaders(w http.ResponseWriter, decision Decision) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(decision.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(decision.Remaining))
	if !decision.ResetAt.IsZero() {
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.Unix(), 10))
	}
}

func writeRateLimitResponse(w http.ResponseWriter, decision Decision) {
	headers := w.Header()
	headers.Set("Content-Type", "application/json; charset=UTF-8")
	setRateLimitHeaders(w, decision)

	if decision.RetryAfter > 0 {
		seconds := int(decision.RetryAfter.Round(time.Second) / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		headers.Set("Retry-After", strconv.Itoa(seconds))
	}

	body, err := json.Marshal(map[string]string{
		"message": "Rate limit exceeded",
	})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}

	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write(body)
}
