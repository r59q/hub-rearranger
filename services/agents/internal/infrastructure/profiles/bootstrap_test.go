package profiles

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestBootstrapPreservesAnExistingMatchingCatalogExactly(t *testing.T) {
	// Arrange.
	v := validator(t)
	canonical := validCatalog(t)
	existing := append([]byte("# Operator comment\n"), canonical...)

	// Act.
	merged, diagnostics := v.MergeBootstrapCatalog(existing, canonical)

	// Assert.
	if len(diagnostics) != 0 || !bytes.Equal(existing, merged) {
		t.Fatal("matching policy was reformatted or replaced")
	}
}

func TestBootstrapAddsOnlyTheMissingProfileAndPreservesOtherPolicies(t *testing.T) {
	// Arrange.
	v := validator(t)
	canonical := validCatalog(t)
	existing := []byte(strings.Replace(string(canonical), "codex-thorough:", "other-profile: # Keep this comment", 1))
	before, _ := v.Validate(existing)

	// Act.
	merged, diagnostics := v.MergeBootstrapCatalog(existing, canonical)
	after, issues := v.Validate(merged)

	// Assert.
	if len(diagnostics) != 0 || len(issues) != 0 || len(after) != 2 || !reflect.DeepEqual(before["other-profile"], after["other-profile"]) || !bytes.Contains(merged, []byte("Keep this comment")) {
		t.Fatalf("existing profile was lost: %v %v", diagnostics, issues)
	}
	second, diagnostics := v.MergeBootstrapCatalog(merged, canonical)
	if len(diagnostics) != 0 || !bytes.Equal(second, merged) {
		t.Fatal("merged catalog was not idempotent")
	}
}

func TestBootstrapNeverEnablesOrReplacesAnExistingProfilePolicy(t *testing.T) {
	v := validator(t)
	canonical := validCatalog(t)
	for _, existing := range [][]byte{
		[]byte(strings.Replace(string(canonical), "enabled: true", "enabled: false", 1)),
		[]byte(strings.Replace(string(canonical), "reasoning_effort: high", "reasoning_effort: medium", 1)),
		[]byte("private-sentinel: ["),
		[]byte("schema_version: 2\nprofiles: {}"),
	} {
		// Act.
		merged, diagnostics := v.MergeBootstrapCatalog(existing, canonical)

		// Assert.
		if len(diagnostics) == 0 || len(merged) != 0 {
			t.Fatal("invalid or changed policy was silently replaced")
		}
		for _, diagnostic := range diagnostics {
			if strings.Contains(diagnostic.Message+diagnostic.Path, "private-sentinel") {
				t.Fatal("private content leaked into diagnostics")
			}
		}
	}
}
