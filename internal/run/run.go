package run

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/almeidazs/righthook/internal/config"
	"github.com/almeidazs/righthook/internal/git"
)

type Options struct {
	Hook       string
	Args       []string
	NoCache    bool
	Fix        bool
	DryRun     bool
	Changed    bool
	Staged     bool
	Only       []string
	Except     []string
	ConfigPath string
}

type Result struct {
	ConfigPath string
	Hook       string
	Jobs       []JobResult
	Duration   time.Duration
}

type JobResult struct {
	Name         string
	Command      string
	Files        []string
	Status       string
	CacheEnabled bool
	Reason       string
	Duration     time.Duration
}

type commandExpansion struct {
	Command                 string
	HasEmptyFilePlaceholder bool
}

type executionContext struct {
	RepoRoot      string
	WorkspaceRoot string
	Workspace     string
	Branch        string
	BaseBranch    string
	BaseRef       string
}

type Executor struct {
	Repo   git.Repository
	Stdout io.Writer
	Stderr io.Writer
}

type fileInventory struct {
	executor     Executor
	all          []string
	changed      []string
	staged       []string
	untracked    []string
	affected     map[string][]string
	hasAll       bool
	hasChanged   bool
	hasStaged    bool
	hasUntracked bool
}

type cacheMetadata struct {
	Key  string
	Path string
	TTL  time.Duration
}

