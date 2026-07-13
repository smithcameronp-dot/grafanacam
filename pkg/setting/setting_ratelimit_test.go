package setting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/ini.v1"
)

func TestReadRateLimitingSettings(t *testing.T) {
	t.Run("defaults when section is absent", func(t *testing.T) {
		cfg := &Cfg{Raw: ini.Empty()}
		require.NoError(t, cfg.readRateLimitingSettings())

		require.False(t, cfg.RateLimiting.Enabled)
		require.Equal(t, 100, cfg.RateLimiting.Default.Requests)
		require.Equal(t, time.Minute, cfg.RateLimiting.Default.Window)
		require.Empty(t, cfg.RateLimiting.Routes)
	})

	t.Run("reads values and per-route overrides", func(t *testing.T) {
		raw := ini.Empty()
		sec, err := raw.NewSection("rate_limiting")
		require.NoError(t, err)
		_, err = sec.NewKey("enabled", "true")
		require.NoError(t, err)
		_, err = sec.NewKey("requests", "50")
		require.NoError(t, err)
		_, err = sec.NewKey("window", "30s")
		require.NoError(t, err)

		routes, err := raw.NewSection("rate_limiting.routes")
		require.NoError(t, err)
		_, err = routes.NewKey("login", "5:2m")
		require.NoError(t, err)
		_, err = routes.NewKey("password_reset", "3")
		require.NoError(t, err)

		cfg := &Cfg{Raw: raw}
		require.NoError(t, cfg.readRateLimitingSettings())

		require.True(t, cfg.RateLimiting.Enabled)
		require.Equal(t, 50, cfg.RateLimiting.Default.Requests)
		require.Equal(t, 30*time.Second, cfg.RateLimiting.Default.Window)

		require.Equal(t, RateLimitRule{Requests: 5, Window: 2 * time.Minute}, cfg.RateLimiting.Routes["login"])
		// Window omitted falls back to the global default window.
		require.Equal(t, RateLimitRule{Requests: 3, Window: 30 * time.Second}, cfg.RateLimiting.Routes["password_reset"])
	})

	t.Run("invalid window returns an error", func(t *testing.T) {
		raw := ini.Empty()
		sec, err := raw.NewSection("rate_limiting")
		require.NoError(t, err)
		_, err = sec.NewKey("window", "not-a-duration")
		require.NoError(t, err)

		cfg := &Cfg{Raw: raw}
		require.Error(t, cfg.readRateLimitingSettings())
	})

	t.Run("invalid route override returns an error", func(t *testing.T) {
		raw := ini.Empty()
		routes, err := raw.NewSection("rate_limiting.routes")
		require.NoError(t, err)
		_, err = routes.NewKey("login", "abc")
		require.NoError(t, err)

		cfg := &Cfg{Raw: raw}
		require.Error(t, cfg.readRateLimitingSettings())
	})
}

func TestParseRateLimitRule(t *testing.T) {
	defaultWindow := time.Minute

	t.Run("requests only", func(t *testing.T) {
		rule, err := parseRateLimitRule("10", defaultWindow)
		require.NoError(t, err)
		require.Equal(t, RateLimitRule{Requests: 10, Window: defaultWindow}, rule)
	})

	t.Run("requests and window", func(t *testing.T) {
		rule, err := parseRateLimitRule("10:15s", defaultWindow)
		require.NoError(t, err)
		require.Equal(t, RateLimitRule{Requests: 10, Window: 15 * time.Second}, rule)
	})

	t.Run("rejects non-integer requests", func(t *testing.T) {
		_, err := parseRateLimitRule("x:1m", defaultWindow)
		require.Error(t, err)
	})

	t.Run("rejects non-positive window", func(t *testing.T) {
		_, err := parseRateLimitRule("10:0s", defaultWindow)
		require.Error(t, err)
	})
}
