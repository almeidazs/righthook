package config

import "testing"

func TestEncodeDecodeAcrossFormats(t *testing.T) {
	cfg := New(true, "smart")
	cfg.Hooks["pre-commit"] = Hook{
		Jobs: map[string]Job{
			"secrets": {Run: "righthook secrets scan {staged}", Files: "staged"},
		},
	}

	for _, format := range []string{"yaml", "json", "toml"} {
		data, err := Encode(cfg, format)
		if err != nil {
			t.Fatalf("encode %s: %v", format, err)
		}
		out, err := Decode(data, format)
		if err != nil {
			t.Fatalf("decode %s: %v", format, err)
		}
		if out.Version != "1" {
			t.Fatalf("version mismatch for %s", format)
		}
	}
}

func TestValidateAllowsDisabledJobWithoutRun(t *testing.T) {
	cfg := New(true, "smart")
	cfg.Hooks["pre-commit"] = Hook{
		Jobs: map[string]Job{
			"typecheck": {Enabled: Enabled(false)},
		},
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestValidateRejectsUnsupportedValues(t *testing.T) {
	cfg := New(true, "smart")
	cfg.Output.Mode = "loud"
	cfg.Hooks["pre-commit"] = Hook{
		Jobs: map[string]Job{
			"lint": {Run: "echo ok", Files: "mystery"},
		},
	}

	if err := Validate(cfg); err == nil {
		t.Fatalf("expected validation error for unsupported values")
	}
}

func TestValidateRejectsInvalidPolicyValues(t *testing.T) {
	cfg := New(true, "smart")
	cfg.Policy.RequiredVersion = "wat"
	cfg.Policy.AllowSkip = "maybe"
	cfg.Policy.RequiredHooks = []string{"post-merge"}
	cfg.Stats.Retention = "tomorrow"

	if err := Validate(cfg); err == nil {
		t.Fatalf("expected validation error for invalid policy values")
	}
}
