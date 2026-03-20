package deps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildLayerMap_ValidLayers(t *testing.T) {
	layers := map[string][]string{
		"0": {"base", "core"},
		"1": {"app", "server"},
		"2": {"cli"},
	}
	m := buildLayerMap(layers)

	tests := []struct {
		crate string
		layer int
	}{
		{"base", 0},
		{"core", 0},
		{"app", 1},
		{"server", 1},
		{"cli", 2},
	}
	for _, tt := range tests {
		got, ok := m[tt.crate]
		if !ok {
			t.Errorf("crate %q not in layer map", tt.crate)
			continue
		}
		if got != tt.layer {
			t.Errorf("m[%q] = %d, want %d", tt.crate, got, tt.layer)
		}
	}
}

func TestBuildLayerMap_NonNumericKeysSkipped(t *testing.T) {
	layers := map[string][]string{
		"0":       {"base"},
		"invalid": {"bad"},
	}
	m := buildLayerMap(layers)

	if _, ok := m["bad"]; ok {
		t.Error("non-numeric layer key should be skipped, but 'bad' is in the map")
	}
	if _, ok := m["base"]; !ok {
		t.Error("'base' should be in the map")
	}
}

func TestBuildLayerMap_Empty(t *testing.T) {
	m := buildLayerMap(nil)
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}

func TestParseCrateDeps_ValidCargo(t *testing.T) {
	dir := t.TempDir()
	cargoToml := filepath.Join(dir, "Cargo.toml")
	content := `[package]
name = "my-crate"

[dependencies]
base = { workspace = true }
serde = "1.0"
core = { workspace = true }
`
	if err := os.WriteFile(cargoToml, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	workspaceCrates := map[string]bool{
		"base": true,
		"core": true,
	}

	name, deps, err := parseCrateDeps(cargoToml, workspaceCrates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "my-crate" {
		t.Errorf("name = %q, want %q", name, "my-crate")
	}
	if len(deps) != 2 {
		t.Errorf("len(deps) = %d, want 2", len(deps))
	}
}

func TestParseCrateDeps_NoDeps(t *testing.T) {
	dir := t.TempDir()
	cargoToml := filepath.Join(dir, "Cargo.toml")
	content := `[package]
name = "standalone"
`
	if err := os.WriteFile(cargoToml, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	name, deps, err := parseCrateDeps(cargoToml, map[string]bool{"base": true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "standalone" {
		t.Errorf("name = %q, want %q", name, "standalone")
	}
	if len(deps) != 0 {
		t.Errorf("expected no deps, got %d", len(deps))
	}
}

func TestParseCrateDeps_MissingPackageName(t *testing.T) {
	dir := t.TempDir()
	cargoToml := filepath.Join(dir, "Cargo.toml")
	content := `[dependencies]
base = { workspace = true }
`
	if err := os.WriteFile(cargoToml, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := parseCrateDeps(cargoToml, map[string]bool{"base": true})
	if err == nil {
		t.Fatal("expected error for missing package name")
	}
}

func TestParseCrateDeps_FileNotFound(t *testing.T) {
	_, _, err := parseCrateDeps("/nonexistent/Cargo.toml", nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFindWorkspaceRoot_NotFound(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	if chdirErr := os.Chdir(dir); chdirErr != nil {
		t.Fatal(chdirErr)
	}

	_, findErr := findWorkspaceRoot()
	if findErr == nil {
		t.Fatal("expected error when no workspace Cargo.toml exists")
	}
}
