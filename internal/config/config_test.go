package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		wantErr string
		check   func(t *testing.T, cfg Config)
	}{
		{
			name: "valid minimal config applies defaults",
			yaml: `targets:
  - name: ups-main
    base_url: https://192.0.2.10
    username: admin
    password: secret
`,
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				if cfg.ListenAddr != defaultListenAddr {
					t.Fatalf("ListenAddr = %q, want %q", cfg.ListenAddr, defaultListenAddr)
				}
				if cfg.ScrapeTimeout.Duration != defaultScrapeTimeout {
					t.Fatalf("ScrapeTimeout = %s, want %s", cfg.ScrapeTimeout.Duration, defaultScrapeTimeout)
				}
				if cfg.APIVersion != defaultAPIVersion {
					t.Fatalf("APIVersion = %q, want %q", cfg.APIVersion, defaultAPIVersion)
				}
			},
		},
		{
			name: "valid explicit config",
			yaml: `listen_addr: ":9999"
scrape_timeout: 3s
api_version: "2.0"
targets:
  - name: ups-main
    base_url: http://127.0.0.1:8080
    username: admin
    password: secret
    insecure_skip_verify: true
`,
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				if cfg.ScrapeTimeout.Duration != 3*time.Second {
					t.Fatalf("ScrapeTimeout = %s, want 3s", cfg.ScrapeTimeout.Duration)
				}
				if !cfg.Targets[0].InsecureSkipVerify {
					t.Fatalf("InsecureSkipVerify = false, want true")
				}
			},
		},
		{
			name:    "missing targets",
			yaml:    `listen_addr: ":9734"`,
			wantErr: "at least one target",
		},
		{
			name: "duplicate target",
			yaml: `targets:
  - name: ups
    base_url: https://192.0.2.10
    username: admin
    password: secret
  - name: ups
    base_url: https://192.0.2.11
    username: admin
    password: secret
`,
			wantErr: "duplicate target name",
		},
		{
			name: "bad url",
			yaml: `targets:
  - name: ups
    base_url: ftp://192.0.2.10
    username: admin
    password: secret
`,
			wantErr: "scheme must be http or https",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			cfg, err := Load(path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}
