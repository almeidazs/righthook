package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	koanftoml "github.com/knadh/koanf/parsers/toml/v2"
	koanfyaml "github.com/knadh/koanf/parsers/yaml"
	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type File struct {
	Version string          `json:"version" yaml:"version" toml:"version"`
	Output  OutputConfig    `json:"output" yaml:"output" toml:"output"`
	Cache   CacheConfig     `json:"cache" yaml:"cache" toml:"cache"`
	Policy  PolicyConfig    `json:"policy" yaml:"policy" toml:"policy"`
	Safety  SafetyConfig    `json:"safety" yaml:"safety" toml:"safety"`
	Hooks   map[string]Hook `json:"hooks,omitempty" yaml:"hooks,omitempty" toml:"hooks,omitempty"`
}

type OutputConfig struct {
	Mode        string `json:"mode" yaml:"mode" toml:"mode"`
	Timing      bool   `json:"timing" yaml:"timing" toml:"timing"`
	ShowSuccess bool   `json:"show_success" yaml:"show_success" toml:"show_success"`
}

type CacheConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled" toml:"enabled"`
	Dir     string `json:"dir" yaml:"dir" toml:"dir"`
	TTL     string `json:"ttl" yaml:"ttl" toml:"ttl"`
}

type PolicyConfig struct {
	RequiredVersion  string   `json:"required_version,omitempty" yaml:"required_version,omitempty" toml:"required_version,omitempty"`
	RequireInstalled bool     `json:"require_installed,omitempty" yaml:"require_installed,omitempty" toml:"require_installed,omitempty"`
	RequiredHooks    []string `json:"required_hooks,omitempty" yaml:"required_hooks,omitempty" toml:"required_hooks,omitempty"`
	AllowSkip        string   `json:"allow_skip,omitempty" yaml:"allow_skip,omitempty" toml:"allow_skip,omitempty"`
}

type SafetyConfig struct {
	Isolation        string `json:"isolation" yaml:"isolation" toml:"isolation"`
	PartialStaging   string `json:"partial_staging" yaml:"partial_staging" toml:"partial_staging"`
	UnstagedStrategy string `json:"unstaged_strategy" yaml:"unstaged_strategy" toml:"unstaged_strategy"`
	OnConflict       string `json:"on_conflict" yaml:"on_conflict" toml:"on_conflict"`
}

type Hook struct {
	Jobs map[string]Job `json:"jobs,omitempty" yaml:"jobs,omitempty" toml:"jobs,omitempty"`
}

type Job struct {
	Run        string   `json:"run" yaml:"run" toml:"run"`
	Enabled    *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty" toml:"enabled,omitempty"`
	Files      string   `json:"files,omitempty" yaml:"files,omitempty" toml:"files,omitempty"`
	Glob       []string `json:"glob,omitempty" yaml:"glob,omitempty" toml:"glob,omitempty"`
	StageFixed bool     `json:"stage_fixed,omitempty" yaml:"stage_fixed,omitempty" toml:"stage_fixed,omitempty"`
	Scope      string   `json:"scope,omitempty" yaml:"scope,omitempty" toml:"scope,omitempty"`
	Base       string   `json:"base,omitempty" yaml:"base,omitempty" toml:"base,omitempty"`
	Workspace  string   `json:"workspace,omitempty" yaml:"workspace,omitempty" toml:"workspace,omitempty"`
	Cache      bool     `json:"cache,omitempty" yaml:"cache,omitempty" toml:"cache,omitempty"`
}

var (
	validOutputModes       = map[string]bool{"compact": true, "verbose": true}
	validPolicyAllowSkip   = map[string]bool{"fail": true, "warn": true, "ignore": true}
	validSafetyIsolation   = map[string]bool{"smart": true, "fast": true, "strict": true, "off": true}
	validPartialStaging    = map[string]bool{"preserve": true, "allow": true, "forbid": true}
	validUnstagedStrategy  = map[string]bool{"stash": true, "ignore": true, "fail": true}
	validConflictPolicies  = map[string]bool{"explain": true, "warn": true, "fail": true, "ignore": true}
	validFileSelectors     = map[string]bool{"all": true, "staged": true, "changed": true, "affected": true}
	validJobScopeSelectors = map[string]bool{"affected": true}
	configBaseNames        = []string{"righthook.local", "righthook"}
	configExtensions       = []string{".yml", ".yaml", ".json", ".toml"}
	configSearchDirs       = []string{"", ".config"}
)

