package github

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
	"net/http"
	"strings"
)

func (f *publicationFixture) workflowRequest(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	respond := func(value any) { _ = json.NewEncoder(w).Encode(value) }
	switch {
	case path == "/repos/octo/demo":
		respond(map[string]any{"id": 42, "owner": map[string]any{"login": "octo"}, "name": "demo", "default_branch": "main", "private": true})
	case path == "/users/github-actions[bot]":
		respond(map[string]any{"id": 41898282, "type": "Bot", "login": "github-actions[bot]"})
	case path == "/apps/github-actions":
		respond(map[string]any{"id": 15368, "slug": "github-actions"})
	case path == "/repos/octo/demo/actions/runs/100" || path == "/repos/octo/demo/actions/runs/100/attempts/1" || path == "/repos/octo/demo/actions/runs/100/attempts/2":
		run := f.base.run(100)
		if strings.HasSuffix(path, "/attempts/1") {
			run["run_attempt"] = 1
		}
		run["head_repository"], run["head_branch"] = map[string]any{"id": 42}, "main"
		respond(run)
	case path == "/repos/octo/demo/actions/runs/100/attempts/1/jobs" || path == "/repos/octo/demo/actions/runs/100/attempts/2/jobs":
		jobs := []any{}
		attempt := 1
		if strings.Contains(path, "/attempts/2/") {
			attempt = 2
		}
		for _, name := range []string{"authorize", "dispatch", "patch"} {
			conclusion := "success"
			if f.artifactVariant == "failed-job" && name == "patch" {
				conclusion = "failure"
			}
			runnerID := 2
			if attempt == 2 && name == "patch" {
				conclusion = "skipped"
				runnerID = 0
			}
			jobs = append(jobs, map[string]any{"name": name, "status": "completed", "conclusion": conclusion, "run_id": 100, "run_attempt": attempt, "head_sha": f.base.sha, "runner_id": runnerID, "runner_name": "approved", "labels": []string{"self-hosted", "linux", "hub-agent-codex"}})
		}
		respond(map[string]any{"total_count": 3, "jobs": jobs})
	case path == "/repos/octo/demo/actions/runs/100/artifacts":
		artifacts := []any{}
		for id, archive := range f.archives {
			name := "agent-invocation-v1-1"
			if id == 51 {
				name = "agent-patch-v1-1"
			}
			if id == 52 {
				name = "agent-invocation-v1-2"
			}
			digest := sha256.Sum256(archive)
			sha := "sha256:" + hex.EncodeToString(digest[:])
			repoID := 42
			if f.artifactVariant == "digest" && id == 51 {
				sha = "sha256:" + strings.Repeat("9", 64)
			}
			if f.artifactVariant == "origin" && id == 51 {
				repoID = 43
			}
			artifacts = append(artifacts, map[string]any{"id": id, "name": name, "size_in_bytes": len(archive), "digest": sha, "expired": f.artifactVariant == "expired" && id == 51, "workflow_run": map[string]any{"id": 100, "repository_id": repoID, "head_repository_id": 42, "head_branch": "main", "head_sha": f.base.sha}})
		}
		respond(map[string]any{"artifacts": artifacts})
	case path == "/repos/octo/demo/actions/artifacts/50/zip" || path == "/repos/octo/demo/actions/artifacts/51/zip" || path == "/repos/octo/demo/actions/artifacts/52/zip":
		id := strings.Split(path, "/")[6]
		w.Header().Set("Location", f.storageURL+"/"+id)
		w.WriteHeader(302)
	case path == "/repos/octo/demo/tarball/"+f.base.head:
		w.Header().Set("Location", f.storageURL+"/source")
		w.WriteHeader(302)
	default:
		return false
	}
	return true
}

