package github

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/infrastructure/policy"
)

type fixture struct {
	mu                         sync.Mutex
	head, role, currentCatalog string
	commentBody                string
	receipts                   []map[string]any
	runs                       []int64
	attempts                   map[int64]int
	creates, edits             int
	variant                    string
	sha                        string
	created                    time.Time
}

func (f *fixture) run(id int64) map[string]any {
	return map[string]any{"id": id, "run_attempt": f.attempts[id], "head_sha": f.sha,
		"path": domain.WorkflowPath, "event": "issue_comment", "display_title": domain.RunTitle(42, 99), "repository": map[string]any{"id": 42},
		"actor": map[string]any{"id": 7, "login": "maintainer"}, "triggering_actor": map[string]any{"id": 7, "login": "maintainer"}, "created_at": f.created.Add(time.Second).Format(time.RFC3339)}
}

func (f *fixture) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("Authorization") != "Bearer synthetic-workflow-token" {
		w.WriteHeader(401)
		return
	}
	respond := func(value any) { _ = json.NewEncoder(w).Encode(value) }
	path := r.URL.Path
	switch {
	case path == "/repos/octo/demo":
		respond(map[string]any{"id": 42, "name": "demo", "full_name": "octo/demo", "owner": map[string]any{"login": "octo"}, "default_branch": "main", "fork": false})
	case path == "/repos/octo/demo/issues/comments/99":
		if f.variant == "deleted" {
			w.WriteHeader(404)
			respond(map[string]any{"message": "private-sentinel"})
			return
		}
		updated := f.created
		if f.variant == "edited" {
			updated = updated.Add(time.Second)
		}
		respond(map[string]any{"id": 99, "body": f.commentBody, "issue_url": "https://api.github.com/repos/octo/demo/issues/3",
			"user": map[string]any{"id": 7, "login": "maintainer", "type": "User"}, "created_at": f.created.Format(time.RFC3339), "updated_at": updated.Format(time.RFC3339)})
	case path == "/repos/octo/demo/issues/3" && r.Method == "GET":
		issue := map[string]any{"number": 3, "state": "open", "url": "https://api.github.com/repos/octo/demo/issues/3"}
		if f.variant == "pr" {
			issue["pull_request"] = map[string]any{"url": "https://api.github.com/repos/octo/demo/pulls/3"}
		}
		respond(issue)
	case strings.HasPrefix(path, "/repos/octo/demo/collaborators/"):
		role := f.role
		if strings.Contains(path, "untrusted") {
			role = "read"
		}
		respond(map[string]any{"permission": "write", "role_name": role, "user": map[string]any{"id": 7, "login": strings.Split(path, "/")[5]}})
	case path == "/repos/octo/demo/git/ref/heads/main":
		respond(map[string]any{"object": map[string]any{"sha": f.head}})
	case strings.HasPrefix(path, "/repos/octo/demo/compare/"):
		base := strings.Split(strings.TrimPrefix(path, "/repos/octo/demo/compare/"), "...")[0]
		status, merge := "ahead", base
		if f.variant == "non-ancestor" {
			status, merge = "diverged", strings.Repeat("9", 40)
		}
		respond(map[string]any{"status": status, "base_commit": map[string]any{"sha": base}, "merge_base_commit": map[string]any{"sha": merge}})
	case path == "/repos/octo/demo/contents/.github/agent-profiles.yml":
		content := f.currentCatalog
		if r.URL.Query().Get("ref") == f.sha {
			content = fixtureCatalog()
		}
		respond(map[string]any{"type": "file", "size": len(content), "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(content))})
	case strings.HasPrefix(path, "/repos/octo/demo/actions/runs/"):
		id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/repos/octo/demo/actions/runs/"), 10, 64)
		run := f.run(id)
		if f.variant == "forged-run" {
			run["path"] = ".github/workflows/unrelated.yml"
		}
		respond(run)
	case path == "/repos/octo/demo/actions/workflows/agent-assignment.yml/runs":
		if !strings.Contains(r.URL.Query().Get("created"), "..") || r.URL.Query().Get("event") != "issue_comment" {
			w.WriteHeader(400)
			return
		}
		var runs []map[string]any
		for _, id := range f.runs {
			runs = append(runs, f.run(id))
		}
		total := len(runs)
		if f.variant == "truncated-history" {
			total = 1001
		}
		if f.variant == "missing-current" {
			runs = nil
		}
		if f.variant == "paginated-history" && r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", fmt.Sprintf(`<%s?page=2>; rel="next"`, "http://"+r.Host+r.URL.Path))
			runs = []map[string]any{f.run(101)}
		} else if f.variant == "paginated-history" {
			runs = []map[string]any{f.run(100)}
		}
		respond(map[string]any{"total_count": total, "workflow_runs": runs})
	case path == "/users/github-actions[bot]":
		respond(map[string]any{"id": 41898282, "type": "Bot"})
	case path == "/repos/octo/demo/issues/3/comments" && r.Method == "GET":
		respond(f.receipts)
	case path == "/repos/octo/demo/issues/3/comments" && r.Method == "POST":
		var comment map[string]any
		_ = json.NewDecoder(r.Body).Decode(&comment)
		comment["id"], comment["user"] = 900, map[string]any{"id": 41898282, "type": "Bot"}
		f.receipts = append(f.receipts, comment)
		f.creates++
		if f.variant == "lost-post-response" {
			w.WriteHeader(503)
			respond(map[string]any{"message": "private-sentinel"})
			return
		}
		respond(comment)
	case path == "/repos/octo/demo/issues/comments/900" && r.Method == "PATCH":
		if f.variant == "failed-edit" {
			w.WriteHeader(503)
			respond(map[string]any{"message": "private-sentinel"})
			return
		}
		var comment map[string]any
		_ = json.NewDecoder(r.Body).Decode(&comment)
		comment["id"], comment["user"] = 900, map[string]any{"id": 41898282, "type": "Bot"}
		f.receipts[0] = comment
		f.edits++
		respond(comment)
	default:
		w.WriteHeader(404)
		respond(map[string]any{"message": "unhandled synthetic boundary"})
	}
}

