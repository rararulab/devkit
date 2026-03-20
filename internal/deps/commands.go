// Package deps enforces crate dependency direction rules.
//
// The workspace crates are organized into layers (0 = lowest, N = highest).
// A crate at layer N must NOT depend on a crate at layer N+1 or higher.
// Layer assignments are read from .devkit.toml.
package deps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/urfave/cli/v3"

	"github.com/rararulab/devkit/internal/config"
)

// Cmd returns the top-level "check-deps" command.
func Cmd() *cli.Command {
	return &cli.Command{
		Name:  "check-deps",
		Usage: "Check crate dependency direction rules",
		Action: func(_ context.Context, _ *cli.Command) error {
			return runCheckDeps()
		},
	}
}

// violation records a single dependency direction breach.
type violation struct {
	From      string
	FromLayer int
	To        string
	ToLayer   int
}

func (v violation) String() string {
	return fmt.Sprintf("%s (layer %d) -> %s (layer %d)", v.From, v.FromLayer, v.To, v.ToLayer)
}

func runCheckDeps() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Build layer map from config
	layerMap := buildLayerMap(cfg.Deps.Layers)
	if len(layerMap) == 0 {
		return fmt.Errorf("no layer definitions found in .devkit.toml [deps.layers]")
	}

	// Find the workspace root by looking for the root Cargo.toml
	root, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("finding workspace root: %w", err)
	}

	fmt.Printf("Workspace root: %s\n", root)

	// Parse workspace dependency aliases from root Cargo.toml
	aliases, err := parseWorkspaceAliases(filepath.Join(root, "Cargo.toml"))
	if err != nil {
		return fmt.Errorf("parsing workspace aliases: %w", err)
	}

	// Find all crate Cargo.toml files
	cratesDir := cfg.Deps.CratesDir
	if cratesDir == "" {
		cratesDir = "crates"
	}
	crateTomlFiles, err := findCrateTomlFiles(root, cratesDir)
	if err != nil {
		return fmt.Errorf("finding crate Cargo.toml files: %w", err)
	}

	var violations []violation
	var unknownCrates []string

	for _, tomlPath := range crateTomlFiles {
		pkgName, deps, err := parseCrateDeps(tomlPath, aliases)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", tomlPath, err)
			continue
		}

		fromLayer, known := layerMap[pkgName]
		if !known {
			unknownCrates = append(unknownCrates, pkgName)
			continue
		}

		for _, dep := range deps {
			toLayer, depKnown := layerMap[dep]
			if !depKnown {
				continue
			}
			if toLayer > fromLayer {
				violations = append(violations, violation{
					From:      pkgName,
					FromLayer: fromLayer,
					To:        dep,
					ToLayer:   toLayer,
				})
			}
		}
	}

	// Report unknown crates
	if len(unknownCrates) > 0 {
		sort.Strings(unknownCrates)
		fmt.Println("\nWarning: crates not in layer map (add them to .devkit.toml [deps.layers]):")
		for _, c := range unknownCrates {
			fmt.Printf("  - %s\n", c)
		}
	}

	// Report violations
	if len(violations) > 0 {
		sort.Slice(violations, func(i, j int) bool {
			return violations[i].String() < violations[j].String()
		})
		fmt.Println("\nDependency direction violations found:")
		for _, v := range violations {
			fmt.Printf("  ERROR: %s\n", v)
		}
		fmt.Printf("\n%d violation(s) found. A lower-layer crate must not depend on a higher-layer crate.\n", len(violations))
		return fmt.Errorf("dependency check failed with %d violation(s)", len(violations))
	}

	fmt.Println("\nAll dependency direction checks passed.")
	return nil
}

// buildLayerMap converts the config layers (map of string keys to crate name
// slices) into a flat crate-name → layer-number map.
func buildLayerMap(layers map[string][]string) map[string]int {
	m := make(map[string]int)
	for key, crates := range layers {
		layer, err := strconv.Atoi(key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: ignoring non-numeric layer key %q\n", key)
			continue
		}
		for _, c := range crates {
			m[c] = layer
		}
	}
	return m
}

// workspaceProbe is a minimal struct to detect whether a Cargo.toml
// contains a [workspace] section.
type workspaceProbe struct {
	Workspace *struct{} `toml:"workspace"`
}

// findWorkspaceRoot walks up from cwd to find the directory containing
// a Cargo.toml with [workspace].
func findWorkspaceRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "Cargo.toml")
		if data, err := os.ReadFile(candidate); err == nil {
			var probe workspaceProbe
			if err := toml.Unmarshal(data, &probe); err == nil && probe.Workspace != nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no workspace Cargo.toml found")
		}
		dir = parent
	}
}

// cargoWorkspace is used to decode the root Cargo.toml.
type cargoWorkspace struct {
	Workspace struct {
		Dependencies map[string]any `toml:"dependencies"`
	} `toml:"workspace"`
}

// parseWorkspaceAliases extracts workspace crate names from the root
// Cargo.toml by looking for [workspace.dependencies] entries that
// have a `path` field (i.e. local workspace crates, not external deps).
func parseWorkspaceAliases(rootToml string) (map[string]bool, error) {
	data, err := os.ReadFile(rootToml)
	if err != nil {
		return nil, err
	}

	var ws cargoWorkspace
	if err := toml.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", rootToml, err)
	}

	aliases := make(map[string]bool)
	for name, val := range ws.Workspace.Dependencies {
		if tbl, ok := val.(map[string]any); ok {
			if _, hasPath := tbl["path"]; hasPath {
				aliases[name] = true
			}
		}
	}

	return aliases, nil
}

// findCrateTomlFiles finds all Cargo.toml files in the crates directory
// and the api/ directory.
func findCrateTomlFiles(root, cratesDir string) ([]string, error) {
	var files []string

	cratesPath := filepath.Join(root, cratesDir)
	err := filepath.Walk(cratesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "Cargo.toml" && !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Also check api/
	apiToml := filepath.Join(root, "api", "Cargo.toml")
	if _, err := os.Stat(apiToml); err == nil {
		files = append(files, apiToml)
	}

	return files, nil
}

// crateCargo is used to decode a crate-level Cargo.toml.
type crateCargo struct {
	Package struct {
		Name string `toml:"name"`
	} `toml:"package"`
	Dependencies      map[string]any `toml:"dependencies"`
	BuildDependencies map[string]any `toml:"build-dependencies"`
	// dev-dependencies are intentionally excluded: they don't affect the
	// runtime dependency graph, so a dev-only import of a higher-layer
	// crate (e.g. a test helper) should not count as a layer violation.
}

// parseCrateDeps extracts the package name and workspace crate dependencies
// from a crate's Cargo.toml file.
func parseCrateDeps(tomlPath string, workspaceCrates map[string]bool) (name string, deps []string, err error) {
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return "", nil, err
	}

	var crate crateCargo
	if err := toml.Unmarshal(data, &crate); err != nil {
		return "", nil, fmt.Errorf("parsing %s: %w", tomlPath, err)
	}

	if crate.Package.Name == "" {
		return "", nil, fmt.Errorf("no package name found in %s", tomlPath)
	}

	for _, section := range []map[string]any{crate.Dependencies, crate.BuildDependencies} {
		for depName, val := range section {
			if !workspaceCrates[depName] {
				continue
			}
			if tbl, ok := val.(map[string]any); ok {
				if ws, exists := tbl["workspace"]; exists {
					if b, ok := ws.(bool); ok && b {
						deps = append(deps, depName)
					}
				}
			}
		}
	}

	return crate.Package.Name, deps, nil
}
