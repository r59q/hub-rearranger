package github

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/evidence"
)

const assignmentYAML = "on:\n  issue_comment:\n    types: [created]\njobs:\n  authorize: {}\n  patch:\n    runs-on: [self-hosted, linux, hub-agent-codex]\n  publish: {}\n"
const diagnosticYAML = "on:\n  workflow_dispatch:\njobs:\n  authorize: {}\n  diagnostic:\n    runs-on: [self-hosted, linux, hub-agent-codex]\n"

func testArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestReadinessReadsCommitPinnedFilesAndAuthenticatesEvidenceOrigin(t *testing.T) {
	for _, scenario := range []string{"verified", "failed", "missing file", "inactive workflow", "queued", "old attempt", "expired", "bad digest", "bad archive", "unsafe evidence", "fork", "wrong workflow", "actions denied", "actions limited", "storage redirect"} {
		t.Run(scenario, func(t *testing.T) {
			// Arrange: realistic go-github REST and separate unauthenticated HTTPS storage.
			now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
			model, effort, version := "gpt-6.1-sol", "high", "codex-cli 0.157.1"
			record := domain.RuntimeEvidence{Version: 1, Repository: "octo/demo", ProfileID: "codex-thorough", ProfileRevision: testSHA, RunnerLabel: "hub-agent-codex", RequestedModel: model, RequestedReasoningEffort: effort, EffectiveModel: &model, EffectiveReasoningEffort: &effort, CLIVersion: &version, RunID: 10, RunAttempt: 2, VerifiedAt: now.Add(-time.Minute), Outcome: "verified", ReasonCode: "verified"}
			if scenario == "failed" {
				record.Outcome = "failed"
				record.ReasonCode = "authentication_unavailable"
				record.EffectiveModel = nil
				record.EffectiveReasoningEffort = nil
			}
			encoded, _ := json.Marshal(record)
			if scenario == "unsafe evidence" {
				encoded = bytes.Replace(encoded, []byte(`"reason_code":"verified"`), []byte(`"reason_code":"private-sentinel"`), 1)
			}
			name := "readiness.json"
			if scenario == "bad archive" {
				name = "../readiness.json"
			}
			archive := testArchive(t, name, encoded)
			storageCalls := 0
			storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				storageCalls++
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("credentials forwarded to storage")
				}
				if scenario == "storage redirect" {
					http.Redirect(w, r, "https://unexpected.invalid", 302)
					return
				}
				_, _ = w.Write(archive)
			}))
			defer storage.Close()
			hash := sha256.Sum256(archive)
			digest := "sha256:" + hex.EncodeToString(hash[:])
			if scenario == "bad digest" {
				digest = "sha256:" + strings.Repeat("f", 64)
			}
			runRepo := map[string]any{"id": 1, "full_name": "octo/demo", "fork": false}
			headRepo := map[string]any{"id": 1}
			if scenario == "fork" {
				headRepo["id"] = 2
			}
			attempt := 2
			if scenario == "old attempt" {
				attempt = 1
			}
			workflowID := 20
			if scenario == "wrong workflow" {
				workflowID = 21
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer synthetic-test-token" {
					t.Error("missing server-only GitHub credential")
				}
				w.Header().Set("Content-Type", "application/json")
				var value any
				switch {
				case strings.Contains(r.URL.Path, "/contents/"):
					if r.URL.Query().Get("ref") != testSHA {
						t.Error("mutable configuration ref")
					}
					if scenario == "missing file" && strings.HasSuffix(r.URL.Path, "docs/agent-workflows.md") {
						http.NotFound(w, r)
						return
					}
					content := "Instructions"
					if strings.HasSuffix(r.URL.Path, "agent-assignment.yml") {
						content = assignmentYAML
					}
					if strings.HasSuffix(r.URL.Path, "agent-profile-diagnostic.yml") {
						content = diagnosticYAML
					}
					value = map[string]any{"type": "file", "size": len(content), "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(content))}
				case strings.HasSuffix(r.URL.Path, "/agent-assignment.yml"), strings.HasSuffix(r.URL.Path, "/agent-profile-diagnostic.yml"):
					state := "active"
					if scenario == "inactive workflow" {
						state = "disabled_manually"
					}
					value = map[string]any{"id": 20, "state": state, "path": ".github/workflows/" + r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]}
				case strings.HasSuffix(r.URL.Path, "/runs"):
					if r.URL.Query().Get("branch") != "main" || r.URL.Query().Get("status") != "" || r.URL.Query().Get("head_sha") != "" || r.URL.Query().Get("per_page") != "1" {
						t.Error("latest run improperly filtered")
					}
					if scenario == "actions denied" {
						w.WriteHeader(403)
						_, _ = w.Write([]byte(`{"message":"private-sentinel"}`))
						return
					}
					if scenario == "actions limited" {
						w.WriteHeader(429)
						_, _ = w.Write([]byte(`{"message":"private-sentinel"}`))
						return
					}
					status := "completed"
					if scenario == "queued" {
						status = "queued"
					}
					value = map[string]any{"workflow_runs": []any{map[string]any{"id": 10, "workflow_id": workflowID, "run_attempt": 2, "repository": runRepo, "head_repository": headRepo, "head_sha": testSHA, "head_branch": "main", "path": domain.DiagnosticWorkflowPath, "event": "workflow_dispatch", "status": status, "run_started_at": now.Add(-2 * time.Minute), "updated_at": now}}}
				case strings.HasSuffix(r.URL.Path, "/attempts/2/jobs"):
					value = map[string]any{"jobs": []any{map[string]any{"name": "authorize", "run_id": 10, "run_attempt": 2, "head_sha": testSHA, "status": "completed", "conclusion": "success"}, map[string]any{"name": "diagnostic", "run_id": 10, "run_attempt": attempt, "head_sha": testSHA, "status": "completed", "conclusion": "success", "runner_id": 7, "runner_name": "dedicated-test-runner", "labels": []string{"self-hosted", "linux", "hub-agent-codex"}, "started_at": now.Add(-2 * time.Minute), "completed_at": now}}}
				case strings.HasSuffix(r.URL.Path, "/runs/10/artifacts"):
					value = map[string]any{"artifacts": []any{map[string]any{"id": 30, "name": "agent-readiness-v1-2", "size_in_bytes": len(archive), "expired": scenario == "expired", "digest": digest, "workflow_run": map[string]any{"id": 10, "repository_id": 1, "head_repository_id": 1, "head_branch": "main", "head_sha": testSHA}}}}
				case strings.HasSuffix(r.URL.Path, "/artifacts/30/zip"):
					w.Header().Set("Location", storage.URL)
					w.WriteHeader(302)
					return
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode(value)
			}))
			defer upstream.Close()
			client := gh.NewClient(upstream.Client()).WithAuthToken("synthetic-test-token")

			client.BaseURL, _ = client.BaseURL.Parse(upstream.URL + "/")
			decoder, err := evidence.NewDecoder()
			if err != nil {
				t.Fatal(err)
			}
			reader := NewReadinessReader(client, decoder)
			reader.download.Transport = storage.Client().Transport

			// Act.
			facts, err := reader.ReadReadiness(context.Background(), domain.Repository{Owner: "octo", Name: "demo"}, domain.ProfileCatalog{DefaultBranch: "main", Revision: testSHA})

			// Assert.
			if scenario == "actions denied" || scenario == "actions limited" || scenario == "storage redirect" {
				want := domain.ErrAccessDenied
				if scenario == "actions limited" {
					want = domain.ErrRateLimited
				}
				if scenario == "storage redirect" {
					want = domain.ErrGitHubUnavailable
				}
				if !errors.Is(err, want) {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "verified" || scenario == "failed" || scenario == "missing file" {
				if facts.Evidence == nil || facts.Evidence.RunAttempt != 2 || facts.Origin == nil || !facts.Origin.Authorized || !facts.Origin.DiagnosticSucceeded {
					t.Fatalf("facts=%+v", facts)
				}
			} else if facts.Evidence != nil {
				t.Fatal("untrusted evidence accepted")
			}
			if scenario == "missing file" && (len(facts.MissingFiles) != 1 || facts.MissingFiles[0] != "docs/agent-workflows.md") {
				t.Fatal("missing path lost")
			}
			if scenario == "inactive workflow" && facts.Diagnostic.Active {
				t.Fatal("disabled workflow accepted")
			}
			if storageCalls > 1 {
				t.Fatal("storage redirects followed")
			}
		})
	}
}

