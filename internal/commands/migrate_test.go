package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/almeidazs/righthook/internal/cli"
)

func TestMigrateLefthookDryRun(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "lefthook.yml"), "pre-commit:\n  commands:\n    format:\n      run: biome check --write {staged}\ncommit-msg:\n  commands:\n    commitlint:\n      run: pnpm commitlint --edit {commit_msg_file}\n")

	var out bytes.Buffer
	err := Migrate(cli.MigrateOptions{
		Target:           "lefthook",
		Path:             root,
		DryRun:           true,
		KeepTargetConfig: true,
	}, cli.Runtime{Stdin: nil, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("migrate dry-run: %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"righthook migrate",
		"target: lefthook",
		"Dry run",
		"format:",
		"biome check --write {staged}",
		"commitlint:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in output, got %q", want, text)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "righthook.yml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write righthook config")
	}
}

func TestMigrateHuskyWritesConfig(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, ".husky", "pre-commit"), "#!/usr/bin/env sh\n. \"$(dirname -- \"$0\")/_/husky.sh\"\npnpm lint {staged}\npnpm test --changed\n")

	var out bytes.Buffer
	err := Migrate(cli.MigrateOptions{
		Target:           "husky",
		Path:             root,
		Write:            true,
		KeepTargetConfig: true,
	}, cli.Runtime{Stdin: nil, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("migrate husky: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "righthook.yml"))
	if err != nil {
		t.Fatalf("read righthook config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "pre-commit:") || !strings.Contains(text, "migrated:") {
		t.Fatalf("expected migrated husky hook in config, got %q", text)
	}
	if !strings.Contains(text, "pnpm lint {staged} && pnpm test --changed") {
		t.Fatalf("expected collapsed husky commands, got %q", text)
	}
}

func TestMigrateMergesIntoExistingConfig(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  pre-commit:\n    jobs:\n      format:\n        run: existing format\n")
	mustWriteFile(t, filepath.Join(root, "lefthook.yml"), "pre-commit:\n  commands:\n    format:\n      run: new format\n    lint:\n      run: new lint\n")

	var out bytes.Buffer
	err := Migrate(cli.MigrateOptions{
		Target:           "lefthook",
		Path:             root,
		Write:            true,
		KeepTargetConfig: true,
	}, cli.Runtime{Stdin: nil, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("migrate merge: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "righthook.yml"))
	if err != nil {
		t.Fatalf("read merged config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "run: existing format") {
		t.Fatalf("expected existing job to win conflict, got %q", text)
	}
	if strings.Contains(text, "run: new format") {
		t.Fatalf("did not expect migrated conflicting job to override existing config, got %q", text)
	}
	if !strings.Contains(text, "lint:") || !strings.Contains(text, "run: new lint") {
		t.Fatalf("expected non-conflicting job to be merged, got %q", text)
	}
}

func TestMigrateRemovesTargetConfigWhenRequested(t *testing.T) {
	root := initRepo(t)
	left := filepath.Join(root, "lefthook.yml")
	mustWriteFile(t, left, "pre-push:\n  commands:\n    test:\n      run: go test ./...\n")

	var out bytes.Buffer
	err := Migrate(cli.MigrateOptions{
		Target:           "lefthook",
		Path:             root,
		Write:            true,
		KeepTargetConfig: false,
	}, cli.Runtime{Stdin: nil, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("migrate remove source: %v", err)
	}

	if _, err := os.Stat(left); !os.IsNotExist(err) {
		t.Fatalf("expected target config to be removed")
	}
}

func TestMigrateRejectsNonInteractiveWithoutWriteOrDryRun(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "lefthook.yml"), "pre-commit:\n  commands:\n    lint:\n      run: pnpm lint {staged}\n")

	var out bytes.Buffer
	err := Migrate(cli.MigrateOptions{
		Target:           "lefthook",
		Path:             root,
		KeepTargetConfig: true,
	}, cli.Runtime{Stdin: nil, Stdout: &out, Stderr: &out})
	if err == nil || !strings.Contains(err.Error(), "--write or --dry-run") {
		t.Fatalf("expected non-interactive error, got %v", err)
	}
}

func TestShouldWriteMigratedConfigUsesPromptWhenInteractive(t *testing.T) {
	prev := confirmMigrateWrite
	t.Cleanup(func() {
		confirmMigrateWrite = prev
	})

	called := false
	confirmMigrateWrite = func(rt cli.Runtime, path string) (bool, error) {
		called = true
		return true, nil
	}

	ok, err := shouldWriteMigratedConfig(cli.ResolvedMigrateOptions{Interactive: true}, cli.Runtime{}, "/tmp/righthook.yml")
	if err != nil {
		t.Fatalf("shouldWriteMigratedConfig: %v", err)
	}
	if !ok || !called {
		t.Fatalf("expected interactive prompt to be used")
	}
}
