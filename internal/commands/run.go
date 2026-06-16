package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/config"
	"github.com/almeidazs/righthook/internal/git"
	runpkg "github.com/almeidazs/righthook/internal/run"
)

func Run(raw cli.RunOptions, rt cli.Runtime) error {
	opts, err := cli.ResolveRunOptions(raw)
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

	result, err := runpkg.Executor{
		Repo:   repo,
		Stdout: rt.Stdout,
		Stderr: rt.Stderr,
	}.Run(cfg, runpkg.Options{
		Hook:       opts.Hook,
		Args:       opts.Args,
		NoCache:    opts.NoCache,
		Fix:        opts.Fix,
		DryRun:     opts.DryRun,
		Changed:    opts.Changed,
		Staged:     opts.Staged,
		Only:       opts.Only,
		Except:     opts.Except,
		ConfigPath: configPath,
	})
	renderRun(rt, repo.Root, cfg.Output, result, opts.DryRun)
	if err != nil {
		return err
	}
	return nil
}

func renderRun(rt cli.Runtime, repoRoot string, outputCfg config.OutputConfig, result runpkg.Result, dryRun bool) {
	fmt.Fprintf(rt.Stdout, "\n◇ righthook run\n")
	fmt.Fprintf(rt.Stdout, "│ repo: %s\n", repoRoot)
	fmt.Fprintf(rt.Stdout, "│ config: %s\n", result.ConfigPath)
	fmt.Fprintf(rt.Stdout, "│ hook: %s\n", result.Hook)
	if dryRun {
		fmt.Fprintln(rt.Stdout, "\n◆ Dry run")
	} else {
		fmt.Fprintln(rt.Stdout, "\n◆ Jobs")
	}
	showVerbose := outputCfg.Mode == "verbose" || dryRun
	showTiming := outputCfg.Timing
	hiddenSuccesses := 0
	for _, job := range result.Jobs {
		if !dryRun && !outputCfg.ShowSuccess && job.Status == "ran" {
			hiddenSuccesses++
			continue
		}
		fmt.Fprintf(rt.Stdout, "├─ %s [%s]\n", job.Name, job.Status)
		if showVerbose && job.Command != "" {
			fmt.Fprintf(rt.Stdout, "│  command: %s\n", job.Command)
		}
		if showVerbose && len(job.Files) > 0 {
			fmt.Fprintf(rt.Stdout, "│  files: %s\n", strings.Join(job.Files, ", "))
		}
		if showVerbose && job.CacheEnabled {
			fmt.Fprintf(rt.Stdout, "│  cache: enabled\n")
		}
		if job.Reason != "" {
			fmt.Fprintf(rt.Stdout, "│  note: %s\n", job.Reason)
		}
		if showTiming {
			fmt.Fprintf(rt.Stdout, "│  timing: %s\n", job.Duration.Round(time.Millisecond))
		}
	}
	if hiddenSuccesses > 0 {
		fmt.Fprintf(rt.Stdout, "│ hidden successful jobs: %d\n", hiddenSuccesses)
	}
	if showTiming {
		fmt.Fprintf(rt.Stdout, "│ total: %s\n", result.Duration.Round(time.Millisecond))
	}
}
