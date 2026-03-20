# devkit — Developer toolkit for rara

GO := env("GO", "go")
GOLANGCI_LINT := env("GOLANGCI_LINT", "golangci-lint")
GOIMPORTS := env("GOIMPORTS", "goimports")

BIN_DIR := env("BIN_DIR", "bin")
CLI_NAME := "devkit"
MODULE_NAME := "github.com/rararulab/devkit"

# Version information
GIT_TAG := trim(`git describe --tags --exact-match 2>/dev/null || echo ""`)
VERSION_FROM_FILE := trim(`grep -o 'Version = ".*"' version.go | cut -d'"' -f2 2>/dev/null || echo "dev"`)
VERSION := if GIT_TAG != "" { trim_start_match(GIT_TAG, "v") } else { VERSION_FROM_FILE }
GIT_COMMIT := trim(`git rev-parse --short HEAD 2>/dev/null || echo "unknown"`)
BUILD_TIME := trim(`date -u '+%Y-%m-%d_%H:%M:%S'`)

LDFLAGS := "-s -w -X main.version=" + VERSION + " -X main.gitCommit=" + GIT_COMMIT + " -X main.buildTime=" + BUILD_TIME

# ========================================================================================
# Help
# ========================================================================================

[group("Help")]
[private]
default:
    @just --list --list-heading 'devkit justfile manual page:\n'

[doc("show help")]
[group("Help")]
help: default

[doc("show version information")]
[group("Help")]
version:
    @echo "Version:    {{ VERSION }}"
    @echo "Git Tag:    {{ GIT_TAG }}"
    @echo "Git Commit: {{ GIT_COMMIT }}"
    @echo "Build Time: {{ BUILD_TIME }}"

[doc("update version.go based on latest git tag")]
[group("Help")]
version-update:
    #!/usr/bin/env bash
    set -euo pipefail
    LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    if [ -z "$LATEST_TAG" ]; then
        echo "Error: No git tags found. Create a tag first with: git tag v0.1.0"
        exit 1
    fi
    VERSION="${LATEST_TAG#v}"
    echo "Updating version.go to: $VERSION (from tag: $LATEST_TAG)"
    sed -i.bak "s/const Version = \".*\"/const Version = \"$VERSION\"/" version.go
    rm version.go.bak
    echo "Done!"

[doc("create a new git tag and update version.go")]
[group("Help")]
version-tag VERSION_NUM:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ ! "{{ VERSION_NUM }}" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-.*)?$ ]]; then
        echo "Error: Invalid version format. Use: x.y.z or x.y.z-suffix"
        exit 1
    fi
    TAG="v{{ VERSION_NUM }}"
    echo "Creating git tag: $TAG"
    git tag -a "$TAG" -m "Release $TAG"
    echo "Updating version.go..."
    sed -i.bak "s/const Version = \".*\"/const Version = \"{{ VERSION_NUM }}\"/" version.go
    rm version.go.bak
    echo "Done! Next steps:"
    echo "  1. git add version.go && git commit -m 'chore: bump version to {{ VERSION_NUM }}'"
    echo "  2. git push origin $TAG"

# ========================================================================================
# Code Quality
# ========================================================================================

[doc("format code")]
[group("Code Quality")]
fmt:
    @echo "Formatting code..."
    find . -name "*.go" ! -path "./vendor/*" -exec gofmt -w -s {} +
    find . -name "*.go" ! -path "./vendor/*" -exec {{ GOIMPORTS }} -w -local {{ MODULE_NAME }} {} +
    @echo "Done!"

[doc("check code formatting")]
[group("Code Quality")]
fmt-check:
    @echo "Checking formatting..."
    @test -z "$(find . -name '*.go' ! -path './vendor/*' -exec gofmt -l {} +)" || (echo "Error: Code is not formatted. Run 'just fmt'" && exit 1)
    @echo "Done!"

[doc("run golangci-lint")]
[group("Code Quality")]
lint:
    @echo "Running linter..."
    {{ GOLANGCI_LINT }} run --timeout 5m
    @echo "Done!"

alias l := lint

[doc("run golangci-lint with auto-fix")]
[group("Code Quality")]
lint-fix:
    @echo "Running linter with auto-fix..."
    {{ GOLANGCI_LINT }} run --fix --timeout 5m
    @echo "Done!"

[doc("run fmt and lint-fix")]
[group("Code Quality")]
fix: fmt lint-fix