func New(cacheEnabled bool, safetyMode string) File {
	return File{
		Version: "1",
		Output: OutputConfig{
			Mode:        "compact",
			Timing:      true,
			ShowSuccess: false,
		},
		Cache: CacheConfig{
			Enabled: cacheEnabled,
			Dir:     ".righthook/cache",
			TTL:     "7d",
		},
		Safety: defaultSafety(safetyMode),
		Hooks:  map[string]Hook{},
	}
}

func WithDefaults(cfg File) File {
	defaults := New(cfg.Cache.Enabled, "smart")
	if cfg.Version == "" {
		cfg.Version = defaults.Version
	}
	if strings.TrimSpace(cfg.Output.Mode) == "" {
		cfg.Output.Mode = defaults.Output.Mode
	}
	if strings.TrimSpace(cfg.Cache.Dir) == "" {
		cfg.Cache.Dir = defaults.Cache.Dir
	}
	if cfg.Safety == (SafetyConfig{}) {
		cfg.Safety = defaults.Safety
	}
	return cfg
}

func defaultSafety(mode string) SafetyConfig {
	switch mode {
	case "fast":
		return SafetyConfig{Isolation: "fast", PartialStaging: "allow", UnstagedStrategy: "ignore", OnConflict: "warn"}
	case "strict":
		return SafetyConfig{Isolation: "strict", PartialStaging: "forbid", UnstagedStrategy: "fail", OnConflict: "fail"}
	case "off":
		return SafetyConfig{Isolation: "off", PartialStaging: "allow", UnstagedStrategy: "ignore", OnConflict: "ignore"}
	default:
		return SafetyConfig{Isolation: "smart", PartialStaging: "preserve", UnstagedStrategy: "stash", OnConflict: "explain"}
	}
}

func Encode(cfg File, format string) ([]byte, error) {
	switch format {
	case "yaml":
		return yaml.Marshal(cfg)
	case "json":
		return json.MarshalIndent(cfg, "", "  ")
	case "toml":
		return toml.Marshal(cfg)
	default:
		return nil, fmt.Errorf("unsupported config format %q", format)
	}
}

func Decode(data []byte, format string) (File, error) {
	var cfg File
	var err error

	switch format {
	case "yaml":
		_, err = koanfyaml.Parser().Unmarshal(data)
		if err == nil {
			err = yaml.Unmarshal(data, &cfg)
		}
	case "toml":
		_, err = koanftoml.Parser().Unmarshal(data)
		if err == nil {
			err = toml.Unmarshal(data, &cfg)
		}
	case "json":
		err = json.Unmarshal(data, &cfg)
	default:
		err = fmt.Errorf("unsupported config format %q", format)
	}
	if err != nil {
		return File{}, err
	}
	cfg = WithDefaults(cfg)
	return cfg, Validate(cfg)
}

func Load(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	return Decode(data, FormatForPath(path))
}

func DefaultPath(root string) string {
	return filepath.Join(root, "righthook.yml")
}

func CandidatePaths(root string) []string {
	paths := make([]string, 0, len(configBaseNames)*len(configExtensions)*len(configSearchDirs))
	for _, base := range configBaseNames {
		for _, dir := range configSearchDirs {
			for _, ext := range configExtensions {
				name := base + ext
				if dir == "" {
					paths = append(paths, filepath.Join(root, name))
					continue
				}
				paths = append(paths, filepath.Join(root, dir, name))
			}
		}
	}
	return paths
}

func FindExistingPath(root string) (string, bool) {
	for _, path := range CandidatePaths(root) {
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

func FormatForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	default:
		return ""
	}
}

func (j Job) IsEnabled() bool {
	return j.Enabled == nil || *j.Enabled
}

func Enabled(v bool) *bool {
	return &v
}

func ActiveJobs(jobs map[string]Job) map[string]Job {
	active := make(map[string]Job, len(jobs))
	for name, job := range jobs {
		if job.IsEnabled() {
			active[name] = job
		}
	}
	return active
}

