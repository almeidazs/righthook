package detect

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Result struct {
	CWD               string
	RepoRoot          string
	GitDir            string
	GitPathKind       string
	CoreHooksPath     string
	EffectiveHooksDir string
	ExistingHooks     map[string]bool
	ConfigExists      bool
	LegacyManagers    []string
	PackageManager    string
	PackageManagers   []string
	FormatterChoices  []string
	Languages         []string
	Frameworks        []string
	Tools             []string
	Scripts           map[string]string
	Monorepo          bool
	MonorepoSignals   []string
	PresetCandidates  []string
	PackageJSONPath   string
	PackageJSON       PackageJSON
	Warnings          []string
}

type PackageJSON struct {
	PackageManager  string
	Scripts         map[string]string
	Dependencies    map[string]string
	DevDependencies map[string]string
	Workspaces      any
	LintStaged      any `json:"lint-staged"`
}

func Scan(cwd string) (Result, error) {
	root, gitEntry, err := findRepoRoot(cwd)
	if err != nil {
		return Result{}, err
	}
	gitDir, kind, err := resolveGitDir(root, gitEntry)
	if err != nil {
		return Result{}, err
	}

	res := Result{
		CWD:           cwd,
		RepoRoot:      root,
		GitDir:        gitDir,
		GitPathKind:   kind,
		ExistingHooks: map[string]bool{},
		Scripts:       map[string]string{},
	}

	res.CoreHooksPath = readCoreHooksPath(filepath.Join(gitDir, "config"))
	res.EffectiveHooksDir = effectiveHooksDir(root, gitDir, res.CoreHooksPath)
	for _, hook := range []string{"pre-commit", "commit-msg", "pre-push"} {
		if fileExists(filepath.Join(res.EffectiveHooksDir, hook)) {
			res.ExistingHooks[hook] = true
		}
	}

	res.ConfigExists = fileExists(filepath.Join(cwd, "righthook.yml")) || fileExists(filepath.Join(root, "righthook.yml"))

	if fileExists(filepath.Join(root, "lefthook.yml")) {
		res.LegacyManagers = append(res.LegacyManagers, "lefthook")
	}
	if dirExists(filepath.Join(root, ".husky")) {
		res.LegacyManagers = append(res.LegacyManagers, "husky")
	}
	if fileExists(filepath.Join(root, ".lintstagedrc")) {
		res.LegacyManagers = append(res.LegacyManagers, "lint-staged")
	}

	pkgPath := filepath.Join(root, "package.json")
	if fileExists(pkgPath) {
		res.PackageJSONPath = pkgPath
		var pkg PackageJSON
		data, err := os.ReadFile(pkgPath)
		if err == nil && json.Unmarshal(data, &pkg) == nil {
			res.PackageJSON = pkg
			for k, v := range pkg.Scripts {
				res.Scripts[k] = v
			}
			if pkg.LintStaged != nil && !contains(res.LegacyManagers, "lint-staged") {
				res.LegacyManagers = append(res.LegacyManagers, "lint-staged")
			}
		}
	}

	res.PackageManagers = detectPackageManagers(root, res.PackageJSON.PackageManager)
	if len(res.PackageManagers) > 0 {
		res.PackageManager = res.PackageManagers[0]
	}

	res.Languages = detectLanguages(root, res.PackageJSON)
	res.Frameworks, res.Tools, res.FormatterChoices = detectEcosystem(root, res.PackageJSON)
	res.Monorepo, res.MonorepoSignals = detectMonorepo(root, res.PackageJSON)
	res.PresetCandidates = detectPresetCandidates(res)

	sort.Strings(res.LegacyManagers)
	sort.Strings(res.Languages)
	sort.Strings(res.Frameworks)
	sort.Strings(res.Tools)
	sort.Strings(res.FormatterChoices)
	sort.Strings(res.MonorepoSignals)
	sort.Strings(res.PresetCandidates)

	return res, nil
}

