package domain

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func readinessFixture() (ProfileCatalog, Profile, ReadinessFacts, time.Time) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	revision := strings.Repeat("a", 40)
	profile := Profile{ID: "codex-thorough", Revision: revision, Configuration: map[string]any{"adapter": map[string]any{"runner_label": "hub-agent-codex"}, "model": map[string]any{"id": "gpt-6.1-sol", "reasoning_effort": "high"}}}
	catalog := ProfileCatalog{Repository: "octo/demo", DefaultBranch: "main", Revision: revision, State: "valid", Profiles: []Profile{profile}, Diagnostics: []Diagnostic{}}
	model, effort, version := "gpt-6.1-sol", "high", "codex-cli 0.157.1"
	workflow := WorkflowFacts{Active: true, TriggerPresent: true, JobsPresent: true, RunnerLabels: []string{"self-hosted", "linux", "hub-agent-codex"}}
	evidence := &RuntimeEvidence{Version: 1, Repository: catalog.Repository, ProfileID: profile.ID, ProfileRevision: revision, RunnerLabel: "hub-agent-codex", RequestedModel: model, RequestedReasoningEffort: effort, EffectiveModel: &model, EffectiveReasoningEffort: &effort, CLIVersion: &version, RunID: 10, RunAttempt: 2, VerifiedAt: now.Add(-time.Minute), Outcome: "verified", ReasonCode: "verified"}
	origin := &EvidenceOrigin{Repository: catalog.Repository, Revision: revision, Branch: "main", WorkflowPath: DiagnosticWorkflowPath, Event: "workflow_dispatch", RunID: 10, RunAttempt: 2, Completed: true, Authorized: true, DiagnosticSucceeded: true, RunnerLabels: workflow.RunnerLabels, StartedAt: now.Add(-2 * time.Minute), CompletedAt: now}
	return catalog, profile, ReadinessFacts{Assignment: workflow, Diagnostic: workflow, Evidence: evidence, Origin: origin}, now
}

func TestReadinessStatesAndTrustRules(t *testing.T) {
	cases := []struct {
		name, state, code string
		change            func(*ReadinessFacts)
	}{
		{"matching success", "runtime_verified", "", func(_ *ReadinessFacts) {}},
		{"canonical repository case", "runtime_verified", "", func(f *ReadinessFacts) { f.Evidence.Repository = "Octo/Demo"; f.Origin.Repository = "Octo/Demo" }},
		{"matching failure", "verification_failed", "RUNTIME_FAILED", func(f *ReadinessFacts) {
			f.Evidence.Outcome = "failed"
			f.Evidence.ReasonCode = "request_failed"
			f.Evidence.EffectiveModel = nil
		}},
		{"missing file", "configuration_missing", "MISSING_FILE", func(f *ReadinessFacts) { f.MissingFiles = []string{"docs/agent-workflows.md"} }},
		{"inactive workflow", "configuration_missing", "WORKFLOW_CONFIGURATION", func(f *ReadinessFacts) { f.Assignment.Active = false }},
		{"wrong configured runner", "configuration_missing", "WORKFLOW_CONFIGURATION", func(f *ReadinessFacts) { f.Diagnostic.RunnerLabels = []string{"self-hosted", "linux", "addons"} }},
		{"missing trigger", "configuration_missing", "WORKFLOW_CONFIGURATION", func(f *ReadinessFacts) { f.Assignment.TriggerPresent = false }},
		{"missing job", "configuration_missing", "WORKFLOW_CONFIGURATION", func(f *ReadinessFacts) { f.Diagnostic.JobsPresent = false }},
		{"no evidence", "verification_pending", "EVIDENCE_MISSING", func(f *ReadinessFacts) { f.Evidence = nil }},
		{"newer queued attempt", "verification_pending", "EVIDENCE_ORIGIN", func(f *ReadinessFacts) { f.Origin.Completed = false }},
		{"stale success", "verification_pending", "EVIDENCE_STALE", func(f *ReadinessFacts) { f.Evidence.VerifiedAt = f.Evidence.VerifiedAt.Add(-24 * time.Hour) }},
		{"stale failure", "verification_pending", "EVIDENCE_STALE", func(f *ReadinessFacts) {
			f.Evidence.Outcome = "failed"
			f.Evidence.VerifiedAt = f.Evidence.VerifiedAt.Add(-24 * time.Hour)
		}},
		{"future record", "verification_pending", "EVIDENCE_STALE", func(f *ReadinessFacts) { f.Evidence.VerifiedAt = f.Evidence.VerifiedAt.Add(time.Hour) }},
		{"before job", "verification_pending", "EVIDENCE_STALE", func(f *ReadinessFacts) { f.Evidence.VerifiedAt = f.Origin.StartedAt.Add(-time.Second) }},
		{"different repository", "verification_pending", "EVIDENCE_POLICY", func(f *ReadinessFacts) { f.Evidence.Repository = "octo/other" }},
		{"different revision", "verification_pending", "EVIDENCE_POLICY", func(f *ReadinessFacts) { f.Evidence.ProfileRevision = strings.Repeat("b", 40) }},
		{"different profile", "verification_pending", "EVIDENCE_POLICY", func(f *ReadinessFacts) { f.Evidence.ProfileID = "other" }},
		{"model mismatch", "verification_pending", "EVIDENCE_POLICY", func(f *ReadinessFacts) { f.Evidence.RequestedModel = "other" }},
		{"effort mismatch", "verification_pending", "EVIDENCE_POLICY", func(f *ReadinessFacts) { f.Evidence.RequestedReasoningEffort = "low" }},
		{"unproven effective policy", "verification_pending", "EVIDENCE_POLICY", func(f *ReadinessFacts) { f.Evidence.EffectiveModel = nil }},
		{"wrong effective policy", "verification_pending", "EVIDENCE_POLICY", func(f *ReadinessFacts) { m := "other"; f.Evidence.EffectiveModel = &m }},
		{"legacy workflow", "verification_pending", "EVIDENCE_ORIGIN", func(f *ReadinessFacts) { f.Origin.WorkflowPath = ".github/workflows/agent-runtime-diagnostic.yml" }},
		{"legacy runner", "verification_pending", "EVIDENCE_ORIGIN", func(f *ReadinessFacts) { f.Origin.RunnerLabels = []string{"self-hosted", "linux", "addons"} }},
		{"untrusted actor", "verification_pending", "EVIDENCE_ORIGIN", func(f *ReadinessFacts) { f.Origin.Authorized = false }},
		{"wrong run", "verification_pending", "EVIDENCE_ORIGIN", func(f *ReadinessFacts) { f.Evidence.RunID++ }},
		{"old attempt", "verification_pending", "EVIDENCE_ORIGIN", func(f *ReadinessFacts) { f.Evidence.RunAttempt-- }},
		{"old branch head", "verification_pending", "EVIDENCE_ORIGIN", func(f *ReadinessFacts) { f.Origin.Revision = strings.Repeat("b", 40) }},
		{"failed infrastructure", "verification_pending", "EVIDENCE_ORIGIN", func(f *ReadinessFacts) { f.Origin.DiagnosticSucceeded = false }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			// Arrange.
			catalog, profile, facts, now := readinessFixture()
			test.change(&facts)

			// Act.
			result := diagnoseReadiness(catalog, profile, facts, now)

			// Assert.
			if result.State != test.state || result.RunnerLabel != "hub-agent-codex" || result.NextAction == "" {
				t.Fatalf("result=%+v", result)
			}
			if test.code != "" && result.Diagnostics[0].Code != test.code {
				t.Fatalf("diagnostics=%+v", result.Diagnostics)
			}
			matched := result.State == "runtime_verified" || result.State == "verification_failed"
			if (result.Evidence != nil) != matched {
				t.Fatal("untrusted evidence exposed")
			}
		})
	}
}

