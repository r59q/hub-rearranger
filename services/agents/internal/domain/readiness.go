package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const AssignmentWorkflowPath = ".github/workflows/agent-assignment.yml"
const DiagnosticWorkflowPath = ".github/workflows/agent-profile-diagnostic.yml"
const EvidenceMaxAge = 24 * time.Hour
const ReadinessFileMaxBytes = 65536

var ReadinessFiles = []string{"AGENTS.md", "docs/agent-workflows.md", AssignmentWorkflowPath, DiagnosticWorkflowPath}

// RuntimeEvidence contains only allowlisted metadata, never process output or credentials.
type RuntimeEvidence struct {
	Version                  int       `json:"version"`
	Repository               string    `json:"repository"`
	ProfileID                string    `json:"profile_id"`
	ProfileRevision          string    `json:"profile_revision"`
	RunnerLabel              string    `json:"runner_label"`
	RequestedModel           string    `json:"requested_model"`
	RequestedReasoningEffort string    `json:"requested_reasoning_effort"`
	EffectiveModel           *string   `json:"effective_model"`
	EffectiveReasoningEffort *string   `json:"effective_reasoning_effort"`
	CLIVersion               *string   `json:"cli_version"`
	RunID                    int64     `json:"run_id"`
	RunAttempt               int       `json:"run_attempt"`
	VerifiedAt               time.Time `json:"verified_at"`
	Outcome                  string    `json:"outcome"`
	ReasonCode               string    `json:"reason_code"`
}

// WorkflowFacts are a narrow projection of commit-pinned configuration and
// registered Actions metadata. They do not certify execution isolation.
type WorkflowFacts struct {
	Active         bool
	TriggerPresent bool
	JobsPresent    bool
	RunnerLabels   []string
}

// Origin is verified from GitHub, independently of the artifact's claims.
type EvidenceOrigin struct {
	Repository, Revision, Branch, WorkflowPath, Event string
	RunID                                             int64
	RunAttempt                                        int
	Completed                                         bool
	Authorized                                        bool
	DiagnosticSucceeded                               bool
	RunnerLabels                                      []string
	StartedAt, CompletedAt                            time.Time
}

type ReadinessFacts struct {
	MissingFiles           []string
	Assignment, Diagnostic WorkflowFacts
	Evidence               *RuntimeEvidence
	Origin                 *EvidenceOrigin
}

type ReadinessReader interface {
	ReadReadiness(context.Context, Repository, ProfileCatalog) (ReadinessFacts, error)
}

type ProfileReadiness struct {
	ID, Revision, State, RunnerLabel, NextAction string
	Diagnostics                                  []Diagnostic
	Evidence                                     *RuntimeEvidence
}

type RepositoryReadiness struct {
	Repository, DefaultBranch, Revision, CatalogState string
	Profiles                                          []ProfileReadiness
	Diagnostics                                       []Diagnostic
}

type ReadinessService struct {
	profiles *ProfileService
	reader   ReadinessReader
	now      func() time.Time
}

func NewReadinessService(profiles *ProfileService, reader ReadinessReader, now func() time.Time) *ReadinessService {
	if now == nil {
		now = time.Now
	}
	return &ReadinessService{profiles: profiles, reader: reader, now: now}
}

func (s *ReadinessService) RepositoryReadiness(ctx context.Context, repo Repository) (RepositoryReadiness, error) {
	catalog, err := s.profiles.RepositoryProfiles(ctx, repo)
	if err != nil {
		return RepositoryReadiness{}, err
	}

	result := RepositoryReadiness{Repository: catalog.Repository, DefaultBranch: catalog.DefaultBranch, Revision: catalog.Revision, CatalogState: catalog.State, Profiles: []ProfileReadiness{}, Diagnostics: catalog.Diagnostics}
	if catalog.State != "valid" {
		return result, nil
	}

	facts, err := s.reader.ReadReadiness(ctx, repo, catalog)
	if err != nil {
		return RepositoryReadiness{}, err
	}
	if err := ctx.Err(); err != nil {
		return RepositoryReadiness{}, err
	}

	for _, profile := range catalog.Profiles {
		result.Profiles = append(result.Profiles, diagnoseReadiness(catalog, profile, facts, s.now()))
	}
	return result, nil
}

