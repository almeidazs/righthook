package presets

import (
	"fmt"
	"strings"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/config"
	"github.com/almeidazs/righthook/internal/detect"
)

type Recommendation struct {
	Config            config.File         `json:"config"`
	SelectedHooks     []string            `json:"selected_hooks"`
	SelectedJobs      map[string][]string `json:"selected_jobs"`
	Warnings          []string            `json:"warnings,omitempty"`
	UpdateGitIgnore   bool                `json:"update_gitignore"`
	SelectedFormatter string              `json:"selected_formatter,omitempty"`
	SelectedPM        string              `json:"selected_pm,omitempty"`
}

func Build(d detect.Result, opts cli.ResolvedInitOptions) (Recommendation, error) {
	pm := opts.PM
	if pm == "" {
		if len(d.PackageManagers) > 1 {
			return Recommendation{}, fmt.Errorf("multiple package managers detected (%s); rerun with --pm <manager>", strings.Join(d.PackageManagers, ", "))
		}
		pm = d.PackageManager
	}

	formatter := ""
	if len(d.FormatterChoices) > 1 && opts.Yes {
		return Recommendation{}, fmt.Errorf("multiple formatters detected (%s); rerun with --mode custom", strings.Join(d.FormatterChoices, ", "))
	}
	if opts.Formatter != "" {
		formatter = opts.Formatter
	}
	if len(d.FormatterChoices) > 0 {
		formatter = d.FormatterChoices[0]
	}
	if opts.Formatter != "" {
		formatter = opts.Formatter
	} else if contains(d.Tools, "biome") {
		formatter = "biome"
	}

	cacheEnabled := true
	if opts.Mode == cli.ModeMinimal {
		cacheEnabled = false
	}
	if opts.CacheEnabled != nil {
		cacheEnabled = *opts.CacheEnabled
	}

	safety := opts.Safety
	if safety == "" {
		switch opts.Mode {
		case cli.ModeStrict:
			safety = "strict"
		case cli.ModeMinimal:
			safety = "fast"
		default:
			safety = "smart"
		}
	}

	cfg := config.New(cacheEnabled, safety)
	rec := Recommendation{
		Config:            cfg,
		SelectedJobs:      map[string][]string{},
		UpdateGitIgnore:   cacheEnabled,
		SelectedFormatter: formatter,
		SelectedPM:        pm,
	}

	if opts.Preset == "" {
		if !opts.SkipPreset {
			opts.Preset = inferPreset(d)
		}
	}

	if opts.Preset == "monorepo" {
		opts.Monorepo = "on"
	}

	selectedHooks := requestedHooks(opts)
	if len(selectedHooks) == 0 {
		switch opts.Mode {
		case cli.ModeMinimal:
			selectedHooks = []string{"pre-commit"}
		default:
			selectedHooks = []string{"pre-commit", "commit-msg", "pre-push"}
		}
	}

	for _, hook := range selectedHooks {
		rec.Config.Hooks[hook] = config.Hook{Jobs: map[string]config.Job{}}
	}

	if contains(selectedHooks, "pre-commit") {
		hook := rec.Config.Hooks["pre-commit"]
		if shouldUseGoPreset(opts.Preset, d) {
			command := "gofmt -w {staged}"
			jobName := "gofmt"
			if contains(d.Tools, "gofumpt") || opts.Formatter == "gofumpt" {
				command = "gofumpt -w {staged}"
				jobName = "gofumpt"
			}
			hook.Jobs[jobName] = config.Job{
				Run:        command,
				Files:      "staged",
				Glob:       []string{"*.go"},
				StageFixed: true,
			}
			rec.SelectedJobs["pre-commit"] = append(rec.SelectedJobs["pre-commit"], jobName)
			if contains(d.Tools, "golangci-lint") {
				hook.Jobs["golangci-lint"] = config.Job{
					Run: "golangci-lint run ./...",
				}
				rec.SelectedJobs["pre-commit"] = append(rec.SelectedJobs["pre-commit"], "golangci-lint")
			}
		} else if shouldUsePythonPreset(opts.Preset, d) {
			if contains(d.Tools, "ruff") {
				hook.Jobs["ruff-check"] = config.Job{
					Run:        "ruff check --fix {staged}",
					Files:      "staged",
					Glob:       []string{"*.py"},
					StageFixed: true,
				}
				rec.SelectedJobs["pre-commit"] = append(rec.SelectedJobs["pre-commit"], "ruff-check")
				hook.Jobs["ruff-format"] = config.Job{
					Run:        "ruff format {staged}",
					Files:      "staged",
					Glob:       []string{"*.py"},
					StageFixed: true,
				}
				rec.SelectedJobs["pre-commit"] = append(rec.SelectedJobs["pre-commit"], "ruff-format")
			} else {
				if contains(d.Tools, "black") {
					hook.Jobs["black"] = config.Job{
						Run:        "black {staged}",
						Files:      "staged",
						Glob:       []string{"*.py"},
						StageFixed: true,
					}
					rec.SelectedJobs["pre-commit"] = append(rec.SelectedJobs["pre-commit"], "black")
				}
				if contains(d.Tools, "isort") {
					hook.Jobs["isort"] = config.Job{
						Run:        "isort {staged}",
						Files:      "staged",
						Glob:       []string{"*.py"},
						StageFixed: true,
					}
					rec.SelectedJobs["pre-commit"] = append(rec.SelectedJobs["pre-commit"], "isort")
				}
			}
		} else if shouldUseRustPreset(opts.Preset, d) {
			hook.Jobs["cargo-fmt"] = config.Job{
				Run: "cargo fmt --all",
			}
			rec.SelectedJobs["pre-commit"] = append(rec.SelectedJobs["pre-commit"], "cargo-fmt")
		} else if contains(d.Tools, "biome") {
			hook.Jobs["biome"] = config.Job{
				Run:        runWithPM(pm, "biome check --write {staged}", "biome"),
				Files:      "staged",
				Glob:       []string{"*.js", "*.jsx", "*.ts", "*.tsx", "*.json"},
				StageFixed: true,
			}
			rec.SelectedJobs["pre-commit"] = append(rec.SelectedJobs["pre-commit"], "biome")
		} else {
			if formatter == "prettier" || contains(d.Tools, "prettier") {
				hook.Jobs["prettier"] = config.Job{
					Run:        runWithPM(pm, "prettier --write {staged}", "prettier"),
					Files:      "staged",
					Glob:       []string{"*.js", "*.jsx", "*.ts", "*.tsx", "*.json", "*.md", "*.css"},
					StageFixed: true,
				}
				rec.SelectedJobs["pre-commit"] = append(rec.SelectedJobs["pre-commit"], "prettier")
			}
			if contains(d.Tools, "eslint") {
				hook.Jobs["eslint"] = config.Job{
					Run:   runWithPM(pm, "eslint {staged}", "eslint"),
					Files: "staged",
					Glob:  []string{"*.js", "*.jsx", "*.ts", "*.tsx"},
				}
				rec.SelectedJobs["pre-commit"] = append(rec.SelectedJobs["pre-commit"], "eslint")
			}
			if contains(d.Tools, "oxlint") {
				hook.Jobs["oxlint"] = config.Job{
					Run:   runWithPM(pm, "oxlint {staged}", "oxlint"),
					Files: "staged",
					Glob:  []string{"*.js", "*.jsx", "*.ts", "*.tsx"},
				}
				rec.SelectedJobs["pre-commit"] = append(rec.SelectedJobs["pre-commit"], "oxlint")
			}
		}
		hook.Jobs["secrets"] = config.Job{Run: "righthook secrets scan {staged}", Files: "staged"}
		rec.SelectedJobs["pre-commit"] = append(rec.SelectedJobs["pre-commit"], "secrets")
		rec.Config.Hooks["pre-commit"] = hook
	}

	if contains(selectedHooks, "commit-msg") && contains(d.Tools, "commitlint") {
		hook := rec.Config.Hooks["commit-msg"]
		hook.Jobs["commitlint"] = config.Job{
			Run: runWithPM(pm, "commitlint --edit {commit_msg_file}", "commitlint"),
		}
		rec.SelectedJobs["commit-msg"] = append(rec.SelectedJobs["commit-msg"], "commitlint")
		rec.Config.Hooks["commit-msg"] = hook
	}

	if contains(selectedHooks, "pre-push") {
		hook := rec.Config.Hooks["pre-push"]
		if cmd := d.Scripts["typecheck"]; cmd != "" {
			hook.Jobs["typecheck"] = pushJob(cmd, d, opts)
			rec.SelectedJobs["pre-push"] = append(rec.SelectedJobs["pre-push"], "typecheck")
		} else if contains(d.Languages, "typescript") {
			hook.Jobs["typecheck"] = pushJob(runWithPM(pm, "tsc --noEmit", "tsc"), d, opts)
			rec.SelectedJobs["pre-push"] = append(rec.SelectedJobs["pre-push"], "typecheck")
		} else if cmd := presetFallbackTypecheck(opts.Preset, d); cmd != "" {
			hook.Jobs["typecheck"] = pushJob(cmd, d, opts)
			rec.SelectedJobs["pre-push"] = append(rec.SelectedJobs["pre-push"], "typecheck")
		}
		if cmd := d.Scripts["test"]; cmd != "" {
			hook.Jobs["test"] = pushJob(cmd, d, opts)
			rec.SelectedJobs["pre-push"] = append(rec.SelectedJobs["pre-push"], "test")
		} else if cmd := presetFallbackTest(opts.Preset, d); cmd != "" {
			hook.Jobs["test"] = pushJob(cmd, d, opts)
			rec.SelectedJobs["pre-push"] = append(rec.SelectedJobs["pre-push"], "test")
		}
		if opts.Mode == cli.ModeStrict {
			if cmd := d.Scripts["build"]; cmd != "" {
				hook.Jobs["build"] = pushJob(cmd, d, opts)
				rec.SelectedJobs["pre-push"] = append(rec.SelectedJobs["pre-push"], "build")
			} else if cmd := presetFallbackBuild(opts.Preset, d); cmd != "" {
				hook.Jobs["build"] = pushJob(cmd, d, opts)
				rec.SelectedJobs["pre-push"] = append(rec.SelectedJobs["pre-push"], "build")
			}
		}
		if len(hook.Jobs) > 0 {
			rec.Config.Hooks["pre-push"] = hook
		}
	}

	for hook, def := range rec.Config.Hooks {
		if len(def.Jobs) == 0 {
			delete(rec.Config.Hooks, hook)
		}
	}

	rec.SelectedHooks = orderedHooks(rec.Config.Hooks)
	return rec, nil
}