func HasEnabledJobs(hook Hook) bool {
	for _, job := range hook.Jobs {
		if job.IsEnabled() {
			return true
		}
	}
	return false
}

func HookNamesWithEnabledJobs(hooks map[string]Hook) []string {
	names := make([]string, 0, len(hooks))
	for name, hook := range hooks {
		if HasEnabledJobs(hook) {
			names = append(names, name)
		}
	}
	return names
}

func Validate(cfg File) error {
	if cfg.Version != "1" {
		return fmt.Errorf("unsupported version %q", cfg.Version)
	}
	if mode := strings.TrimSpace(cfg.Output.Mode); mode != "" && !validOutputModes[mode] {
		return fmt.Errorf("unsupported output.mode %q", cfg.Output.Mode)
	}
	if allowSkip := strings.TrimSpace(cfg.Policy.AllowSkip); allowSkip != "" && !validPolicyAllowSkip[allowSkip] {
		return fmt.Errorf("unsupported policy.allow_skip %q", cfg.Policy.AllowSkip)
	}
	if constraint := strings.TrimSpace(cfg.Policy.RequiredVersion); constraint != "" {
		if _, err := semver.NewConstraint(constraint); err != nil {
			return fmt.Errorf("invalid policy.required_version %q: %w", cfg.Policy.RequiredVersion, err)
		}
	}
	for _, hook := range cfg.Policy.RequiredHooks {
		if err := validateHookName(strings.TrimSpace(hook)); err != nil {
			return fmt.Errorf("unsupported policy.required_hooks entry %q", hook)
		}
	}
	if isolation := strings.TrimSpace(cfg.Safety.Isolation); isolation != "" && !validSafetyIsolation[isolation] {
		return fmt.Errorf("unsupported safety.isolation %q", cfg.Safety.Isolation)
	}
	if partial := strings.TrimSpace(cfg.Safety.PartialStaging); partial != "" && !validPartialStaging[partial] {
		return fmt.Errorf("unsupported safety.partial_staging %q", cfg.Safety.PartialStaging)
	}
	if strategy := strings.TrimSpace(cfg.Safety.UnstagedStrategy); strategy != "" && !validUnstagedStrategy[strategy] {
		return fmt.Errorf("unsupported safety.unstaged_strategy %q", cfg.Safety.UnstagedStrategy)
	}
	if policy := strings.TrimSpace(cfg.Safety.OnConflict); policy != "" && !validConflictPolicies[policy] {
		return fmt.Errorf("unsupported safety.on_conflict %q", cfg.Safety.OnConflict)
	}
	for hookName, hook := range cfg.Hooks {
		if hookName == "" {
			return fmt.Errorf("hook name cannot be empty")
		}
		for jobName, job := range hook.Jobs {
			if jobName == "" {
				return fmt.Errorf("job name cannot be empty")
			}
			if job.IsEnabled() && strings.TrimSpace(job.Run) == "" {
				return fmt.Errorf("job %s in %s has empty run command", jobName, hookName)
			}
			if files := strings.TrimSpace(job.Files); files != "" && !validFileSelectors[files] {
				return fmt.Errorf("job %s in %s uses unsupported files selector %q", jobName, hookName, job.Files)
			}
			if scope := strings.TrimSpace(job.Scope); scope != "" && !validJobScopeSelectors[scope] {
				return fmt.Errorf("job %s in %s uses unsupported scope %q", jobName, hookName, job.Scope)
			}
			if workspace := strings.TrimSpace(job.Workspace); workspace != "" && !validJobScopeSelectors[workspace] {
				return fmt.Errorf("job %s in %s uses unsupported workspace %q", jobName, hookName, job.Workspace)
			}
			for _, pattern := range job.Glob {
				if _, err := filepath.Match(pattern, "example"); err != nil {
					return fmt.Errorf("job %s in %s has invalid glob %q: %w", jobName, hookName, pattern, err)
				}
			}
		}
	}
	return nil
}

func validateHookName(name string) error {
	if name == "" {
		return fmt.Errorf("hook name cannot be empty")
	}
	switch name {
	case "pre-commit", "commit-msg", "pre-push":
		return nil
	default:
		return fmt.Errorf("unsupported hook %q", name)
	}
}