type readinessReader struct {
	facts ReadinessFacts
	err   error
	calls int
}

func (r *readinessReader) ReadReadiness(_ context.Context, _ Repository, _ ProfileCatalog) (ReadinessFacts, error) {
	r.calls++
	return r.facts, r.err
}

func TestReadinessUseCasePreservesCatalogFailuresAndReadErrors(t *testing.T) {
	// Arrange.
	catalog, profile, facts, now := readinessFixture()
	reader := &catalogReader{file: CatalogFile{DefaultBranch: "main", Revision: catalog.Revision}}
	validator := &catalogValidator{profiles: map[string]map[string]any{profile.ID: profile.Configuration}}
	source := &readinessReader{facts: facts}
	service := NewReadinessService(NewProfileService(reader, validator), source, func() time.Time { return now })

	// Act.
	first, err := service.RepositoryReadiness(context.Background(), Repository{Owner: "octo", Name: "demo"})

	validator.diagnostics = []Diagnostic{{Code: "INVALID_YAML", Path: "/", Message: "Fix the catalog."}}

	invalid, invalidErr := service.RepositoryReadiness(context.Background(), Repository{Owner: "octo", Name: "demo"})

	validator.diagnostics = nil
	source.err = ErrAccessDenied

	_, denied := service.RepositoryReadiness(context.Background(), Repository{Owner: "octo", Name: "demo"})

	// Assert.
	if err != nil || first.Profiles[0].State != "runtime_verified" || invalidErr != nil || invalid.CatalogState != "invalid" || len(invalid.Profiles) != 0 || source.calls != 2 || !errors.Is(denied, ErrAccessDenied) {
		t.Fatalf("first=%+v invalid=%+v error=%v", first, invalid, denied)
	}
}

func TestEvidenceFreshnessBoundary(t *testing.T) {
	// Arrange.
	catalog, profile, facts, now := readinessFixture()
	facts.Evidence.VerifiedAt = now.Add(-EvidenceMaxAge)
	facts.Origin.StartedAt = facts.Evidence.VerifiedAt.Add(-time.Minute)
	facts.Origin.CompletedAt = facts.Evidence.VerifiedAt.Add(time.Minute)

	// Act.
	atBoundary := diagnoseReadiness(catalog, profile, facts, now)
	pastBoundary := diagnoseReadiness(catalog, profile, facts, now.Add(time.Nanosecond))

	// Assert.
	if atBoundary.State != "runtime_verified" || pastBoundary.State != "verification_pending" || pastBoundary.Diagnostics[0].Code != "EVIDENCE_STALE" {
		t.Fatalf("boundary=%+v past=%+v", atBoundary, pastBoundary)
	}
}
