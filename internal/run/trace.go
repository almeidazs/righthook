package run

import (
	"fmt"
	"strings"
	"time"

	"github.com/almeidazs/righthook/internal/config"
)

type TraceResult struct {
	RepoRoot       string           `json:"repo_root"`
	ConfigPath     string           `json:"config_path"`
	Hook           string           `json:"hook"`
	ResolvedConfig config.File      `json:"resolved_config"`
	Options        TraceOptions     `json:"options"`
	Environment    TraceEnvironment `json:"environment"`
	Files          TraceFiles       `json:"files"`
	Jobs           []TraceJobResult `json:"jobs"`
	Duration       string           `json:"duration"`
}

type TraceOptions struct {
	Args    []string `json:"args,omitempty"`
	NoCache bool     `json:"no_cache"`
	Fix     bool     `json:"fix"`
	DryRun  bool     `json:"dry_run"`
	Changed bool     `json:"changed"`
	Staged  bool     `json:"staged"`
	Only    []string `json:"only,omitempty"`
	Except  []string `json:"except,omitempty"`
}

type TraceEnvironment struct {
	InitialCWD   string         `json:"initial_cwd"`
	EffectiveCWD string         `json:"effective_cwd"`
	UsedWorktree bool           `json:"used_worktree"`
	StashRef     string         `json:"stash_ref,omitempty"`
	HasHEAD      bool           `json:"has_head"`
	RepoState    TraceRepoState `json:"repo_state"`
	InjectedEnv  []string       `json:"injected_env"`
}

type TraceRepoState struct {
	Changed   []string `json:"changed"`
	Staged    []string `json:"staged"`
	Unstaged  []string `json:"unstaged"`
	Untracked []string `json:"untracked"`
	Partial   []string `json:"partial"`
}

type TraceFiles struct {
	All       []string            `json:"all,omitempty"`
	Changed   []string            `json:"changed,omitempty"`
	Staged    []string            `json:"staged,omitempty"`
	Untracked []string            `json:"untracked,omitempty"`
	Affected  map[string][]string `json:"affected,omitempty"`
}

type TraceJobResult struct {
	Name            string           `json:"name"`
	Run             string           `json:"run"`
	ExpandedCommand string           `json:"expanded_command"`
	Status          string           `json:"status"`
	Reason          string           `json:"reason,omitempty"`
	Duration        string           `json:"duration"`
	CWD             string           `json:"cwd"`
	Env             []string         `json:"env"`
	Files           []string         `json:"files,omitempty"`
	FileSelector    string           `json:"file_selector"`
	Glob            []string         `json:"glob,omitempty"`
	Scope           string           `json:"scope,omitempty"`
	Workspace       string           `json:"workspace,omitempty"`
	Base            string           `json:"base,omitempty"`
	StageFixed      bool             `json:"stage_fixed"`
	Cache           TraceCacheResult `json:"cache"`
}

type TraceCacheResult struct {
	Enabled bool   `json:"enabled"`
	Key     string `json:"key,omitempty"`
	Path    string `json:"path,omitempty"`
	TTL     string `json:"ttl,omitempty"`
	Hit     bool   `json:"hit"`
}

