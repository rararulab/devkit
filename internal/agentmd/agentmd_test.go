package agentmd

import (
	"os"
	"path/filepath"
	"testing"
)

// setupCratesDir creates a temp directory with the given crates,
// optionally adding AGENT.md to each.
func setupCratesDir(t *testing.T, crates map[string]bool) string {
	t.Helper()
	dir := t.TempDir()
	cratesDir := filepath.Join(dir, "crates")
	if mkErr := os.MkdirAll(cratesDir, 0o750); mkErr != nil {
		t.Fatal(mkErr)
	}

	for name, hasAgentMD := range crates {
		crateDir := filepath.Join(cratesDir, name)
		if mkErr := os.MkdirAll(crateDir, 0o750); mkErr != nil {
			t.Fatal(mkErr)
		}
		if hasAgentMD {
			agentPath := filepath.Join(crateDir, "AGENT.md")
			if writeErr := os.WriteFile(agentPath, []byte("# Agent\n"), 0o600); writeErr != nil {
				t.Fatal(writeErr)
			}
		}
	}

	// Write a minimal .devkit.toml pointing to "crates"
	tomlContent := `[agent-md]
crates_dir = "crates"
`
	tomlPath := filepath.Join(dir, ".devkit.toml")
	if writeErr := os.WriteFile(tomlPath, []byte(tomlContent), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}

	return dir
}

func TestRunCheck_AllPresent(t *testing.T) {
	dir := setupCratesDir(t, map[string]bool{
		"base": true,
		"core": true,
		"app":  true,
	})

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if chdirErr := os.Chdir(dir); chdirErr != nil {
		t.Fatal(chdirErr)
	}

	if runErr := runCheck(); runErr != nil {
		t.Fatalf("expected no error when all AGENT.md present, got: %v", runErr)
	}
}

func TestRunCheck_SomeMissing(t *testing.T) {
	dir := setupCratesDir(t, map[string]bool{
		"base":    true,
		"core":    false, // missing
		"missing": false, // missing
	})

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if chdirErr := os.Chdir(dir); chdirErr != nil {
		t.Fatal(chdirErr)
	}

	runErr := runCheck()
	if runErr == nil {
		t.Fatal("expected error when AGENT.md files are missing")
	}
}

func TestRunCheck_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	cratesDir := filepath.Join(dir, "crates")
	if mkErr := os.MkdirAll(cratesDir, 0o750); mkErr != nil {
		t.Fatal(mkErr)
	}

	tomlPath := filepath.Join(dir, ".devkit.toml")
	if writeErr := os.WriteFile(tomlPath, []byte(`[agent-md]
crates_dir = "crates"
`), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if chdirErr := os.Chdir(dir); chdirErr != nil {
		t.Fatal(chdirErr)
	}

	if runErr := runCheck(); runErr != nil {
		t.Fatalf("expected no error for empty crates dir, got: %v", runErr)
	}
}
