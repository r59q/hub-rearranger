package agents

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func TestAssignmentPlannerReadsPublicContractWithoutCredentials(t *testing.T) {
	for _, variant := range []string{"valid", "unknown", "large", "failure"} {
		t.Run(variant, func(t *testing.T) {
			sha := strings.Repeat("a", 40)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/v1/repositories/octo/demo/issues/3/assignment" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("private identity crossed read boundary")
				}
				w.Header().Set("Content-Type", "application/json")
				if variant == "failure" {
					w.WriteHeader(403)
					return
				}
				if variant == "large" {
					_, _ = w.Write([]byte(strings.Repeat("x", 65537)))
					return
				}
				data := map[string]any{"repository": "octo/demo", "repository_id": 42, "number": 3, "title": "Task", "body": "Context", "url": "https://github.com/octo/demo/issues/3", "issue_state": "open", "profile_revision": sha, "assignable": true, "reason": "Ready to review", "command": "/agent assign codex-thorough@" + sha + " authority=branch-draft-pr", "last_comment_id": 10, "requests": []any{}}
				if variant == "unknown" {
					data["token"] = "private-sentinel"
				}
				_ = json.NewEncoder(w).Encode(data)
			}))
			defer server.Close()
			result, err := (Planner{URL: server.URL, Client: server.Client()}).Assignment(context.Background(), domain.Repository{Owner: "octo", Name: "demo"}, 3)
			if variant == "valid" {
				if err != nil || !result.Assignable || result.Revision != sha || result.RepositoryID != 42 {
					t.Fatal(result, err)
				}
			} else if err == nil {
				t.Fatal("invalid contract accepted")
			}
		})
	}
}
