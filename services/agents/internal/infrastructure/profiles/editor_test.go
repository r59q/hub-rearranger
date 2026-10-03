package profiles

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestEditorExplicitlyChangesSelectedProfileAndPreservesOtherPolicies(t *testing.T) {
	// Arrange.
	v, canonical := validator(t), validCatalog(t)
	other := strings.Replace(string(canonical), "codex-thorough:", "other-profile: # Preserve me", 1)
	existing, _ := v.MergeBootstrapCatalog([]byte(other), canonical)
	before, _ := v.Validate(existing)
	draft, _ := v.ReadProfileDraft(existing, canonical)
	draft.Name = "Reviewed name"
	draft.Description = "Use only the issue and repository."
	draft.Enabled = false
	draft.ReviewComments = false
	draft.ContextSources = []string{"repository", "issue"}

	// Act.
	merged, issues := v.MergeProfileDraft(existing, canonical, draft)
	after, invalid := v.Validate(merged)

	// Assert.
	if len(issues)+len(invalid) != 0 || after["codex-thorough"]["enabled"] != false || after["codex-thorough"]["name"] != draft.Name || !reflect.DeepEqual(before["other-profile"], after["other-profile"]) || !bytes.Contains(merged, []byte("Preserve me")) {
		t.Fatalf("profile edit lost selected choices or unrelated policy: %v %v", issues, invalid)
	}
	repeat, issues := v.MergeProfileDraft(merged, canonical, draft)
	if len(issues) != 0 || !bytes.Equal(merged, repeat) {
		t.Fatal("repeat editor plan was not idempotent")
	}
}

func TestEditorPreservesUnchangedCatalogBytesAndComments(t *testing.T) {
	v, canonical := validator(t), validCatalog(t)
	existing := append([]byte("# Keep catalog comments\n"), canonical...)
	draft, _ := v.ReadProfileDraft(existing, canonical)

	merged, issues := v.MergeProfileDraft(existing, canonical, draft)

	if len(issues) != 0 || !bytes.Equal(existing, merged) {
		t.Fatal("unchanged selected policy was reformatted")
	}
}

func TestEditorCanonicalSchemaRejectsInvalidChoicesSafely(t *testing.T) {
	v, canonical := validator(t), validCatalog(t)
	for _, variant := range []string{"blank-name", "duplicate", "missing-source", "review-context", "unknown-source", "oversize"} {
		t.Run(variant, func(t *testing.T) {
			draft, _ := v.ReadProfileDraft(nil, canonical)
			switch variant {
			case "blank-name":
				draft.Name = " "
			case "duplicate":
				draft.ContextSources = append(draft.ContextSources, "issue")
			case "missing-source":
				draft.ContextSources = []string{"repository"}
			case "review-context":
				draft.ContextSources = []string{"repository", "issue"}
			case "unknown-source":
				draft.ContextSources = append(draft.ContextSources, "private-sentinel")
			case "oversize":
				draft.Description = strings.Repeat("x", 501)
			}

			merged, issues := v.MergeProfileDraft(nil, canonical, draft)

			if len(merged) != 0 || len(issues) == 0 {
				t.Fatal("invalid choices produced publishable source")
			}
			for _, issue := range issues {
				if strings.Contains(issue.Message+issue.Path, "private-sentinel") {
					t.Fatal("source value leaked in diagnostics")
				}
			}
		})
	}
}

func TestEditorCannotReplaceAnUnsupportedFixedPolicyOrInvalidCatalog(t *testing.T) {
	v, canonical := validator(t), validCatalog(t)
	draft, _ := v.ReadProfileDraft(nil, canonical)
	for _, source := range []string{strings.Replace(string(canonical), "reasoning_effort: high", "reasoning_effort: medium", 1), "schema_version: 2\nprofiles: {}", "private-sentinel: ["} {
		merged, issues := v.MergeProfileDraft([]byte(source), canonical, draft)
		state, readIssues := v.ReadProfileDraft([]byte(source), canonical)
		if len(merged) != 0 || len(issues) == 0 || len(readIssues) == 0 || state.Name != "" {
			t.Fatal("editor replaced unsupported policy")
		}
	}
}
