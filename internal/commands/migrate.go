package commands

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/config"
	"github.com/almeidazs/righthook/internal/git"
	migratepkg "github.com/almeidazs/righthook/internal/migrate"
	"github.com/almeidazs/righthook/internal/output"
	"github.com/orochaa/go-clack/prompts"
)

var confirmMigrateWrite = func(rt cli.Runtime, path string) (bool, error) {
	return prompts.Confirm(prompts.ConfirmParams{
		Input:        rt.Stdin,
		Output:       os.Stdout,
		Message:      fmt.Sprintf("Write merged config to %s?", path),
		InitialValue: true,
		Active:       "write",
		Inactive:     "cancel",
	})
}

func Migrate(raw cli.MigrateOptions, rt cli.Runtime) error {
	opts, err := cli.ResolveMigrateOptions(raw, rt)
	if err != nil {
		return err
	}

	repo, err := git.ResolveRepository(opts.Path)
	if err != nil {
		return err
	}

	target, err := migratepkg.Load(repo.Root, opts.Target)
	if err != nil {
		return err
	}

	configPath := resolveRepoConfigPath(repo.Root, opts.ConfigPath)
	existing, state, err := loadMigrateBaseConfig(configPath)
	if err != nil {
		return err
	}
	merged := migratepkg.Merge(existing, target)

	format := config.FormatForPath(configPath)
	if format == "" {
		return fmt.Errorf("unsupported config extension for %q", configPath)
	}
	data, err := config.Encode(merged.Config, format)
	if err != nil {
		return err
	}

	renderMigrationPreview(rt, repo.Root, configPath, state, merged, data, opts.DryRun)

	if opts.DryRun {
		return nil
	}

	shouldWrite, err := shouldWriteMigratedConfig(opts, rt, configPath)
	if err != nil {
		return err
	}
	if !shouldWrite {
		return nil
	}

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return err
	}

	removed := []string{}
	if !opts.KeepTargetConfig {
		removed, err = removeMigrationSources(merged.SourcePaths)
		if err != nil {
			return err
		}
	}

	renderer := output.New(rt.Stdout, false, false)
	renderer.Section("Applied")
	renderer.Item("written config", configPath)
	if len(removed) > 0 {
		renderer.Item("removed target config", strings.Join(removed, ", "))
	}
	renderer.Countered()
	return nil
}

func loadMigrateBaseConfig(configPath string) (config.File, string, error) {
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			cfg := config.New(true, "smart")
			cfg.Hooks = map[string]config.Hook{}
			return cfg, "missing", nil
		}
		return config.File{}, "missing", err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return config.File{}, "invalid", fmt.Errorf("load existing righthook config %s: %w", configPath, err)
	}
	return cfg, "found", nil
}

func shouldWriteMigratedConfig(opts cli.ResolvedMigrateOptions, rt cli.Runtime, configPath string) (bool, error) {
	if opts.Write {
		return true, nil
	}
	if !opts.Interactive {
		return false, errors.New("non-interactive migrate requires --write or --dry-run")
	}
	return confirmMigrateWrite(rt, configPath)
}

func renderMigrationPreview(rt cli.Runtime, repoRoot, configPath, state string, merged migratepkg.Result, data []byte, dryRun bool) {
	renderer := output.New(rt.Stdout, false, false)
	renderer.Intro("righthook migrate")
	renderer.Item("repo", repoRoot)
	renderer.Item("target", merged.Target)
	renderer.Item("target config", strings.Join(merged.SourcePaths, ", "))
	renderer.Item("righthook config", configPath)
	renderer.Item("existing config", state)
	if len(merged.MigratedHooks) > 0 {
		renderer.List("migrated hooks", renderHookPlan(merged.MigratedHooks, merged.MigratedJobs))
	} else {
		renderer.Warn("no hooks were added to the merged config")
	}
	for _, warning := range merged.Warnings {
		renderer.Warn(warning)
	}
	if dryRun {
		renderer.Section("Dry run")
	} else {
		renderer.Section("Preview")
	}
	fmt.Fprintln(rt.Stdout, string(data))
}

func removeMigrationSources(paths []string) ([]string, error) {
	removed, err := git.RemoveFiles(paths)
	if err != nil {
		return nil, err
	}
	sort.Strings(removed)
	return removed, nil
}
