package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/config"
	"github.com/almeidazs/righthook/internal/detect"
	"github.com/almeidazs/righthook/internal/git"
	"github.com/almeidazs/righthook/internal/output"
	"github.com/almeidazs/righthook/internal/presets"
	"github.com/orochaa/go-clack/prompts"
)

func Init(raw cli.InitOptions, rt cli.Runtime) error {
	opts, err := cli.ResolveInitOptions(raw, rt)
	if err != nil {
		return err
	}

	renderer := output.New(rt.Stdout, opts.NoEmoji, opts.JSON)

	if !opts.Yes && !opts.Interactive && !opts.PrintOnly && !opts.JSON {
		return errors.New("interactive init requires a TTY; rerun with `righthook init --yes --install`")
	}

	detection, err := detect.Scan(opts.CWD)
	if err != nil {
		return err
	}

	effectiveOpts := opts
	var rec presets.Recommendation
	if opts.Interactive && !opts.Yes {
		effectiveOpts, rec, err = interactiveRecommendation(opts, detection)
	} else {
		rec, err = presets.Build(detection, effectiveOpts)
	}
	if err != nil {
		return err
	}

	configBytes, err := config.Encode(rec.Config, opts.ConfigFormat)
	if err != nil {
		return err
	}
	if _, err := config.Decode(configBytes, opts.ConfigFormat); err != nil {
		return fmt.Errorf("validate generated config: %w", err)
	}

	install := opts.Install
	if !raw.InstallSpecified && opts.Interactive && !opts.Yes && !opts.PrintOnly && !opts.DryRun {
		renderer.Spacer()
		install, err = prompts.Confirm(prompts.ConfirmParams{
			Input:        rt.Stdin,
			Output:       os.Stdout,
			Message:      "Install Git hook scripts now?",
			InitialValue: true,
			Active:       "yes",
			Inactive:     "no",
		})
		if err != nil {
			return err
		}
	}

	gitignoreAdditions := []string{}
	if rec.UpdateGitIgnore {
		gitignoreAdditions = []string{".righthook/cache", ".righthook/stats.json", "righthook.local.yml"}
	}
	plan := git.BuildInstallPlan(
		opts.ConfigPath,
		detection.EffectiveHooksDir,
		rec.SelectedHooks,
		install && !opts.PrintOnly && !opts.DryRun,
		filepath.Join(detection.RepoRoot, ".gitignore"),
		gitignoreAdditions,
	)

	if !opts.Force {
		if plan.ConfigWillOverwrite {
			if opts.Yes || !opts.Interactive {
				return fmt.Errorf("%s already exists; rerun with --force to overwrite", opts.ConfigPath)
			}
			ok, err := prompts.Confirm(prompts.ConfirmParams{
				Input:        rt.Stdin,
				Output:       os.Stdout,
				Message:      fmt.Sprintf("Overwrite existing %s?", opts.ConfigPath),
				InitialValue: false,
				Active:       "overwrite",
				Inactive:     "cancel",
			})
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("init cancelled")
			}
		}
		if len(plan.HooksWillOverwrite) > 0 {
			if opts.Yes || !opts.Interactive {
				return fmt.Errorf("existing Git hooks detected (%s); rerun with --force to overwrite", strings.Join(plan.HooksWillOverwrite, ", "))
			}
			ok, err := prompts.Confirm(prompts.ConfirmParams{
				Input:        rt.Stdin,
				Output:       os.Stdout,
				Message:      fmt.Sprintf("Overwrite existing hooks (%s)?", strings.Join(plan.HooksWillOverwrite, ", ")),
				InitialValue: false,
				Active:       "overwrite",
				Inactive:     "cancel",
			})
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("init cancelled")
			}
		}
	}

	if opts.PrintOnly {
		_, err := rt.Stdout.Write(configBytes)
		return err
	}

	if !opts.JSON {
		renderer.Intro("righthook init")
		renderer.Item("repo", detection.RepoRoot)
		renderer.Item("config", opts.ConfigPath)
		if len(detection.Languages) > 0 {
			renderer.Item("languages", strings.Join(detection.Languages, ", "))
		}
		if effectiveOpts.Preset != "" || (!effectiveOpts.SkipPreset && (contains(detection.PresetCandidates, "go") || contains(detection.PresetCandidates, "rust") || contains(detection.PresetCandidates, "python") || contains(detection.PresetCandidates, "node") || contains(detection.PresetCandidates, "next") || contains(detection.PresetCandidates, "nestjs") || contains(detection.PresetCandidates, "monorepo"))) {
			preset := effectiveOpts.Preset
			if preset == "" && !effectiveOpts.SkipPreset {
				preset = inferredPresetLabel(detection)
			}
			if preset != "" {
				renderer.Item("preset", preset)
			}
		}
		if rec.SelectedPM != "" {
			renderer.Item("package manager", rec.SelectedPM)
		}
		renderer.List("planned hooks", renderHookPlan(rec.SelectedHooks, rec.SelectedJobs))
		for _, warning := range rec.Warnings {
			renderer.Warn(warning)
		}
	}

	if opts.DryRun {
		renderer.DryRun(plan, rec.SelectedHooks, rec.SelectedJobs)
		if opts.JSON {
			return renderer.JSON(output.Result{
				Detection: detection,
				Options:   effectiveOpts,
				Plan:      plan,
				Selected:  rec,
				Validated: true,
			})
		}
		return nil
	}

	if len(plan.GitIgnoreAdditions) > 0 && opts.Interactive && !opts.Yes && !opts.JSON {
		renderer.Spacer()
		ok, err := prompts.Confirm(prompts.ConfirmParams{
			Input:        rt.Stdin,
			Output:       os.Stdout,
			Message:      "Update .gitignore with Righthook cache/local config entries?",
			InitialValue: true,
			Active:       "yes",
			Inactive:     "no",
		})
		if err != nil {
			return err
		}
		if !ok {
			plan.GitIgnoreAdditions = nil
		}
	}

	if err := git.Apply(plan, configBytes); err != nil {
		return err
	}

	loaded, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("validate written config: %w", err)
	}
	if err := config.Validate(loaded); err != nil {
		return err
	}

	if opts.JSON {
		return renderer.JSON(output.Result{
			Detection:  detection,
			Options:    effectiveOpts,
			Plan:       plan,
			Selected:   rec,
			Validated:  true,
			Validation: "ok",
		})
	}

	renderer.Success("Righthook initialized")
	renderer.Section("Summary")
	renderer.Item("config path", opts.ConfigPath)
	if install {
		renderer.Item("installed hooks", strings.Join(rec.SelectedHooks, ", "))
	} else {
		renderer.Item("installed hooks", "none")
	}
	if len(plan.GitIgnoreAdditions) > 0 {
		renderer.Item("gitignore", strings.Join(plan.GitIgnoreAdditions, ", "))
	}
	renderer.List("useful commands", []string{
		"righthook run pre-commit",
		"righthook run pre-push",
		"righthook validate",
	})
	renderer.Countered()
	return nil
}

