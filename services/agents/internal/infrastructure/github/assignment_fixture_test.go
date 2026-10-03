package github

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/evidence"
)

type assignmentFixture struct {
	reader                                      *AssignmentReader
	issue, comment, run, pr, ref, commit, check map[string]any
	result, invocation                          map[string]any
	artifactOrigin                              map[string]any
	jobs                                        []map[string]any
	patch, summary                              []byte
	variant                                     string
	reads                                       int
}

func jsonBytes(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assignmentArchive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		data := files[name]
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func assignmentFixtureFor(t *testing.T) *assignmentFixture {
	t.Helper()
	base, workflow, head := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	command := "/agent assign codex-thorough@" + base + " authority=branch-draft-pr"
	f := &assignmentFixture{patch: []byte("diff --git a/source b/source\n"), summary: []byte(`{"summary":"safe"}`)}
	user := map[string]any{"id": 7, "login": "octocat", "type": "User"}
	bot := map[string]any{"id": 41898282, "login": "github-actions[bot]", "type": "Bot"}
	repo := map[string]any{"id": 42, "full_name": "octo/demo", "default_branch": "main"}
	f.issue = map[string]any{"number": 3, "title": "Task", "body": "Issue context", "html_url": "https://github.com/octo/demo/issues/3", "state": "open"}
	f.comment = map[string]any{"id": 99, "body": command, "html_url": "https://github.com/octo/demo/issues/3#issuecomment-99", "user": user, "created_at": now, "updated_at": now}
	f.run = map[string]any{"id": 321, "run_attempt": 1, "head_sha": workflow, "head_branch": "main", "path": ".github/workflows/agent-assignment.yml", "event": "issue_comment", "display_title": "Agent assignment 42:99", "repository": repo, "head_repository": repo, "actor": user, "created_at": now.Add(time.Second), "status": "completed", "conclusion": "success"}
	a := assignmentMetadata{Source: "https://github.com/octo/demo/issues/3", RequestCommentID: 99, Request: "https://github.com/octo/demo/issues/3#issuecomment-99", ProfileID: "codex-thorough", ProfileRevision: base, RequesterID: 7, Requester: "octocat", Authority: "branch-draft-pr", Run: "https://github.com/octo/demo/actions/runs/321/attempts/1"}
	p := publicationMetadata{Version: 1, AssignmentID: "42:99", BaseSHA: base, HeadSHA: head, ArtifactID: 88, Attempt: 1, PatchSHA256: digest(f.patch)}
	f.pr = map[string]any{"number": 5, "html_url": "https://github.com/octo/demo/pull/5", "state": "open", "draft": true, "user": bot, "head": map[string]any{"ref": "agent/codex-thorough/42-99", "sha": head, "repo": repo}, "base": map[string]any{"ref": "main", "sha": base, "repo": repo}, "body": "<!-- agent-assignment:v1\n" + string(jsonBytes(t, a)) + "\n-->\n<!-- agent-publication:v1\n" + string(jsonBytes(t, p)) + "\n-->"}
	f.ref = map[string]any{"object": map[string]any{"sha": head, "type": "commit"}}
	author := map[string]any{"name": "github-actions[bot]", "email": "41898282+github-actions[bot]@users.noreply.github.com", "date": now.Add(time.Second)}
	f.commit = map[string]any{"sha": head, "tree": map[string]any{"sha": strings.Repeat("d", 40)}, "parents": []any{map[string]any{"sha": base}}, "author": author, "committer": author, "message": fmt.Sprintf("Propose issue #3 for agent assignment 42:99\n\nProfile: codex-thorough@%s\nBase: %s\nProposal: 321/1 artifact 88\nPatch-SHA256: %s", base, base, digest(f.patch))}
	f.check = map[string]any{"id": 77, "name": "Agent proposal / repository-check", "external_id": "agent-publication:42:99", "head_sha": head, "app": map[string]any{"id": 15368}, "details_url": "https://github.com/octo/demo/runs/77", "status": "completed", "conclusion": "neutral", "output": map[string]any{"summary": "Draft patch proposal: [PR #5](https://github.com/octo/demo/pull/5). Repository validation: **failed**.\n\n[Assignment request](https://github.com/octo/demo/issues/3#issuecomment-99); [verified proposal attempt](https://github.com/octo/demo/actions/runs/321/attempts/1). Human review is required."}}
	f.result = map[string]any{"contract_version": 1, "assignment_id": "42:99", "operation": "assignment", "profile_id": "codex-thorough", "profile_revision": base, "policy_revision": base, "base_sha": base, "head_sha": base, "run_id": 321, "run_attempt": 1, "outcome": "ready", "reason_code": "PROPOSAL_READY", "message": "Patch proposal ready.", "next_action": "Review the original workflow attempt and its verified artifacts.", "policy": map[string]any{"requested_model": "gpt-6.1-sol", "requested_reasoning_effort": "high", "effective_model": "gpt-6.1-sol", "effective_reasoning_effort": "high", "cli_version": "codex-cli 0.159.3", "authority": "branch-draft-pr", "sandbox": "workspace-write", "network": false, "isolation": "verified"}, "validation": []any{map[string]any{"id": "repository-check", "outcome": "failed", "evidence": "Repository check failed; inspect privately."}}, "proposal": map[string]any{"patch": "proposal.patch", "patch_sha256": digest(f.patch), "summary": "summary.json", "summary_sha256": digest(f.summary)}}
	f.invocation = map[string]any{"contract_version": 1, "assignment_id": "42:99", "operation": "assignment", "profile_id": "codex-thorough", "authority": "branch-draft-pr", "source_url": a.Source, "request_url": a.Request, "run_url": a.Run, "repository": map[string]any{"id": 42, "owner": "octo", "name": "demo"}, "issue_number": 3, "request_comment_id": 99, "requester": user, "profile_revision": base, "base_sha": base, "run_id": 321, "run_attempt": 1}
	f.artifactOrigin = map[string]any{"id": 321, "repository_id": 42, "head_repository_id": 42, "head_sha": workflow, "head_branch": "main"}
	for _, name := range []string{"authorize", "dispatch", "patch", "publish"} {
		f.jobs = append(f.jobs, map[string]any{"id": int64(len(f.jobs) + 1), "run_id": 321, "head_sha": workflow, "name": name, "status": "completed", "conclusion": "success", "runner_id": 12, "labels": []string{"self-hosted", "linux", "hub-agent-codex"}})
	}
	storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credential leaked to artifact storage")
		}
		files := f.artifactFiles(t, r.URL.Path)
		content := assignmentArchive(t, files)
		if f.variant == "corrupt-archive" {
			content = append(content, 'x')
		}
		_, _ = w.Write(content)
	}))
	t.Cleanup(storage.Close)
	// Archives use a deterministic ordering so their metadata digest matches storage bytes.
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f.serve(t, w, r, storage.URL, repo, bot) }))
	t.Cleanup(api.Close)
	client := gh.NewClient(api.Client())
	client.BaseURL, _ = client.BaseURL.Parse(api.URL + "/")
	decoder, err := evidence.NewResultDecoder()
	if err != nil {
		t.Fatal(err)
	}
	f.reader = NewAssignmentReader(client.WithAuthToken("synthetic-discovery-never-live"), decoder)
	f.reader.download = storage.Client()
	f.reader.download.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return f
}

