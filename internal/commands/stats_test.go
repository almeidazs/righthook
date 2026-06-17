package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/almeidazs/righthook/internal/cli"
)

func TestRunWritesStatsWhenEnabled(t *testing.T) {
	root := initGitRepo(t)
	gitRun(t, root, "commit", "--allow-empty", "-m", "init")
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nstats:\n  enabled: true\n  retention: 30d\nhooks:\n  pre-push:\n    jobs:\n      test:\n        run: printf 'run\\n' >> runs.txt\n")

	var out bytes.Buffer
	err := Run(cli.RunOptions{
		Hook: "pre-push",
		Path: root,
	}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".righthook", "stats.json"))
	if err != nil {
		t.Fatalf("read stats file: %v", err)
	}
	if !strings.Contains(string(data), "\"hook\": \"pre-push\"") {
		t.Fatalf("expected pre-push stats entry, got %q", string(data))
	}
}

func TestStatsShowsAggregates(t *testing.T) {
	root := initRepo(t)
	mustWriteFile(t, filepath.Join(root, "righthook.yml"), "version: \"1\"\nstats:\n  enabled: true\n  retention: 30d\n")
	mustWriteFile(t, filepath.Join(root, ".righthook", "stats.json"), `{
  "runs": [
    {
      "timestamp": "2026-06-17T12:00:00Z",
      "hook": "pre-commit",
      "duration_ms": 1200,
      "jobs": [
        {"name":"lint","duration_ms":1200,"status":"ran","cache_enabled":true}
      ]
    },
    {
      "timestamp": "2026-06-17T12:10:00Z",
      "hook": "pre-push",
      "duration_ms": 8400,
      "jobs": [
        {"name":"typecheck","duration_ms":6800,"status":"ran","cache_enabled":false},
        {"name":"test","duration_ms":4100,"status":"cached","cache_enabled":true}
      ]
    },
    {
      "timestamp": "2026-06-17T12:20:00Z",
      "hook": "pre-push",
      "duration_ms": 8400,
      "jobs": [
        {"name":"test","duration_ms":4100,"status":"ran","cache_enabled":true}
      ]
    }
  ]
}`)

	var out bytes.Buffer
	err := Stats(cli.StatsOptions{Path: root}, cli.Runtime{Stdin: os.Stdin, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"◇ Righthook stats",
		"Average pre-commit:",
		"1.2s",
		"Average pre-push:",
		"8.4s",
		"Cache hit rate:     33%",
		"typecheck",
		"6.8s avg",
		"test",
		"4.1s avg",
		"lint",
		"1.2s avg",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in output, got %q", want, text)
		}
	}
}
