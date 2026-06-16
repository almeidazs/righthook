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

func TestTraceWritesJSONFileAndRespectsOnly(t *testing.T) {
	root := initGitRepo(t)
	gitRun(t, root, "commit", "--allow-empty", "-m", "init")
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\ncache:\n  enabled: true\n  dir: .righthook/cache\n  ttl: 1h\nhooks:\n  pre-commit:\n    jobs:\n      gofmt:\n        run: gofmt -w {staged}\n        files: staged\n        glob:\n          - '*.go'\n        stage_fixed: true\n      test:\n        run: go test ./...\n        cache: true\n")
	mustWriteFile(t, filepath.Join(root, "main.go"), "package main\n")
	gitRun(t, root, "add", "main.go")

	outputPath := filepath.Join(root, "trace.json")
	var out bytes.Buffer
	err := Trace(cli.TraceOptions{
		Hook:       "pre-commit",
		Path:       root,
		Only:       []string{"gofmt"},
		OutputPath: outputPath,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("trace: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read trace json: %v", err)
	}

	var trace struct {
		Hook string `json:"hook"`
		Jobs []struct {
			Name            string `json:"name"`
			ExpandedCommand string `json:"expanded_command"`
			FileSelector    string `json:"file_selector"`
			CWD             string `json:"cwd"`
			Status          string `json:"status"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatalf("unmarshal trace json: %v", err)
	}

	if trace.Hook != "pre-commit" {
		t.Fatalf("expected pre-commit hook, got %q", trace.Hook)
	}
	if len(trace.Jobs) != 1 || trace.Jobs[0].Name != "gofmt" {
		t.Fatalf("expected only gofmt job, got %+v", trace.Jobs)
	}
	if trace.Jobs[0].FileSelector != "staged" {
		t.Fatalf("expected staged file selector, got %+v", trace.Jobs[0])
	}
	if !strings.Contains(trace.Jobs[0].ExpandedCommand, "main.go") {
		t.Fatalf("expected expanded command to include staged file, got %+v", trace.Jobs[0])
	}
	if trace.Jobs[0].CWD != root {
		t.Fatalf("expected cwd %q, got %+v", root, trace.Jobs[0])
	}
	if !strings.Contains(out.String(), "trace json:") {
		t.Fatalf("expected json output path in terminal, got %q", out.String())
	}
}

func TestTracePrintsDetailedTerminalOutputWithoutJSONFile(t *testing.T) {
	root := initGitRepo(t)
	gitRun(t, root, "commit", "--allow-empty", "-m", "init")
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nhooks:\n  pre-push:\n    jobs:\n      test:\n        run: printf 'ok\\n'\n")

	var out bytes.Buffer
	err := Trace(cli.TraceOptions{
		Hook: "pre-push",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("trace without json output: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "◇ righthook trace") {
		t.Fatalf("expected trace header, got %q", text)
	}
	if !strings.Contains(text, "◆ Jobs") || !strings.Contains(text, "command: printf 'ok\\n'") {
		t.Fatalf("expected detailed job output, got %q", text)
	}
	if strings.Contains(text, "trace json:") {
		t.Fatalf("did not expect json path in terminal output, got %q", text)
	}
}
