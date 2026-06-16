package git

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Repository struct {
	Root              string `json:"root"`
	GitDir            string `json:"git_dir"`
	GitPathKind       string `json:"git_path_kind"`
	CoreHooksPath     string `json:"core_hooks_path,omitempty"`
	EffectiveHooksDir string `json:"effective_hooks_dir"`
}

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
	Exists        bool   `json:"exists,omitempty"`
	WillOverwrite bool   `json:"will_overwrite"`
	IsRighthook   bool   `json:"is_righthook,omitempty"`
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
				Exists:        exists(path),
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

func ResolveRepository(path string) (Repository, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Repository{}, fmt.Errorf("stat git path: %w", err)
	}

	switch {
	case info.IsDir() && exists(filepath.Join(path, ".git")):
		return resolveRepositoryFromRoot(path)
	case info.IsDir() && filepath.Base(path) == ".git" && exists(filepath.Join(path, "config")):
		return resolveRepositoryFromGitEntry(filepath.Dir(path), path)
	case !info.IsDir() && filepath.Base(path) == ".git":
		return resolveRepositoryFromGitEntry(filepath.Dir(path), path)
	default:
		return Repository{}, fmt.Errorf("%s is not a Git repository root or .git path", path)
	}
}

func resolveRepositoryFromRoot(root string) (Repository, error) {
	return resolveRepositoryFromGitEntry(root, filepath.Join(root, ".git"))
}

func resolveRepositoryFromGitEntry(root, gitEntry string) (Repository, error) {
	gitDir, kind, err := resolveGitDir(root, gitEntry)
	if err != nil {
		return Repository{}, err
	}
	coreHooksPath := readCoreHooksPath(filepath.Join(gitDir, "config"))
	return Repository{
		Root:              root,
		GitDir:            gitDir,
		GitPathKind:       kind,
		CoreHooksPath:     coreHooksPath,
		EffectiveHooksDir: effectiveHooksDir(root, gitDir, coreHooksPath),
	}, nil
}

func resolveGitDir(root, gitPath string) (string, string, error) {
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return gitPath, "directory", nil
	}
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", "", err
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir:") {
		return "", "", fmt.Errorf("unsupported .git file format")
	}
	target := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	return filepath.Clean(target), "file", nil
}

func readCoreHooksPath(configPath string) string {
	f, err := os.Open(configPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inCore := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inCore = line == "[core]"
			continue
		}
		if !inCore || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if strings.TrimSpace(parts[0]) == "hooksPath" {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func effectiveHooksDir(root, gitDir, coreHooksPath string) string {
	if strings.TrimSpace(coreHooksPath) == "" {
		return filepath.Join(gitDir, "hooks")
	}
	if filepath.IsAbs(coreHooksPath) {
		return coreHooksPath
	}
	return filepath.Join(root, coreHooksPath)
}

func ListInstalledHooks(repo Repository, hooks []string) []HookFile {
	files := make([]HookFile, 0, len(hooks))
	for _, hook := range hooks {
		path := filepath.Join(repo.EffectiveHooksDir, hook)
		if !exists(path) {
			continue
		}
		files = append(files, HookFile{
			Name:        hook,
			Path:        path,
			Exists:      true,
			IsRighthook: IsRighthookHook(path),
		})
	}
	return files
}

func IsRighthookHook(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(data)
	return strings.Contains(text, "righthook run ") &&
		strings.Contains(text, "command -v righthook") &&
		strings.Contains(text, "./node_modules/.bin/righthook")
}

func WriteHookFiles(hookDir string, hooks []HookFile) error {
	if len(hooks) == 0 {
		return nil
	}
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return err
	}
	for _, hook := range hooks {
		if err := os.WriteFile(hook.Path, []byte(hook.Content), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func RemoveHookFiles(hooks []HookFile) (removed []string, skipped []string, err error) {
	for _, hook := range hooks {
		if !hook.Exists {
			skipped = append(skipped, hook.Name)
			continue
		}
		if !hook.IsRighthook {
			skipped = append(skipped, hook.Name)
			continue
		}
		if err := os.Remove(hook.Path); err != nil {
			return removed, skipped, err
		}
		removed = append(removed, hook.Name)
	}
	sort.Strings(removed)
	sort.Strings(skipped)
	return removed, skipped, nil
}

func RemoveFiles(paths []string) ([]string, error) {
	removed := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		if !exists(path) {
			continue
		}
		if err := os.Remove(path); err != nil {
			return removed, err
		}
		removed = append(removed, path)
	}
	sort.Strings(removed)
	return removed, nil
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
