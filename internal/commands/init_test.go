package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/almeidazs/righthook/internal/cli"
)

func TestInitPrintModePrintsOnlyConfig(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "package.json"), `{"packageManager":"pnpm@9.0.0"}`)

	var out bytes.Buffer
	err := Init(cli.InitOptions{
		Yes:        true,
		PrintOnly:  true,
		CWD:        root,
		ConfigPath: filepath.Join(root, "righthook.yml"),
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(out.String(), "version: \"1\"") {
		t.Fatalf("expected config output, got %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, "righthook.yml")); !os.IsNotExist(err) {
		t.Fatalf("print mode should not write file")
	}
}

func TestInitDryRunDoesNotWrite(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "package.json"), `{"packageManager":"pnpm@9.0.0"}`)

	var out bytes.Buffer
	err := Init(cli.InitOptions{
		Yes:        true,
		DryRun:     true,
		CWD:        root,
		ConfigPath: filepath.Join(root, "righthook.yml"),
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(out.String(), "Dry run") {
		t.Fatalf("expected dry-run output")
	}
	if _, err := os.Stat(filepath.Join(root, "righthook.yml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write file")
	}
}

func TestInitRejectsNonTTYWithoutYes(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "package.json"), `{"packageManager":"pnpm@9.0.0"}`)

	var out bytes.Buffer
	err := Init(cli.InitOptions{
		CWD:        root,
		ConfigPath: filepath.Join(root, "righthook.yml"),
	}, cli.Runtime{Stdin: nil, Stdout: &out, Stderr: &out})
	if err == nil || !strings.Contains(err.Error(), "--yes --install") {
		t.Fatalf("expected non-tty error, got %v", err)
	}
}

func TestInitProtectsOverwriteWithoutForce(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "package.json"), `{"packageManager":"pnpm@9.0.0"}`)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\n")

	var out bytes.Buffer
	err := Init(cli.InitOptions{
		Yes:        true,
		CWD:        root,
		ConfigPath: filepath.Join(root, "righthook.yml"),
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected overwrite error, got %v", err)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(root, ".git", "config"), "[core]\n")
	return root
}

func mustWriteFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
