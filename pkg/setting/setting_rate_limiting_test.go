package setting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	ini "gopkg.in/ini.v1"
)

func TestReadRateLimitingSettings(t *testing.T) {
	t.Parallel()

	iniFile, err := ini.Load([]byte(`
[rate_limiting]
enabled = true
default_limit = 50
default_window = 30s
bypass_paths = /custom/health, /other

[rate_limiting.rule.ds_query]
path = /api/ds/query
methods = POST,GET
limit = 10
window = 1m

[rate_limiting.rule.login_ping]
path = api/login/ping
methods = GET
limit = 5
window = 10s
`))
	require.NoError(t, err)

	cfg := &Cfg{Raw: iniFile}
	require.NoError(t, cfg.readRateLimitingSettings())

	require.True(t, cfg.RateLimiting.Enabled)
	require.Equal(t, 50, cfg.RateLimiting.DefaultLimit)
	require.Equal(t, 30*time.Second, cfg.RateLimiting.DefaultWindow)
	require.Equal(t, []string{"/custom/health", "/other"}, cfg.RateLimiting.BypassPaths)
	require.Len(t, cfg.RateLimiting.Rules, 2)

	require.Equal(t, RateLimitRule{
		Name:    "ds_query",
		Path:    "/api/ds/query",
		Methods: []string{"POST", "GET"},
		Limit:   10,
		Window:  time.Minute,
	}, cfg.RateLimiting.Rules[0])

	require.Equal(t, RateLimitRule{
		Name:    "login_ping",
		Path:    "/api/login/ping",
		Methods: []string{"GET"},
		Limit:   5,
		Window:  10 * time.Second,
	}, cfg.RateLimiting.Rules[1])
}

func TestReadRateLimitingSettingsValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "invalid default limit",
			config: `
[rate_limiting]
default_limit = 0
`,
			wantErr: "default_limit must be positive",
		},
		{
			name: "missing rule path",
			config: `
[rate_limiting]
[rate_limiting.rule.bad]
limit = 1
window = 1m
`,
			wantErr: `path must be set`,
		},
		{
			name: "invalid rule limit",
			config: `
[rate_limiting]
[rate_limiting.rule.bad]
path = /api/a
limit = 0
window = 1m
`,
			wantErr: `limit must be positive`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			iniFile, err := ini.Load([]byte(tt.config))
			require.NoError(t, err)

			cfg := &Cfg{Raw: iniFile}
			err = cfg.readRateLimitingSettings()
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