var execCommand = func(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func (e Executor) Run(cfg config.File, opts Options) (Result, error) {
	hook, ok := cfg.Hooks[opts.Hook]
	if !ok {
		return Result{}, fmt.Errorf("hook %s is not configured", opts.Hook)
	}

	jobs, err := selectJobs(hook, opts)
	if err != nil {
		return Result{}, err
	}

	res := Result{
		ConfigPath: opts.ConfigPath,
		Hook:       opts.Hook,
		Jobs:       make([]JobResult, 0, len(jobs)),
	}
	startedAt := time.Now()
	files := newFileInventory(e)
	ctx, err := e.buildExecutionContext(config.Job{}, opts)
	if err != nil {
		return res, err
	}

	env, err := e.prepareEnvironment(cfg, opts, files, jobs)
	if err != nil {
		return res, err
	}
	defer func() {
		if cleanupErr := env.cleanup(); cleanupErr != nil {
			fmt.Fprintf(e.stderr(), "righthook cleanup failed: %v\n", cleanupErr)
		}
	}()

	for _, selected := range jobs {
		jobStartedAt := time.Now()

		runFiles, err := e.filesForJob(selected.Job, opts, files)
		if err != nil {
			return res, fmt.Errorf("%s: %w", selected.Name, err)
		}

		jobCtx, err := e.buildExecutionContext(selected.Job, opts)
		if err != nil {
			return res, fmt.Errorf("%s: %w", selected.Name, err)
		}
		if jobCtx.RepoRoot == "" {
			jobCtx = ctx
		}

		expansion, err := expandCommand(selected.Job.Run, runFiles, files, opts.Args, jobCtx)
		if err != nil {
			return res, fmt.Errorf("%s: %w", selected.Name, err)
		}

		jobResult := JobResult{
			Name:         selected.Name,
			Command:      expansion.Command,
			Files:        append([]string(nil), runFiles...),
			CacheEnabled: cacheEnabled(cfg, selected.Job, opts),
			Status:       "pending",
		}

		if expansion.HasEmptyFilePlaceholder {
			jobResult.Status = "skipped"
			jobResult.Reason = "no files matched command placeholders"
			jobResult.Duration = time.Since(jobStartedAt)
			res.Jobs = append(res.Jobs, jobResult)
			continue
		}

		if opts.DryRun {
			jobResult.Status = "dry-run"
			jobResult.Duration = time.Since(jobStartedAt)
			res.Jobs = append(res.Jobs, jobResult)
			continue
		}

		if jobResult.CacheEnabled {
			hit, err := e.cacheHit(cfg, opts.Hook, selected.Name, expansion.Command, runFiles)
			if err != nil {
				return res, err
			}
			if hit {
				jobResult.Status = "cached"
				jobResult.Reason = "cache hit"
				jobResult.Duration = time.Since(jobStartedAt)
				res.Jobs = append(res.Jobs, jobResult)
				continue
			}
		}

		if strings.TrimSpace(expansion.Command) == "" {
			jobResult.Status = "skipped"
			jobResult.Reason = "empty command after placeholder expansion"
			jobResult.Duration = time.Since(jobStartedAt)
			res.Jobs = append(res.Jobs, jobResult)
			continue
		}

		if err := e.executeCommand(expansion.Command, env.workDir, opts); err != nil {
			jobResult.Status = "failed"
			jobResult.Duration = time.Since(jobStartedAt)
			res.Jobs = append(res.Jobs, jobResult)
			return res, err
		}

		if selected.Job.StageFixed && len(runFiles) > 0 {
			if err := env.syncStageFixed(runFiles); err != nil {
				return res, fmt.Errorf("%s: stage_fixed git add failed: %w", selected.Name, err)
			}
		}

		if jobResult.CacheEnabled {
			if err := e.writeCache(cfg, opts.Hook, selected.Name, expansion.Command, runFiles); err != nil {
				return res, err
			}
		}

		jobResult.Status = "ran"
		jobResult.Duration = time.Since(jobStartedAt)
		res.Jobs = append(res.Jobs, jobResult)
	}

	res.Duration = time.Since(startedAt)
	return res, nil
}

type selectedJob struct {
	Name string
	Job  config.Job
}

func selectJobs(hook config.Hook, opts Options) ([]selectedJob, error) {
	only := toSet(opts.Only)
	except := toSet(opts.Except)

	jobNames := make([]string, 0, len(hook.Jobs))
	for name, job := range hook.Jobs {
		if !job.IsEnabled() {
			continue
		}
		if len(only) > 0 && !only[name] {
			continue
		}
		if except[name] {
			continue
		}
		jobNames = append(jobNames, name)
	}
	sort.Strings(jobNames)
	if len(jobNames) == 0 {
		return nil, errors.New("no enabled jobs selected for this hook")
	}

	jobs := make([]selectedJob, 0, len(jobNames))
	for _, name := range jobNames {
		jobs = append(jobs, selectedJob{Name: name, Job: hook.Jobs[name]})
	}
	return jobs, nil
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func newFileInventory(executor Executor) *fileInventory {
	return &fileInventory{
		executor: executor,
		affected: map[string][]string{},
	}
}

func (f *fileInventory) All() ([]string, error) {
	if f.hasAll {
		return append([]string(nil), f.all...), nil
	}
	all, err := f.executor.gitLines("ls-files")
	if err != nil {
		return nil, fmt.Errorf("list tracked files: %w", err)
	}
	f.all = uniqueSorted(all)
	f.hasAll = true
	return append([]string(nil), f.all...), nil
}

func (f *fileInventory) Changed() ([]string, error) {
	if f.hasChanged {
		return append([]string(nil), f.changed...), nil
	}
	changed, err := f.executor.gitLines("diff", "--name-only")
	if err != nil {
		return nil, fmt.Errorf("list changed files: %w", err)
	}
	f.changed = uniqueSorted(changed)
	f.hasChanged = true
	return append([]string(nil), f.changed...), nil
}

func (f *fileInventory) Staged() ([]string, error) {
	if f.hasStaged {
		return append([]string(nil), f.staged...), nil
	}
	staged, err := f.executor.gitLines("diff", "--cached", "--name-only")
	if err != nil {
		return nil, fmt.Errorf("list staged files: %w", err)
	}
	f.staged = uniqueSorted(staged)
	f.hasStaged = true
	return append([]string(nil), f.staged...), nil
}

func (f *fileInventory) Untracked() ([]string, error) {
	if f.hasUntracked {
		return append([]string(nil), f.untracked...), nil
	}
	untracked, err := f.executor.gitLines("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("list untracked files: %w", err)
	}
	f.untracked = uniqueSorted(untracked)
	f.hasUntracked = true
	return append([]string(nil), f.untracked...), nil
}

func (f *fileInventory) Affected(base string) ([]string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "origin/main"
	}
	if cached, ok := f.affected[base]; ok {
		return append([]string(nil), cached...), nil
	}
	affected, err := f.executor.gitLines("diff", "--name-only", base+"...HEAD")
	if err != nil {
		return nil, fmt.Errorf("resolve affected files from base %s: %w", base, err)
	}
	selected := uniqueSorted(affected)
	f.affected[base] = selected
	return append([]string(nil), selected...), nil
}

func resolveFileSource(job config.Job, opts Options) string {
	switch {
	case opts.Staged:
		return "staged"
	case opts.Changed:
		return "changed"
	case job.Scope == "affected" || job.Workspace == "affected":
		return "affected"
	case strings.TrimSpace(job.Files) != "":
		return strings.TrimSpace(job.Files)
	default:
		return "all"
	}
}

func (e Executor) filesForJob(job config.Job, opts Options, files *fileInventory) ([]string, error) {
	source := resolveFileSource(job, opts)
	var selected []string
	var err error
	switch source {
	case "staged":
		selected, err = files.Staged()
	case "changed":
		selected, err = files.Changed()
	case "affected":
		selected, err = files.Affected(job.Base)
	case "all", "":
		selected, err = files.All()
	default:
		return nil, fmt.Errorf("unsupported files selector %q", source)
	}
	if err != nil {
		return nil, err
	}

	return filterGlobs(selected, job.Glob), nil
}

func filterGlobs(files []string, globs []string) []string {
	if len(globs) == 0 {
		return append([]string(nil), files...)
	}
	filtered := make([]string, 0, len(files))
	for _, file := range files {
		for _, pattern := range globs {
			if globMatches(pattern, file) {
				filtered = append(filtered, file)
				break
			}
		}
	}
	return uniqueSorted(filtered)
}

func globMatches(pattern, path string) bool {
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)
	if ok, _ := filepath.Match(pattern, path); ok {
		return true
	}
	if ok, _ := filepath.Match(pattern, filepath.Base(path)); ok {
		return true
	}
	return false
}

