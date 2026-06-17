package commands

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/almeidazs/righthook/internal/cli"
)

func TestRunDryRunRespectsOnlyAndExcept(t *testing.T) {
	root := initGitRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  pre-commit:\n    jobs:\n      format:\n        run: echo format {staged}\n      lint:\n        run: echo lint {staged}\n      typecheck:\n        enabled: false\n")
	mustWriteFile(t, filepath.Join(root, "file.ts"), "const x = 1;\n")
	gitRun(t, root, "add", "file.ts")

	var out bytes.Buffer
	err := Run(cli.RunOptions{
		Hook:   "pre-commit",
		Path:   root,
		DryRun: true,
		Only:   []string{"format", "lint"},
		Except: []string{"lint"},
		Staged: true,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("run dry-run: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "format [dry-run]") {
		t.Fatalf("expected selected job in output, got %q", text)
	}
	if strings.Contains(text, "lint [dry-run]") || strings.Contains(text, "typecheck") {
		t.Fatalf("unexpected excluded job in output, got %q", text)
	}
	if !strings.Contains(text, "'file.ts'") {
		t.Fatalf("expected staged file expansion in output, got %q", text)
	}
}

func TestRunChangedExecutesCommand(t *testing.T) {
	root := initGitRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  pre-commit:\n    jobs:\n      lint:\n        run: printf '%s\\n' {changed} > .changed.out\n")
	mustWriteFile(t, filepath.Join(root, "file.ts"), "const x = 1;\n")
	gitRun(t, root, "add", "file.ts")
	gitRun(t, root, "commit", "-m", "init")
	mustWriteFile(t, filepath.Join(root, "file.ts"), "const x = 2;\n")

	var out bytes.Buffer
	err := Run(cli.RunOptions{
		Hook:    "pre-commit",
		Path:    root,
		Changed: true,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("run changed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".changed.out"))
	if err != nil {
		t.Fatalf("read command output file: %v", err)
	}
	if !strings.Contains(string(data), "file.ts") {
		t.Fatalf("expected changed file to be passed to command, got %q", string(data))
	}
}

func TestRunPrefersLocalConfigOverDefaultConfig(t *testing.T) {
	root := initGitRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  pre-push:\n    jobs:\n      default:\n        run: printf 'default\\n' >> runs.txt\n")
	mustWriteFile(t, filepath.Join(root, "righthook.local.json"), "{\n  \"version\": \"1\",\n  \"hooks\": {\n    \"pre-push\": {\n      \"jobs\": {\n        \"local\": {\n          \"run\": \"printf 'local\\\\n' >> runs.txt\"\n        }\n      }\n    }\n  }\n}\n")

	var out bytes.Buffer
	err := Run(cli.RunOptions{
		Hook: "pre-push",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "runs.txt"))
	if err != nil {
		t.Fatalf("read runs file: %v", err)
	}
	if strings.Contains(string(data), "default") || !strings.Contains(string(data), "local") {
		t.Fatalf("expected local config to win, got %q", string(data))
	}
	if !strings.Contains(out.String(), filepath.Join(root, "righthook.local.json")) {
		t.Fatalf("expected resolved local config path in output, got %q", out.String())
	}
}

func TestRunCommitMsgRequiresArgument(t *testing.T) {
	root := initGitRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  commit-msg:\n    jobs:\n      lint:\n        run: cat {commit_msg_file}\n")

	var out bytes.Buffer
	err := Run(cli.RunOptions{
		Hook: "commit-msg",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err == nil || !strings.Contains(err.Error(), "{commit_msg_file}") {
		t.Fatalf("expected commit message file error, got %v", err)
	}
}

func TestRunStageFixedAddsFilesBackToIndex(t *testing.T) {
	root := initGitRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  pre-commit:\n    jobs:\n      format:\n        run: printf 'formatted\\n' > file.ts\n        files: staged\n        stage_fixed: true\n")
	mustWriteFile(t, filepath.Join(root, "file.ts"), "before\n")
	gitRun(t, root, "add", "file.ts")

	var out bytes.Buffer
	err := Run(cli.RunOptions{
		Hook: "pre-commit",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("run stage_fixed: %v", err)
	}

	cached := gitOutput(t, root, "diff", "--cached", "--name-only")
	if !strings.Contains(cached, "file.ts") {
		t.Fatalf("expected file.ts to remain staged after stage_fixed, got %q", cached)
	}
}

func TestRunCacheSkipsSecondExecution(t *testing.T) {
	root := initGitRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\ncache:\n  enabled: true\n  dir: .righthook/cache\n  ttl: 1h\noutput:\n  mode: compact\n  timing: true\n  show_success: false\nsafety:\n  isolation: smart\n  partial_staging: preserve\n  unstaged_strategy: stash\n  on_conflict: explain\nhooks:\n  pre-push:\n    jobs:\n      test:\n        run: printf 'run\\n' >> runs.txt\n        cache: true\n")

	var first bytes.Buffer
	err := Run(cli.RunOptions{
		Hook: "pre-push",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &first, Stderr: &first})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}

	var second bytes.Buffer
	err = Run(cli.RunOptions{
		Hook: "pre-push",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &second, Stderr: &second})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "runs.txt"))
	if err != nil {
		t.Fatalf("read runs file: %v", err)
	}
	if strings.Count(string(data), "run") != 1 {
		t.Fatalf("expected cached second run to skip execution, got %q", string(data))
	}
	if !strings.Contains(second.String(), "test [cached]") {
		t.Fatalf("expected cached status in output, got %q", second.String())
	}
}

func TestRunOutputHidesSuccessfulJobsWhenConfigured(t *testing.T) {
	root := initGitRepo(t)
	gitRun(t, root, "commit", "--allow-empty", "-m", "init")
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\noutput:\n  mode: compact\n  timing: false\n  show_success: false\nhooks:\n  pre-push:\n    jobs:\n      test:\n        run: printf 'run\\n' >> runs.txt\n")

	var out bytes.Buffer
	err := Run(cli.RunOptions{
		Hook: "pre-push",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	text := out.String()
	if strings.Contains(text, "test [ran]") {
		t.Fatalf("expected successful job to be hidden, got %q", text)
	}
	if !strings.Contains(text, "hidden successful jobs: 1") {
		t.Fatalf("expected hidden summary, got %q", text)
	}
}

func TestRunSkipsWhenFilePlaceholderResolvesToEmpty(t *testing.T) {
	root := initGitRepo(t)
	gitRun(t, root, "commit", "--allow-empty", "-m", "init")
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  pre-commit:\n    jobs:\n      gofmt:\n        run: gofmt -w {staged}\n        files: staged\n        glob:\n          - '*.go'\n        stage_fixed: true\n")

	var out bytes.Buffer
	err := Run(cli.RunOptions{
		Hook: "pre-commit",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("run should skip empty staged placeholder, got %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "gofmt [skipped]") {
		t.Fatalf("expected skipped job, got %q", text)
	}
	if !strings.Contains(text, "no files matched command placeholders") {
		t.Fatalf("expected skip reason, got %q", text)
	}
}

func TestRunSafetyForbidsPartialStagingWhenConfigured(t *testing.T) {
	root := initGitRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nsafety:\n  isolation: smart\n  partial_staging: forbid\n  unstaged_strategy: ignore\n  on_conflict: fail\nhooks:\n  pre-commit:\n    jobs:\n      lint:\n        run: echo ok\n")
	mustWriteFile(t, filepath.Join(root, "file.ts"), "before\n")
	gitRun(t, root, "add", "file.ts")
	gitRun(t, root, "commit", "-m", "init")
	mustWriteFile(t, filepath.Join(root, "file.ts"), "staged\n")
	gitRun(t, root, "add", "file.ts")
	mustWriteFile(t, filepath.Join(root, "file.ts"), "staged\nunstaged\n")

	var out bytes.Buffer
	err := Run(cli.RunOptions{
		Hook: "pre-commit",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err == nil || !strings.Contains(err.Error(), "partially staged files are not allowed") {
		t.Fatalf("expected partial staging safety error, got %v", err)
	}
}

func TestRunSafetyFailsOnUnstagedChangesWhenConfigured(t *testing.T) {
	root := initGitRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nsafety:\n  isolation: fast\n  partial_staging: allow\n  unstaged_strategy: fail\n  on_conflict: fail\nhooks:\n  pre-commit:\n    jobs:\n      lint:\n        run: echo ok\n")
	mustWriteFile(t, filepath.Join(root, "file.ts"), "before\n")
	gitRun(t, root, "add", "file.ts")
	gitRun(t, root, "commit", "-m", "init")
	mustWriteFile(t, filepath.Join(root, "file.ts"), "after\n")

	var out bytes.Buffer
	err := Run(cli.RunOptions{
		Hook: "pre-commit",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err == nil || !strings.Contains(err.Error(), "unstaged changes are not allowed") {
		t.Fatalf("expected unstaged safety error, got %v", err)
	}
}

func TestRunExpandsExtendedPlaceholders(t *testing.T) {
	root := initGitRepo(t)
	mustWriteFile(t, filepath.Join(root, "a.go"), "package main\n")
	gitRun(t, root, "add", "a.go")
	gitRun(t, root, "commit", "-m", "init")
	mustWriteFile(t, filepath.Join(root, "b.go"), "package main\n")
	gitRun(t, root, "add", "b.go")
	gitRun(t, root, "commit", "-m", "second")
	currentBranch := strings.TrimSpace(gitOutput(t, root, "branch", "--show-current"))

	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  pre-push:\n    jobs:\n      meta:\n        run: printf '%s\\n' {all} > .all.out && printf '%s\\n' {affected} > .affected.out && printf '%s\\n' {branch} {base_branch} {workspace} {workspace_root} {repo_root} > .meta.out\n        files: all\n        base: HEAD~1\n")

	var out bytes.Buffer
	err := Run(cli.RunOptions{
		Hook: "pre-push",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("run placeholders: %v", err)
	}

	allData, err := os.ReadFile(filepath.Join(root, ".all.out"))
	if err != nil {
		t.Fatalf("read all placeholder output: %v", err)
	}
	if !strings.Contains(string(allData), "a.go") || !strings.Contains(string(allData), "b.go") {
		t.Fatalf("expected all files in output, got %q", string(allData))
	}

	affectedData, err := os.ReadFile(filepath.Join(root, ".affected.out"))
	if err != nil {
		t.Fatalf("read affected placeholder output: %v", err)
	}
	if !strings.Contains(string(affectedData), "b.go") {
		t.Fatalf("expected affected files in output, got %q", string(affectedData))
	}

	metaData, err := os.ReadFile(filepath.Join(root, ".meta.out"))
	if err != nil {
		t.Fatalf("read meta placeholder output: %v", err)
	}
	metaLines := strings.Split(strings.TrimSpace(string(metaData)), "\n")
	if len(metaLines) != 5 {
		t.Fatalf("expected 5 meta placeholder lines, got %q", string(metaData))
	}
	if metaLines[0] != currentBranch {
		t.Fatalf("expected current branch %q, got %q", currentBranch, metaLines[0])
	}
	if metaLines[1] != "HEAD~1" {
		t.Fatalf("expected base branch HEAD~1, got %q", metaLines[1])
	}
	if metaLines[2] != filepath.Base(root) {
		t.Fatalf("expected workspace %q, got %q", filepath.Base(root), metaLines[2])
	}
	if metaLines[3] != root || metaLines[4] != root {
		t.Fatalf("expected workspace_root/repo_root %q, got %q", root, string(metaData))
	}
}

func TestRunStrictIsolationSyncsStageFixedFilesBack(t *testing.T) {
	root := initGitRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nsafety:\n  isolation: strict\n  partial_staging: preserve\n  unstaged_strategy: stash\n  on_conflict: fail\nhooks:\n  pre-commit:\n    jobs:\n      format:\n        run: printf 'formatted\\n' > file.ts\n        files: staged\n        stage_fixed: true\n")
	mustWriteFile(t, filepath.Join(root, "file.ts"), "before\n")
	gitRun(t, root, "add", "file.ts")
	gitRun(t, root, "commit", "-m", "init")
	mustWriteFile(t, filepath.Join(root, "file.ts"), "changed\n")
	gitRun(t, root, "add", "file.ts")

	var out bytes.Buffer
	err := Run(cli.RunOptions{
		Hook: "pre-commit",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("run strict isolation: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "file.ts"))
	if err != nil {
		t.Fatalf("read synced file: %v", err)
	}
	if string(data) != "formatted\n" {
		t.Fatalf("expected strict stage_fixed sync, got %q", string(data))
	}
	cached := gitOutput(t, root, "diff", "--cached", "--name-only")
	if !strings.Contains(cached, "file.ts") {
		t.Fatalf("expected strict stage_fixed file to remain staged, got %q", cached)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitRun(t, root, "init")
	gitRun(t, root, "config", "user.email", "test@example.com")
	gitRun(t, root, "config", "user.name", "Test User")
	return root
}

func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}
