package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/git"
)

func TestStatusShowsHealthyInstall(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\ncache:\n  enabled: true\n  dir: .righthook/cache\n  ttl: 7d\noutput:\n  mode: compact\n  timing: true\n  show_success: false\nsafety:\n  isolation: smart\n  partial_staging: preserve\n  unstaged_strategy: stash\n  on_conflict: explain\nhooks:\n  pre-commit:\n    jobs:\n      fmt:\n        run: gofmt -w {staged}\n  commit-msg:\n    jobs:\n      lint:\n        run: commitlint --edit {commit_msg_file}\n  pre-push:\n    jobs:\n      test:\n        run: go test ./...\n")
	for _, hook := range []string{"pre-commit", "commit-msg", "pre-push"} {
		mustWriteFile(t, filepath.Join(root, ".git", "hooks", hook), git.HookScript(hook))
	}

	var out bytes.Buffer
	err := Status(cli.StatusOptions{Path: root}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"Config found:",
		"Git hooks installed",
		"pre-commit installed",
		"commit-msg installed",
		"pre-push installed",
		"Cache enabled",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in output, got %q", want, text)
		}
	}
}

func TestStatusTreatsDisabledJobsAsNotExpected(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\ncache:\n  enabled: true\n  dir: .righthook/cache\n  ttl: 7d\noutput:\n  mode: compact\n  timing: true\n  show_success: false\nsafety:\n  isolation: smart\n  partial_staging: preserve\n  unstaged_strategy: stash\n  on_conflict: explain\nhooks:\n  pre-commit:\n    jobs:\n      typecheck:\n        enabled: false\n  pre-push:\n    jobs:\n      test:\n        run: go test ./...\n")
	mustWriteFile(t, filepath.Join(root, ".git", "hooks", "pre-push"), git.HookScript("pre-push"))

	var out bytes.Buffer
	err := Status(cli.StatusOptions{Path: root}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "Git hooks installed") {
		t.Fatalf("expected healthy hook status, got %q", text)
	}
	if !strings.Contains(text, "pre-commit missing") {
		t.Fatalf("expected disabled hook to be reported as not installed, got %q", text)
	}
	if !strings.Contains(text, "pre-push installed") {
		t.Fatalf("expected active hook to be reported installed, got %q", text)
	}
}

func TestStatusShowsBrokenStateButReturnsNil(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"2\"\n")
	mustWriteFile(t, filepath.Join(root, ".git", "hooks", "pre-commit"), "#!/bin/sh\necho custom\n")

	var out bytes.Buffer
	err := Status(cli.StatusOptions{Path: root}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("status should not fail, got %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"Config invalid:",
		"Git hooks missing or incomplete",
		"pre-commit occupied by a non-Righthook script",
		"commit-msg missing",
		"pre-push missing",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in output, got %q", want, text)
		}
	}
}

func TestStatusWithInvalidRepoReturnsNil(t *testing.T) {
	var out bytes.Buffer
	err := Status(cli.StatusOptions{Path: t.TempDir()}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("status should not fail, got %v", err)
	}
	if !strings.Contains(out.String(), "is not a Git repository root or .git path") {
		t.Fatalf("expected git path error in output, got %q", out.String())
	}
}
