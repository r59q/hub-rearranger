package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

type bootstrapFixture struct {
	mu                                                       sync.Mutex
	proposal                                                 domain.BootstrapProposal
	commit                                                   *gh.Commit
	ref                                                      *gh.Reference
	pr                                                       *gh.PullRequest
	posts                                                    map[string]int
	loseRef, losePR, failPR, extraFile, stale, closed, moved bool
}

func newBootstrapFixture(t *testing.T) (BootstrapPublisher, *bootstrapFixture) {
	t.Helper()
	f := &bootstrapFixture{proposal: domain.BootstrapProposal{Repository: "octo/demo", DefaultBranch: "main", BaseRevision: strings.Repeat("a", 40), Digest: strings.Repeat("d", 64), Changes: []domain.BootstrapChange{{Path: "docs/agent-workflows.md", Content: "reviewed setup\n"}}}, posts: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer synthetic-user-token" {
			t.Error("write/read used discovery authority")
		}
		w.Header().Set("Content-Type", "application/json")
		path := strings.TrimPrefix(r.URL.Path, "/repos/octo/demo")
		if r.Method == "POST" {
			f.posts[path]++
		}
		var value any
		switch {
		case path == "":
			value = map[string]any{"id": 42, "full_name": "octo/demo", "default_branch": "main"}
		case path == "/branches/main":
			sha := f.proposal.BaseRevision
			if f.stale {
				sha = strings.Repeat("b", 40)
			}
			value = map[string]any{"commit": map[string]string{"sha": sha}}
		case path == "/git/commits/"+f.proposal.BaseRevision:
			value = &gh.Commit{SHA: gh.Ptr(f.proposal.BaseRevision), Tree: &gh.Tree{SHA: gh.Ptr(strings.Repeat("b", 40))}}
		case path == "/git/trees/"+strings.Repeat("b", 40):
			value = fixtureTree(f, false)
		case path == "/git/trees/"+strings.Repeat("c", 40):
			value = fixtureTree(f, true)
		case path == "/git/trees" && r.Method == "POST":
			var tree struct {
				BaseTree string          `json:"base_tree"`
				Entries  []*gh.TreeEntry `json:"tree"`
			}
			if json.NewDecoder(r.Body).Decode(&tree) != nil || tree.BaseTree != strings.Repeat("b", 40) || len(tree.Entries) != 1 || tree.Entries[0].GetPath() != f.proposal.Changes[0].Path || tree.Entries[0].GetContent() != f.proposal.Changes[0].Content {
				t.Error("unreviewed tree write")
			}
			value = &gh.Tree{SHA: gh.Ptr(strings.Repeat("c", 40))}
		case path == "/git/commits" && r.Method == "POST":
			var body struct {
				Message, Tree     string
				Parents           []string
				Author, Committer *gh.CommitAuthor
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil || len(body.Parents) != 1 {
				t.Error("invalid commit")
				return
			}
			f.commit = &gh.Commit{SHA: gh.Ptr(strings.Repeat("e", 40)), Message: gh.Ptr(body.Message), Tree: &gh.Tree{SHA: gh.Ptr(body.Tree)}, Parents: []*gh.Commit{{SHA: gh.Ptr(body.Parents[0])}}, Author: body.Author, Committer: body.Committer}
			value = f.commit
		case path == "/git/commits/"+strings.Repeat("e", 40):
			value = f.commit
		case path == "/git/refs" && r.Method == "POST":
			if f.ref != nil {
				w.WriteHeader(422)
				return
			}
			var body struct{ Ref, SHA string }
			if json.NewDecoder(r.Body).Decode(&body) != nil || body.SHA != f.commit.GetSHA() {
				t.Error("wrong branch commit")
			}
			f.ref = &gh.Reference{Ref: gh.Ptr(body.Ref), Object: &gh.GitObject{Type: gh.Ptr("commit"), SHA: gh.Ptr(body.SHA)}}
			if f.loseRef {
				w.WriteHeader(502)
				return
			}
			value = f.ref
		case strings.HasPrefix(path, "/git/ref/heads/"):
			if f.ref == nil {
				http.NotFound(w, r)
				return
			}
			value = f.ref
		case path == "/pulls" && r.Method == "GET":
			if r.URL.Query().Get("state") != "all" || r.URL.Query().Get("head") != "octo:"+strings.TrimPrefix(f.ref.GetRef(), "refs/heads/") {
				t.Error("unscoped PR reconciliation")
			}
			if f.pr == nil {
				value = []*gh.PullRequest{}
			} else {
				value = []*gh.PullRequest{f.pr}
			}
		case path == "/pulls" && r.Method == "POST":
			if f.pr != nil {
				w.WriteHeader(422)
				return
			}
			if f.failPR {
				w.WriteHeader(502)
				return
			}
			var body gh.NewPullRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil || !body.GetDraft() || body.GetMaintainerCanModify() {
				t.Error("unsafe PR write")
			}
			repo := &gh.Repository{FullName: gh.Ptr("octo/demo")}
			f.pr = &gh.PullRequest{Number: gh.Ptr(3), HTMLURL: gh.Ptr("https://github.com/octo/demo/pull/3"), State: gh.Ptr("open"), Draft: body.Draft, Title: body.Title, Body: body.Body, User: &gh.User{ID: gh.Ptr(int64(7))}, Head: &gh.PullRequestBranch{Ref: body.Head, SHA: f.commit.SHA, Repo: repo}, Base: &gh.PullRequestBranch{Ref: body.Base, Repo: repo}}
			if f.moved {
				f.ref.Object.SHA = gh.Ptr(strings.Repeat("f", 40))
			}
			if f.closed {
				f.pr.State = gh.Ptr("closed")
			}
			if f.losePR {
				w.WriteHeader(502)
				return
			}
			value = f.pr
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(value)
	}))
	t.Cleanup(server.Close)
	client := gh.NewClient(server.Client())
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")
	return BootstrapPublisher{API: client, Origin: "https://hub.example.com"}, f
}

