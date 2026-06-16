package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/almeidazs/righthook/internal/config"
	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type Result struct {
	Target         string
	SourcePaths    []string
	Config         config.File
	MigratedHooks  []string
	MigratedJobs   map[string][]string
	Warnings       []string
	RemovedTargets []string
}

type lefthookFile struct {
	PreCommit lefthookHook `yaml:"pre-commit" toml:"pre-commit" json:"pre-commit"`
	CommitMsg lefthookHook `yaml:"commit-msg" toml:"commit-msg" json:"commit-msg"`
	PrePush   lefthookHook `yaml:"pre-push" toml:"pre-push" json:"pre-push"`
}

type lefthookHook struct {
	Commands map[string]lefthookJob `yaml:"commands" toml:"commands" json:"commands"`
}

type lefthookJob struct {
	Run   string `yaml:"run" toml:"run" json:"run"`
	Skip  bool   `yaml:"skip" toml:"skip" json:"skip"`
	Files string `yaml:"files" toml:"files" json:"files"`
}

func Discover(root, target string) ([]string, error) {
	switch target {
	case "lefthook":
		var paths []string
		for _, name := range []string{"lefthook.yml", "lefthook.yaml", "lefthook.toml"} {
			path := filepath.Join(root, name)
			if fileExists(path) {
				paths = append(paths, path)
			}
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("could not find Lefthook config in %s", root)
		}
		return paths, nil
	case "husky":
		base := filepath.Join(root, ".husky")
		var paths []string
		for _, hook := range []string{"pre-commit", "commit-msg", "pre-push"} {
			path := filepath.Join(base, hook)
			if fileExists(path) {
				paths = append(paths, path)
			}
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("could not find Husky hook files in %s", base)
		}
		return paths, nil
	default:
		return nil, fmt.Errorf("unsupported migration target %q", target)
	}
}

func Load(root, target string) (Result, error) {
	paths, err := Discover(root, target)
	if err != nil {
		return Result{}, err
	}

	switch target {
	case "lefthook":
		return loadLefthook(paths)
	case "husky":
		return loadHusky(paths)
	default:
		return Result{}, fmt.Errorf("unsupported migration target %q", target)
	}
}

func Merge(existing config.File, incoming Result) Result {
	merged := incoming
	merged.Config = existing
	if merged.Config.Hooks == nil {
		merged.Config.Hooks = map[string]config.Hook{}
	}
	if merged.MigratedJobs == nil {
		merged.MigratedJobs = map[string][]string{}
	}

	for hookName, hook := range incoming.Config.Hooks {
		current, ok := merged.Config.Hooks[hookName]
		if !ok {
			merged.Config.Hooks[hookName] = hook
			merged.MigratedHooks = append(merged.MigratedHooks, hookName)
			for jobName := range hook.Jobs {
				merged.MigratedJobs[hookName] = append(merged.MigratedJobs[hookName], jobName)
			}
			sort.Strings(merged.MigratedJobs[hookName])
			continue
		}
		if current.Jobs == nil {
			current.Jobs = map[string]config.Job{}
		}
		addedAny := false
		for jobName, job := range hook.Jobs {
			if _, exists := current.Jobs[jobName]; exists {
				merged.Warnings = append(merged.Warnings, fmt.Sprintf("skipped %s.%s because righthook config already defines it", hookName, jobName))
				continue
			}
			current.Jobs[jobName] = job
			merged.MigratedJobs[hookName] = append(merged.MigratedJobs[hookName], jobName)
			addedAny = true
		}
		if addedAny && !contains(merged.MigratedHooks, hookName) {
			merged.MigratedHooks = append(merged.MigratedHooks, hookName)
		}
		sort.Strings(merged.MigratedJobs[hookName])
		merged.Config.Hooks[hookName] = current
	}

	sort.Strings(merged.MigratedHooks)
	sort.Strings(merged.SourcePaths)
	sort.Strings(merged.Warnings)
	return merged
}

