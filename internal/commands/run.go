package commands

import (
	"fmt"
	"strings"

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
	renderRun(rt, repo.Root, result, opts.DryRun)
	if err != nil {
		return err
	}
	return nil
}

func renderRun(rt cli.Runtime, repoRoot string, result runpkg.Result, dryRun bool) {
	fmt.Fprintf(rt.Stdout, "\n◇ righthook run\n")
	fmt.Fprintf(rt.Stdout, "│ repo: %s\n", repoRoot)
	fmt.Fprintf(rt.Stdout, "│ config: %s\n", result.ConfigPath)
	fmt.Fprintf(rt.Stdout, "│ hook: %s\n", result.Hook)
	if dryRun {
		fmt.Fprintln(rt.Stdout, "\n◆ Dry run")
	} else {
		fmt.Fprintln(rt.Stdout, "\n◆ Jobs")
	}
	for _, job := range result.Jobs {
		fmt.Fprintf(rt.Stdout, "├─ %s [%s]\n", job.Name, job.Status)
		if job.Command != "" {
			fmt.Fprintf(rt.Stdout, "│  command: %s\n", job.Command)
		}
		if len(job.Files) > 0 {
			fmt.Fprintf(rt.Stdout, "│  files: %s\n", strings.Join(job.Files, ", "))
		}
		if job.CacheEnabled {
			fmt.Fprintf(rt.Stdout, "│  cache: enabled\n")
		}
		if job.Reason != "" {
			fmt.Fprintf(rt.Stdout, "│  note: %s\n", job.Reason)
		}
	}
}
