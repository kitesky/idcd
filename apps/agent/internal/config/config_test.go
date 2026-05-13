package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid config",
			config: Config{
				NodeID:     "node-123",
				GatewayURL: "https://gateway.example.com",
				SecretKey:  "secret-key-123",
				DataDir:    "/var/lib/idcd-agent",
			},
			expectErr: false,
		},
		{
			name: "missing node_id",
			config: Config{
				GatewayURL: "https://gateway.example.com",
				SecretKey:  "secret-key-123",
			},
			expectErr: true,
			errMsg:    "node_id is required",
		},
		{
			name: "missing gateway_url",
			config: Config{
				NodeID:    "node-123",
				SecretKey: "secret-key-123",
			},
			expectErr: true,
			errMsg:    "gateway_url is required",
		},
		{
			name: "missing secret_key",
			config: Config{
				NodeID:     "node-123",
				GatewayURL: "https://gateway.example.com",
			},
			expectErr: true,
			errMsg:    "secret_key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got none")
				} else if err.Error() != tt.errMsg {
					t.Errorf("Expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	config := Config{
		NodeID:     "node-123",
		GatewayURL: "https://gateway.example.com",
		SecretKey:  "secret-key-123",
		// DataDir, PollInterval, BatchSize intentionally omitted
	}

	err := config.validate()
	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	// Check that defaults are applied
	if config.DataDir != "/var/lib/idcd-agent" {
		t.Errorf("Expected default DataDir %q, got %q", "/var/lib/idcd-agent", config.DataDir)
	}

	if config.PollInterval != "30s" {
		t.Errorf("Expected default PollInterval %q, got %q", "30s", config.PollInterval)
	}

	if config.BatchSize != 100 {
		t.Errorf("Expected default BatchSize %d, got %d", 100, config.BatchSize)
	}
}

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	configContent := `
node_id: "test-node-123"
gateway_url: "https://test-gateway.example.com"
secret_key: "test-secret-key"
data_dir: "/tmp/test-agent"
poll_interval: "60s"
batch_size: 50
cert_path: "/etc/ssl/client.pem"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load and validate config
	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded values
	if cfg.NodeID != "test-node-123" {
		t.Errorf("Expected NodeID %q, got %q", "test-node-123", cfg.NodeID)
	}

	if cfg.GatewayURL != "https://test-gateway.example.com" {
		t.Errorf("Expected GatewayURL %q, got %q", "https://test-gateway.example.com", cfg.GatewayURL)
	}

	if cfg.SecretKey != "test-secret-key" {
		t.Errorf("Expected SecretKey %q, got %q", "test-secret-key", cfg.SecretKey)
	}

	if cfg.DataDir != "/tmp/test-agent" {
		t.Errorf("Expected DataDir %q, got %q", "/tmp/test-agent", cfg.DataDir)
	}

	if cfg.PollInterval != "60s" {
		t.Errorf("Expected PollInterval %q, got %q", "60s", cfg.PollInterval)
	}

	if cfg.BatchSize != 50 {
		t.Errorf("Expected BatchSize %d, got %d", 50, cfg.BatchSize)
	}

	if cfg.CertPath != "/etc/ssl/client.pem" {
		t.Errorf("Expected CertPath %q, got %q", "/etc/ssl/client.pem", cfg.CertPath)
	}
}

func TestLoadInvalidConfig(t *testing.T) {
	// Test non-existent file
	_, err := Load("/non/existent/config.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file, got none")
	}

	// Test invalid YAML
	tempDir := t.TempDir()
	invalidFile := filepath.Join(tempDir, "invalid.yaml")

	invalidContent := `
node_id: "test
gateway_url: [invalid yaml
`

	err = os.WriteFile(invalidFile, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	_, err = Load(invalidFile)
	if err == nil {
		t.Error("Expected error for invalid YAML, got none")
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("AGENT_NODE_ID", "env-node-456")
	os.Setenv("AGENT_DATA_DIR", "/env/data")
	os.Setenv("AGENT_GATEWAY_URL", "https://env-gateway.example.com")
	defer func() {
		os.Unsetenv("AGENT_NODE_ID")
		os.Unsetenv("AGENT_DATA_DIR")
		os.Unsetenv("AGENT_GATEWAY_URL")
	}()

	config := Config{
		NodeID:     "config-node-123",
		GatewayURL: "https://config-gateway.example.com",
		DataDir:    "/config/data",
		SecretKey:  "secret-key-123",
	}

	// Apply environment overrides
	config.applyEnvOverrides()

	// Check that environment variables override config values
	if config.NodeID != "env-node-456" {
		t.Errorf("Expected NodeID from env %q, got %q", "env-node-456", config.NodeID)
	}

	if config.DataDir != "/env/data" {
		t.Errorf("Expected DataDir from env %q, got %q", "/env/data", config.DataDir)
	}

	if config.GatewayURL != "https://env-gateway.example.com" {
		t.Errorf("Expected GatewayURL from env %q, got %q", "https://env-gateway.example.com", config.GatewayURL)
	}

	// SecretKey should not be overridden (no env var set)
	if config.SecretKey != "secret-key-123" {
		t.Errorf("Expected SecretKey to remain %q, got %q", "secret-key-123", config.SecretKey)
	}
}

func TestDefaultPath(t *testing.T) {
	// Test without environment variable
	os.Unsetenv("IDCD_CONFIG")
	path := DefaultPath()
	expectedDefault := "/etc/idcd-agent/config.yaml"

	if path != expectedDefault {
		t.Errorf("Expected default path %q, got %q", expectedDefault, path)
	}

	// Test with environment variable
	customPath := "/custom/config.yaml"
	os.Setenv("IDCD_CONFIG", customPath)
	defer os.Unsetenv("IDCD_CONFIG")

	path = DefaultPath()
	if path != customPath {
		t.Errorf("Expected custom path %q, got %q", customPath, path)
	}
}