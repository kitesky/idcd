package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigCmd_setKey(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"config", "set-key", "sk_test_abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("config set-key failed: %v", err)
	}

	cfgFile := filepath.Join(tmpDir, ".idcd", "config.yaml")
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "sk_test_abc123") {
		t.Errorf("API key not saved, got: %s", content)
	}
	if !strings.Contains(content, "api_key:") {
		t.Errorf("missing api_key field, got: %s", content)
	}
}

func TestConfigCmd_setKeyAndRead(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"config", "set-key", "sk_live_xyz9876"})
	if err := root.Execute(); err != nil {
		t.Fatalf("config set-key failed: %v", err)
	}

	key := loadConfig()
	if key != "sk_live_xyz9876" {
		t.Errorf("loadConfig returned %q, want sk_live_xyz9876", key)
	}
}

func TestConfigCmd_getEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	os.Unsetenv("IDCD_API_KEY")
	globalAPIKey = ""

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"config", "get"})
	if err := root.Execute(); err != nil {
		t.Fatalf("config get failed: %v", err)
	}

	if !strings.Contains(buf.String(), "not set") {
		t.Errorf("expected 'not set' output, got: %s", buf.String())
	}
}

func TestConfigCmd_filePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"config", "set-key", "sk_test_perm"})
	if err := root.Execute(); err != nil {
		t.Fatalf("config set-key failed: %v", err)
	}

	cfgFile := filepath.Join(tmpDir, ".idcd", "config.yaml")
	info, err := os.Stat(cfgFile)
	if err != nil {
		t.Fatalf("cannot stat config file: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("expected file mode 0600, got %o", mode)
	}
}
