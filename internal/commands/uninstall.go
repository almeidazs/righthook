package commands

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/config"
	"github.com/almeidazs/righthook/internal/git"
	"github.com/almeidazs/righthook/internal/output"
	"github.com/orochaa/go-clack/prompts"
)

func Uninstall(raw cli.UninstallOptions, rt cli.Runtime) error {
	opts, err := cli.ResolveUninstallOptions(raw, rt)
	if err != nil {
		return err
	}

	repo, err := git.ResolveRepository(opts.Path)
	if err != nil {
		return err
	}

	installed := git.ListInstalledHooks(repo, cli.SupportedHooks)
	installedByName := mapHookFiles(installed)
	righthookInstalled := ownedHookFiles(installed)

	selected, err := selectHooksForRemoval(opts, rt, righthookInstalled, installedByName)
	if err != nil {
		return err
	}

	removedHooks := []string{}
	skippedHooks := []string{}
	if len(selected) > 0 {
		removedHooks, skippedHooks, err = git.RemoveHookFiles(selected)
		if err != nil {
			return err
		}
	}

	removedConfigs := []string{}
	if opts.RemoveConfig {
		configTargets := []string{resolveRepoConfigPath(repo.Root, "")}
		if opts.ConfigPath != "" {
			configTargets = append(configTargets, resolveRepoConfigPath(repo.Root, opts.ConfigPath))
		} else {
			configTargets = append(configTargets, config.DefaultPath(repo.Root))
		}
		removedConfigs, err = git.RemoveFiles(configTargets)
		if err != nil {
			return err
		}
	}

	if len(removedHooks) == 0 && len(removedConfigs) == 0 {
		return errors.New("nothing to uninstall")
	}

	renderer := output.New(rt.Stdout, false, false)
	renderer.Intro("righthook uninstall")
	renderer.Item("repo", repo.Root)
	if len(removedHooks) > 0 {
		renderer.Item("removed hooks", joinSorted(removedHooks))
	}
	if len(skippedHooks) > 0 {
		renderer.Warn(fmt.Sprintf("skipped hooks not managed by Righthook: %s", joinSorted(skippedHooks)))
	}
	if len(removedConfigs) > 0 {
		renderer.Item("removed config", joinSorted(removedConfigs))
	}
	renderer.Countered()
	return nil
}

func selectHooksForRemoval(opts cli.ResolvedUninstallOptions, rt cli.Runtime, managed []git.HookFile, installed map[string]git.HookFile) ([]git.HookFile, error) {
	if opts.Hook != "" {
		hook, ok := installed[opts.Hook]
		if !ok {
			return nil, fmt.Errorf("hook %s is not installed", opts.Hook)
		}
		if !hook.IsRighthook {
			return nil, fmt.Errorf("hook %s is not managed by Righthook", opts.Hook)
		}
		return []git.HookFile{hook}, nil
	}

	if opts.All {
		if len(managed) == 0 {
			return nil, nil
		}
		return managed, nil
	}

	if len(managed) == 0 {
		return nil, nil
	}

	if !opts.Interactive {
		return nil, errors.New("non-interactive uninstall requires --all or --hook")
	}

	if len(managed) == 1 {
		return managed, nil
	}

	options := make([]*prompts.MultiSelectOption[string], 0, len(managed))
	initial := make([]string, 0, len(managed))
	for _, hook := range managed {
		options = append(options, &prompts.MultiSelectOption[string]{
			Label:      hook.Name,
			Value:      hook.Name,
			IsSelected: true,
		})
		initial = append(initial, hook.Name)
	}
	values, err := prompts.MultiSelect(prompts.MultiSelectParams[string]{
		Input:        rt.Stdin,
		Output:       os.Stdout,
		Message:      "Which hooks should Righthook remove?",
		Options:      options,
		InitialValue: initial,
		Required:     true,
	})
	if err != nil {
		return nil, err
	}

	selection := make([]git.HookFile, 0, len(values))
	for _, value := range values {
		selection = append(selection, installed[value])
	}
	return selection, nil
}

func mapHookFiles(files []git.HookFile) map[string]git.HookFile {
	out := make(map[string]git.HookFile, len(files))
	for _, file := range files {
		out[file.Name] = file
	}
	return out
}

func ownedHookFiles(files []git.HookFile) []git.HookFile {
	owned := make([]git.HookFile, 0, len(files))
	for _, file := range files {
		if file.IsRighthook {
			owned = append(owned, file)
		}
	}
	sort.Slice(owned, func(i, j int) bool {
		return owned[i].Name < owned[j].Name
	})
	return owned
}
