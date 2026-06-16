package cli

import (
	"strings"
	"testing"
)

func TestResolveTraceOptionsRequiresJSONOutput(t *testing.T) {
	_, err := ResolveTraceOptions(TraceOptions{
		Hook:       "pre-commit",
		Path:       ".",
		OutputPath: "trace.txt",
	})
	if err == nil || !strings.Contains(err.Error(), ".json") {
		t.Fatalf("expected .json validation error, got %v", err)
	}
}

func TestResolveTraceOptionsAllowsEmptyOutput(t *testing.T) {
	resolved, err := ResolveTraceOptions(TraceOptions{
		Hook: "pre-commit",
		Path: ".",
	})
	if err != nil {
		t.Fatalf("expected empty output to be allowed, got %v", err)
	}
	if resolved.OutputPath != "" {
		t.Fatalf("expected empty output path, got %q", resolved.OutputPath)
	}
}
