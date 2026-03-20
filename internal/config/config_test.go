package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFrom_HappyPath(t *testing.T) {
	dir := t.TempDir()
	content := `
[agent-md]
crates_dir = "src"

[deps]
crates_dir = "src"

[deps.layers]
0 = ["base", "core"]
1 = ["app"]
`
	if err := os.WriteFile(filepath.Join(dir, ".devkit.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFrom(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AgentMD.CratesDir != "src" {
		t.Errorf("AgentMD.CratesDir = %q, want %q", cfg.AgentMD.CratesDir, "src")
	}
	if cfg.Deps.CratesDir != "src" {
		t.Errorf("Deps.CratesDir = %q, want %q", cfg.Deps.CratesDir, "src")
	}
	if len(cfg.Deps.Layers) != 2 {
		t.Errorf("len(Deps.Layers) = %d, want 2", len(cfg.Deps.Layers))
	}
}

func TestLoadFrom_WalksUp(t *testing.T) {
	root := t.TempDir()
	content := `[agent-md]
crates_dir = "crates"
`
	if err := os.WriteFile(filepath.Join(root, ".devkit.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	child := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(child, 0o750); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFrom(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AgentMD.CratesDir != "crates" {
		t.Errorf("AgentMD.CratesDir = %q, want %q", cfg.AgentMD.CratesDir, "crates")
	}
}

func TestLoadFrom_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := loadFrom(dir)
	if err == nil {
		t.Fatal("expected error for missing .devkit.toml")
	}
}

func TestLoadFrom_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".devkit.toml"), []byte("{{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := loadFrom(dir)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}
