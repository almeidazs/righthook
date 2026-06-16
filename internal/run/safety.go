package run

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/almeidazs/righthook/internal/config"
)

type executionEnv struct {
	workDir        string
	cleanupFuncs   []func() error
	stageFixedFunc func([]string) error
}

func (e executionEnv) cleanup() error {
	var errs []string
	for i := len(e.cleanupFuncs) - 1; i >= 0; i-- {
		if err := e.cleanupFuncs[i](); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "; "))
}

func (e executionEnv) syncStageFixed(files []string) error {
	if e.stageFixedFunc == nil {
		return nil
	}
	return e.stageFixedFunc(files)
}

type repoState struct {
	changed   []string
	staged    []string
	unstaged  []string
	untracked []string
	partial   []string
}

func (e Executor) prepareEnvironment(cfg config.File, opts Options, files *fileInventory, jobs []selectedJob) (executionEnv, error) {
	env := executionEnv{
		workDir: e.Repo.Root,
		stageFixedFunc: func(files []string) error {
			args := append([]string{"add", "--"}, files...)
			return e.git(args...)
		},
	}

	if opts.DryRun {
		return env, nil
	}

	state, err := e.repoState(files)
	if err != nil {
		return env, err
	}
	hasHead := e.hasHEAD()
	if err := e.enforceSafety(cfg.Safety, state, jobs); err != nil {
		return env, err
	}

	switch cfg.Safety.Isolation {
	case "strict":
		if !hasHead {
			return env, errors.New("strict safety isolation requires at least one commit")
		}
		return e.prepareStrictEnvironment(env)
	case "", "smart", "fast", "off":
		if cfg.Safety.UnstagedStrategy == "stash" && needsSafetyStash(cfg.Safety, state) {
			if !hasHead {
				if len(state.unstaged) > 0 || len(state.partial) > 0 {
					return env, errors.New("cannot preserve unstaged or partially staged changes before the initial commit")
				}
				return env, nil
			}
			ref, err := e.pushSafetyStash()
			if err != nil {
				return env, err
			}
			if ref != "" {
				env.cleanupFuncs = append(env.cleanupFuncs, func() error {
					return e.restoreSafetyStash(ref, cfg.Safety.OnConflict)
				})
			}
		}
		return env, nil
	default:
		return env, fmt.Errorf("unsupported safety isolation %q", cfg.Safety.Isolation)
	}
}

func (e Executor) hasHEAD() bool {
	cmd := execCommand("git", "-C", e.Repo.Root, "rev-parse", "--verify", "HEAD")
	return cmd.Run() == nil
}

func (e Executor) repoState(files *fileInventory) (repoState, error) {
	changed, err := files.Changed()
	if err != nil {
		return repoState{}, err
	}
	staged, err := files.Staged()
	if err != nil {
		return repoState{}, err
	}
	untracked, err := files.Untracked()
	if err != nil {
		return repoState{}, err
	}

	state := repoState{
		changed:   changed,
		staged:    staged,
		untracked: untracked,
	}
	changedSet := toSet(changed)
	stagedSet := toSet(staged)
	for _, file := range changed {
		if !stagedSet[file] {
			state.unstaged = append(state.unstaged, file)
		}
	}
	for _, file := range staged {
		if changedSet[file] {
			state.partial = append(state.partial, file)
		}
	}
	state.unstaged = uniqueSorted(state.unstaged)
	state.partial = uniqueSorted(state.partial)
	return state, nil
}

func (e Executor) enforceSafety(safety config.SafetyConfig, state repoState, jobs []selectedJob) error {
	if safety.Isolation == "off" {
		return nil
	}

	if safety.PartialStaging == "forbid" && len(state.partial) > 0 {
		return fmt.Errorf("partially staged files are not allowed: %s", strings.Join(state.partial, ", "))
	}

	if safety.UnstagedStrategy == "fail" && (len(state.unstaged) > 0 || len(state.untracked) > 0) {
		files := append(append([]string(nil), state.unstaged...), state.untracked...)
		return fmt.Errorf("unstaged changes are not allowed: %s", strings.Join(uniqueSorted(files), ", "))
	}

	if len(state.partial) == 0 {
		return nil
	}

	if safety.PartialStaging == "preserve" {
		for _, job := range jobs {
			if job.Job.StageFixed {
				return nil
			}
		}
	}

	return nil
}

