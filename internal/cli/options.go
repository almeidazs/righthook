package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

type Mode string

const (
	ModeRecommended Mode = "recommended"
	ModeMinimal     Mode = "minimal"
	ModeStrict      Mode = "strict"
	ModeCustom      Mode = "custom"
)

type Runtime struct {
	Stdin  *os.File
	Stdout io.Writer
	Stderr io.Writer
}

type InitOptions struct {
	Yes               bool
	Install           bool
	NoInstall         bool
	InstallSpecified  bool
	Force             bool
	DryRun            bool
	PrintOnly         bool
	Mode              string
	ModeSpecified     bool
	Preset            string
	PM                string
	PMSpecified       bool
	Hooks             string
	Cache             bool
	NoCache           bool
	CacheSpecified    bool
	Safety            string
	SafetySpecified   bool
	Monorepo          string
	MonorepoSpecified bool
	Base              string
	Migrate           string
	ConfigPath        string
	CWD               string
	NoColor           bool
	NoEmoji           bool
	JSON              bool
}

type ResolvedInitOptions struct {
	Yes           bool
	Force         bool
	DryRun        bool
	PrintOnly     bool
	Mode          Mode
	ModeSpecified bool
	Preset        string
	PM            string
	PMProvided    bool
	Hooks         []string
	CacheEnabled  *bool
	Safety        string
	Install       bool
	Base          string
	Migrate       string
	ConfigPath    string
	ConfigFormat  string
	CWD           string
	NoColor       bool
	NoEmoji       bool
	JSON          bool
	Monorepo      string
	Formatter     string
	SkipPreset    bool
	Interactive   bool
	OutputPathsOK bool
}

func ResolveInitOptions(opts InitOptions, rt Runtime) (ResolvedInitOptions, error) {
	if opts.Install && opts.NoInstall {
		return ResolvedInitOptions{}, errors.New("--install and --no-install cannot be used together")
	}

	if opts.Cache && opts.NoCache {
		return ResolvedInitOptions{}, errors.New("--cache and --no-cache cannot be used together")
	}

	mode := ModeRecommended
	if opts.Mode != "" {
		mode = Mode(strings.ToLower(opts.Mode))
	}
	switch mode {
	case ModeRecommended, ModeMinimal, ModeStrict, ModeCustom:
	default:
		return ResolvedInitOptions{}, fmt.Errorf("unsupported --mode %q", opts.Mode)
	}

	pm := strings.ToLower(strings.TrimSpace(opts.PM))
	if pm != "" && pm != "pnpm" && pm != "npm" && pm != "yarn" && pm != "bun" {
		return ResolvedInitOptions{}, fmt.Errorf("unsupported --pm %q", opts.PM)
	}

	preset := strings.ToLower(strings.TrimSpace(opts.Preset))
	if preset != "" {
		switch preset {
		case "node", "next", "nestjs", "monorepo", "go", "rust", "python":
		default:
			return ResolvedInitOptions{}, fmt.Errorf("unsupported --preset %q", opts.Preset)
		}
	}

	safety := strings.ToLower(strings.TrimSpace(opts.Safety))
	if safety != "" {
		switch safety {
		case "smart", "fast", "strict", "off":
		default:
			return ResolvedInitOptions{}, fmt.Errorf("unsupported --safety %q", opts.Safety)
		}
	}

	monorepo := strings.ToLower(strings.TrimSpace(opts.Monorepo))
	if monorepo == "" {
		monorepo = "auto"
	}
	switch monorepo {
	case "auto", "on", "off":
	default:
		return ResolvedInitOptions{}, fmt.Errorf("unsupported --monorepo %q", opts.Monorepo)
	}

	migrate := strings.ToLower(strings.TrimSpace(opts.Migrate))
	if migrate != "" {
		switch migrate {
		case "lefthook", "husky", "lint-staged":
		default:
			return ResolvedInitOptions{}, fmt.Errorf("unsupported --migrate %q", opts.Migrate)
		}
	}

	cwd := opts.CWD
	if cwd == "" {
		cwd = "."
	}
	absCWD, err := filepath.Abs(cwd)
	if err != nil {
		return ResolvedInitOptions{}, fmt.Errorf("resolve cwd: %w", err)
	}

	cfgPath := opts.ConfigPath
	if cfgPath == "" {
		cfgPath = "righthook.yml"
	}
	if !filepath.IsAbs(cfgPath) {
		cfgPath = filepath.Join(absCWD, cfgPath)
	}

	format := configFormatForPath(cfgPath)
	if format == "" {
		return ResolvedInitOptions{}, fmt.Errorf("unsupported config extension for %q", cfgPath)
	}

	var hooks []string
	if strings.TrimSpace(opts.Hooks) != "" {
		parts := strings.Split(opts.Hooks, ",")
		seen := map[string]bool{}
		for _, part := range parts {
			hook := strings.TrimSpace(part)
			if hook == "" {
				continue
			}
			switch hook {
			case "pre-commit", "commit-msg", "pre-push":
			default:
				return ResolvedInitOptions{}, fmt.Errorf("unsupported hook %q", hook)
			}
			if !seen[hook] {
				seen[hook] = true
				hooks = append(hooks, hook)
			}
		}
	}

	var cacheEnabled *bool
	if opts.CacheSpecified {
		v := opts.Cache && !opts.NoCache
		cacheEnabled = &v
	}

	install := false
	if opts.InstallSpecified {
		install = opts.Install && !opts.NoInstall
	}

	interactive := rt.Stdin != nil && term.IsTerminal(int(rt.Stdin.Fd())) && rt.Stdout != nil

	return ResolvedInitOptions{
		Yes:           opts.Yes,
		Force:         opts.Force,
		DryRun:        opts.DryRun,
		PrintOnly:     opts.PrintOnly,
		Mode:          mode,
		ModeSpecified: opts.ModeSpecified,
		Preset:        preset,
		PM:            pm,
		PMProvided:    opts.PMSpecified && pm != "",
		Hooks:         hooks,
		CacheEnabled:  cacheEnabled,
		Safety:        safety,
		Install:       install,
		Base:          opts.Base,
		Migrate:       migrate,
		ConfigPath:    cfgPath,
		ConfigFormat:  format,
		CWD:           absCWD,
		NoColor:       opts.NoColor,
		NoEmoji:       opts.NoEmoji,
		JSON:          opts.JSON,
		Monorepo:      monorepo,
		Interactive:   interactive,
	}, nil
}

func configFormatForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yml", ".yaml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	default:
		return ""
	}
}
