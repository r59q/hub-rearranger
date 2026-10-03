package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func TestAssignmentWritesOnlyExactUserCommentAndReconcilesLostResponse(t *testing.T) {
	for _, variant := range []string{"success", "lost", "uncertain", "existing", "other-author", "edited", "closed", "pull", "stale", "comments-changed", "revoked", "incomplete"} {
		t.Run(variant, func(t *testing.T) {
			repo := domain.Repository{Owner: "octo", Name: "demo"}
			user := domain.User{ID: 7, Login: "octocat"}
			sha := strings.Repeat("a", 40)
			plan := domain.AssignmentPlan{Repository: "octo/demo", RepositoryID: 42, Number: 3, Revision: sha, Command: "/agent assign codex-thorough@" + sha + " authority=branch-draft-pr", LastCommentID: 10}
			now := time.Now().UTC().Truncate(time.Second)
			comment := map[string]any{"id": 11, "body": plan.Command, "html_url": "https://github.com/octo/demo/issues/3#issuecomment-11", "created_at": now, "updated_at": now, "user": map[string]any{"id": 7, "login": "octocat", "type": "User"}}
			posted := variant == "existing" || variant == "other-author" || variant == "edited"
			if variant == "other-author" {
				comment["user"] = map[string]any{"id": 8, "login": "github-actions[bot]", "type": "Bot"}
			}
			if variant == "edited" {
				comment["updated_at"] = now.Add(time.Second)
			}
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer ghu_synthetic-never-live" {
					t.Error("wrong credential")
				}
				w.Header().Set("Content-Type", "application/json")
				var value any
				switch r.URL.Path {
				case "/repos/octo/demo":
					value = map[string]any{"id": 42, "full_name": "octo/demo", "default_branch": "main"}
				case "/repos/octo/demo/branches/main":
					head := sha
					if variant == "stale" {
						head = strings.Repeat("b", 40)
					}
					value = map[string]any{"commit": map[string]any{"sha": head}}
				case "/repos/octo/demo/issues/3":
					value = map[string]any{"number": 3, "state": "open", "html_url": "https://github.com/octo/demo/issues/3"}
					if variant == "closed" {
						value.(map[string]any)["state"] = "closed"
					}
					if variant == "pull" {
						value.(map[string]any)["pull_request"] = map[string]any{"url": "pull"}
					}
				case "/repos/octo/demo/issues/3/comments":
					if r.Method == "POST" {
						posts++
						var payload map[string]any
						_ = json.NewDecoder(r.Body).Decode(&payload)
						if len(payload) != 1 || payload["body"] != plan.Command {
							t.Error("noncanonical comment")
						}
						if variant == "lost" || variant == "uncertain" {
							posted = variant == "lost"
							w.WriteHeader(502)
							return
						}
						posted = true
						value = comment
					} else {
						values := []any{map[string]any{"id": 10, "body": "context"}}
						if posted {
							values = append(values, comment)
						}
						if variant == "comments-changed" {
							values = append(values, map[string]any{"id": 12, "body": "changed"})
						}
						if variant == "incomplete" {
							w.Header().Set("Link", fmt.Sprintf("<http://example.invalid/?page=%d>; rel=\"next\"", 21))
						}
						value = values
					}
				default:
					t.Errorf("unexpected endpoint %s", r.URL.Path)
					w.WriteHeader(500)
					return
				}
				_ = json.NewEncoder(w).Encode(value)
			}))
			defer server.Close()
			client := gh.NewClient(server.Client())
			client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")
			publisher := AssignmentPublisher{API: client}
			guard := func(context.Context) error {
				if variant == "revoked" {
					return domain.ErrReconnect
				}
				return nil
			}
			result, err := publisher.Assign(context.Background(), "ghu_synthetic-never-live", repo, user, plan, guard)
			success := variant == "success" || variant == "lost" || variant == "existing"
			if success {
				if err != nil || result.CommentID != 11 {
					t.Fatal(result, err)
				}
			} else if err == nil {
				t.Fatal("unsafe write succeeded")
			}
			expectedPosts := 0
			if variant == "success" || variant == "lost" || variant == "uncertain" {
				expectedPosts = 1
			}
			if posts != expectedPosts {
				t.Fatalf("posts=%d want=%d", posts, expectedPosts)
			}
			if variant == "uncertain" && !errors.Is(err, domain.ErrAssignmentUncertain) {
				t.Fatal(err)
			}
		})
	}
}