func fixtureTree(f *bootstrapFixture, after bool) *gh.Tree {
	entries := []*gh.TreeEntry{{Path: gh.Ptr("README.md"), Mode: gh.Ptr("100644"), Type: gh.Ptr("blob"), SHA: gh.Ptr(strings.Repeat("f", 40))}}
	if after {
		change := f.proposal.Changes[0]
		entry := bootstrapEntryForContent(change.Content)
		entries = append(entries, &gh.TreeEntry{Path: gh.Ptr(change.Path), Mode: gh.Ptr("100644"), Type: gh.Ptr("blob"), SHA: gh.Ptr(entry)})
		if f.extraFile {
			entries = append(entries, &gh.TreeEntry{Path: gh.Ptr("unreviewed.txt"), Mode: gh.Ptr("100644"), Type: gh.Ptr("blob"), SHA: gh.Ptr(strings.Repeat("f", 40))})
		}
	}
	return &gh.Tree{Entries: entries}
}

func publishFixture(p BootstrapPublisher, f *bootstrapFixture, guard func(context.Context) error) (domain.BootstrapResult, error) {
	return p.Publish(context.Background(), "synthetic-user-token", domain.Repository{Owner: "octo", Name: "demo"}, domain.User{ID: 7, Login: "octocat"}, f.proposal, guard)
}

func TestBootstrapPublicationReconcilesExactGitHubState(t *testing.T) {
	for _, variant := range []string{"success", "lost-ref", "lost-pr", "partial-pr"} {
		t.Run(variant, func(t *testing.T) {
			publisher, f := newBootstrapFixture(t)
			f.loseRef, f.losePR, f.failPR = variant == "lost-ref", variant == "lost-pr", variant == "partial-pr"
			guard := func(context.Context) error { return nil }
			result, err := publishFixture(publisher, f, guard)
			if variant == "partial-pr" {
				if !errors.Is(err, domain.ErrBootstrapIncomplete) || f.ref == nil {
					t.Fatal("partial publication was claimed complete")
				}
				f.failPR = false
				result, err = publishFixture(publisher, f, guard)
			}
			if err != nil || result.PullRequestNumber != 3 {
				t.Fatalf("result %+v, error %v", result, err)
			}
			if !strings.Contains(f.pr.GetBody(), "https://hub.example.com/agents/bootstrap?repository=octo%2Fdemo") || !strings.Contains(f.pr.GetBody(), "- [ ] Provision a dedicated") {
				t.Fatal("setup/checklist missing")
			}
			for range 3 {
				if _, err := publishFixture(publisher, f, guard); err != nil {
					t.Fatal(err)
				}
			}
			if f.posts["/git/refs"] != 1 || f.posts["/git/commits"] != 1 || f.posts["/git/trees"] != 1 || f.posts["/pulls"] != map[bool]int{true: 2, false: 1}[variant == "partial-pr"] {
				t.Fatalf("duplicate writes: %+v", f.posts)
			}
		})
	}
}

func TestBootstrapRejectsUnreviewedOrChangedResults(t *testing.T) {
	for _, variant := range []string{"stale", "extra-file", "closed-pr", "moved-ref", "protected-path"} {
		t.Run(variant, func(t *testing.T) {
			publisher, f := newBootstrapFixture(t)
			f.stale, f.extraFile, f.closed, f.moved = variant == "stale", variant == "extra-file", variant == "closed-pr", variant == "moved-ref"
			if variant == "protected-path" {
				f.proposal.Changes[0].Path = ".github/workflows/unreviewed.yml"
			}
			_, err := publishFixture(publisher, f, func(context.Context) error { return nil })
			if err == nil {
				t.Fatal("unsafe result accepted")
			}
			if (variant == "stale" || variant == "protected-path") && len(f.posts) != 0 {
				t.Fatal("wrote before validating review")
			}
			if variant == "extra-file" && f.ref != nil {
				t.Fatal("exposed unreviewed tree")
			}
		})
	}
}

func TestBootstrapReauthorizesEveryWriteAndCompletion(t *testing.T) {
	for blockedPhase := 1; blockedPhase <= 5; blockedPhase++ {
		publisher, f := newBootstrapFixture(t)
		checks := 0
		_, err := publishFixture(publisher, f, func(context.Context) error {
			checks++
			if checks == blockedPhase {
				return domain.ErrForbidden
			}
			return nil
		})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("phase %d: %v", blockedPhase, err)
		}
		if blockedPhase == 1 && len(f.posts) != 0 || blockedPhase <= 3 && f.ref != nil || blockedPhase <= 4 && f.pr != nil {
			t.Fatal("write continued after revocation")
		}
	}
}

func TestConcurrentBootstrapAttemptsKeepOneBranchAndDraftPR(t *testing.T) {
	publisher, f := newBootstrapFixture(t)
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			result, err := publishFixture(publisher, f, func(context.Context) error { return nil })
			if err != nil || result.PullRequestNumber != 3 {
				t.Errorf("concurrent result %v: %v", result, err)
			}
		})
	}
	group.Wait()
	if f.ref == nil || f.pr == nil || f.pr.GetNumber() != 3 {
		t.Fatal("publication did not converge")
	}
}
