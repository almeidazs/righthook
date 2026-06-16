package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	koanftoml "github.com/knadh/koanf/parsers/toml/v2"
	koanfyaml "github.com/knadh/koanf/parsers/yaml"
	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type File struct {
	Version string          `json:"version" yaml:"version" toml:"version"`
	Output  OutputConfig    `json:"output" yaml:"output" toml:"output"`
	Cache   CacheConfig     `json:"cache" yaml:"cache" toml:"cache"`
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
	Files      string   `json:"files,omitempty" yaml:"files,omitempty" toml:"files,omitempty"`
	Glob       []string `json:"glob,omitempty" yaml:"glob,omitempty" toml:"glob,omitempty"`
	StageFixed bool     `json:"stage_fixed,omitempty" yaml:"stage_fixed,omitempty" toml:"stage_fixed,omitempty"`
	Scope      string   `json:"scope,omitempty" yaml:"scope,omitempty" toml:"scope,omitempty"`
	Base       string   `json:"base,omitempty" yaml:"base,omitempty" toml:"base,omitempty"`
	Workspace  string   `json:"workspace,omitempty" yaml:"workspace,omitempty" toml:"workspace,omitempty"`
	Cache      bool     `json:"cache,omitempty" yaml:"cache,omitempty" toml:"cache,omitempty"`
}

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
	return cfg, Validate(cfg)
}

func Load(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	return Decode(data, FormatForPath(path))
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

func Validate(cfg File) error {
	if cfg.Version != "1" {
		return fmt.Errorf("unsupported version %q", cfg.Version)
	}
	for hookName, hook := range cfg.Hooks {
		if hookName == "" {
			return fmt.Errorf("hook name cannot be empty")
		}
		for jobName, job := range hook.Jobs {
			if jobName == "" {
				return fmt.Errorf("job name cannot be empty")
			}
			if strings.TrimSpace(job.Run) == "" {
				return fmt.Errorf("job %s in %s has empty run command", jobName, hookName)
			}
		}
	}
	return nil
}