func findRepoRoot(start string) (string, string, error) {
	dir := start
	for {
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return dir, gitPath, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", fmt.Errorf("no Git repository found from %s", start)
		}
		dir = parent
	}
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
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "hooksPath" {
			return val
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

func detectPackageManagers(root, declared string) []string {
	var pms []string
	hasNodeProject := fileExists(filepath.Join(root, "package.json"))
	add := func(name string) {
		if !contains(pms, name) {
			pms = append(pms, name)
		}
	}

	switch {
	case strings.HasPrefix(declared, "pnpm@"):
		add("pnpm")
	case strings.HasPrefix(declared, "npm@"):
		add("npm")
	case strings.HasPrefix(declared, "yarn@"):
		add("yarn")
	case strings.HasPrefix(declared, "bun@"):
		add("bun")
	}

	for _, item := range []struct {
		path string
		name string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"bun.lock", "bun"},
		{"bun.lockb", "bun"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
	} {
		if fileExists(filepath.Join(root, item.path)) {
			add(item.name)
			hasNodeProject = true
		}
	}

	if len(pms) == 0 && hasNodeProject {
		for _, bin := range []string{"pnpm", "bun", "yarn", "npm"} {
			if _, err := exec.LookPath(bin); err == nil {
				add(bin)
				break
			}
		}
	}

	return pms
}

func detectPresetCandidates(res Result) []string {
	var presets []string
	if contains(res.Frameworks, "next") {
		presets = append(presets, "next")
	}
	if contains(res.Frameworks, "nestjs") {
		presets = append(presets, "nestjs")
	}
	if res.Monorepo {
		presets = append(presets, "monorepo")
	}
	if contains(res.Languages, "go") {
		presets = append(presets, "go")
	}
	if contains(res.Languages, "rust") {
		presets = append(presets, "rust")
	}
	if contains(res.Languages, "python") {
		presets = append(presets, "python")
	}
	if contains(res.Languages, "javascript") || contains(res.Languages, "typescript") {
		presets = append(presets, "node")
	}
	return presets
}

func detectLanguages(root string, _ PackageJSON) []string {
	langs := []string{}
	if fileExists(filepath.Join(root, "package.json")) {
		addUnique(&langs, "javascript")
	}
	if fileExists(filepath.Join(root, "tsconfig.json")) {
		addUnique(&langs, "typescript")
	}
	if fileExists(filepath.Join(root, "go.mod")) {
		addUnique(&langs, "go")
	}
	if fileExists(filepath.Join(root, "Cargo.toml")) {
		addUnique(&langs, "rust")
	}
	if fileExists(filepath.Join(root, "pyproject.toml")) || fileExists(filepath.Join(root, "requirements.txt")) {
		addUnique(&langs, "python")
	}
	return langs
}

func detectEcosystem(root string, pkg PackageJSON) ([]string, []string, []string) {
	var frameworks []string
	var tools []string
	var formatters []string
	pyproject := readTextIfExists(
		filepath.Join(root, "pyproject.toml"),
		filepath.Join(root, "requirements.txt"),
		filepath.Join(root, "setup.cfg"),
		filepath.Join(root, "tox.ini"),
	)
	goMod := readTextIfExists(filepath.Join(root, "go.mod"))
	cargoToml := readTextIfExists(filepath.Join(root, "Cargo.toml"))

	hasDep := func(name string) bool {
		_, ok := pkg.Dependencies[name]
		if ok {
			return true
		}
		_, ok = pkg.DevDependencies[name]
		return ok
	}

	if hasDep("next") {
		addUnique(&frameworks, "next")
	}
	if hasDep("@nestjs/core") {
		addUnique(&frameworks, "nestjs")
	}
	if hasDep("vite") {
		addUnique(&frameworks, "vite")
	}
	if textContainsAny(pyproject, "fastapi", "flask", "django") {
		addUnique(&frameworks, "python")
	}
	if textContainsAny(cargoToml, "[package]", "[dependencies]") {
		addUnique(&frameworks, "rust")
	}
	if fileExists(filepath.Join(root, "package.json")) && len(frameworks) == 0 {
		addUnique(&frameworks, "node")
	}

	for _, item := range []struct {
		dep  string
		name string
	}{
		{"@biomejs/biome", "biome"},
		{"eslint", "eslint"},
		{"prettier", "prettier"},
		{"oxlint", "oxlint"},
		{"vitest", "vitest"},
		{"jest", "jest"},
		{"@commitlint/cli", "commitlint"},
	} {
		if hasDep(item.dep) {
			addUnique(&tools, item.name)
		}
	}

	if fileExists(filepath.Join(root, ".prettierrc")) || fileExists(filepath.Join(root, "prettier.config.js")) {
		addUnique(&tools, "prettier")
	}
	if fileExists(filepath.Join(root, ".eslintrc")) || fileExists(filepath.Join(root, "eslint.config.js")) {
		addUnique(&tools, "eslint")
	}
	if fileExists(filepath.Join(root, "biome.json")) || fileExists(filepath.Join(root, "biome.jsonc")) {
		addUnique(&tools, "biome")
	}
	if fileExists(filepath.Join(root, ".commitlintrc")) || fileExists(filepath.Join(root, "commitlint.config.js")) {
		addUnique(&tools, "commitlint")
	}
	if fileExists(filepath.Join(root, "vitest.config.ts")) || fileExists(filepath.Join(root, "vitest.config.js")) {
		addUnique(&tools, "vitest")
	}
	if fileExists(filepath.Join(root, "jest.config.ts")) || fileExists(filepath.Join(root, "jest.config.js")) {
		addUnique(&tools, "jest")
	}
	if anyFileExists(
		filepath.Join(root, ".golangci.yml"),
		filepath.Join(root, ".golangci.yaml"),
		filepath.Join(root, ".golangci.toml"),
		filepath.Join(root, ".golangci.json"),
	) || textContainsAny(goMod, "github.com/golangci/golangci-lint") {
		addUnique(&tools, "golangci-lint")
	}
	if textContainsAny(goMod, "mvdan.cc/gofumpt", "gofumpt") {
		addUnique(&tools, "gofumpt")
	}
	if textContainsAny(pyproject, "ruff", "[tool.ruff") {
		addUnique(&tools, "ruff")
	}
	if textContainsAny(pyproject, "black", "[tool.black") {
		addUnique(&tools, "black")
	}
	if textContainsAny(pyproject, "isort", "[tool.isort") {
		addUnique(&tools, "isort")
	}
	if textContainsAny(pyproject, "mypy", "[tool.mypy") {
		addUnique(&tools, "mypy")
	}
	if textContainsAny(pyproject, "pytest", "[tool.pytest", "[pytest]") {
		addUnique(&tools, "pytest")
	}

	if contains(tools, "biome") {
		addUnique(&formatters, "biome")
	}
	if contains(tools, "prettier") {
		addUnique(&formatters, "prettier")
	}
	if contains(tools, "gofumpt") {
		addUnique(&formatters, "gofumpt")
	}
	if contains(tools, "ruff") {
		addUnique(&formatters, "ruff")
	}
	if contains(tools, "black") {
		addUnique(&formatters, "black")
	}
	if contains(tools, "isort") {
		addUnique(&formatters, "isort")
	}
	return frameworks, tools, formatters
}

func detectMonorepo(root string, pkg PackageJSON) (bool, []string) {
	var signals []string
	for _, file := range []string{"pnpm-workspace.yaml", "turbo.json", "nx.json", "lerna.json", "rush.json"} {
		if fileExists(filepath.Join(root, file)) {
			signals = append(signals, file)
		}
	}
	if pkg.Workspaces != nil {
		signals = append(signals, "package.json#workspaces")
	}
	return len(signals) > 0, signals
}

func addUnique(items *[]string, value string) {
	if !contains(*items, value) {
		*items = append(*items, value)
	}
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func anyFileExists(paths ...string) bool {
	for _, path := range paths {
		if fileExists(path) {
			return true
		}
	}
	return false
}

func readTextIfExists(paths ...string) string {
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			return strings.ToLower(string(data))
		}
	}
	return ""
}

func textContainsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, strings.ToLower(value)) {
			return true
		}
	}
	return false
}
