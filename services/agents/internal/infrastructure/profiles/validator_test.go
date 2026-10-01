package profiles

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func validCatalog(t *testing.T) []byte {
	t.Helper()
	content, err := os.ReadFile("../../../../../.github/agent-profiles.yml")
	if err != nil {
		t.Fatal(err)
	}
	return content
}
func validator(t *testing.T) *Validator {
	t.Helper()
	validator, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	return validator
}

func TestValidCodexThoroughExposesPolicyWithoutRuntimeVerification(t *testing.T) {
	// Arrange.
	v := validator(t)
	content := validCatalog(t)
	// Act.
	profiles, diagnostics := v.Validate(content)
	// Assert.
	if len(diagnostics) != 0 || len(profiles) != 1 {
		t.Fatalf("profiles = %v, diagnostics = %v", profiles, diagnostics)
	}
	profile := profiles["codex-thorough"]
	if profile["model"].(map[string]any)["id"] != "gpt-6.1-sol" || profile["authority"].(map[string]any)["mode"] != "branch-draft-pr" {
		t.Fatal("policy was not preserved")
	}
}

func TestInvalidCatalogsAreBoundedSafeAndActionable(t *testing.T) {
	cases := []struct {
		name    string
		content []byte
		code    string
	}{
		{"malformed", []byte("private-sentinel: ["), "INVALID_YAML"},
		{"invalid UTF8", []byte{255}, "INVALID_YAML"},
		{"empty", nil, "INVALID_TYPE"},
		{"sequence", []byte("[]"), "INVALID_TYPE"},
		{"unsupported", []byte("schema_version: 2\nprofiles: {}"), "UNSUPPORTED_VERSION"},
		{"missing fields", []byte("schema_version: 1\nprofiles: {private-sentinel: {}}"), "MISSING_FIELD"},
		{"duplicate", []byte("schema_version: 1\nschema_version: 1"), "DUPLICATE_KEY"},
		{"anchor", []byte("profiles: &private-sentinel {}"), "INVALID_YAML"},
		{"alias", []byte("profiles: *private-sentinel"), "INVALID_YAML"},
		{"merge", []byte("profiles: {<<: {}}"), "INVALID_YAML"},
		{"tag", []byte("profiles: !!map {}"), "INVALID_YAML"},
		{"tagged key", []byte("!!str schema_version: 1"), "INVALID_YAML"},
		{"multiple documents", []byte("schema_version: 1\n---\nprivate-sentinel: true"), "INVALID_YAML"},
		{"complex key", []byte("[private-sentinel]: value"), "INVALID_YAML"},
		{"size", []byte(strings.Repeat(" ", MaxBytes+1)), "LIMIT_EXCEEDED"},
		{"depth", []byte(strings.Repeat("[", maxDepth+1) + strings.Repeat("]", maxDepth+1)), "LIMIT_EXCEEDED"},
		{"depth boundary", []byte(strings.Repeat("[", maxDepth) + strings.Repeat("]", maxDepth)), "INVALID_TYPE"},
		{"size boundary", []byte(strings.Repeat(" ", MaxBytes)), "INVALID_TYPE"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			// Arrange.
			v := validator(t)
			// Act.
			profiles, diagnostics := v.Validate(item.content)
			// Assert.
			if profiles != nil || len(diagnostics) == 0 || diagnostics[0].Code != item.code {
				t.Fatalf("profiles = %v, diagnostics = %v", profiles, diagnostics)
			}
			encoded, _ := json.Marshal(diagnostics)
			if strings.Contains(string(encoded), "private-sentinel") {
				t.Fatal("source data leaked")
			}
		})
	}
}

func TestSchemaRulesAndSafeDiagnosticPaths(t *testing.T) {
	cases := []struct{ before, after, code, path string }{
		{"enabled: true", "enabled: True", "INVALID_TYPE", "/profiles/<profile>/enabled"},
		{"enabled: true", "enabled: on", "INVALID_TYPE", "/profiles/<profile>/enabled"},
		{"enabled: true", "enabled: yes", "INVALID_TYPE", "/profiles/<profile>/enabled"},
		{"mode: branch-draft-pr", "mode: private-sentinel", "UNSUPPORTED_VALUE", "/profiles/<profile>/authority/mode"},
		{"runner_label: hub-agent-codex", "runner_label: self-hosted", "INVALID_FORMAT", "/profiles/<profile>/adapter/runner_label"},
		{"runner_label: hub-agent-codex", "runner_label: linux", "INVALID_FORMAT", "/profiles/<profile>/adapter/runner_label"},
		{"network: false", "network: 0", "INVALID_TYPE", "/profiles/<profile>/authority/network"},
		{"reasoning_effort: high", "reasoning_effort: private-sentinel", "UNSUPPORTED_VALUE", "/profiles/<profile>/model/reasoning_effort"},
		{"fallback: none", "fallback: none\n      private-sentinel: secret", "UNSUPPORTED_FIELD", "/profiles/<profile>/model"},
		{"issue_comments,", "issue,", "DUPLICATE_VALUE", "/profiles/<profile>/context/sources"},
		{"checks,", "instructions,", "DUPLICATE_VALUE", "/profiles/<profile>/context/sources"},
	}
	for _, item := range cases {
		t.Run(item.after, func(t *testing.T) {
			// Arrange.
			content := strings.Replace(string(validCatalog(t)), item.before, item.after, 1)
			v := validator(t)
			// Act.
			profiles, diagnostics := v.Validate([]byte(content))
			// Assert.
			found := false
			for _, issue := range diagnostics {
				if issue.Code == item.code && issue.Path == item.path {
					found = true
				}
			}
			if profiles != nil || !found {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
			encoded, _ := json.Marshal(diagnostics)
			if strings.Contains(string(encoded), "private-sentinel") {
				t.Fatal("source data leaked")
			}
		})
	}
}

func TestReviewContinuationNamesMissingContext(t *testing.T) {
	// Arrange.
	content := strings.Replace(string(validCatalog(t)), "review_thread,", "instructions,", 1)
	v := validator(t)
	// Act.
	_, diagnostics := v.Validate([]byte(content))
	// Assert.
	found := false
	for _, issue := range diagnostics {
		if issue.Code == "MISSING_CONTEXT" && issue.Path == "/profiles/<profile>/context/sources" && issue.Message == "Include context source: review_thread." {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestDisabledProfilesAndUnverifiedModelsRemainConfiguration(t *testing.T) {
	// Arrange.
	content := strings.Replace(string(validCatalog(t)), "enabled: true", "enabled: false", 1)
	content = strings.Replace(content, "id: gpt-6.1-sol", "id: future-model", 1)
	v := validator(t)
	// Act.
	profiles, diagnostics := v.Validate([]byte(content))
	// Assert.
	if len(diagnostics) != 0 || profiles["codex-thorough"]["enabled"] != false || profiles["codex-thorough"]["model"].(map[string]any)["id"] != "future-model" {
		t.Fatalf("profiles = %v, diagnostics = %v", profiles, diagnostics)
	}
}