func expandCommand(command string, resolvedFiles []string, files *fileInventory, hookArgs []string, ctx executionContext) (commandExpansion, error) {
	replaced := command
	hasEmptyFilePlaceholder := false
	replaced = strings.ReplaceAll(replaced, "{files}", shellJoin(resolvedFiles))
	if strings.Contains(command, "{files}") && len(resolvedFiles) == 0 {
		hasEmptyFilePlaceholder = true
	}
	if strings.Contains(replaced, "{staged}") {
		stagedFiles, err := files.Staged()
		if err != nil {
			return commandExpansion{}, err
		}
		replaced = strings.ReplaceAll(replaced, "{staged}", shellJoin(stagedFiles))
		if len(stagedFiles) == 0 {
			hasEmptyFilePlaceholder = true
		}
	}
	if strings.Contains(replaced, "{changed}") {
		changedFiles, err := files.Changed()
		if err != nil {
			return commandExpansion{}, err
		}
		replaced = strings.ReplaceAll(replaced, "{changed}", shellJoin(changedFiles))
		if len(changedFiles) == 0 {
			hasEmptyFilePlaceholder = true
		}
	}
	if strings.Contains(replaced, "{affected}") {
		affectedFiles, err := files.Affected(ctx.BaseRef)
		if err != nil {
			return commandExpansion{}, err
		}
		replaced = strings.ReplaceAll(replaced, "{affected}", shellJoin(affectedFiles))
		if len(affectedFiles) == 0 {
			hasEmptyFilePlaceholder = true
		}
	}
	if strings.Contains(replaced, "{all}") {
		allFiles, err := files.All()
		if err != nil {
			return commandExpansion{}, err
		}
		replaced = strings.ReplaceAll(replaced, "{all}", shellJoin(allFiles))
		if len(allFiles) == 0 {
			hasEmptyFilePlaceholder = true
		}
	}
	if strings.Contains(replaced, "{commit_msg_file}") {
		if len(hookArgs) == 0 || strings.TrimSpace(hookArgs[0]) == "" {
			return commandExpansion{}, errors.New("hook requires {commit_msg_file} but no commit message file argument was provided")
		}
		replaced = strings.ReplaceAll(replaced, "{commit_msg_file}", shellEscape(hookArgs[0]))
	}
	replaced = strings.ReplaceAll(replaced, "{branch}", shellEscape(ctx.Branch))
	replaced = strings.ReplaceAll(replaced, "{base_branch}", shellEscape(ctx.BaseBranch))
	replaced = strings.ReplaceAll(replaced, "{workspace}", shellEscape(ctx.Workspace))
	replaced = strings.ReplaceAll(replaced, "{workspace_root}", shellEscape(ctx.WorkspaceRoot))
	replaced = strings.ReplaceAll(replaced, "{repo_root}", shellEscape(ctx.RepoRoot))
	return commandExpansion{Command: replaced, HasEmptyFilePlaceholder: hasEmptyFilePlaceholder}, nil
}

