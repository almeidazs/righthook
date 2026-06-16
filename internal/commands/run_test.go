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