func (e Executor) Trace(cfg config.File, opts Options) (TraceResult, error) {
	hook, ok := cfg.Hooks[opts.Hook]
	if !ok {
		return TraceResult{}, fmt.Errorf("hook %s is not configured", opts.Hook)
	}

	jobs, err := selectJobs(hook, opts)
	if err != nil {
		return TraceResult{}, err
	}

	startedAt := time.Now()
	files := newFileInventory(e)
	trace := TraceResult{
		RepoRoot:       e.Repo.Root,
		ConfigPath:     opts.ConfigPath,
		Hook:           opts.Hook,
		ResolvedConfig: cfg,
		Options: TraceOptions{
			Args:    append([]string(nil), opts.Args...),
			NoCache: opts.NoCache,
			Fix:     opts.Fix,
			DryRun:  opts.DryRun,
			Changed: opts.Changed,
			Staged:  opts.Staged,
			Only:    append([]string(nil), opts.Only...),
			Except:  append([]string(nil), opts.Except...),
		},
		Jobs: make([]TraceJobResult, 0, len(jobs)),
	}

	env, err := e.prepareEnvironment(cfg, opts, files, jobs)
	if err != nil {
		return trace, err
	}
	defer func() {
		if cleanupErr := env.cleanup(); cleanupErr != nil {
			fmt.Fprintf(e.stderr(), "righthook cleanup failed: %v\n", cleanupErr)
		}
	}()

	trace.Environment = TraceEnvironment{
		InitialCWD:   e.Repo.Root,
		EffectiveCWD: env.workDir,
		UsedWorktree: env.usedWorktree,
		StashRef:     env.stashRef,
		HasHEAD:      env.hasHEAD,
		RepoState: TraceRepoState{
			Changed:   append([]string(nil), env.repoState.changed...),
			Staged:    append([]string(nil), env.repoState.staged...),
			Unstaged:  append([]string(nil), env.repoState.unstaged...),
			Untracked: append([]string(nil), env.repoState.untracked...),
			Partial:   append([]string(nil), env.repoState.partial...),
		},
		InjectedEnv: []string{
			"RIGHTHOOK_HOOK=" + opts.Hook,
			fmt.Sprintf("RIGHTHOOK_FIX=%t", opts.Fix),
		},
	}

	for _, selected := range jobs {
		jobStartedAt := time.Now()
		runFiles, err := e.filesForJob(selected.Job, opts, files)
		if err != nil {
			return trace, fmt.Errorf("%s: %w", selected.Name, err)
		}

		expansion, err := expandCommand(selected.Job.Run, runFiles, files, opts.Args)
		if err != nil {
			return trace, fmt.Errorf("%s: %w", selected.Name, err)
		}

		jobTrace := TraceJobResult{
			Name:            selected.Name,
			Run:             selected.Job.Run,
			ExpandedCommand: expansion.Command,
			CWD:             env.workDir,
			Env:             append([]string(nil), e.commandEnv(opts)...),
			Files:           append([]string(nil), runFiles...),
			FileSelector:    resolveFileSource(selected.Job, opts),
			Glob:            append([]string(nil), selected.Job.Glob...),
			Scope:           strings.TrimSpace(selected.Job.Scope),
			Workspace:       strings.TrimSpace(selected.Job.Workspace),
			Base:            strings.TrimSpace(selected.Job.Base),
			StageFixed:      selected.Job.StageFixed,
			Cache: TraceCacheResult{
				Enabled: cacheEnabled(cfg, selected.Job, opts),
			},
		}
		if jobTrace.Cache.Enabled {
			meta, err := e.cacheMetadata(cfg, opts.Hook, selected.Name, expansion.Command, runFiles)
			if err != nil {
				return trace, err
			}
			jobTrace.Cache.Key = meta.Key
			jobTrace.Cache.Path = meta.Path
			jobTrace.Cache.TTL = meta.TTL.String()
		}

		switch {
		case expansion.HasEmptyFilePlaceholder:
			jobTrace.Status = "skipped"
			jobTrace.Reason = "no files matched command placeholders"
		case opts.DryRun:
			jobTrace.Status = "dry-run"
		case strings.TrimSpace(expansion.Command) == "":
			jobTrace.Status = "skipped"
			jobTrace.Reason = "empty command after placeholder expansion"
		default:
			if jobTrace.Cache.Enabled {
				hit, err := e.cacheHit(cfg, opts.Hook, selected.Name, expansion.Command, runFiles)
				if err != nil {
					return trace, err
				}
				jobTrace.Cache.Hit = hit
				if hit {
					jobTrace.Status = "cached"
					jobTrace.Reason = "cache hit"
					jobTrace.Duration = time.Since(jobStartedAt).String()
					trace.Jobs = append(trace.Jobs, jobTrace)
					continue
				}
			}

			if err := e.executeCommand(expansion.Command, env.workDir, opts); err != nil {
				jobTrace.Status = "failed"
				jobTrace.Duration = time.Since(jobStartedAt).String()
				trace.Jobs = append(trace.Jobs, jobTrace)
				return trace, err
			}
			if selected.Job.StageFixed && len(runFiles) > 0 {
				if err := env.syncStageFixed(runFiles); err != nil {
					return trace, fmt.Errorf("%s: stage_fixed git add failed: %w", selected.Name, err)
				}
			}
			if jobTrace.Cache.Enabled {
				if err := e.writeCache(cfg, opts.Hook, selected.Name, expansion.Command, runFiles); err != nil {
					return trace, err
				}
			}
			jobTrace.Status = "ran"
		}

		jobTrace.Duration = time.Since(jobStartedAt).String()
		trace.Jobs = append(trace.Jobs, jobTrace)
	}

	all, _ := files.All()
	changed, _ := files.Changed()
	staged, _ := files.Staged()
	untracked, _ := files.Untracked()
	affected := make(map[string][]string, len(files.affected))
	for base, resolved := range files.affected {
		affected[base] = append([]string(nil), resolved...)
	}
	trace.Files = TraceFiles{
		All:       all,
		Changed:   changed,
		Staged:    staged,
		Untracked: untracked,
		Affected:  affected,
	}
	trace.Duration = time.Since(startedAt).String()
	return trace, nil
}