func runWithPM(pm, command, bin string) string {
	switch pm {
	case "pnpm":
		return "pnpm " + command
	case "yarn":
		return "yarn " + command
	case "bun":
		return "bunx " + command
	case "npm":
		if bin != "" {
			return "npx " + command
		}
	}
	return command
}

func inferPreset(d detect.Result) string {
	for _, preset := range []string{"next", "nestjs", "monorepo", "go", "rust", "python", "node"} {
		if contains(d.PresetCandidates, preset) {
			return preset
		}
	}
	return ""
}

func shouldUseGoPreset(preset string, d detect.Result) bool {
	return preset == "go" || (preset == "" && contains(d.Languages, "go"))
}

func shouldUsePythonPreset(preset string, d detect.Result) bool {
	return preset == "python" || (preset == "" && contains(d.Languages, "python"))
}

func shouldUseRustPreset(preset string, d detect.Result) bool {
	return preset == "rust" || (preset == "" && contains(d.Languages, "rust"))
}

func pushJob(command string, d detect.Result, opts cli.ResolvedInitOptions) config.Job {
	job := config.Job{Run: command}
	monorepoEnabled := opts.Monorepo == "on" || (opts.Monorepo == "auto" && d.Monorepo)
	if monorepoEnabled {
		job.Scope = "affected"
		job.Base = opts.Base
		job.Workspace = "affected"
		job.Cache = true
	}
	return job
}