func interactiveRecommendation(opts cli.ResolvedInitOptions, d detect.Result) (cli.ResolvedInitOptions, presets.Recommendation, error) {
	next := opts
	if !next.ModeSpecified {
		mode, err := prompts.Select(prompts.SelectParams[string]{
			Input:   os.Stdin,
			Output:  os.Stdout,
			Message: "How opinionated should the starter config be?",
			Options: []*prompts.SelectOption[string]{
				{Label: "recommended", Value: "recommended", Hint: "balanced default"},
				{Label: "minimal", Value: "minimal", Hint: "lightweight"},
				{Label: "strict", Value: "strict", Hint: "heavier checks"},
				{Label: "custom", Value: "custom", Hint: "pick hooks and jobs"},
			},
			InitialValue: string(cli.ModeRecommended),
		})
		if err != nil {
			return opts, presets.Recommendation{}, err
		}
		next.Mode = cli.Mode(mode)
	}

	if next.Preset == "" {
		detectedPreset := inferredPresetLabel(d)
		if detectedPreset != "" {
			action, err := prompts.Select(prompts.SelectParams[string]{
				Input:   os.Stdin,
				Output:  os.Stdout,
				Message: fmt.Sprintf("We found the preset %q. Is this right?", detectedPreset),
				Options: []*prompts.SelectOption[string]{
					{Label: "Yes", Value: "yes", Hint: "use the detected preset"},
					{Label: "No, pick one", Value: "pick", Hint: "choose another preset"},
					{Label: "Skip preset", Value: "skip", Hint: "use raw detection only"},
				},
				InitialValue: "yes",
			})
			if err != nil {
				return opts, presets.Recommendation{}, err
			}
			switch action {
			case "yes":
				next.Preset = detectedPreset
			case "pick":
				preset, skipped, err := promptSupportedPresetSelection(detectedPreset)
				if err != nil {
					return opts, presets.Recommendation{}, err
				}
				next.Preset = preset
				next.SkipPreset = skipped
			case "skip":
				next.SkipPreset = true
			}
		} else {
			preset, skipped, err := promptSupportedPresetSelection("")
			if err != nil {
				return opts, presets.Recommendation{}, err
			}
			next.Preset = preset
			next.SkipPreset = skipped
		}
	}

	pm := d.PackageManager
	if len(d.PackageManagers) > 1 {
		var err error
		pm, err = prompts.Select(prompts.SelectParams[string]{
			Input:   os.Stdin,
			Output:  os.Stdout,
			Message: "Multiple package managers detected. Which one should Righthook use?",
			Options: selectOptions(d.PackageManagers),
		})
		if err != nil {
			return opts, presets.Recommendation{}, err
		}
	}

	next.PM = pm
	next.PMProvided = pm != ""
	if len(d.FormatterChoices) > 1 {
		formatter, err := prompts.Select(prompts.SelectParams[string]{
			Input:   os.Stdin,
			Output:  os.Stdout,
			Message: "Multiple formatters detected. Which formatter should Righthook prefer?",
			Options: selectOptions(d.FormatterChoices),
		})
		if err != nil {
			return opts, presets.Recommendation{}, err
		}
		next.Formatter = formatter
	}

	rec, err := presets.Build(d, next)
	if err != nil {
		return next, rec, err
	}

	if len(next.Hooks) == 0 {
		hookOptions := make([]*prompts.MultiSelectOption[string], 0, len(rec.SelectedHooks))
		initialHooks := make([]string, 0, len(rec.SelectedHooks))
		for _, hook := range rec.SelectedHooks {
			hookOptions = append(hookOptions, &prompts.MultiSelectOption[string]{
				Label:      hook,
				Value:      hook,
				IsSelected: true,
			})
			initialHooks = append(initialHooks, hook)
		}
		if len(hookOptions) > 1 || next.Mode == cli.ModeCustom {
			hooks, err := prompts.MultiSelect(prompts.MultiSelectParams[string]{
				Input:        os.Stdin,
				Output:       os.Stdout,
				Message:      "Which hooks should Righthook enable?",
				Options:      hookOptions,
				InitialValue: initialHooks,
				Required:     true,
			})
			if err != nil {
				return opts, presets.Recommendation{}, err
			}
			next.Hooks = hooks
			rec, err = presets.Build(d, next)
			if err != nil {
				return next, rec, err
			}
		}
	}

	if next.CacheEnabled == nil {
		cacheEnabled, err := prompts.Confirm(prompts.ConfirmParams{
			Input:        os.Stdin,
			Output:       os.Stdout,
			Message:      "Enable cache?",
			InitialValue: rec.Config.Cache.Enabled,
			Active:       "yes",
			Inactive:     "no",
		})
		if err != nil {
			return opts, presets.Recommendation{}, err
		}
		next.CacheEnabled = &cacheEnabled
		rec, err = presets.Build(d, next)
		if err != nil {
			return next, rec, err
		}
	}

	if next.Safety == "" {
		safety, err := prompts.Select(prompts.SelectParams[string]{
			Input:   os.Stdin,
			Output:  os.Stdout,
			Message: "Which safety mode should Righthook use?",
			Options: []*prompts.SelectOption[string]{
				{Label: "smart", Value: "smart", Hint: "recommended"},
				{Label: "fast", Value: "fast"},
				{Label: "strict", Value: "strict"},
				{Label: "off", Value: "off"},
			},
			InitialValue: rec.Config.Safety.Isolation,
		})
		if err != nil {
			return opts, presets.Recommendation{}, err
		}
		next.Safety = safety
		rec, err = presets.Build(d, next)
		if err != nil {
			return next, rec, err
		}
	}

	jobOptions := make([]*prompts.MultiSelectOption[string], 0)
	initial := make([]string, 0)
	for _, hook := range rec.SelectedHooks {
		for _, job := range rec.SelectedJobs[hook] {
			value := hook + ":" + job
			jobOptions = append(jobOptions, &prompts.MultiSelectOption[string]{
				Label:      value,
				Value:      value,
				IsSelected: true,
			})
			initial = append(initial, value)
		}
	}
	if len(jobOptions) > 1 || next.Mode == cli.ModeCustom {
		selectedJobs, err := prompts.MultiSelect(prompts.MultiSelectParams[string]{
			Input:        os.Stdin,
			Output:       os.Stdout,
			Message:      "Which jobs should Righthook enable?",
			Options:      jobOptions,
			InitialValue: initial,
			Required:     true,
		})
		if err != nil {
			return opts, presets.Recommendation{}, err
		}
		filterRecommendationJobs(&rec, selectedJobs)
	}
	return next, rec, nil
}

