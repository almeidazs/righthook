package commands

import (
	"fmt"
	"os"
	"sort"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/config"
	"github.com/almeidazs/righthook/internal/git"
	"github.com/almeidazs/righthook/internal/output"
)

func Status(raw cli.StatusOptions, rt cli.Runtime) error {
	opts, err := cli.ResolveStatusOptions(raw)
	renderer := output.New(rt.Stdout, false, false)
	renderer.Intro("Righthook status")
	if err != nil {
		renderer.Error(err.Error())
		return nil
	}

	repo, err := git.ResolveRepository(opts.Path)
	if err != nil {
		renderer.Error(err.Error())
		return nil
	}

	renderer.Item("repo", repo.Root)

	configPath := resolveRepoConfigPath(repo.Root, opts.ConfigPath)
	cfg, cfgState, cfgErr := loadStatusConfig(configPath)
	if cfgErr != nil {
		renderer.Error(fmt.Sprintf("Config invalid: %s (%v)", configPath, cfgErr))
	} else if cfgState == "found" {
		renderer.Success(fmt.Sprintf("Config found: %s", configPath))
	} else {
		renderer.Error(fmt.Sprintf("Config missing: %s", configPath))
	}

	expectedHooks := statusExpectedHooks(cfg, cfgState)
	installed := git.ListInstalledHooks(repo, cli.SupportedHooks)
	installedByName := mapHookFiles(installed)

	allInstalled := len(expectedHooks) > 0
	for _, hook := range expectedHooks {
		file, ok := installedByName[hook]
		if !ok || !file.IsRighthook {
			allInstalled = false
			break
		}
	}

	if allInstalled {
		renderer.Success("Git hooks installed")
	} else {
		renderer.Error("Git hooks missing or incomplete")
	}

	for _, hook := range cli.SupportedHooks {
		file, ok := installedByName[hook]
		switch {
		case ok && file.IsRighthook:
			renderer.Success(fmt.Sprintf("%s installed", hook))
		case ok:
			renderer.Error(fmt.Sprintf("%s occupied by a non-Righthook script", hook))
		default:
			renderer.Error(fmt.Sprintf("%s missing", hook))
		}
	}

	if cfgErr == nil && cfgState == "found" {
		if cfg.Cache.Enabled {
			renderer.Success("Cache enabled")
		} else {
			renderer.Error("Cache disabled")
		}
	}

	return nil
}

func loadStatusConfig(configPath string) (config.File, string, error) {
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return config.File{}, "missing", nil
		}
		return config.File{}, "missing", err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return config.File{}, "invalid", err
	}
	return cfg, "found", nil
}

func statusExpectedHooks(cfg config.File, state string) []string {
	if state == "found" && len(cfg.Hooks) > 0 {
		hooks := make([]string, 0, len(cfg.Hooks))
		for hook := range cfg.Hooks {
			hooks = append(hooks, hook)
		}
		sort.Strings(hooks)
		return hooks
	}
	return append([]string(nil), cli.SupportedHooks...)
}
