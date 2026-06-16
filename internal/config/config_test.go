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
