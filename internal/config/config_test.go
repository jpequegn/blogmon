// internal/config/config_test.go
package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()

	if cfg.Fetch.Concurrency != 5 {
		t.Errorf("expected concurrency 5, got %d", cfg.Fetch.Concurrency)
	}
	if cfg.Fetch.TimeoutSeconds != 30 {
		t.Errorf("expected timeout 30, got %d", cfg.Fetch.TimeoutSeconds)
	}
}

func TestConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("BLOGMON_HOME", tmpDir)
	defer os.Unsetenv("BLOGMON_HOME")

	dir := Dir()
	if dir != tmpDir {
		t.Errorf("expected %s, got %s", tmpDir, dir)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("BLOGMON_HOME", tmpDir)
	defer os.Unsetenv("BLOGMON_HOME")

	cfg := Default()
	cfg.Fetch.Concurrency = 10

	if err := Save(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.Fetch.Concurrency != 10 {
		t.Errorf("expected concurrency 10, got %d", loaded.Fetch.Concurrency)
	}
}
