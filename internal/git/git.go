package git

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type InstallPlan struct {
	ConfigPath          string     `json:"config_path"`
	ConfigWillOverwrite bool       `json:"config_will_overwrite"`
	HookDir             string     `json:"hook_dir"`
	HookFiles           []HookFile `json:"hook_files,omitempty"`
	HooksWillOverwrite  []string   `json:"hooks_will_overwrite,omitempty"`
	GitIgnorePath       string     `json:"gitignore_path,omitempty"`
	GitIgnoreAdditions  []string   `json:"gitignore_additions,omitempty"`
}

type HookFile struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	WillOverwrite bool   `json:"will_overwrite"`
	Content       string `json:"-"`
}

func BuildInstallPlan(configPath, hookDir string, hooks []string, install bool, gitignorePath string, gitignoreAdditions []string) InstallPlan {
	plan := InstallPlan{
		ConfigPath:          configPath,
		ConfigWillOverwrite: exists(configPath),
		HookDir:             hookDir,
		GitIgnorePath:       gitignorePath,
		GitIgnoreAdditions:  gitignoreAdditions,
	}

	if install {
		for _, hook := range hooks {
			path := filepath.Join(hookDir, hook)
			file := HookFile{
				Name:          hook,
				Path:          path,
				WillOverwrite: exists(path),
				Content:       HookScript(hook),
			}
			plan.HookFiles = append(plan.HookFiles, file)
			if file.WillOverwrite {
				plan.HooksWillOverwrite = append(plan.HooksWillOverwrite, hook)
			}
		}
		sort.Strings(plan.HooksWillOverwrite)
	}

	return plan
}

func HookScript(hook string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

if command -v righthook >/dev/null 2>&1; then
  exec righthook run %s "$@"
fi

if [ -x "./node_modules/.bin/righthook" ]; then
  exec ./node_modules/.bin/righthook run %s "$@"
fi

if [ -x "./righthook" ]; then
  exec ./righthook run %s "$@"
fi

echo "righthook not found in PATH, ./node_modules/.bin, or ./righthook" >&2
exit 1
`, hook, hook, hook)
}

func Apply(plan InstallPlan, configBytes []byte) error {
	if err := os.MkdirAll(filepath.Dir(plan.ConfigPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(plan.ConfigPath, configBytes, 0o644); err != nil {
		return err
	}

	if len(plan.HookFiles) > 0 {
		if err := os.MkdirAll(plan.HookDir, 0o755); err != nil {
			return err
		}
	}
	for _, hook := range plan.HookFiles {
		if err := os.WriteFile(hook.Path, []byte(hook.Content), 0o755); err != nil {
			return err
		}
	}

	if len(plan.GitIgnoreAdditions) > 0 {
		if err := updateGitIgnore(plan.GitIgnorePath, plan.GitIgnoreAdditions); err != nil {
			return err
		}
	}

	return nil
}

func updateGitIgnore(path string, additions []string) error {
	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}
	trimmed := strings.TrimRight(existing, "\n")
	lines := map[string]bool{}
	for _, line := range strings.Split(trimmed, "\n") {
		if strings.TrimSpace(line) != "" {
			lines[line] = true
		}
	}
	for _, addition := range additions {
		if !lines[addition] {
			if trimmed != "" {
				trimmed += "\n"
			}
			trimmed += addition
		}
	}
	if trimmed != "" {
		trimmed += "\n"
	}
	return os.WriteFile(path, []byte(trimmed), 0o644)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