func needsSafetyStash(safety config.SafetyConfig, state repoState) bool {
	if safety.Isolation == "off" || safety.Isolation == "strict" {
		return false
	}
	if len(state.unstaged) == 0 && len(state.partial) == 0 {
		return false
	}
	if safety.PartialStaging == "preserve" && len(state.partial) > 0 {
		return true
	}
	return safety.UnstagedStrategy == "stash" && len(state.unstaged) > 0
}

func (e Executor) pushSafetyStash() (string, error) {
	before, _ := e.gitOutput("stash", "list", "--format=%gd", "-n", "1")
	cmd := execCommand("git", "-C", e.Repo.Root, "stash", "push", "--keep-index", "--include-untracked", "-m", "righthook-safety")
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return "", fmt.Errorf("stash safety changes: %v: %s", err, text)
	}
	if strings.Contains(text, "No local changes to save") {
		return "", nil
	}
	after, err := e.gitOutput("stash", "list", "--format=%gd", "-n", "1")
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(after)
	if ref == "" || ref == strings.TrimSpace(before) {
		return "", nil
	}
	return ref, nil
}

func (e Executor) restoreSafetyStash(ref, policy string) error {
	cmd := execCommand("git", "-C", e.Repo.Root, "stash", "pop", "--index", ref)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	text := strings.TrimSpace(string(out))
	switch policy {
	case "ignore":
		return nil
	case "warn":
		fmt.Fprintf(e.stderr(), "righthook warning: could not restore stashed changes cleanly: %s\n", text)
		return nil
	case "explain":
		fmt.Fprintf(e.stderr(), "righthook: safety restore needs attention.\n")
		fmt.Fprintf(e.stderr(), "The hook ran successfully, but restoring stashed changes conflicted.\n")
		fmt.Fprintf(e.stderr(), "%s\n", text)
		return nil
	default:
		return fmt.Errorf("restore stashed safety changes: %s", text)
	}
}

func (e Executor) prepareStrictEnvironment(base executionEnv) (executionEnv, error) {
	tempDir, err := os.MkdirTemp("", "righthook-worktree-*")
	if err != nil {
		return base, fmt.Errorf("create strict isolation worktree: %w", err)
	}

	base.cleanupFuncs = append(base.cleanupFuncs, func() error {
		return os.RemoveAll(tempDir)
	})

	if err := e.git("worktree", "add", "--detach", tempDir, "HEAD"); err != nil {
		return base, fmt.Errorf("create strict worktree: %w", err)
	}
	base.cleanupFuncs = append(base.cleanupFuncs, func() error {
		return e.git("worktree", "remove", "--force", tempDir)
	})

	patch, err := e.gitOutput("diff", "--cached", "--binary", "--no-ext-diff", "HEAD", "--")
	if err != nil {
		return base, err
	}
	if strings.TrimSpace(patch) != "" {
		cmd := execCommand("git", "-C", tempDir, "apply", "--binary", "--whitespace=nowarn")
		cmd.Stdin = strings.NewReader(patch)
		out, applyErr := cmd.CombinedOutput()
		if applyErr != nil {
			return base, fmt.Errorf("apply staged snapshot to strict worktree: %v: %s", applyErr, strings.TrimSpace(string(out)))
		}
	}

	base.workDir = tempDir
	base.stageFixedFunc = func(files []string) error {
		for _, file := range files {
			src := filepath.Join(tempDir, file)
			dst := filepath.Join(e.Repo.Root, file)
			if err := syncStrictFile(src, dst); err != nil {
				return err
			}
		}
		args := append([]string{"add", "-A", "--"}, files...)
		return e.git(args...)
	}
	return base, nil
}

func syncStrictFile(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			if removeErr := os.Remove(dst); removeErr != nil && !os.IsNotExist(removeErr) {
				return removeErr
			}
			return nil
		}
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
