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

func TestInstallHookSpecific(t *testing.T) {
	root := initRepo(t)

	var out bytes.Buffer
	err := Install(cli.InstallOptions{
		Hook: "pre-commit",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	path := filepath.Join(root, ".git", "hooks", "pre-commit")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	if !strings.Contains(string(data), "righthook run pre-commit") {
		t.Fatalf("expected installed pre-commit hook")
	}
	if !strings.Contains(out.String(), "hook source: explicit --hook") {
		t.Fatalf("expected explicit hook source in output, got %q", out.String())
	}
}

func TestInstallUsesConfigHooksByDefault(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  pre-push:\n    jobs:\n      test:\n        run: go test ./...\n")

	var out bytes.Buffer
	err := Install(cli.InstallOptions{
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".git", "hooks", "pre-push")); err != nil {
		t.Fatalf("expected pre-push hook: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "hooks", "pre-commit")); !os.IsNotExist(err) {
		t.Fatalf("did not expect pre-commit hook to be installed")
	}
	if !strings.Contains(out.String(), "hook source: "+filepath.Join(root, "righthook.yml")) {
		t.Fatalf("expected config hook source in output, got %q", out.String())
	}
}

func TestResolveRepoConfigPathPrefersLocalConfig(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\n")
	mustWriteFile(t, filepath.Join(root, "righthook.local.toml"), "version = \"1\"\n")

	got := resolveRepoConfigPath(root, "")
	want := filepath.Join(root, "righthook.local.toml")
	if got != want {
		t.Fatalf("expected local config %q, got %q", want, got)
	}
}

func TestResolveRepoConfigPathSupportsConfigDirectory(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".config", "righthook.json"), "{\n  \"version\": \"1\"\n}\n")

	got := resolveRepoConfigPath(root, "")
	want := filepath.Join(root, ".config", "righthook.json")
	if got != want {
		t.Fatalf("expected .config config %q, got %q", want, got)
	}
}

func TestResolveRepoConfigPathKeepsExplicitConfigPriority(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "righthook.local.yml"), "version: \"1\"\n")

	got := resolveRepoConfigPath(root, "custom/righthook.toml")
	want := filepath.Join(root, "custom", "righthook.toml")
	if got != want {
		t.Fatalf("expected explicit config %q, got %q", want, got)
	}
}

func TestInstallIgnoresHooksWithOnlyDisabledJobs(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  pre-commit:\n    jobs:\n      typecheck:\n        enabled: false\n  pre-push:\n    jobs:\n      test:\n        run: go test ./...\n")

	var out bytes.Buffer
	err := Install(cli.InstallOptions{
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".git", "hooks", "pre-push")); err != nil {
		t.Fatalf("expected pre-push hook: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "hooks", "pre-commit")); !os.IsNotExist(err) {
		t.Fatalf("did not expect pre-commit hook to be installed")
	}
}

func TestInstallProtectsOverwriteWithoutForce(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, ".git", "hooks", "pre-commit"), "#!/bin/sh\n")

	var out bytes.Buffer
	err := Install(cli.InstallOptions{
		Hook: "pre-commit",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected overwrite error, got %v", err)
	}
}

func TestInstallAcceptsGitDirectoryPath(t *testing.T) {
	root := initRepo(t)

	var out bytes.Buffer
	err := Install(cli.InstallOptions{
		Hook: "pre-push",
		Path: filepath.Join(root, ".git"),
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("install with .git path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "hooks", "pre-push")); err != nil {
		t.Fatalf("expected pre-push hook: %v", err)
	}
}

func TestInstallFallsBackWhenConfigIsInvalid(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"2\"\n")

	var out bytes.Buffer
	err := Install(cli.InstallOptions{
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("install with invalid config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "hooks", "pre-commit")); err != nil {
		t.Fatalf("expected fallback pre-commit hook: %v", err)
	}
	if !strings.Contains(out.String(), "falling back to supported v1 hooks") {
		t.Fatalf("expected fallback warning in output, got %q", out.String())
	}
}

func TestUninstallAllRemovesHooksAndConfig(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\n")
	mustWriteFile(t, filepath.Join(root, ".git", "hooks", "pre-commit"), git.HookScript("pre-commit"))
	mustWriteFile(t, filepath.Join(root, ".git", "hooks", "pre-push"), git.HookScript("pre-push"))

	var out bytes.Buffer
	err := Uninstall(cli.UninstallOptions{
		All:          true,
		RemoveConfig: true,
		Path:         root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".git", "hooks", "pre-commit")); !os.IsNotExist(err) {
		t.Fatalf("expected pre-commit hook to be removed")
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "hooks", "pre-push")); !os.IsNotExist(err) {
		t.Fatalf("expected pre-push hook to be removed")
	}
	if _, err := os.Stat(filepath.Join(root, "righthook.yml")); !os.IsNotExist(err) {
		t.Fatalf("expected config to be removed")
	}
}

func TestUninstallRemoveConfigRemovesDiscoveredLocalConfig(t *testing.T) {
	root := initRepo(t)
	localConfig := filepath.Join(root, "righthook.local.yaml")
	mustWriteFile(t, localConfig, "version: \"1\"\n")
	mustWriteFile(t, filepath.Join(root, ".git", "hooks", "pre-commit"), git.HookScript("pre-commit"))

	var out bytes.Buffer
	err := Uninstall(cli.UninstallOptions{
		All:          true,
		RemoveConfig: true,
		Path:         root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	if _, err := os.Stat(localConfig); !os.IsNotExist(err) {
		t.Fatalf("expected discovered local config to be removed")
	}
}

func TestUninstallRejectsNonTTYWithoutHookOrAll(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, ".git", "hooks", "pre-commit"), git.HookScript("pre-commit"))

	var out bytes.Buffer
	err := Uninstall(cli.UninstallOptions{
		Path: root,
	}, cli.Runtime{Stdin: nil, Stdout: &out, Stderr: &out})
	if err == nil || !strings.Contains(err.Error(), "--all or --hook") {
		t.Fatalf("expected non-interactive uninstall error, got %v", err)
	}
}

func TestUninstallRejectsHookNotManagedByRighthook(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, ".git", "hooks", "pre-commit"), "#!/bin/sh\necho custom\n")

	var out bytes.Buffer
	err := Uninstall(cli.UninstallOptions{
		Hook: "pre-commit",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err == nil || !strings.Contains(err.Error(), "not managed by Righthook") {
		t.Fatalf("expected managed-by-righthook error, got %v", err)
	}
}
