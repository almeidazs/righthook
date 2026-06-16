package presets

import (
	"testing"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/detect"
)

func TestBuildRecommendedConfig(t *testing.T) {
	d := detect.Result{
		PackageManager:   "pnpm",
		PackageManagers:  []string{"pnpm"},
		Tools:            []string{"biome", "commitlint"},
		Languages:        []string{"typescript"},
		Scripts:          map[string]string{"typecheck": "pnpm typecheck", "test": "pnpm test"},
		FormatterChoices: []string{"biome"},
	}
	opts := cli.ResolvedInitOptions{Mode: cli.ModeRecommended, Monorepo: "off", Base: "origin/main"}
	rec, err := Build(d, opts)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, ok := rec.Config.Hooks["pre-commit"].Jobs["biome"]; !ok {
		t.Fatalf("expected biome job")
	}
	if _, ok := rec.Config.Hooks["commit-msg"].Jobs["commitlint"]; !ok {
		t.Fatalf("expected commitlint job")
	}
	if _, ok := rec.Config.Hooks["pre-push"].Jobs["typecheck"]; !ok {
		t.Fatalf("expected typecheck job")
	}
}

func TestBuildRecommendedConfigForGoRepo(t *testing.T) {
	d := detect.Result{
		Languages:        []string{"go"},
		PresetCandidates: []string{"go"},
		Scripts:          map[string]string{},
	}
	opts := cli.ResolvedInitOptions{Mode: cli.ModeRecommended, Monorepo: "off", Base: "origin/main"}
	rec, err := Build(d, opts)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, ok := rec.Config.Hooks["pre-commit"].Jobs["gofmt"]; !ok {
		t.Fatalf("expected gofmt job")
	}
	if _, ok := rec.Config.Hooks["pre-push"].Jobs["test"]; !ok {
		t.Fatalf("expected go test job")
	}
}

func TestBuildFailsOnAmbiguousPMInYesMode(t *testing.T) {
	d := detect.Result{
		PackageManagers: []string{"pnpm", "npm"},
	}
	opts := cli.ResolvedInitOptions{Mode: cli.ModeRecommended, Yes: true}
	if _, err := Build(d, opts); err == nil {
		t.Fatalf("expected ambiguity error")
	}
}

func TestBuildUsesGoPresetFallback(t *testing.T) {
	d := detect.Result{
		Languages:       []string{"go"},
		PackageManagers: []string{"pnpm"},
	}
	opts := cli.ResolvedInitOptions{Mode: cli.ModeStrict, Preset: "go", Monorepo: "off", Base: "origin/main"}
	rec, err := Build(d, opts)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, ok := rec.Config.Hooks["pre-push"].Jobs["test"]; !ok {
		t.Fatalf("expected go preset fallback test job")
	}
}

func TestBuildRecommendedConfigForPythonRepo(t *testing.T) {
	d := detect.Result{
		Languages:        []string{"python"},
		PresetCandidates: []string{"python"},
		Tools:            []string{"ruff", "mypy", "pytest"},
	}
	opts := cli.ResolvedInitOptions{Mode: cli.ModeRecommended, Monorepo: "off", Base: "origin/main"}
	rec, err := Build(d, opts)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, ok := rec.Config.Hooks["pre-commit"].Jobs["ruff-check"]; !ok {
		t.Fatalf("expected ruff check job")
	}
	if _, ok := rec.Config.Hooks["pre-push"].Jobs["typecheck"]; !ok {
		t.Fatalf("expected python typecheck job")
	}
	if _, ok := rec.Config.Hooks["pre-push"].Jobs["test"]; !ok {
		t.Fatalf("expected python test job")
	}
}
