package setting

import (
	"fmt"
	"strings"
	"time"
)

const (
	rateLimitingSectionName = "rate_limiting"
	rateLimitingRulePrefix  = "rate_limiting.rule."
)

// RateLimitRule defines a per-route rate limit override.
type RateLimitRule struct {
	Name    string
	Path    string
	Methods []string
	Limit   int
	Window  time.Duration
}

// RateLimitingSettings configures HTTP API rate limiting.
type RateLimitingSettings struct {
	Enabled       bool
	DefaultLimit  int
	DefaultWindow time.Duration
	BypassPaths   []string
	Rules         []RateLimitRule
}

func (cfg *Cfg) readRateLimitingSettings() error {
	section := cfg.Raw.Section(rateLimitingSectionName)
	cfg.RateLimiting.Enabled = section.Key("enabled").MustBool(false)
	cfg.RateLimiting.DefaultLimit = section.Key("default_limit").MustInt(100)
	cfg.RateLimiting.DefaultWindow = section.Key("default_window").MustDuration(time.Minute)

	bypassPaths := strings.TrimSpace(section.Key("bypass_paths").String())
	if bypassPaths != "" {
		cfg.RateLimiting.BypassPaths = splitTrim(bypassPaths, ",")
	}

	rules := make([]RateLimitRule, 0)
	seenNames := make(map[string]struct{})

	for _, iniSection := range cfg.Raw.Sections() {
		sectionName := iniSection.Name()
		if !strings.HasPrefix(sectionName, rateLimitingRulePrefix) {
			continue
		}

		name := strings.TrimPrefix(sectionName, rateLimitingRulePrefix)
		if name == "" {
			return fmt.Errorf("invalid rate limiting rule section %q: missing rule name", sectionName)
		}
		if _, exists := seenNames[name]; exists {
			return fmt.Errorf("duplicate rate limiting rule name %q", name)
		}
		seenNames[name] = struct{}{}

		path := strings.TrimSpace(iniSection.Key("path").String())
		if path == "" {
			return fmt.Errorf("rate limiting rule %q: path must be set", name)
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		limit := iniSection.Key("limit").MustInt(0)
		if limit <= 0 {
			return fmt.Errorf("rate limiting rule %q: limit must be positive", name)
		}

		window := iniSection.Key("window").MustDuration(time.Minute)
		if window <= 0 {
			return fmt.Errorf("rate limiting rule %q: window must be positive", name)
		}

		methodsStr := strings.TrimSpace(iniSection.Key("methods").String())
		var methods []string
		if methodsStr != "" {
			for _, method := range splitTrim(methodsStr, ",") {
				methods = append(methods, strings.ToUpper(method))
			}
		}

		rules = append(rules, RateLimitRule{
			Name:    name,
			Path:    path,
			Methods: methods,
			Limit:   limit,
			Window:  window,
		})
	}

	if cfg.RateLimiting.DefaultLimit <= 0 {
		return fmt.Errorf("rate_limiting.default_limit must be positive")
	}
	if cfg.RateLimiting.DefaultWindow <= 0 {
		return fmt.Errorf("rate_limiting.default_window must be positive")
	}

	cfg.RateLimiting.Rules = rules
	return nil
}
