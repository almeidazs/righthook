package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookScriptIncludesDevBinaryFallback(t *testing.T) {
	script := HookScript("pre-commit")

	if !strings.Contains(script, `if [ -x "./righthook" ]; then`) {
		t.Fatalf("expected local dev binary fallback in hook script")
	}
	if !strings.Contains(script, `exec ./righthook run pre-commit "$@"`) {
		t.Fatalf("expected local dev binary exec path in hook script")
	}
	if !strings.Contains(script, `PATH, ./node_modules/.bin, or ./righthook`) {
		t.Fatalf("expected updated not-found message")
	}
}

func TestResolveRepositoryFromRootAndDotGit(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[core]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repoFromRoot, err := ResolveRepository(root)
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	if repoFromRoot.Root != root {
		t.Fatalf("expected root %s, got %s", root, repoFromRoot.Root)
	}
	if repoFromRoot.EffectiveHooksDir != filepath.Join(root, ".git", "hooks") {
		t.Fatalf("unexpected hooks dir %s", repoFromRoot.EffectiveHooksDir)
	}

	repoFromDotGit, err := ResolveRepository(filepath.Join(root, ".git"))
	if err != nil {
		t.Fatalf("resolve .git: %v", err)
	}
	if repoFromDotGit.Root != root {
		t.Fatalf("expected root %s from .git path, got %s", root, repoFromDotGit.Root)
	}
}

func TestListInstalledHooksMarksRighthookOwnership(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[core]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "hooks", "pre-commit"), []byte(HookScript("pre-commit")), 0o755); err != nil {
		t.Fatal(err)
	}

	repo, err := ResolveRepository(root)
	if err != nil {
		t.Fatalf("resolve repo: %v", err)
	}
	files := ListInstalledHooks(repo, []string{"pre-commit", "pre-push"})
	if len(files) != 1 {
		t.Fatalf("expected one installed hook, got %d", len(files))
	}
	if !files[0].IsRighthook {
		t.Fatalf("expected hook to be marked as Righthook-managed")
	}
}
