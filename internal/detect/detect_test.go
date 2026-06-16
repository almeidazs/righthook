package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDetectsRepoAndManagers(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git", "hooks"))
	mustWrite(t, filepath.Join(root, ".git", "config"), "[core]\n\thooksPath = .githooks\n")
	mustMkdir(t, filepath.Join(root, ".githooks"))
	mustWrite(t, filepath.Join(root, ".githooks", "pre-commit"), "#!/bin/sh\n")
	mustWrite(t, filepath.Join(root, "package.json"), `{
  "packageManager": "pnpm@9.0.0",
  "scripts": {"lint":"eslint .","typecheck":"tsc --noEmit","test":"vitest"},
  "devDependencies": {
    "@biomejs/biome": "1.0.0",
    "eslint": "9.0.0",
    "@commitlint/cli": "19.0.0",
    "next": "15.0.0"
  },
  "lint-staged": {"*.ts":"eslint"}
}`)
	mustWrite(t, filepath.Join(root, "pnpm-workspace.yaml"), "packages:\n  - apps/*\n")
	mustWrite(t, filepath.Join(root, "lefthook.yml"), "pre-commit: {}\n")
	mustWrite(t, filepath.Join(root, "tsconfig.json"), "{}\n")

	got, err := Scan(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if got.RepoRoot != root {
		t.Fatalf("repo root mismatch: %s", got.RepoRoot)
	}
	if got.PackageManager != "pnpm" {
		t.Fatalf("package manager = %s", got.PackageManager)
	}
	if !got.ExistingHooks["pre-commit"] {
		t.Fatalf("expected pre-commit hook detection")
	}
	if !contains(got.LegacyManagers, "lefthook") || !contains(got.LegacyManagers, "lint-staged") {
		t.Fatalf("legacy managers missing: %+v", got.LegacyManagers)
	}
	if !got.Monorepo {
		t.Fatalf("expected monorepo detection")
	}
	if !contains(got.Tools, "biome") || !contains(got.Tools, "commitlint") {
		t.Fatalf("tools missing: %+v", got.Tools)
	}
}

func TestScanDoesNotInventPMForGoRepo(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git", "hooks"))
	mustWrite(t, filepath.Join(root, ".git", "config"), "[core]\n")
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/test\n\ngo 1.24.0\n")

	got, err := Scan(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if got.PackageManager != "" {
		t.Fatalf("expected empty package manager, got %q", got.PackageManager)
	}
	if !contains(got.PresetCandidates, "go") {
		t.Fatalf("expected go preset candidate, got %+v", got.PresetCandidates)
	}
}

func TestScanDetectsPythonTooling(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git", "hooks"))
	mustWrite(t, filepath.Join(root, ".git", "config"), "[core]\n")
	mustWrite(t, filepath.Join(root, "pyproject.toml"), `
[project]
name = "demo"
dependencies = ["fastapi"]

[tool.ruff]
line-length = 100

[tool.mypy]
python_version = "3.12"
`)

	got, err := Scan(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !contains(got.PresetCandidates, "python") {
		t.Fatalf("expected python preset candidate, got %+v", got.PresetCandidates)
	}
	if !contains(got.Tools, "ruff") || !contains(got.Tools, "mypy") {
		t.Fatalf("expected python tools, got %+v", got.Tools)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
