package middleware

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	contextmodel "github.com/grafana/grafana/pkg/services/contexthandler/model"
	"github.com/grafana/grafana/pkg/services/ratelimit"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/web"
)

// rateLimitBypassPaths are never rate limited so that health checks and metrics
// scraping keep working even when a client is otherwise over its limit.
var rateLimitBypassPaths = map[string]bool{
	"/healthz":    true,
	"/api/health": true,
	"/metrics":    true,
	"/robots.txt": true,
}

// RateLimit returns middleware that applies the global default limit to all
// /api requests, keyed per client IP. It is intended to be mounted globally.
// Non-API paths and the bypass paths above are not limited.
func RateLimit(store ratelimit.Store, cfg *setting.Cfg) web.Handler {
	rule := cfg.RateLimiting.Default
	return func(c *contextmodel.ReqContext) {
		if !cfg.RateLimiting.Enabled {
			return
		}
		path := c.Req.URL.Path
		if !strings.HasPrefix(path, "/api") || rateLimitBypassPaths[path] {
			return
		}
		applyRateLimit(c, store, "global:"+c.RemoteAddr(), rule.Requests, rule.Window)
	}
}

// RateLimitFor returns middleware for a single route with its own budget. The
// name identifies an optional override in the [rate_limiting.routes] config
// section; when absent the provided defaults are used. Unlike RateLimit this is
// attached to specific routes, so it applies regardless of path prefix.
func RateLimitFor(store ratelimit.Store, cfg *setting.Cfg, name string, requests int, window time.Duration) web.Handler {
	if override, ok := cfg.RateLimiting.Routes[name]; ok {
		requests = override.Requests
		window = override.Window
	}
	return func(c *contextmodel.ReqContext) {
		if !cfg.RateLimiting.Enabled {
			return
		}
		if rateLimitBypassPaths[c.Req.URL.Path] {
			return
		}
		applyRateLimit(c, store, name+":"+c.RemoteAddr(), requests, window)
	}
}

// applyRateLimit performs the limit check and, when exceeded, writes a 429
// response which short-circuits the handler chain. It fails open: if the limiter
// backend errors the request is allowed through rather than blocked.
func applyRateLimit(c *contextmodel.ReqContext, store ratelimit.Store, key string, requests int, window time.Duration) {
	if store == nil || requests <= 0 || window <= 0 {
		return
	}

	res, err := store.Allow(c.Req.Context(), key, requests, window)
	if err != nil {
		c.Logger.Warn("Rate limit check failed, allowing request", "error", err)
		return
	}

	setRateLimitHeaders(c, res)

	if !res.Allowed {
		c.Resp.Header().Set("Retry-After", strconv.Itoa(secondsCeil(res.RetryAfter)))
		c.JsonApiErr(http.StatusTooManyRequests, "Rate limit exceeded", nil)
	}
}

func setRateLimitHeaders(c *contextmodel.ReqContext, res ratelimit.Result) {
	h := c.Resp.Header()
	h.Set("X-RateLimit-Limit", strconv.Itoa(res.Limit))
	h.Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
	h.Set("X-RateLimit-Reset", strconv.Itoa(secondsCeil(res.Reset)))
}

func secondsCeil(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int(math.Ceil(d.Seconds()))
}
