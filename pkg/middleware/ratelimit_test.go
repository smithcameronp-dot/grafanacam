package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/infra/log"
	contextmodel "github.com/grafana/grafana/pkg/services/contexthandler/model"
	"github.com/grafana/grafana/pkg/services/ratelimit"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/web"
)

func newStore() ratelimit.Store {
	// A nil remote cache with default (non-distributed) config yields the
	// in-process store.
	return ratelimit.ProvideService(&setting.Cfg{}, nil)
}

func rateLimitCfg(enabled bool, requests int, window time.Duration, routes map[string]setting.RateLimitRule) *setting.Cfg {
	return &setting.Cfg{
		RateLimiting: setting.RateLimitingSettings{
			Enabled: enabled,
			Default: setting.RateLimitRule{Requests: requests, Window: window},
			Routes:  routes,
		},
	}
}

func invokeRateLimit(handler web.Handler, method, path, ip string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Real-IP", ip)
	reqCtx := &contextmodel.ReqContext{
		Context: &web.Context{Req: req, Resp: web.NewResponseWriter(method, rec)},
		Logger:  log.New("test"),
	}
	h, ok := handler.(func(*contextmodel.ReqContext))
	if !ok {
		panic("rate limit handler has unexpected type")
	}
	h(reqCtx)
	return rec
}

func TestRateLimit_GlobalAllowsThenBlocks(t *testing.T) {
	cfg := rateLimitCfg(true, 2, time.Minute, nil)
	handler := RateLimit(newStore(), cfg)

	rec := invokeRateLimit(handler, http.MethodGet, "/api/foo", "1.2.3.4")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "2", rec.Header().Get("X-RateLimit-Limit"))
	require.Equal(t, "1", rec.Header().Get("X-RateLimit-Remaining"))

	rec = invokeRateLimit(handler, http.MethodGet, "/api/foo", "1.2.3.4")
	require.Equal(t, http.StatusOK, rec.Code)

	rec = invokeRateLimit(handler, http.MethodGet, "/api/foo", "1.2.3.4")
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.NotEmpty(t, rec.Header().Get("Retry-After"))
	require.Equal(t, "0", rec.Header().Get("X-RateLimit-Remaining"))
}

func TestRateLimit_PerIP(t *testing.T) {
	cfg := rateLimitCfg(true, 1, time.Minute, nil)
	handler := RateLimit(newStore(), cfg)

	require.Equal(t, http.StatusOK, invokeRateLimit(handler, http.MethodGet, "/api/foo", "1.1.1.1").Code)
	// Different IP has its own budget.
	require.Equal(t, http.StatusOK, invokeRateLimit(handler, http.MethodGet, "/api/foo", "2.2.2.2").Code)
	// First IP is now over the limit.
	require.Equal(t, http.StatusTooManyRequests, invokeRateLimit(handler, http.MethodGet, "/api/foo", "1.1.1.1").Code)
}

func TestRateLimit_BypassPaths(t *testing.T) {
	cfg := rateLimitCfg(true, 1, time.Minute, nil)
	handler := RateLimit(newStore(), cfg)

	for i := 0; i < 5; i++ {
		rec := invokeRateLimit(handler, http.MethodGet, "/api/health", "1.2.3.4")
		require.Equal(t, http.StatusOK, rec.Code)
		require.Empty(t, rec.Header().Get("X-RateLimit-Limit"))
	}
}

func TestRateLimit_NonAPIPathsNotLimited(t *testing.T) {
	cfg := rateLimitCfg(true, 1, time.Minute, nil)
	handler := RateLimit(newStore(), cfg)

	for i := 0; i < 5; i++ {
		rec := invokeRateLimit(handler, http.MethodGet, "/public/build/app.js", "1.2.3.4")
		require.Equal(t, http.StatusOK, rec.Code)
	}
}

func TestRateLimit_DisabledIsNoOp(t *testing.T) {
	cfg := rateLimitCfg(false, 1, time.Minute, nil)
	handler := RateLimit(newStore(), cfg)

	for i := 0; i < 5; i++ {
		rec := invokeRateLimit(handler, http.MethodGet, "/api/foo", "1.2.3.4")
		require.Equal(t, http.StatusOK, rec.Code)
		require.Empty(t, rec.Header().Get("X-RateLimit-Limit"))
	}
}

func TestRateLimitFor_UsesProvidedDefault(t *testing.T) {
	cfg := rateLimitCfg(true, 100, time.Minute, nil)
	handler := RateLimitFor(newStore(), cfg, "login", 1, time.Minute)

	require.Equal(t, http.StatusOK, invokeRateLimit(handler, http.MethodPost, "/login", "1.2.3.4").Code)
	require.Equal(t, http.StatusTooManyRequests, invokeRateLimit(handler, http.MethodPost, "/login", "1.2.3.4").Code)
}

func TestRateLimitFor_ConfigOverrideTakesPrecedence(t *testing.T) {
	cfg := rateLimitCfg(true, 100, time.Minute, map[string]setting.RateLimitRule{
		"login": {Requests: 3, Window: time.Minute},
	})
	// The code default of 1 is overridden by the config value of 3.
	handler := RateLimitFor(newStore(), cfg, "login", 1, time.Minute)

	for i := 0; i < 3; i++ {
		require.Equal(t, http.StatusOK, invokeRateLimit(handler, http.MethodPost, "/login", "1.2.3.4").Code)
	}
	require.Equal(t, http.StatusTooManyRequests, invokeRateLimit(handler, http.MethodPost, "/login", "1.2.3.4").Code)
}