func TestWorkflowMetadataRejectsAmbiguousYAMLAndDynamicRunner(t *testing.T) {
	for _, content := range []string{"on: [", "on: {workflow_dispatch: null}\non: {push: null}", strings.ReplaceAll(diagnosticYAML, "[self-hosted, linux, hub-agent-codex]", "${{ inputs.runner }}"), strings.ReplaceAll(diagnosticYAML, "workflow_dispatch:", "workflow_dispatch:\n  pull_request:")} {
		facts := workflowFacts([]byte(content), true)

		if facts.TriggerPresent && facts.JobsPresent && len(facts.RunnerLabels) == 3 {
			t.Fatalf("unsafe workflow metadata accepted: %+v", facts)
		}
	}
	// A cancel must pass through the go-github boundary without unsafe error text.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{}`) }))
	defer server.Close()
	decoder, _ := evidence.NewDecoder()
	client := gh.NewClient(server.Client())

	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewReadinessReader(client, decoder).ReadReadiness(ctx, domain.Repository{}, domain.ProfileCatalog{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}

func TestArtifactArchiveLimitsAndFileTypes(t *testing.T) {
	decoder, err := evidence.NewDecoder()
	if err != nil {
		t.Fatal(err)
	}
	reader := NewReadinessReader(nil, decoder)
	for _, scenario := range []string{"regular", "empty", "multiple files", "symlink", "directory", "oversized"} {
		t.Run(scenario, func(t *testing.T) {
			// Arrange.
			var buffer bytes.Buffer
			writer := zip.NewWriter(&buffer)
			if scenario != "empty" {
				header := &zip.FileHeader{Name: "readiness.json"}
				if scenario == "symlink" {
					header.SetMode(0777 | os.ModeSymlink)
				}
				if scenario == "directory" {
					header.Name = "readiness.json/"
				}
				file, err := writer.CreateHeader(header)
				if err != nil {
					t.Fatal(err)
				}
				content := fmt.Sprintf(`{"version":1,"repository":"octo/demo","profile_id":"codex-thorough","profile_revision":"%s","runner_label":"hub-agent-codex","requested_model":"gpt-6.1-sol","requested_reasoning_effort":"high","effective_model":null,"effective_reasoning_effort":null,"cli_version":null,"run_id":10,"run_attempt":2,"verified_at":"2026-10-01T12:00:00Z","outcome":"failed","reason_code":"request_failed"}`, testSHA)
				if scenario == "oversized" {
					content += strings.Repeat(" ", evidence.MaxBytes+1)
				}
				if scenario != "directory" {
					if _, err := file.Write([]byte(content)); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "multiple files" {
					if _, err := writer.Create("other.json"); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}

			// Act.
			record := reader.decodeArchive(buffer.Bytes())

			// Assert.
			if (record != nil) != (scenario == "regular") {
				t.Fatal("ZIP file types/limits not honored")
			}
		})
	}
}
