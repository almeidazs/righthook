package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/config"
	"github.com/almeidazs/righthook/internal/git"
	runpkg "github.com/almeidazs/righthook/internal/run"
)

func Trace(raw cli.TraceOptions, rt cli.Runtime) error {
	opts, err := cli.ResolveTraceOptions(raw)
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
	}.Trace(cfg, runpkg.Options{
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
	if err != nil {
		return err
	}

	renderTrace(rt, result)

	if opts.OutputPath != "" {
		if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
			return err
		}
		file, err := os.Create(opts.OutputPath)
		if err != nil {
			return err
		}
		defer file.Close()

		enc := json.NewEncoder(file)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return err
		}
		fmt.Fprintf(rt.Stdout, "\ntrace json: %s\n", opts.OutputPath)
	}
	return nil
}

func renderTrace(rt cli.Runtime, result runpkg.TraceResult) {
	fmt.Fprintf(rt.Stdout, "\n◇ righthook trace\n")
	fmt.Fprintf(rt.Stdout, "│ repo: %s\n", result.RepoRoot)
	fmt.Fprintf(rt.Stdout, "│ config: %s\n", result.ConfigPath)
	fmt.Fprintf(rt.Stdout, "│ hook: %s\n", result.Hook)
	fmt.Fprintf(rt.Stdout, "│ cwd: %s\n", result.Environment.EffectiveCWD)
	fmt.Fprintf(rt.Stdout, "│ total: %s\n", result.Duration)

	fmt.Fprintln(rt.Stdout, "\n◆ Environment")
	fmt.Fprintf(rt.Stdout, "├─ has_head: %t\n", result.Environment.HasHEAD)
	fmt.Fprintf(rt.Stdout, "├─ used_worktree: %t\n", result.Environment.UsedWorktree)
	if result.Environment.StashRef != "" {
		fmt.Fprintf(rt.Stdout, "├─ stash_ref: %s\n", result.Environment.StashRef)
	}
	fmt.Fprintf(rt.Stdout, "├─ repo_state.changed: %s\n", joinOrNone(result.Environment.RepoState.Changed))
	fmt.Fprintf(rt.Stdout, "├─ repo_state.staged: %s\n", joinOrNone(result.Environment.RepoState.Staged))
	fmt.Fprintf(rt.Stdout, "├─ repo_state.unstaged: %s\n", joinOrNone(result.Environment.RepoState.Unstaged))
	fmt.Fprintf(rt.Stdout, "├─ repo_state.untracked: %s\n", joinOrNone(result.Environment.RepoState.Untracked))
	fmt.Fprintf(rt.Stdout, "└─ injected_env: %s\n", joinOrNone(result.Environment.InjectedEnv))

	fmt.Fprintln(rt.Stdout, "\n◆ Files")
	fmt.Fprintf(rt.Stdout, "├─ staged: %s\n", joinOrNone(result.Files.Staged))
	fmt.Fprintf(rt.Stdout, "├─ changed: %s\n", joinOrNone(result.Files.Changed))
	fmt.Fprintf(rt.Stdout, "├─ untracked: %s\n", joinOrNone(result.Files.Untracked))
	if len(result.Files.Affected) == 0 {
		fmt.Fprintf(rt.Stdout, "└─ affected: none\n")
	} else {
		i := 0
		for base, files := range result.Files.Affected {
			prefix := "├─"
			if i == len(result.Files.Affected)-1 {
				prefix = "└─"
			}
			fmt.Fprintf(rt.Stdout, "%s affected[%s]: %s\n", prefix, base, joinOrNone(files))
			i++
		}
	}

	fmt.Fprintln(rt.Stdout, "\n◆ Jobs")
	for _, job := range result.Jobs {
		fmt.Fprintf(rt.Stdout, "├─ %s [%s]\n", job.Name, job.Status)
		fmt.Fprintf(rt.Stdout, "│  duration: %s\n", job.Duration)
		fmt.Fprintf(rt.Stdout, "│  selector: %s\n", job.FileSelector)
		fmt.Fprintf(rt.Stdout, "│  cwd: %s\n", job.CWD)
		fmt.Fprintf(rt.Stdout, "│  run: %s\n", job.Run)
		fmt.Fprintf(rt.Stdout, "│  command: %s\n", job.ExpandedCommand)
		fmt.Fprintf(rt.Stdout, "│  files: %s\n", joinOrNone(job.Files))
		if len(job.Glob) > 0 {
			fmt.Fprintf(rt.Stdout, "│  glob: %s\n", joinOrNone(job.Glob))
		}
		if job.Scope != "" {
			fmt.Fprintf(rt.Stdout, "│  scope: %s\n", job.Scope)
		}
		if job.Workspace != "" {
			fmt.Fprintf(rt.Stdout, "│  workspace: %s\n", job.Workspace)
		}
		if job.Base != "" {
			fmt.Fprintf(rt.Stdout, "│  base: %s\n", job.Base)
		}
		if job.Cache.Enabled {
			fmt.Fprintf(rt.Stdout, "│  cache: enabled\n")
			fmt.Fprintf(rt.Stdout, "│  cache_key: %s\n", job.Cache.Key)
			fmt.Fprintf(rt.Stdout, "│  cache_path: %s\n", job.Cache.Path)
			fmt.Fprintf(rt.Stdout, "│  cache_ttl: %s\n", job.Cache.TTL)
			fmt.Fprintf(rt.Stdout, "│  cache_hit: %t\n", job.Cache.Hit)
		}
		fmt.Fprintf(rt.Stdout, "│  env: %s\n", joinOrNone(job.Env))
		if job.Reason != "" {
			fmt.Fprintf(rt.Stdout, "│  note: %s\n", job.Reason)
		}
	}
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	if len(values) <= 3 {
		return strings.Join(values, ", ")
	}
	return fmt.Sprintf("%s, [%d more...]", strings.Join(values[:3], ", "), len(values)-3)
}