func selectOptions(values []string) []*prompts.SelectOption[string] {
	out := make([]*prompts.SelectOption[string], 0, len(values))
	for _, value := range values {
		out = append(out, &prompts.SelectOption[string]{Label: value, Value: value})
	}
	return out
}

func supportedPresetOptions(recommended string, includeSkip bool) []*prompts.SelectOption[string] {
	options := make([]*prompts.SelectOption[string], 0, 8)
	for _, preset := range []string{"node", "next", "nestjs", "monorepo", "go", "rust", "python"} {
		hint := supportedPresetHint(preset)
		if preset == recommended {
			hint = "recommended"
		}
		options = append(options, &prompts.SelectOption[string]{
			Label: preset,
			Value: preset,
			Hint:  hint,
		})
	}
	if includeSkip {
		options = append(options, &prompts.SelectOption[string]{
			Label: "Skip preset",
			Value: "__skip__",
			Hint:  "use raw detection only",
		})
	}
	return options
}

func supportedPresetHint(preset string) string {
	switch preset {
	case "node":
		return "generic JS/TS repo"
	case "next":
		return "Next.js app"
	case "nestjs":
		return "NestJS service"
	case "monorepo":
		return "workspace-aware setup"
	case "go":
		return "Go module"
	case "rust":
		return "Cargo crate or workspace"
	case "python":
		return "Python project"
	default:
		return ""
	}
}

