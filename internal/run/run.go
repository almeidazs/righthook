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
}

type JobResult struct {
	Name         string
	Command      string
	Files        []string
	Status       string
	CacheEnabled bool
	Reason       string
}

type Executor struct {
	Repo   git.Repository
	Stdout io.Writer
	Stderr io.Writer
}

type fileSets struct {
	All      []string
	Changed  []string
	Staged   []string
	Affected map[string][]string
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

	files, err := e.collectFiles()
	if err != nil {
		return Result{}, err
	}

	res := Result{
		ConfigPath: opts.ConfigPath,
		Hook:       opts.Hook,
		Jobs:       make([]JobResult, 0, len(jobs)),
	}

	for _, selected := range jobs {
		runFiles, err := e.filesForJob(selected.Job, cfg, opts, &files)
		if err != nil {
			return res, fmt.Errorf("%s: %w", selected.Name, err)
		}

		command, err := expandCommand(selected.Job.Run, runFiles, files.Staged, files.Changed, opts.Args)
		if err != nil {
			return res, fmt.Errorf("%s: %w", selected.Name, err)
		}

		jobResult := JobResult{
			Name:         selected.Name,
			Command:      command,
			Files:        append([]string(nil), runFiles...),
			CacheEnabled: cacheEnabled(cfg, selected.Job, opts),
			Status:       "pending",
		}

		if opts.DryRun {
			jobResult.Status = "dry-run"
			res.Jobs = append(res.Jobs, jobResult)
			continue
		}

		if jobResult.CacheEnabled {
			hit, err := e.cacheHit(cfg, opts.Hook, selected.Name, command, runFiles)
			if err != nil {
				return res, err
			}
			if hit {
				jobResult.Status = "cached"
				jobResult.Reason = "cache hit"
				res.Jobs = append(res.Jobs, jobResult)
				continue
			}
		}

		if strings.TrimSpace(command) == "" {
			jobResult.Status = "skipped"
			jobResult.Reason = "empty command after placeholder expansion"
			res.Jobs = append(res.Jobs, jobResult)
			continue
		}

		if err := e.executeCommand(command, opts); err != nil {
			jobResult.Status = "failed"
			res.Jobs = append(res.Jobs, jobResult)
			return res, err
		}

		if selected.Job.StageFixed && len(runFiles) > 0 {
			args := append([]string{"add", "--"}, runFiles...)
			if err := e.git(args...); err != nil {
				return res, fmt.Errorf("%s: stage_fixed git add failed: %w", selected.Name, err)
			}
		}

		if jobResult.CacheEnabled {
			if err := e.writeCache(cfg, opts.Hook, selected.Name, command, runFiles); err != nil {
				return res, err
			}
		}

		jobResult.Status = "ran"
		res.Jobs = append(res.Jobs, jobResult)
	}

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

func (e Executor) collectFiles() (fileSets, error) {
	all, err := e.gitLines("ls-files")
	if err != nil {
		return fileSets{}, fmt.Errorf("list tracked files: %w", err)
	}
	changed, err := e.gitLines("diff", "--name-only")
	if err != nil {
		return fileSets{}, fmt.Errorf("list changed files: %w", err)
	}
	staged, err := e.gitLines("diff", "--cached", "--name-only")
	if err != nil {
		return fileSets{}, fmt.Errorf("list staged files: %w", err)
	}
	return fileSets{
		All:      uniqueSorted(all),
		Changed:  uniqueSorted(changed),
		Staged:   uniqueSorted(staged),
		Affected: map[string][]string{},
	}, nil
}

func (e Executor) filesForJob(job config.Job, cfg config.File, opts Options, files *fileSets) ([]string, error) {
	source := ""
	switch {
	case opts.Staged:
		source = "staged"
	case opts.Changed:
		source = "changed"
	case job.Scope == "affected" || job.Workspace == "affected":
		source = "affected"
	case strings.TrimSpace(job.Files) != "":
		source = strings.TrimSpace(job.Files)
	default:
		source = "all"
	}

	var selected []string
	switch source {
	case "staged":
		selected = files.Staged
	case "changed":
		selected = files.Changed
	case "affected":
		base := strings.TrimSpace(job.Base)
		if base == "" {
			base = "origin/main"
		}
		if cached, ok := files.Affected[base]; ok {
			selected = cached
		} else {
			affected, err := e.gitLines("diff", "--name-only", base+"...HEAD")
			if err != nil {
				return nil, fmt.Errorf("resolve affected files from base %s: %w", base, err)
			}
			selected = uniqueSorted(affected)
			files.Affected[base] = selected
		}
	case "all", "":
		selected = files.All
	default:
		return nil, fmt.Errorf("unsupported files selector %q", source)
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

func expandCommand(command string, resolvedFiles, stagedFiles, changedFiles, hookArgs []string) (string, error) {
	replaced := command
	replaced = strings.ReplaceAll(replaced, "{staged}", shellJoin(stagedFiles))
	replaced = strings.ReplaceAll(replaced, "{changed}", shellJoin(changedFiles))
	replaced = strings.ReplaceAll(replaced, "{files}", shellJoin(resolvedFiles))
	if strings.Contains(replaced, "{commit_msg_file}") {
		if len(hookArgs) == 0 || strings.TrimSpace(hookArgs[0]) == "" {
			return "", errors.New("hook requires {commit_msg_file} but no commit message file argument was provided")
		}
		replaced = strings.ReplaceAll(replaced, "{commit_msg_file}", shellEscape(hookArgs[0]))
	}
	return replaced, nil
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

func (e Executor) executeCommand(command string, opts Options) error {
	cmd := execCommand("sh", "-c", command)
	cmd.Dir = e.Repo.Root
	cmd.Stdout = e.stdout()
	cmd.Stderr = e.stderr()
	cmd.Env = append(os.Environ(),
		"RIGHTHOOK_HOOK="+opts.Hook,
		fmt.Sprintf("RIGHTHOOK_FIX=%t", opts.Fix),
	)
	return cmd.Run()
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
