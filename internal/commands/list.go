package commands

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/config"
	"github.com/almeidazs/righthook/internal/git"
)

type listHook struct {
	Name string    `json:"name"`
	Jobs []listJob `json:"jobs"`
}

type listJob struct {
	Hook    string `json:"hook,omitempty"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Run     string `json:"run,omitempty"`
	Status  string `json:"status"`
}

type listResult struct {
	ConfigPath string     `json:"config_path"`
	Hooks      []listHook `json:"hooks,omitempty"`
	Jobs       []listJob  `json:"jobs,omitempty"`
}

func List(raw cli.ListOptions, rt cli.Runtime) error {
	opts, err := cli.ResolveListOptions(raw)
	if err != nil {
		return err
	}

	repo, err := git.ResolveRepository(opts.Path)
	if err != nil {
		return err
	}

	configPath := resolveRepoConfigPath(repo.Root, opts.ConfigPath)
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	result := buildListResult(configPath, cfg, opts.OnlyJobs)
	if opts.JSON {
		enc := json.NewEncoder(rt.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	return renderList(rt, result, opts.OnlyJobs)
}

func buildListResult(configPath string, cfg config.File, onlyJobs bool) listResult {
	hookNames := make([]string, 0, len(cfg.Hooks))
	for name := range cfg.Hooks {
		hookNames = append(hookNames, name)
	}
	sort.Strings(hookNames)

	result := listResult{ConfigPath: configPath}
	if onlyJobs {
		for _, hookName := range hookNames {
			result.Jobs = append(result.Jobs, flattenHookJobs(hookName, cfg.Hooks[hookName])...)
		}
		return result
	}

	result.Hooks = make([]listHook, 0, len(hookNames))
	for _, hookName := range hookNames {
		hook := cfg.Hooks[hookName]
		result.Hooks = append(result.Hooks, listHook{
			Name: hookName,
			Jobs: flattenHookJobs(hookName, hook),
		})
	}
	return result
}

func flattenHookJobs(hookName string, hook config.Hook) []listJob {
	jobNames := make([]string, 0, len(hook.Jobs))
	for name := range hook.Jobs {
		jobNames = append(jobNames, name)
	}
	sort.Strings(jobNames)

	jobs := make([]listJob, 0, len(jobNames))
	for _, jobName := range jobNames {
		job := hook.Jobs[jobName]
		enabled := job.IsEnabled()
		status := "enabled"
		if !enabled {
			status = "disabled"
		}
		jobs = append(jobs, listJob{
			Hook:    hookName,
			Name:    jobName,
			Enabled: enabled,
			Run:     job.Run,
			Status:  status,
		})
	}
	return jobs
}

func renderList(rt cli.Runtime, result listResult, onlyJobs bool) error {
	if onlyJobs {
		for _, job := range result.Jobs {
			fmt.Fprintln(rt.Stdout, formatJobLine(job))
		}
		return nil
	}

	for i, hook := range result.Hooks {
		if i > 0 {
			fmt.Fprintln(rt.Stdout)
		}
		fmt.Fprintln(rt.Stdout, hook.Name)
		for _, job := range hook.Jobs {
			fmt.Fprintf(rt.Stdout, "  %s\n", formatJobLine(job))
		}
	}

	return nil
}

func formatJobLine(job listJob) string {
	if !job.Enabled {
		return fmt.Sprintf("- %-12s disabled", job.Name)
	}
	return fmt.Sprintf("✓ %-12s %s", job.Name, job.Run)
}
