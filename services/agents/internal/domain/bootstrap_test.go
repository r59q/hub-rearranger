package domain

import (
	"reflect"
	"strings"
	"testing"
)

type bootstrapMerger struct{ diagnostics []Diagnostic }

func (m bootstrapMerger) MergeBootstrapCatalog(_, proposed []byte) ([]byte, []Diagnostic) {
	return proposed, m.diagnostics
}

func TestBootstrapPlanIsSortedAndIdempotentAfterApplyingItsFiles(t *testing.T) {
	// Arrange.
	templates := map[string]string{"z.txt": "last\n", "AGENTS.md": BootstrapInstructions, ProfileCatalogPath: "catalog\n"}
	existing := map[string]string{"AGENTS.md": "# Existing rules\n\nKeep this text.\n", "unrelated.txt": "untouched"}

	// Act.
	first := PlanBootstrap(templates, existing, bootstrapMerger{})
	applied := map[string]string{}
	for _, file := range first.Files {
		applied[file.Path] = file.Content
	}
	second := PlanBootstrap(templates, applied, bootstrapMerger{})

	// Assert.
	if first.Files[0].Path != ProfileCatalogPath || first.Files[1].Status != "update" || first.Files[2].Status != "create" {
		t.Fatalf("unexpected plan: %+v", first.Files)
	}
	if !strings.HasPrefix(first.Files[1].Content, existing["AGENTS.md"]) || len(first.Files) != 3 {
		t.Fatal("unrelated content was modified or included")
	}
	for i, file := range second.Files {
		if file.Status != "unchanged" || file.Content != first.Files[i].Content || file.SHA256 != first.Files[i].SHA256 {
			t.Fatal("applying the plan did not converge")
		}
	}
	if !reflect.DeepEqual(first, PlanBootstrap(templates, existing, bootstrapMerger{})) {
		t.Fatal("same inputs produced a different plan")
	}
}

func TestBootstrapInstructionUpdatesPreserveSurroundingRules(t *testing.T) {
	// Arrange.
	old := "before\n" + strings.Replace(BootstrapInstructions, "GitHub remains", "Old text remains", 1) + "after\n"

	// Act.
	content, diagnostics := mergeBootstrapInstructions(old, BootstrapInstructions)

	// Assert.
	if len(diagnostics) != 0 || content != "before\n"+BootstrapInstructions+"after\n" {
		t.Fatalf("merge changed surrounding rules: %v", diagnostics)
	}
}

func TestAmbiguousInstructionsAndCatalogPolicyStopExportablePlans(t *testing.T) {
	for _, content := range []string{
		"<!-- hub-agent-bootstrap:v1:begin -->",
		"<!-- hub-agent-bootstrap:v1:end --><!-- hub-agent-bootstrap:v1:begin -->",
		BootstrapInstructions + BootstrapInstructions,
	} {
		// Arrange.
		templates := map[string]string{"AGENTS.md": BootstrapInstructions, ProfileCatalogPath: "catalog"}
		merger := bootstrapMerger{diagnostics: []Diagnostic{{Code: "PROFILE_CONFLICT", Message: "Review policy."}}}

		// Act.
		plan := PlanBootstrap(templates, map[string]string{"AGENTS.md": content}, merger)

		// Assert.
		if len(plan.Diagnostics) != 2 || plan.Files[0].Status != "conflict" || plan.Files[1].Content != "" {
			t.Fatal("ambiguous existing policy became an exportable replacement")
		}
	}
}

func TestInstructionMergeCannotProduceAnUnreadableNextSnapshot(t *testing.T) {
	// Arrange.
	existing := map[string]string{"AGENTS.md": strings.Repeat("x", ReadinessFileMaxBytes)}

	// Act.
	plan := PlanBootstrap(map[string]string{"AGENTS.md": BootstrapInstructions}, existing, bootstrapMerger{})

	// Assert.
	if len(plan.Diagnostics) != 1 || plan.Diagnostics[0].Code != "LIMIT_EXCEEDED" || plan.Files[0].Status != "conflict" {
		t.Fatal("successful generation would not fit a subsequent snapshot")
	}
}
