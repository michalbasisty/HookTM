package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_EmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestLoad_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
forward: localhost:3000
port: 8080
db: /custom/path/hooks.db
lang: go
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Forward != "localhost:3000" {
		t.Fatalf("Forward=%q, want localhost:3000", cfg.Forward)
	}
	if cfg.Port != 8080 {
		t.Fatalf("Port=%d, want 8080", cfg.Port)
	}
	if cfg.DBPath != "/custom/path/hooks.db" {
		t.Fatalf("DBPath=%q, want /custom/path/hooks.db", cfg.DBPath)
	}
	if cfg.Lang != "go" {
		t.Fatalf("Lang=%q, want go", cfg.Lang)
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	_, err := Load("/non/existent/config.yaml")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
forward: [invalid
yaml content
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
