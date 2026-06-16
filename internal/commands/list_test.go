package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/almeidazs/righthook/internal/cli"
)

func TestListRendersHooksAndJobs(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  pre-commit:\n    jobs:\n      format:\n        run: biome check --write {staged}\n      typecheck:\n        enabled: false\n  pre-push:\n    jobs:\n      test:\n        run: pnpm test --changed\n")

	var out bytes.Buffer
	err := List(cli.ListOptions{Path: root}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"pre-commit",
		"  ✓ format",
		"biome check --write {staged}",
		"  - typecheck",
		"disabled",
		"pre-push",
		"pnpm test --changed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got %q", want, got)
		}
	}
}

func TestListOnlyJobs(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  commit-msg:\n    jobs:\n      commitlint:\n        run: pnpm commitlint --edit {commit_msg_file}\n")

	var out bytes.Buffer
	err := List(cli.ListOptions{Path: root, OnlyJobs: true}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("list --only-jobs: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if strings.Contains(got, "commit-msg") {
		t.Fatalf("did not expect hook heading in output, got %q", got)
	}
	if !strings.Contains(got, "✓ commitlint") {
		t.Fatalf("expected job-only output, got %q", got)
	}
}

func TestListJSONOnlyJobs(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  pre-commit:\n    jobs:\n      lint:\n        run: pnpm lint {staged}\n      typecheck:\n        enabled: false\n")

	var out bytes.Buffer
	err := List(cli.ListOptions{Path: root, JSON: true, OnlyJobs: true}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("list --json --only-jobs: %v", err)
	}

	var result struct {
		ConfigPath string `json:"config_path"`
		Jobs       []struct {
			Hook    string `json:"hook"`
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
			Run     string `json:"run"`
			Status  string `json:"status"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if len(result.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(result.Jobs))
	}
	if result.Jobs[0].Hook != "pre-commit" || result.Jobs[0].Name != "lint" || !result.Jobs[0].Enabled {
		t.Fatalf("unexpected first job: %+v", result.Jobs[0])
	}
	if result.Jobs[1].Name != "typecheck" || result.Jobs[1].Enabled || result.Jobs[1].Status != "disabled" {
		t.Fatalf("unexpected second job: %+v", result.Jobs[1])
	}
}
