package setting

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RateLimitRule defines a request budget: at most Requests requests per Window.
type RateLimitRule struct {
	Requests int
	Window   time.Duration
}

// RateLimitingSettings configures HTTP API rate limiting.
type RateLimitingSettings struct {
	// Enabled turns rate limiting on. It is off by default so existing
	// deployments are unaffected until explicitly opted in.
	Enabled bool
	// Default applies to all /api requests unless a per-route override matches.
	Default RateLimitRule
	// Routes maps a route override name (as passed to middleware.RateLimitFor)
	// to its budget, allowing custom limits for specific routes.
	Routes map[string]RateLimitRule
}

// readRateLimitingSettings reads the [rate_limiting] section and any per-route
// overrides from the [rate_limiting.routes] section. Route override values use
// the form "requests" or "requests:window", e.g. "5" or "5:1m"; when the window
// is omitted the global default window is used.
func (cfg *Cfg) readRateLimitingSettings() error {
	sec := cfg.Raw.Section("rate_limiting")

	cfg.RateLimiting.Enabled = sec.Key("enabled").MustBool(false)

	window, err := time.ParseDuration(valueAsString(sec, "window", "1m"))
	if err != nil {
		return fmt.Errorf("invalid rate_limiting.window: %w", err)
	}
	if window <= 0 {
		return fmt.Errorf("rate_limiting.window must be positive, got %s", window)
	}

	cfg.RateLimiting.Default = RateLimitRule{
		Requests: sec.Key("requests").MustInt(100),
		Window:   window,
	}

	cfg.RateLimiting.Routes = map[string]RateLimitRule{}
	for _, key := range cfg.Raw.Section("rate_limiting.routes").Keys() {
		rule, err := parseRateLimitRule(key.Value(), cfg.RateLimiting.Default.Window)
		if err != nil {
			return fmt.Errorf("invalid rate_limiting.routes.%s: %w", key.Name(), err)
		}
		cfg.RateLimiting.Routes[key.Name()] = rule
	}

	return nil
}

func parseRateLimitRule(value string, defaultWindow time.Duration) (RateLimitRule, error) {
	requestsStr, windowStr, hasWindow := strings.Cut(strings.TrimSpace(value), ":")

	requests, err := strconv.Atoi(strings.TrimSpace(requestsStr))
	if err != nil {
		return RateLimitRule{}, fmt.Errorf("requests must be an integer: %w", err)
	}

	window := defaultWindow
	if hasWindow {
		window, err = time.ParseDuration(strings.TrimSpace(windowStr))
		if err != nil {
			return RateLimitRule{}, fmt.Errorf("window must be a duration: %w", err)
		}
		if window <= 0 {
			return RateLimitRule{}, fmt.Errorf("window must be positive, got %s", window)
		}
	}

	return RateLimitRule{Requests: requests, Window: window}, nil
}
