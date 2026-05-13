package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		checkFunc func(*testing.T, *Config)
	}{
		{
			name: "valid config with all fields",
			yaml: `
redis:
  addr: "localhost:6379"
  password: "secret"
  db: 0
database:
  dsn: "postgresql://user:pass@localhost/idcd"
leader:
  key: "scheduler:leader"
  ttl: 15s
worker:
  count: 8
`,
			wantErr: false,
			checkFunc: func(t *testing.T, c *Config) {
				if c.Redis.Addr != "localhost:6379" {
					t.Errorf("Redis.Addr = %q, want localhost:6379", c.Redis.Addr)
				}
				if c.Redis.Password != "secret" {
					t.Errorf("Redis.Password = %q, want secret", c.Redis.Password)
				}
				if c.Database.DSN != "postgresql://user:pass@localhost/idcd" {
					t.Errorf("Database.DSN = %q", c.Database.DSN)
				}
				if c.Leader.Key != "scheduler:leader" {
					t.Errorf("Leader.Key = %q", c.Leader.Key)
				}
				if c.Leader.TTL != 15*time.Second {
					t.Errorf("Leader.TTL = %v, want 15s", c.Leader.TTL)
				}
				if c.Worker.Count != 8 {
					t.Errorf("Worker.Count = %d, want 8", c.Worker.Count)
				}
			},
		},
		{
			name: "valid config with defaults",
			yaml: `
redis:
  addr: "localhost:6379"
database:
  dsn: "postgresql://user:pass@localhost/idcd"
`,
			wantErr: false,
			checkFunc: func(t *testing.T, c *Config) {
				if c.Leader.Key != "scheduler:leader" {
					t.Errorf("Leader.Key = %q, want default scheduler:leader", c.Leader.Key)
				}
				if c.Leader.TTL != 10*time.Second {
					t.Errorf("Leader.TTL = %v, want default 10s", c.Leader.TTL)
				}
				if c.Worker.Count != 4 {
					t.Errorf("Worker.Count = %d, want default 4", c.Worker.Count)
				}
			},
		},
		{
			name: "missing redis.addr",
			yaml: `
database:
  dsn: "postgresql://user:pass@localhost/idcd"
`,
			wantErr: true,
		},
		{
			name: "missing database.dsn",
			yaml: `
redis:
  addr: "localhost:6379"
`,
			wantErr: true,
		},
		{
			name: "invalid leader.ttl",
			yaml: `
redis:
  addr: "localhost:6379"
database:
  dsn: "postgresql://user:pass@localhost/idcd"
leader:
  ttl: 500ms
`,
			wantErr: true,
		},
		{
			name: "worker.count = 0 uses default",
			yaml: `
redis:
  addr: "localhost:6379"
database:
  dsn: "postgresql://user:pass@localhost/idcd"
worker:
  count: 0
`,
			wantErr: false,
			checkFunc: func(t *testing.T, c *Config) {
				if c.Worker.Count != 4 {
					t.Errorf("Worker.Count = %d, want default 4", c.Worker.Count)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			cfgPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(cfgPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}

			cfg, err := Load(cfgPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, cfg)
			}
		})
	}
}

func TestDefaultPath(t *testing.T) {
	// Save original env and restore after test
	origEnv := os.Getenv("IDCD_CONFIG")
	defer os.Setenv("IDCD_CONFIG", origEnv)

	t.Run("with env var", func(t *testing.T) {
		os.Setenv("IDCD_CONFIG", "/custom/path/config.yaml")
		path := DefaultPath()
		if path != "/custom/path/config.yaml" {
			t.Errorf("DefaultPath() = %q, want /custom/path/config.yaml", path)
		}
	})

	t.Run("without env var", func(t *testing.T) {
		os.Unsetenv("IDCD_CONFIG")
		path := DefaultPath()
		if path != "config/dev.env.yaml" {
			t.Errorf("DefaultPath() = %q, want config/dev.env.yaml", path)
		}
	})
}
