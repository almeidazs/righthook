package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/config"
	"github.com/almeidazs/righthook/internal/git"
	"github.com/almeidazs/righthook/internal/output"
)

func Install(raw cli.InstallOptions, rt cli.Runtime) error {
	opts, err := cli.ResolveInstallOptions(raw, rt)
	if err != nil {
		return err
	}

	repo, err := git.ResolveRepository(opts.Path)
	if err != nil {
		return err
	}

	configPath := resolveRepoConfigPath(repo.Root, opts.ConfigPath)
	hooks, source, warnings, err := installHooksForTarget(configPath, opts.Hook)
	if err != nil {
		return err
	}

	plan := git.BuildInstallPlan(configPath, repo.EffectiveHooksDir, hooks, true, "", nil)
	if len(plan.HooksWillOverwrite) > 0 && !opts.Force {
		return fmt.Errorf("existing Git hooks detected (%s); rerun with --force to overwrite", joinSorted(plan.HooksWillOverwrite))
	}

	if err := git.WriteHookFiles(repo.EffectiveHooksDir, plan.HookFiles); err != nil {
		return err
	}

	renderer := output.New(rt.Stdout, false, false)
	renderer.Intro("righthook install")
	renderer.Item("repo", repo.Root)
	renderer.Item("hook dir", repo.EffectiveHooksDir)
	renderer.Item("hook source", source)
	renderer.Item("planned hooks", joinSorted(hooks))
	for _, warning := range warnings {
		renderer.Warn(warning)
	}
	renderer.Item("installed hooks", joinSorted(hooks))
	renderer.Countered()
	return nil
}

func installHooksForTarget(configPath, explicitHook string) ([]string, string, []string, error) {
	if explicitHook != "" {
		return []string{explicitHook}, "explicit --hook", nil, nil
	}

	if _, err := os.Stat(configPath); err == nil {
		cfg, err := config.Load(configPath)
		if err != nil {
			hooks := append([]string(nil), cli.SupportedHooks...)
			sort.Strings(hooks)
			return hooks, fmt.Sprintf("fallback because %s is invalid", configPath), []string{
				fmt.Sprintf("could not load %s: %v", configPath, err),
				"falling back to supported v1 hooks",
			}, nil
		}
		hooks := config.HookNamesWithEnabledJobs(cfg.Hooks)
		sort.Strings(hooks)
		return hooks, configPath, nil, nil
	}

	hooks := append([]string(nil), cli.SupportedHooks...)
	sort.Strings(hooks)
	return hooks, "supported v1 defaults", nil, nil
}

func resolveRepoConfigPath(repoRoot, configPath string) string {
	if configPath == "" {
		return filepath.Join(repoRoot, "righthook.yml")
	}
	if filepath.IsAbs(configPath) {
		return configPath
	}
	return filepath.Join(repoRoot, configPath)
}

func joinSorted(values []string) string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}
