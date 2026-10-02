package evidence

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEvidenceUsesClosedSchemaAndSafeValues(t *testing.T) {
	base := map[string]any{"version": 1, "repository": "octo/demo", "profile_id": "codex-thorough", "profile_revision": strings.Repeat("a", 40), "runner_label": "hub-agent-codex", "requested_model": "gpt-6.1-sol", "requested_reasoning_effort": "high", "effective_model": "gpt-6.1-sol", "effective_reasoning_effort": "high", "cli_version": "codex-cli 0.157.1", "run_id": 10, "run_attempt": 1, "verified_at": "2026-10-01T12:00:00Z", "outcome": "verified", "reason_code": "verified"}
	decoder, err := NewDecoder()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		change func(map[string]any)
		valid  bool
	}{
		{"success", func(_ map[string]any) {}, true},
		{"safe failure", func(v map[string]any) {
			v["outcome"] = "failed"
			v["reason_code"] = "authentication_unavailable"
			v["effective_model"] = nil
			v["effective_reasoning_effort"] = nil
			v["cli_version"] = nil
		}, true},
		{"unknown version", func(v map[string]any) { v["version"] = 2 }, false},
		{"raw output", func(v map[string]any) { v["stderr"] = "private-sentinel" }, false},
		{"unsafe reason", func(v map[string]any) { v["reason_code"] = "private-sentinel" }, false},
		{"unsafe version", func(v map[string]any) { v["cli_version"] = "private-sentinel" }, false},
		{"wrong model", func(v map[string]any) { v["effective_model"] = "other" }, false},
		{"unproven model", func(v map[string]any) { v["effective_model"] = nil }, false},
		{"bad date", func(v map[string]any) { v["verified_at"] = "2026-02-30T12:00:00Z" }, false},
		{"missing field", func(v map[string]any) { delete(v, "repository") }, false},
		{"inconsistent failure", func(v map[string]any) { v["outcome"] = "failed" }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Arrange.
			value := map[string]any{}
			for k, v := range base {
				value[k] = v
			}
			test.change(value)
			content, _ := json.Marshal(value)

			// Act.
			result, err := decoder.Decode(content)

			// Assert.
			if (err == nil) != test.valid || (result != nil) != test.valid {
				t.Fatalf("result=%+v error=%v", result, err)
			}
		})
	}
	for _, content := range []string{`{"version":1,"version":1}`, strings.Repeat(" ", MaxBytes+1), `{"version":1} {}`} {
		if _, err := decoder.Decode([]byte(content)); err == nil {
			t.Fatal("ambiguous/unbounded JSON accepted")
		}
	}
}
