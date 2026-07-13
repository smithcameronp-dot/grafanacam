package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/setting"
)

func testCfg(rl setting.RateLimitingSettings) *setting.Cfg {
	return &setting.Cfg{RateLimiting: rl}
}

func TestMiddlewareRateLimitsAPIRequests(t *testing.T) {
	t.Parallel()

	cfg := testCfg(setting.RateLimitingSettings{
		Enabled:       true,
		DefaultLimit:  2,
		DefaultWindow: time.Minute,
	})

	service := ProvideService(cfg, prometheus.NewRegistry())
	handler := service.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
		req.RemoteAddr = "192.168.1.10:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "request %d", i+1)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	req.RemoteAddr = "192.168.1.10:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "2", rec.Header().Get("X-RateLimit-Limit"))
	require.Equal(t, "0", rec.Header().Get("X-RateLimit-Remaining"))
	require.NotEmpty(t, rec.Header().Get("Retry-After"))
}

func TestMiddlewareBypassesHealthChecks(t *testing.T) {
	t.Parallel()

	cfg := testCfg(setting.RateLimitingSettings{
		Enabled:       true,
		DefaultLimit:  1,
		DefaultWindow: time.Minute,
	})

	service := ProvideService(cfg, prometheus.NewRegistry())
	handler := service.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	paths := []string{"/healthz", "/api/health", "/livez", "/readyz", "/metrics"}
	for _, path := range paths {
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.RemoteAddr = "192.168.1.10:1234"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, "path %s request %d", path, i+1)
		}
	}
}

func TestMiddlewareSkipsNonAPIRoutes(t *testing.T) {
	t.Parallel()

	cfg := testCfg(setting.RateLimitingSettings{
		Enabled:       true,
		DefaultLimit:  1,
		DefaultWindow: time.Minute,
	})

	service := ProvideService(cfg, prometheus.NewRegistry())
	handler := service.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		req.RemoteAddr = "192.168.1.10:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	}
}

func TestMiddlewarePerRouteRule(t *testing.T) {
	t.Parallel()

	cfg := testCfg(setting.RateLimitingSettings{
		Enabled:       true,
		DefaultLimit:  100,
		DefaultWindow: time.Minute,
		Rules: []setting.RateLimitRule{
			{
				Name:    "ds_query",
				Path:    "/api/ds/query",
				Methods: []string{"POST"},
				Limit:   1,
				Window:  time.Minute,
			},
		},
	})

	service := ProvideService(cfg, prometheus.NewRegistry())
	handler := service.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/ds/query", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodPost, "/api/ds/query", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/search", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestMiddlewareDisabled(t *testing.T) {
	t.Parallel()

	cfg := testCfg(setting.RateLimitingSettings{
		Enabled:       false,
		DefaultLimit:  1,
		DefaultWindow: time.Minute,
	})

	service := ProvideService(cfg, prometheus.NewRegistry())
	handler := service.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
		req.RemoteAddr = "192.168.1.10:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	}
}
