package github

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func TestExecutionReadinessRequiresLatestOriginAndMatchingJob(t *testing.T) {
	for _, variant := range []string{"verified", "latest-pending", "wrong-workflow", "fork", "wrong-revision", "failed-job", "wrong-label", "wrong-attempt", "outside-job-time", "bad-digest"} {
		t.Run(variant, func(t *testing.T) {
			_, client, _, _ := setup(t)
			input := domain.Invocation{Repository: domain.Repository{ID: 42, Owner: "octo", Name: "demo", DefaultBranch: "main"}, PolicyRevision: strings.Repeat("2", 40)}
			now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
			verified := now
			if variant == "outside-job-time" {
				verified = now.Add(time.Hour)
			}
			record, _ := json.Marshal(map[string]any{"run_id": 90, "run_attempt": 2, "verified_at": verified})
			archive := executionArchive(t, map[string][]byte{"readiness.json": record})
			storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("token leaked to readiness storage")
				}
				_, _ = w.Write(archive)
			}))
			t.Cleanup(storage.Close)
			client.download = storage.Client()
			digest := sha256.Sum256(archive)
			digestValue := "sha256:" + hex.EncodeToString(digest[:])
			if variant == "bad-digest" {
				digestValue = "sha256:" + strings.Repeat("0", 64)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				respond := func(value any) { _ = json.NewEncoder(w).Encode(value) }
				switch r.URL.Path {
				case "/repos/octo/demo/actions/workflows/agent-profile-diagnostic.yml":
					respond(map[string]any{"id": 5, "path": ".github/workflows/agent-profile-diagnostic.yml", "state": "active"})
				case "/repos/octo/demo/actions/workflows/agent-profile-diagnostic.yml/runs":
					run := map[string]any{"id": 90, "workflow_id": 5, "event": "workflow_dispatch", "path": ".github/workflows/agent-profile-diagnostic.yml", "repository": map[string]any{"id": 42, "private": true, "fork": variant == "fork"}, "head_repository": map[string]any{"id": 42}, "head_sha": input.PolicyRevision, "head_branch": "main", "status": "completed", "run_attempt": 2}
					if variant == "latest-pending" {
						run["status"] = "queued"
					}
					if variant == "wrong-workflow" {
						run["workflow_id"] = 6
					}
					if variant == "wrong-revision" {
						run["head_sha"] = strings.Repeat("3", 40)
					}
					if r.URL.Query().Get("status") != "" {
						t.Error("reader filtered away the newest failed/pending run")
					}
					respond(map[string]any{"workflow_runs": []any{run}})
				case "/repos/octo/demo/actions/runs/90/attempts/2/jobs":
					jobs := []any{}
					for _, name := range []string{"authorize", "diagnostic"} {
						job := map[string]any{"name": name, "run_id": 90, "run_attempt": 2, "head_sha": input.PolicyRevision, "status": "completed", "conclusion": "success", "runner_id": 30, "runner_name": "approved", "labels": []string{"self-hosted", "linux", "hub-agent-codex"}, "started_at": now.Add(-time.Minute), "completed_at": now.Add(time.Minute)}
						if name == "diagnostic" {
							switch variant {
							case "failed-job":
								job["conclusion"] = "failure"
							case "wrong-label":
								job["labels"] = []string{"self-hosted", "linux", "addons"}
							case "wrong-attempt":
								job["run_attempt"] = 1
							}
						}
						jobs = append(jobs, job)
					}
					respond(map[string]any{"total_count": len(jobs), "jobs": jobs})
				case "/repos/octo/demo/actions/runs/90/artifacts":
					respond(map[string]any{"artifacts": []any{map[string]any{"id": 60, "name": "agent-readiness-v1-2", "size_in_bytes": len(archive), "digest": digestValue, "workflow_run": map[string]any{"id": 90, "repository_id": 42, "head_repository_id": 42, "head_sha": input.PolicyRevision, "head_branch": "main"}}}})
				case "/repos/octo/demo/actions/artifacts/60/zip":
					w.Header().Set("Location", storage.URL)
					w.WriteHeader(302)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			t.Cleanup(server.Close)
			client.api.BaseURL, _ = url.Parse(server.URL + "/")

			content, err := client.ReadinessRecord(context.Background(), input)

			if variant == "verified" {
				if err != nil || string(content) != string(record) {
					t.Fatalf("matching readiness origin rejected: %v", err)
				}
			} else if err == nil {
				t.Fatal("untrusted/stale diagnostic origin accepted")
			}
		})
	}
}
