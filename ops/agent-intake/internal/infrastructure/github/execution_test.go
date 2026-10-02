package github

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func executionArchive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = file.Write(content)
	}
	if writer.Close() != nil {
		t.Fatal("archive unavailable")
	}
	return buffer.Bytes()
}

func TestExecutionInputVerifiesOriginDigestJobsAndLiveAccess(t *testing.T) {
	for _, variant := range []string{"valid", "digest", "branch", "default-branch-changed", "repository-id", "repository-unavailable", "job", "attempt", "unknown-file", "role", "public", "edited", "redirect", "storage-auth"} {
		t.Run(variant, func(t *testing.T) {
			service, client, fixture, event := setup(t)
			fixture.run(100)
			decision, err := service.Intake(context.Background(), event)
			if err != nil {
				t.Fatal(err)
			}
			content, _ := json.Marshal(decision.Invocation)
			files := map[string][]byte{"invocation.json": content}
			if variant == "unknown-file" {
				files["../escape"] = []byte("synthetic-sentinel")
			}
			archive := executionArchive(t, files)
			storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("GitHub token leaked to artifact storage")
				}
				if variant == "redirect" {
					w.Header().Set("Location", "https://example.invalid")
					w.WriteHeader(302)
					return
				}
				_, _ = w.Write(archive)
			}))
			t.Cleanup(storage.Close)
			client.download = storage.Client()
			digest := sha256.Sum256(archive)
			digestValue := "sha256:" + hex.EncodeToString(digest[:])
			if variant == "digest" {
				digestValue = "sha256:" + strings.Repeat("f", 64)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer synthetic-workflow-token" {
					t.Error("missing GitHub boundary token")
				}
				respond := func(value any) { _ = json.NewEncoder(w).Encode(value) }
				switch r.URL.Path {
				case "/repos/octo/demo":
					if variant == "repository-unavailable" {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					repo := map[string]any{"id": 42, "owner": map[string]any{"login": "octo"}, "name": "demo", "default_branch": "main", "private": variant != "public"}
					if variant == "default-branch-changed" {
						repo["default_branch"] = "replacement"
					}
					if variant == "repository-id" {
						repo["id"] = 43
					}
					respond(repo)
				case "/repos/octo/demo/actions/runs/100":
					run := fixture.run(100)
					// Real Actions responses omit default_branch in this reduced object.
					run["repository"] = map[string]any{"id": 42, "private": true}
					run["head_repository"] = map[string]any{"id": 42}
					run["head_branch"] = "main"
					if variant == "branch" {
						run["head_branch"] = "untrusted"
					}
					if variant == "attempt" {
						run["run_attempt"] = 2
					}
					respond(run)
				case "/repos/octo/demo/actions/runs/100/attempts/1/jobs":
					conclusion := "success"
					if variant == "job" {
						conclusion = "failure"
					}
					respond(map[string]any{"total_count": 1, "jobs": []any{map[string]any{"name": "authorize", "run_id": 100, "run_attempt": 1, "head_sha": fixture.sha, "status": "completed", "conclusion": conclusion}}})
				case "/repos/octo/demo/actions/runs/100/artifacts":
					respond(map[string]any{"artifacts": []any{map[string]any{"id": 50, "name": "agent-invocation-v1-1", "size_in_bytes": len(archive), "digest": digestValue, "workflow_run": map[string]any{"id": 100, "repository_id": 42, "head_repository_id": 42, "head_sha": fixture.sha, "head_branch": "main"}}}})
				case "/repos/octo/demo/actions/artifacts/50/zip":
					w.Header().Set("Location", storage.URL)
					w.WriteHeader(302)
				default:
					fixture.handler(w, r)
				}
			}))
			t.Cleanup(server.Close)
			client.api.BaseURL, _ = url.Parse(server.URL + "/")
			if variant == "role" {
				fixture.role = "read"
			}
			if variant == "edited" {
				fixture.variant = "edited"
			}

			input, err := client.ExecutionInput(context.Background(), event)
			if err == nil {
				_, err = service.Reauthorize(context.Background(), event, input)
			}

			if variant == "valid" || variant == "storage-auth" {
				if err != nil {
					t.Fatalf("verified execution input rejected: %v", err)
				}
			} else if err == nil {
				t.Fatal("unverified origin, artifact, or access accepted")
			}
			if fixture.creates != 1 || fixture.edits != 0 {
				t.Fatal("execution verification wrote a receipt")
			}
		})
	}
}

func TestPriorExecutionNeverBlindlyRetriesAnAttemptThatReachedRunner(t *testing.T) {
	for _, variant := range []string{"skipped", "legacy", "failed", "cancelled", "running", "missing-artifact", "truncated-jobs", "ambiguous-job"} {
		t.Run(variant, func(t *testing.T) {
			_, client, fixture, event := setup(t)
			input := domain.Invocation{Repository: domain.Repository{ID: 42, Owner: "octo", Name: "demo", DefaultBranch: "main"}, RunID: event.RunID, RunAttempt: 2, RequestCommentID: 99}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				respond := func(value any) { _ = json.NewEncoder(w).Encode(value) }
				switch r.URL.Path {
				case "/repos/octo/demo/actions/runs/100/attempts/1":
					respond(fixture.run(100))
				case "/repos/octo/demo/actions/runs/100/attempts/1/jobs":
					job := map[string]any{"name": "patch", "run_id": 100, "run_attempt": 1, "head_sha": fixture.sha, "status": "completed", "conclusion": "success", "runner_id": 30, "labels": []string{"hub-agent-codex"}}
					if variant == "skipped" {
						job["conclusion"], job["runner_id"] = "skipped", 0
					}
					if variant == "failed" || variant == "cancelled" {
						job["conclusion"] = variant
					}
					if variant == "running" {
						job["status"] = "in_progress"
					}
					jobs := []any{job, map[string]any{"name": "authorize", "run_id": 100, "run_attempt": 1, "head_sha": fixture.sha, "status": "completed", "conclusion": "success"}}
					if variant == "legacy" {
						job["name"] = "Execution disabled (AW-012 required)"
						job["labels"] = []string{"ubuntu-24.04"}
					}
					if variant == "ambiguous-job" {
						jobs = append(jobs, job)
					}
					total := len(jobs)
					if variant == "truncated-jobs" {
						total++
					}
					respond(map[string]any{"total_count": total, "jobs": jobs})
				case "/repos/octo/demo/actions/runs/100/artifacts":
					respond(map[string]any{"artifacts": []any{}})
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			t.Cleanup(server.Close)
			client.api.BaseURL, _ = url.Parse(server.URL + "/")

			_, _, err := client.PriorExecution(context.Background(), input)

			if variant == "skipped" || variant == "legacy" {
				if err != nil {
					t.Fatalf("safe hosted-only history could not resume: %v", err)
				}
			} else if err == nil {
				t.Fatal("ambiguous prior execution authorized a fresh workload")
			}
		})
	}
}

func TestExecutionArchiveRejectsUnsafeShapes(t *testing.T) {
	for _, files := range []map[string][]byte{
		{"../result.json": []byte("synthetic")},
		{"result.json": bytes.Repeat([]byte("x"), maxExecutionArchive+1)},
	} {
		archive := executionArchive(t, files)

		_, err := decodeExecutionArchive(archive)

		if err == nil {
			t.Fatal("unsafe archive accepted")
		}
	}
	// A malformed archive must not surface source/transport errors.
	_, err := decodeExecutionArchive([]byte("synthetic-secret"))
	if err == nil || strings.Contains(fmt.Sprint(err), "synthetic-secret") {
		t.Fatal("unsafe archive error exposed source content")
	}
}