func (f *publicationFixture) gitRequest(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	respond := func(value any) { _ = json.NewEncoder(w).Encode(value) }
	switch {
	case path == "/repos/octo/demo/git/commits/"+f.base.head:
		respond(&gh.Commit{SHA: gh.Ptr(f.base.head), Tree: &gh.Tree{SHA: gh.Ptr(strings.Repeat("3", 40))}})
	case path == "/repos/octo/demo/git/blobs" && r.Method == "POST":
		respond(&gh.Blob{SHA: gh.Ptr(strings.Repeat("5", 40))})
	case path == "/repos/octo/demo/git/trees" && r.Method == "POST":
		var request map[string]any
		_ = json.NewDecoder(r.Body).Decode(&request)
		if request["base_tree"] != strings.Repeat("3", 40) {
			f.t.Error("untouched base tree lost")
		}
		respond(&gh.Tree{SHA: gh.Ptr(strings.Repeat("6", 40))})
	case path == "/repos/octo/demo/git/commits" && r.Method == "POST":
		f.posts["objects"]++
		var request struct {
			Author    *gh.CommitAuthor `json:"author"`
			Committer *gh.CommitAuthor `json:"committer"`
			Message   string           `json:"message"`
			Tree      string           `json:"tree"`
			Parents   []string         `json:"parents"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			f.t.Error("invalid Git commit wire shape")
		}
		parents := []*gh.Commit{}
		for _, sha := range request.Parents {
			parents = append(parents, &gh.Commit{SHA: gh.Ptr(sha)})
		}
		// GitHub returns commit messages without their final newline.
		f.commit = &gh.Commit{SHA: gh.Ptr(strings.Repeat("4", 40)), Author: request.Author, Committer: request.Committer, Message: gh.Ptr(strings.TrimRight(request.Message, "\n")), Tree: &gh.Tree{SHA: gh.Ptr(request.Tree)}, Parents: parents}
		respond(f.commit)
	case path == "/repos/octo/demo/git/commits/"+strings.Repeat("4", 40):
		stored := *f.commit
		if f.artifactVariant == "commit-message" {
			stored.Message = gh.Ptr(f.commit.GetMessage() + " altered")
		}
		respond(&stored)
	case path == "/repos/octo/demo/git/ref/heads/"+domain.PublicationBranch(f.input):
		if f.ref == nil {
			w.WriteHeader(404)
		} else {
			respond(f.ref)
		}
	case path == "/repos/octo/demo/git/refs" && r.Method == "POST":
		if f.ref != nil {
			w.WriteHeader(422)
			break
		}
		var request struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			f.t.Error("invalid Git ref wire shape")
		}
		f.ref = &gh.Reference{Ref: gh.Ptr(request.Ref), Object: &gh.GitObject{SHA: gh.Ptr(request.SHA), Type: gh.Ptr("commit")}}
		f.replyPost(w, "branch", f.ref)
	default:
		return false
	}
	return true
}

func (f *publicationFixture) objectsRequest(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	respond := func(value any) { _ = json.NewEncoder(w).Encode(value) }
	switch {
	case path == "/repos/octo/demo/pulls" && r.Method == "GET":
		if r.URL.Query().Get("state") != "all" || r.URL.Query().Get("head") != "octo:"+domain.PublicationBranch(f.input) {
			f.t.Error("PR reconciliation filter missing")
		}
		pulls := []*gh.PullRequest{}
		if f.pull != nil {
			pulls = append(pulls, f.pull)
		}
		respond(pulls)
	case path == "/repos/octo/demo/pulls" && r.Method == "POST":
		if f.pull != nil {
			w.WriteHeader(422)
			break
		}
		var request gh.NewPullRequest
		_ = json.NewDecoder(r.Body).Decode(&request)
		f.pull = &gh.PullRequest{Number: gh.Ptr(12), State: gh.Ptr("open"), Draft: request.Draft, MaintainerCanModify: request.MaintainerCanModify, Title: request.Title, Body: request.Body, HTMLURL: gh.Ptr("https://github.com/octo/demo/pull/12"), User: &gh.User{ID: gh.Ptr(int64(41898282)), Type: gh.Ptr("Bot")}, Head: &gh.PullRequestBranch{Ref: request.Head, SHA: f.ref.Object.SHA, Repo: &gh.Repository{ID: gh.Ptr(int64(42))}}, Base: &gh.PullRequestBranch{Ref: request.Base, SHA: gh.Ptr(f.input.BaseSHA), Repo: &gh.Repository{ID: gh.Ptr(int64(42))}}}
		f.replyPost(w, "pr", f.pull)
	case path == "/repos/octo/demo/pulls/12" && r.Method == "GET":
		respond(f.pull)
	case path == "/repos/octo/demo/issues/comments/900" && r.Method == "GET":
		respond(f.base.receipts[0])
	case path == "/repos/octo/demo/issues/3/comments" && r.Method == "POST" && f.input.AssignmentID != "":
		var comment map[string]any
		_ = json.NewDecoder(r.Body).Decode(&comment)
		comment["id"], comment["user"] = 901, map[string]any{"id": 41898282, "type": "Bot"}
		f.base.receipts = append(f.base.receipts, comment)
		f.replyPost(w, "comment", comment)
	case path == "/repos/octo/demo/commits/"+strings.Repeat("4", 40)+"/check-runs":
		respond(map[string]any{"total_count": len(f.checks), "check_runs": f.checks})
	case path == "/repos/octo/demo/check-runs" && r.Method == "POST":
		var options gh.CreateCheckRunOptions
		_ = json.NewDecoder(r.Body).Decode(&options)
		check := &gh.CheckRun{ID: gh.Ptr(int64(20)), Name: gh.Ptr(options.Name), HeadSHA: gh.Ptr(options.HeadSHA), ExternalID: options.ExternalID, DetailsURL: options.DetailsURL, Status: options.Status, Conclusion: options.Conclusion, Output: options.Output, App: &gh.App{ID: gh.Ptr(int64(15368))}}
		f.checks = append(f.checks, check)
		f.replyPost(w, "check", check)
	default:
		return false
	}
	return true
}