func diagnoseReadiness(catalog ProfileCatalog, profile Profile, facts ReadinessFacts, now time.Time) ProfileReadiness {
	adapter := profile.Configuration["adapter"].(map[string]any)
	model := profile.Configuration["model"].(map[string]any)
	runner := adapter["runner_label"].(string)
	result := ProfileReadiness{ID: profile.ID, Revision: profile.Revision, RunnerLabel: runner, State: "configuration_missing", Diagnostics: []Diagnostic{}}

	for _, path := range facts.MissingFiles {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "MISSING_FILE", Path: path, Message: "Add this required repository file on the default branch."})
	}

	for _, workflow := range []struct {
		path  string
		facts WorkflowFacts
	}{{AssignmentWorkflowPath, facts.Assignment}, {DiagnosticWorkflowPath, facts.Diagnostic}} {
		if !workflow.facts.Active || !workflow.facts.TriggerPresent || !workflow.facts.JobsPresent || !hasRunner(workflow.facts.RunnerLabels, runner) {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "WORKFLOW_CONFIGURATION", Path: workflow.path, Message: fmt.Sprintf("Register an active workflow with the required trigger/jobs and runner label %s.", runner)})
		}
	}

	if len(result.Diagnostics) > 0 {
		result.NextAction = "Complete the repository bootstrap files and workflow configuration, then run the profile diagnostic."
		return result
	}

	result.State = "verification_pending"
	result.NextAction = "Run the profile diagnostic on a private repository’s default branch using the required dedicated runner."
	reason := evidenceProblem(catalog, profile, model, runner, facts, now)
	if reason != "" {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: reason, Path: DiagnosticWorkflowPath, Message: "A fresh diagnostic matching this revision, runner, and exact model policy is required."})
		return result
	}

	result.Evidence = facts.Evidence
	if facts.Evidence.Outcome == "failed" {
		result.State = "verification_failed"
		result.NextAction = "Inspect the dedicated runner locally using the recovery guide, then rerun the profile diagnostic."
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "RUNTIME_FAILED", Path: DiagnosticWorkflowPath, Message: "The matching diagnostic could not verify the runtime. See its safe reason code."})
	} else {
		result.State = "runtime_verified"
		result.NextAction = "Runtime is verified for this revision; refresh the diagnostic after changes or within 24 hours."
	}
	return result
}

func hasRunner(labels []string, expected string) bool {
	for _, required := range []string{"self-hosted", "linux", expected} {
		found := false
		for _, label := range labels {
			if label == required {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func evidenceProblem(catalog ProfileCatalog, profile Profile, model map[string]any, runner string, facts ReadinessFacts, now time.Time) string {
	e, o := facts.Evidence, facts.Origin
	if e == nil || o == nil {
		return "EVIDENCE_MISSING"
	}

	if !o.Completed || !o.Authorized || !o.DiagnosticSucceeded || o.WorkflowPath != DiagnosticWorkflowPath || o.Event != "workflow_dispatch" || !strings.EqualFold(o.Repository, catalog.Repository) || o.Branch != catalog.DefaultBranch || o.Revision != catalog.Revision || !hasRunner(o.RunnerLabels, runner) || e.RunID != o.RunID || e.RunAttempt != o.RunAttempt {
		return "EVIDENCE_ORIGIN"
	}

	if e.Version != 1 || !strings.EqualFold(e.Repository, catalog.Repository) || e.ProfileID != profile.ID || e.ProfileRevision != profile.Revision || e.RunnerLabel != runner || e.RequestedModel != model["id"] || e.RequestedReasoningEffort != model["reasoning_effort"] {
		return "EVIDENCE_POLICY"
	}

	if e.VerifiedAt.After(now.Add(time.Minute)) || now.Sub(e.VerifiedAt) > EvidenceMaxAge || e.VerifiedAt.Before(o.StartedAt) || e.VerifiedAt.After(o.CompletedAt.Add(time.Minute)) || o.CompletedAt.After(now.Add(time.Minute)) {
		return "EVIDENCE_STALE"
	}

	if e.Outcome == "verified" && (e.EffectiveModel == nil || *e.EffectiveModel != e.RequestedModel || e.EffectiveReasoningEffort == nil || *e.EffectiveReasoningEffort != e.RequestedReasoningEffort || e.CLIVersion == nil || e.ReasonCode != "verified") {
		return "EVIDENCE_POLICY"
	}
	if e.Outcome != "verified" && e.Outcome != "failed" {
		return "EVIDENCE_POLICY"
	}
	return ""
}