func promptSupportedPresetSelection(recommended string) (string, bool, error) {
	message := "I couldn't confidently detect a preset. Pick one or skip."
	if recommended != "" {
		message = "Pick a preset or skip."
	}
	preset, err := prompts.Select(prompts.SelectParams[string]{
		Input:        os.Stdin,
		Output:       os.Stdout,
		Message:      message,
		Options:      supportedPresetOptions(recommended, true),
		InitialValue: recommended,
	})
	if err != nil {
		return "", false, err
	}
	if preset == "__skip__" {
		return "", true, nil
	}
	return preset, false, nil
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func inferredPresetLabel(d detect.Result) string {
	for _, preset := range []string{"next", "nestjs", "monorepo", "go", "rust", "python", "node"} {
		if contains(d.PresetCandidates, preset) {
			return preset
		}
	}
	return ""
}

func renderHookPlan(hooks []string, jobs map[string][]string) []string {
	lines := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		selectedJobs := jobs[hook]
		if len(selectedJobs) == 0 {
			lines = append(lines, hook)
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", hook, strings.Join(selectedJobs, ", ")))
	}
	return lines
}

func filterRecommendationJobs(rec *presets.Recommendation, selected []string) {
	keep := map[string]bool{}
	for _, item := range selected {
		keep[item] = true
	}

	for hookName, hook := range rec.Config.Hooks {
		jobs := map[string]config.Job{}
		rec.SelectedJobs[hookName] = nil
		for jobName, job := range hook.Jobs {
			key := hookName + ":" + jobName
			if keep[key] {
				jobs[jobName] = job
				rec.SelectedJobs[hookName] = append(rec.SelectedJobs[hookName], jobName)
			}
		}
		if len(jobs) == 0 {
			delete(rec.Config.Hooks, hookName)
			delete(rec.SelectedJobs, hookName)
			continue
		}
		hook.Jobs = jobs
		rec.Config.Hooks[hookName] = hook
	}
	rec.SelectedHooks = nil
	for _, hook := range []string{"pre-commit", "commit-msg", "pre-push"} {
		if _, ok := rec.Config.Hooks[hook]; ok {
			rec.SelectedHooks = append(rec.SelectedHooks, hook)
		}
	}
}