func fixtureCatalog() string {
	return `schema_version: 1
profiles:
  codex-thorough:
    enabled: true
    name: Codex Thorough
    description: Synthetic fixture
    role: implementation
    adapter: {id: codex-chatgpt-private-runner, contract_version: 1, runner_label: hub-agent-codex}
    model: {id: gpt-6.1-sol, reasoning_effort: high, fallback: none}
    triggers: {assignment: issue_comment.created}
    context: {sources: [issue, repository], images: false}
    authority: {mode: branch-draft-pr, sandbox: workspace-write, network: false}
    validation: {checks: [repository-check], on_failure: draft-with-evidence}
    continuation: {review_comments: false, pipeline: disabled}
`
}

func setup(t *testing.T) (*domain.Service, *Client, *fixture, domain.Event) {
	t.Helper()
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	python := os.Getenv("INTAKE_PYTHON")
	if python == "" {
		python = filepath.Join(root, "ops/private-runner/.venv/bin/python")
	}
	if _, err := os.Stat(python); err != nil {
		t.Fatal("Install pinned Python tools or set INTAKE_PYTHON to an absolute path")
	}
	sha := strings.Repeat("1", 40)
	f := &fixture{head: strings.Repeat("2", 40), sha: sha, role: "maintain", currentCatalog: fixtureCatalog(), runs: []int64{100}, attempts: map[int64]int{100: 1}, created: time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)}
	f.commentBody = "/agent assign codex-thorough@" + sha + " authority=branch-draft-pr"
	server := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(server.Close)
	api := gh.NewClient(server.Client()).WithAuthToken("synthetic-workflow-token")
	api.BaseURL, _ = url.Parse(server.URL + "/")
	client := New(api)
	event := domain.Event{Repository: domain.Repository{ID: 42, Owner: "octo", Name: "demo"}, IssueNumber: 3, CommentID: 99, Body: f.commentBody,
		Requester: domain.User{ID: 7, Login: "maintainer"}, Actor: "maintainer", RerunActor: "maintainer", RunID: 100, Attempt: 1, WorkflowSHA: sha}
	return domain.New(client, policy.Python{Executable: python, Script: filepath.Join(root, "ops/agent-intake/profile_policy.py")}), client, f, event
}
