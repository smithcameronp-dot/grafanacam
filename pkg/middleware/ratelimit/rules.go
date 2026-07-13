package ratelimit

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/grafana/grafana/pkg/setting"
)

const defaultRuleName = "default"

var builtInBypassPaths = []string{
	"/healthz",
	"/api/health",
	"/livez",
	"/readyz",
	"/metrics",
}

// ResolvedRule is the effective limit for a request.
type ResolvedRule struct {
	Name   string
	Path   string
	Limit  int
	Window time.Duration
}

// Matcher resolves bypass checks and per-route limits.
type Matcher struct {
	enabled       bool
	defaultLimit  int
	defaultWindow time.Duration
	bypassPaths   map[string]struct{}
	rules         []RateLimitRuleView
}

// RateLimitRuleView is a read-only view of a configured rule.
type RateLimitRuleView struct {
	Name    string
	Path    string
	Methods map[string]struct{}
	Limit   int
	Window  time.Duration
}

// NewMatcher builds a matcher from Grafana configuration.
func NewMatcher(cfg *setting.Cfg) *Matcher {
	rl := cfg.RateLimiting
	bypass := make(map[string]struct{}, len(builtInBypassPaths)+len(rl.BypassPaths))
	for _, path := range builtInBypassPaths {
		bypass[normalizePath(path)] = struct{}{}
	}
	for _, path := range rl.BypassPaths {
		bypass[normalizePath(path)] = struct{}{}
	}

	rules := make([]RateLimitRuleView, 0, len(rl.Rules))
	for _, rule := range rl.Rules {
		methods := make(map[string]struct{}, len(rule.Methods))
		for _, method := range rule.Methods {
			methods[strings.ToUpper(method)] = struct{}{}
		}
		rules = append(rules, RateLimitRuleView{
			Name:    rule.Name,
			Path:    normalizePath(rule.Path),
			Methods: methods,
			Limit:   rule.Limit,
			Window:  rule.Window,
		})
	}

	sort.Slice(rules, func(i, j int) bool {
		return len(rules[i].Path) > len(rules[j].Path)
	})

	return &Matcher{
		enabled:       rl.Enabled,
		defaultLimit:  rl.DefaultLimit,
		defaultWindow: rl.DefaultWindow,
		bypassPaths:   bypass,
		rules:         rules,
	}
}

func (m *Matcher) Enabled() bool {
	return m.enabled
}

func (m *Matcher) ShouldBypass(path string) bool {
	return m.isBypassPath(normalizePath(path))
}

func (m *Matcher) InScope(path string) bool {
	path = normalizePath(path)
	return strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/apis/")
}

func (m *Matcher) Resolve(path, method string) ResolvedRule {
	path = normalizePath(path)
	method = strings.ToUpper(method)

	for _, rule := range m.rules {
		if !strings.HasPrefix(path, rule.Path) {
			continue
		}
		if len(rule.Methods) > 0 {
			if _, ok := rule.Methods[method]; !ok {
				continue
			}
		}
		return ResolvedRule{
			Name:   rule.Name,
			Path:   rule.Path,
			Limit:  rule.Limit,
			Window: rule.Window,
		}
	}

	return ResolvedRule{
		Name:   defaultRuleName,
		Path:   path,
		Limit:  m.defaultLimit,
		Window: m.defaultWindow,
	}
}

func (m *Matcher) isBypassPath(path string) bool {
	_, ok := m.bypassPaths[path]
	return ok
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimRight(path, "/")
	}
	return path
}

// ScopeFromRequest returns the request path used for matching.
func ScopeFromRequest(r *http.Request) (path, method string) {
	return r.URL.Path, r.Method
}