func (f *assignmentFixture) artifactFiles(t *testing.T, path string) map[string][]byte {
	if path == "/89" {
		return map[string][]byte{"invocation.json": jsonBytes(t, f.invocation)}
	}
	return map[string][]byte{"result.json": jsonBytes(t, f.result), "proposal.patch": f.patch, "summary.json": f.summary}
}

func (f *assignmentFixture) serve(t *testing.T, w http.ResponseWriter, r *http.Request, storage string, repo, bot map[string]any) {
	t.Helper()
	f.reads++
	if r.Method != "GET" {
		t.Errorf("projection wrote GitHub: %s", r.Method)
	}
	if r.Header.Get("Authorization") != "Bearer synthetic-discovery-never-live" {
		t.Error("missing discovery credential")
	}
	w.Header().Set("Content-Type", "application/json")
	var value any
	switch r.URL.Path {
	case "/repos/octo/demo":
		value = repo
	case "/repos/octo/demo/issues/3":
		value = f.issue
	case "/repos/octo/demo/issues/3/comments":
		if f.variant == "incomplete-comments" {
			w.Header().Set("Link", "<https://api.example.invalid/comments?page=21>; rel=\"next\"")
		}
		value = []any{f.comment}
	case "/repos/octo/demo/actions/workflows/agent-assignment.yml/runs":
		runs := []any{f.run}
		if f.variant == "no-run" {
			runs = []any{}
		}
		total := len(runs)
		if f.variant == "incomplete-runs" {
			total = 1001
		}
		if f.variant == "canonical-run" {
			earlier := map[string]any{}
			for key, value := range f.run {
				earlier[key] = value
			}
			earlier["id"] = 320
			runs = append(runs, earlier)
		}
		value = map[string]any{"total_count": total, "workflow_runs": runs}
	case "/repos/octo/demo/actions/runs/321/attempts/1":
		value = f.run
	case "/repos/octo/demo/actions/runs/321/attempts/1/jobs", "/repos/octo/demo/actions/runs/320/attempts/1/jobs":
		value = map[string]any{"jobs": f.jobs}
	case "/repos/octo/demo/pulls":
		if f.variant == "no-pr" || f.variant == "blocked" || f.variant == "canonical-run" {
			value = []any{}
		} else {
			value = []any{f.pr}
		}
	case "/repos/octo/demo/pulls/5":
		value = f.pr
	case "/users/github-actions[bot]":
		value = bot
	case "/repos/octo/demo/git/ref/heads/agent/codex-thorough/42-99":
		value = f.ref
	case "/repos/octo/demo/git/commits/" + strings.Repeat("c", 40):
		value = f.commit
	case "/apps/github-actions":
		value = map[string]any{"id": 15368, "slug": "github-actions"}
	case "/repos/octo/demo/commits/" + strings.Repeat("c", 40) + "/check-runs":
		checks := []any{f.check}
		if f.variant == "no-check" {
			checks = []any{}
		}
		value = map[string]any{"check_runs": checks}
	case "/repos/octo/demo/actions/runs/321/artifacts":
		artifacts := []any{}
		for id, name := range map[int64]string{88: "agent-patch-v1-1", 89: "agent-invocation-v1-1"} {
			if f.variant == "missing-artifact" && id == 88 {
				continue
			}
			data := assignmentArchive(t, f.artifactFiles(t, fmt.Sprintf("/%d", id)))
			artifact := map[string]any{"id": id, "name": name, "size_in_bytes": len(data), "digest": "sha256:" + digest(data), "workflow_run": f.artifactOrigin, "expired": f.variant == "expired-artifact"}
			artifacts = append(artifacts, artifact)
			if f.variant == "duplicate-artifact" && id == 88 {
				artifacts = append(artifacts, artifact)
			}
		}
		value = map[string]any{"artifacts": artifacts}
	case "/repos/octo/demo/actions/artifacts/88/zip", "/repos/octo/demo/actions/artifacts/89/zip":
		id := "88"
		if strings.Contains(r.URL.Path, "/89/") {
			id = "89"
		}
		w.Header().Set("Location", storage+"/"+id)
		w.WriteHeader(302)
		return
	default:
		t.Errorf("unexpected endpoint %s", r.URL.Path)
		w.WriteHeader(500)
		return
	}
	_ = json.NewEncoder(w).Encode(value)
}