func shellJoin(values []string) string {
	if len(values) == 0 {
		return ""
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, shellEscape(value))
	}
	return strings.Join(out, " ")
}

func shellEscape(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func cacheEnabled(cfg config.File, job config.Job, opts Options) bool {
	return cfg.Cache.Enabled && job.Cache && !opts.NoCache
}

func (e Executor) cacheMetadata(cfg config.File, hook, jobName, command string, files []string) (cacheMetadata, error) {
	path, ttl, err := e.cachePath(cfg, hook, jobName, command, files)
	if err != nil {
		return cacheMetadata{}, err
	}
	key := strings.TrimSuffix(filepath.Base(path), ".cache")
	return cacheMetadata{Key: key, Path: path, TTL: ttl}, nil
}

func (e Executor) cacheHit(cfg config.File, hook, jobName, command string, files []string) (bool, error) {
	path, ttl, err := e.cachePath(cfg, hook, jobName, command, files)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if ttl <= 0 {
		return true, nil
	}
	return time.Since(info.ModTime()) <= ttl, nil
}

func (e Executor) writeCache(cfg config.File, hook, jobName, command string, files []string) error {
	path, _, err := e.cachePath(cfg, hook, jobName, command, files)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339Nano)), 0o644)
}

func (e Executor) cachePath(cfg config.File, hook, jobName, command string, files []string) (string, time.Duration, error) {
	cacheDir := cfg.Cache.Dir
	if !filepath.IsAbs(cacheDir) {
		cacheDir = filepath.Join(e.Repo.Root, cacheDir)
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{hook, jobName, command, strings.Join(files, "\n")}, "\x00")))
	key := hex.EncodeToString(sum[:])
	ttl, err := parseTTL(cfg.Cache.TTL)
	if err != nil {
		return "", 0, fmt.Errorf("parse cache ttl: %w", err)
	}
	return filepath.Join(cacheDir, hook, jobName, key+".cache"), ttl, nil
}

func parseTTL(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	if strings.HasSuffix(value, "d") {
		days := strings.TrimSuffix(value, "d")
		n, err := strconv.Atoi(days)
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(value)
}

func (e Executor) executeCommand(command, workDir string, opts Options) error {
	cmd := execCommand("sh", "-c", command)
	cmd.Dir = workDir
	cmd.Stdout = e.stdout()
	cmd.Stderr = e.stderr()
	cmd.Env = e.commandEnv(opts)
	return cmd.Run()
}

func (e Executor) commandEnv(opts Options) []string {
	return append(os.Environ(),
		"RIGHTHOOK_HOOK="+opts.Hook,
		fmt.Sprintf("RIGHTHOOK_FIX=%t", opts.Fix),
	)
}

func (e Executor) buildExecutionContext(job config.Job, opts Options) (executionContext, error) {
	baseRef := strings.TrimSpace(job.Base)
	if baseRef == "" {
		baseRef = "origin/main"
	}
	branch, err := e.currentBranch()
	if err != nil {
		return executionContext{}, err
	}
	workspaceRoot := e.Repo.Root
	workspaceName := filepath.Base(workspaceRoot)
	return executionContext{
		RepoRoot:      e.Repo.Root,
		WorkspaceRoot: workspaceRoot,
		Workspace:     workspaceName,
		Branch:        branch,
		BaseBranch:    shortRefName(baseRef),
		BaseRef:       baseRef,
	}, nil
}

func (e Executor) currentBranch() (string, error) {
	out, err := e.gitOutput("branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("resolve current branch: %w", err)
	}
	branch := strings.TrimSpace(out)
	if branch == "" {
		return "HEAD", nil
	}
	return branch, nil
}

func shortRefName(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}

func (e Executor) gitLines(args ...string) ([]string, error) {
	out, err := e.gitOutput(args...)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return uniqueSorted(lines), nil
}

func (e Executor) gitOutput(args ...string) (string, error) {
	cmd := execCommand("git", append([]string{"-C", e.Repo.Root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (e Executor) git(args ...string) error {
	cmd := execCommand("git", append([]string{"-C", e.Repo.Root}, args...)...)
	cmd.Stdout = e.stdout()
	cmd.Stderr = e.stderr()
	return cmd.Run()
}

func (e Executor) stdout() io.Writer {
	if e.Stdout == nil {
		return io.Discard
	}
	return e.Stdout
}

func (e Executor) stderr() io.Writer {
	if e.Stderr == nil {
		return io.Discard
	}
	return e.Stderr
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