[doc("run all quality checks")]
[group("Code Quality")]
check: fmt-check lint
    {{ GO }} vet ./...
    @echo "Done!"

alias c := check

# ========================================================================================
# Testing
# ========================================================================================

[doc("run tests")]
[group("Testing")]
test:
    @echo "Running tests..."
    {{ GO }} test -v -race -cover ./...
    @echo "Done!"

alias t := test

[doc("run tests with coverage report")]
[group("Testing")]
test-coverage:
    @echo "Running tests with coverage..."
    {{ GO }} test -v -race -coverprofile=coverage.out -covermode=atomic ./...
    {{ GO }} tool cover -html=coverage.out -o coverage.html
    @echo "Done: coverage.html generated!"

# ========================================================================================
# Build
# ========================================================================================

[doc("build devkit binary")]
[group("Build")]
build:
    @echo "Building devkit (v{{ VERSION }})..."
    mkdir -p {{ BIN_DIR }}
    {{ GO }} build -v -ldflags="{{ LDFLAGS }}" -o {{ BIN_DIR }}/{{ CLI_NAME }} .
    @echo "Done: {{ BIN_DIR }}/{{ CLI_NAME }}"

[doc("build release binaries for all platforms")]
[group("Build")]
build-release:
    @echo "Building release binaries (v{{ VERSION }})..."
    mkdir -p {{ BIN_DIR }}
    just build-platform linux amd64
    just build-platform darwin amd64
    just build-platform darwin arm64
    @echo "Done!"
    @ls -lh {{ BIN_DIR }}/

[group("Build")]
[private]
build-platform os arch:
    CGO_ENABLED=0 GOOS={{ os }} GOARCH={{ arch }} {{ GO }} build -ldflags="{{ LDFLAGS }}" -o {{ BIN_DIR }}/{{ CLI_NAME }}-{{ os }}-{{ arch }} .

[doc("install devkit globally via go install")]
[group("Build")]
install:
    @echo "Installing devkit..."
    {{ GO }} install -ldflags="{{ LDFLAGS }}" .
    @echo "Done!"

# ========================================================================================
# Maintenance
# ========================================================================================

[doc("clean build artifacts")]
[group("Maintenance")]
clean:
    @echo "Cleaning..."
    rm -rf {{ BIN_DIR }}/
    rm -rf coverage*.out coverage*.html
    @echo "Done!"

[doc("tidy and verify dependencies")]
[group("Maintenance")]
tidy:
    @echo "Tidying dependencies..."
    {{ GO }} mod tidy
    {{ GO }} mod verify
    @echo "Done!"

[doc("update all dependencies")]
[group("Maintenance")]
update:
    @echo "Updating dependencies..."
    {{ GO }} get -u ./...
    {{ GO }} mod tidy
    @echo "Done!"

# ========================================================================================
# Development
# ========================================================================================

[doc("initialize development environment")]
[group("Development")]
init:
    @echo "Installing development tools..."
    {{ GO }} install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    {{ GO }} install golang.org/x/tools/cmd/goimports@latest
    @echo "Downloading dependencies..."
    {{ GO }} mod download
    @echo "Done!"

[doc("run all pre-commit checks")]
[group("Development")]
pre-commit: fmt lint test
    @echo "All pre-commit checks passed!"

[doc("simulate CI pipeline")]
[group("Development")]
ci: clean tidy fmt-check lint test
    @echo "CI simulation passed!"

# ========================================================================================
# Release
# ========================================================================================

[doc("preview changelog for unreleased changes")]
[group("Release")]
changelog:
    @command -v git-cliff >/dev/null 2>&1 || (echo "Error: git-cliff not found. Install: brew install git-cliff" && exit 1)
    git-cliff --unreleased

[doc("generate full changelog")]
[group("Release")]
changelog-full:
    @command -v git-cliff >/dev/null 2>&1 || (echo "Error: git-cliff not found. Install: brew install git-cliff" && exit 1)
    git-cliff -o CHANGELOG.md
    @echo "Done: CHANGELOG.md generated!"

# ========================================================================================
# Info
# ========================================================================================

[doc("show project statistics")]
[group("Info")]
stats:
    @echo "Go files:"
    @find . -name "*.go" ! -path "./vendor/*" | wc -l
    @echo "Lines of code:"
    @find . -name "*.go" ! -path "./vendor/*" -exec cat {} \; | wc -l
    @echo "Packages:"
    @{{ GO }} list ./... | wc -l
