package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/setting"
)

func TestMatcherBypassAndScope(t *testing.T) {
	t.Parallel()

	matcher := NewMatcher(testCfg(setting.RateLimitingSettings{
		Enabled:     true,
		BypassPaths: []string{"/custom"},
	}))

	require.True(t, matcher.ShouldBypass("/healthz"))
	require.True(t, matcher.ShouldBypass("/api/health"))
	require.True(t, matcher.ShouldBypass("/livez"))
	require.True(t, matcher.ShouldBypass("/readyz"))
	require.True(t, matcher.ShouldBypass("/metrics"))
	require.True(t, matcher.ShouldBypass("/custom"))
	require.False(t, matcher.ShouldBypass("/api/folders"))

	require.True(t, matcher.InScope("/api/folders"))
	require.True(t, matcher.InScope("/apis/folder.grafana.app/v1beta1/namespaces/default/folders"))
	require.False(t, matcher.InScope("/login"))
	require.False(t, matcher.InScope("/public/build/app.js"))
}

func TestMatcherResolveLongestPrefixAndMethods(t *testing.T) {
	t.Parallel()

	matcher := NewMatcher(testCfg(setting.RateLimitingSettings{
		DefaultLimit:  100,
		DefaultWindow: time.Minute,
		Rules: []setting.RateLimitRule{
			{
				Name:   "api",
				Path:   "/api",
				Limit:  50,
				Window: time.Minute,
			},
			{
				Name:    "ds_query",
				Path:    "/api/ds/query",
				Methods: []string{"POST"},
				Limit:   10,
				Window:  30 * time.Second,
			},
		},
	}))

	defaultRule := matcher.Resolve("/api/search", "GET")
	require.Equal(t, "api", defaultRule.Name)
	require.Equal(t, 50, defaultRule.Limit)

	specificRule := matcher.Resolve("/api/ds/query", "POST")
	require.Equal(t, "ds_query", specificRule.Name)
	require.Equal(t, 10, specificRule.Limit)
	require.Equal(t, 30*time.Second, specificRule.Window)

	fallbackRule := matcher.Resolve("/api/ds/query", "GET")
	require.Equal(t, "api", fallbackRule.Name)
	require.Equal(t, 50, fallbackRule.Limit)
}

func TestMatcherDefaultRule(t *testing.T) {
	t.Parallel()

	matcher := NewMatcher(testCfg(setting.RateLimitingSettings{
		DefaultLimit:  25,
		DefaultWindow: 2 * time.Minute,
	}))

	rule := matcher.Resolve("/api/annotations", "GET")
	require.Equal(t, defaultRuleName, rule.Name)
	require.Equal(t, 25, rule.Limit)
	require.Equal(t, 2*time.Minute, rule.Window)
}
