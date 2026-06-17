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

func TestPolicyCheckFailsWhenRequiredHookIsMissing(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\npolicy:\n  required_version: \">=1.0.0\"\n  require_installed: true\n  required_hooks:\n    - pre-commit\n    - commit-msg\n  allow_skip: fail\n")
	mustWriteFile(t, filepath.Join(root, ".git", "hooks", "pre-commit"), git.HookScript("pre-commit"))

	prev := currentVersion
	currentVersion = "1.0.0"
	t.Cleanup(func() {
		currentVersion = prev
	})

	var out bytes.Buffer
	err := PolicyCheck(cli.PolicyCheckOptions{Path: root}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err == nil || !strings.Contains(err.Error(), "policy check failed") {
		t.Fatalf("expected policy failure, got %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"Righthook policy",
		"Version satisfies >=1.0.0",
		"pre-commit installed",
		"commit-msg not installed",
		"Fix",
		"righthook install --hook commit-msg",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in output, got %q", want, text)
		}
	}
}

func TestPolicyCheckWarnModeDoesNotFailProcess(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\npolicy:\n  require_installed: true\n  required_hooks:\n    - commit-msg\n  allow_skip: warn\n")

	var out bytes.Buffer
	err := PolicyCheck(cli.PolicyCheckOptions{Path: root}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("expected warn mode to return nil, got %v", err)
	}
	if !strings.Contains(out.String(), "commit-msg not installed") {
		t.Fatalf("expected missing hook warning in output, got %q", out.String())
	}
}

func TestPolicyCheckSucceedsWithoutConfiguredPolicy(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\n")

	var out bytes.Buffer
	err := PolicyCheck(cli.PolicyCheckOptions{Path: root}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("expected empty policy to succeed, got %v", err)
	}
	if !strings.Contains(out.String(), "No policy configured") {
		t.Fatalf("expected no policy message, got %q", out.String())
	}
}