func presetFallbackTest(preset string, d detect.Result) string {
	switch preset {
	case "go":
		if contains(d.Languages, "go") {
			return "go test ./..."
		}
	case "rust":
		if contains(d.Languages, "rust") {
			return "cargo test"
		}
	case "python":
		if contains(d.Tools, "pytest") || contains(d.Languages, "python") {
			return "pytest"
		}
	}
	return ""
}

func presetFallbackTypecheck(preset string, d detect.Result) string {
	switch preset {
	case "go":
		if contains(d.Tools, "golangci-lint") {
			return "golangci-lint run ./..."
		}
		return "go vet ./..."
	case "python":
		if contains(d.Tools, "mypy") {
			return "mypy ."
		}
	case "rust":
		if contains(d.Languages, "rust") {
			return "cargo clippy --all-targets --all-features"
		}
	}
	return ""
}

func presetFallbackBuild(preset string, d detect.Result) string {
	switch preset {
	case "go":
		if contains(d.Languages, "go") {
			return "go build ./..."
		}
	case "rust":
		if contains(d.Languages, "rust") {
			return "cargo build"
		}
	case "python":
		if contains(d.Languages, "python") {
			return "python -m compileall ."
		}
	}
	return ""
}

func requestedHooks(opts cli.ResolvedInitOptions) []string {
	if len(opts.Hooks) == 0 {
		return nil
	}
	return append([]string(nil), opts.Hooks...)
}

func orderedHooks(hooks map[string]config.Hook) []string {
	order := []string{"pre-commit", "commit-msg", "pre-push"}
	var out []string
	for _, name := range order {
		if _, ok := hooks[name]; ok {
			out = append(out, name)
		}
	}
	return out
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
