package agents

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func TestPlannerUsesFreshPublicContractWithoutUserCredentials(t *testing.T) {
	for _, variant := range []string{"valid", "stale", "hash", "extra-field", "conflict", "duplicate", "unknown-status", "error"} {
		t.Run(variant, func(t *testing.T) {
			review := domain.BootstrapReview{BaseRevision: strings.Repeat("a", 40), Digest: strings.Repeat("d", 64)}
			file := map[string]any{"path": "AGENTS.md", "status": "create", "base_sha": nil, "sha256": fmt.Sprintf("%x", sha256.Sum256([]byte("setup"))), "content": "setup"}
			body := map[string]any{"repository": "octo/demo", "default_branch": "main", "base_revision": review.BaseRevision, "digest": review.Digest, "state": "ready", "private": true, "diff": "safe diff", "files": []any{file}, "diagnostics": []any{}}
			switch variant {
			case "stale":
				body["base_revision"] = strings.Repeat("b", 40)
			case "hash":
				file["sha256"] = strings.Repeat("f", 64)
			case "extra-field":
				body["access_token"] = "synthetic-never-live"
			case "conflict":
				body["state"] = "conflict"
			case "duplicate":
				body["files"] = []any{file, file}
			case "unknown-status":
				file["status"] = "delete"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/v1/repositories/octo/demo/bootstrap" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
					t.Error("private identity escaped planner boundary")
				}
				if variant == "error" {
					w.WriteHeader(502)
				}
				_ = json.NewEncoder(w).Encode(body)
			}))
			defer server.Close()
			proposal, err := (Planner{URL: server.URL, Client: server.Client()}).Proposal(context.Background(), domain.Repository{Owner: "octo", Name: "demo"}, review)
			if variant == "valid" {
				if err != nil || len(proposal.Changes) != 1 || proposal.Changes[0].Content != "setup" {
					t.Fatal(proposal, err)
				}
			} else if err == nil {
				t.Fatal("invalid/stale source authorized publication")
			}
			if variant == "stale" && !errors.Is(err, domain.ErrBootstrapStale) {
				t.Fatal("stale recovery lost")
			}
		})
	}
}