func loadLefthook(paths []string) (Result, error) {
	if len(paths) == 0 {
		return Result{}, fmt.Errorf("no Lefthook config paths provided")
	}
	path := paths[0]
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}

	var file lefthookFile
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yml", ".yaml":
		err = yaml.Unmarshal(data, &file)
	case ".toml":
		err = toml.Unmarshal(data, &file)
	default:
		err = fmt.Errorf("unsupported Lefthook config extension for %s", path)
	}
	if err != nil {
		return Result{}, fmt.Errorf("load %s: %w", path, err)
	}

	res := Result{
		Target:       "lefthook",
		SourcePaths:  paths,
		Config:       config.New(true, "smart"),
		MigratedJobs: map[string][]string{},
	}
	res.Config.Hooks = map[string]config.Hook{}

	res.addLefthookJobs("pre-commit", file.PreCommit.Commands)
	res.addLefthookJobs("commit-msg", file.CommitMsg.Commands)
	res.addLefthookJobs("pre-push", file.PrePush.Commands)

	if len(res.Config.Hooks) == 0 {
		res.Warnings = append(res.Warnings, "no supported Lefthook hooks or commands were found")
	}
	sort.Strings(res.MigratedHooks)
	sort.Strings(res.Warnings)
	return res, nil
}

func (r *Result) addLefthookJobs(hookName string, jobs map[string]lefthookJob) {
	if len(jobs) == 0 {
		return
	}
	names := make([]string, 0, len(jobs))
	for name := range jobs {
		names = append(names, name)
	}
	sort.Strings(names)

	hook := config.Hook{Jobs: map[string]config.Job{}}
	for _, name := range names {
		job := jobs[name]
		if strings.TrimSpace(job.Run) == "" && !job.Skip {
			r.Warnings = append(r.Warnings, fmt.Sprintf("skipped %s.%s because it has no run command", hookName, name))
			continue
		}
		next := config.Job{
			Run:   strings.TrimSpace(job.Run),
			Files: job.Files,
		}
		if job.Skip {
			next.Enabled = config.Enabled(false)
		}
		hook.Jobs[name] = next
		r.MigratedJobs[hookName] = append(r.MigratedJobs[hookName], name)
	}
	if len(hook.Jobs) == 0 {
		return
	}
	r.Config.Hooks[hookName] = hook
	r.MigratedHooks = append(r.MigratedHooks, hookName)
}

func loadHusky(paths []string) (Result, error) {
	res := Result{
		Target:       "husky",
		SourcePaths:  append([]string(nil), paths...),
		Config:       config.New(true, "smart"),
		MigratedJobs: map[string][]string{},
	}
	res.Config.Hooks = map[string]config.Hook{}

	for _, path := range paths {
		hookName := filepath.Base(path)
		script, warning, err := parseHuskyScript(path)
		if err != nil {
			return Result{}, err
		}
		if warning != "" {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s: %s", hookName, warning))
		}
		if strings.TrimSpace(script) == "" {
			res.Warnings = append(res.Warnings, fmt.Sprintf("skipped %s because it has no runnable commands after removing Husky boilerplate", hookName))
			continue
		}
		res.Config.Hooks[hookName] = config.Hook{
			Jobs: map[string]config.Job{
				"migrated": {Run: script},
			},
		}
		res.MigratedHooks = append(res.MigratedHooks, hookName)
		res.MigratedJobs[hookName] = []string{"migrated"}
	}

	sort.Strings(res.MigratedHooks)
	sort.Strings(res.Warnings)
	return res, nil
}

func parseHuskyScript(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(string(data), "\n")
	commands := make([]string, 0, len(lines))
	removedBoilerplate := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#!") {
			removedBoilerplate = true
			continue
		}
		if trimmed == ". \"$(dirname -- \"$0\")/_/husky.sh\"" ||
			trimmed == ". $(dirname \"$0\")/_/husky.sh" ||
			trimmed == "source \"$(dirname -- \"$0\")/_/husky.sh\"" {
			removedBoilerplate = true
			continue
		}
		commands = append(commands, trimmed)
	}

	warning := ""
	if removedBoilerplate && len(commands) > 1 {
		warning = "collapsed multi-line Husky script into a single shell command joined with &&"
	}
	return strings.Join(commands, " && "), warning, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
