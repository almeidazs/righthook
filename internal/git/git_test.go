package git

import (
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
