package github

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

type publicationFixture struct {
	mu              sync.Mutex
	t               *testing.T
	base            *fixture
	client          *Client
	service         *domain.Service
	event           domain.Event
	input           domain.Invocation
	proposal        domain.Proposal
	archives        map[int64][]byte
	source          []byte
	storageURL      string
	ref             *gh.Reference
	pull            *gh.PullRequest
	commit          *gh.Commit
	checks          []*gh.CheckRun
	posts           map[string]int
	lose, fail      string
	artifactVariant string
}

func publicationArchive(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gzipWriter)
	content := []byte("before\n")
	if writer.WriteHeader(&tar.Header{Name: "synthetic/app.txt", Mode: 0644, Size: int64(len(content))}) != nil {
		t.Fatal("archive header")
	}
	_, _ = writer.Write(content)
	if writer.Close() != nil || gzipWriter.Close() != nil {
		t.Fatal("archive closure")
	}
	return buffer.Bytes()
}

func publicationSetup(t *testing.T) *publicationFixture {
	t.Helper()
	service, client, base, event := setup(t)
	f := &publicationFixture{t: t, base: base, client: client, service: service, event: event, posts: map[string]int{}, archives: map[int64][]byte{}, source: publicationArchive(t)}
	server := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(server.Close)
	client.api.BaseURL, _ = url.Parse(server.URL + "/")
	storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("GitHub token reached archive storage")
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/source":
			_, _ = w.Write(f.source)
		default:
			id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/"), 10, 64)
			if body, ok := f.archives[id]; ok {
				_, _ = w.Write(body)
			} else {
				w.WriteHeader(404)
			}
		}
	}))
	t.Cleanup(storage.Close)
	f.storageURL = storage.URL
	client.download = storage.Client()
	decision, err := service.Intake(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	serialized, _ := json.Marshal(decision.Invocation)
	if json.Unmarshal(serialized, &f.input) != nil {
		t.Fatal("input unavailable")
	}
	f.proposal = domain.Proposal{Invocation: f.input, ArtifactID: 51, Attempt: 1, Created: base.created.Add(1e9), Patch: []byte("diff --git a/app.txt b/app.txt\n--- a/app.txt\n+++ b/app.txt\n@@ -1 +1 @@\n-before\n+after\n")}
	check := map[string]any{"id": "repository-check", "outcome": "failed", "evidence": "Repository check failed; inspect privately."}
	f.proposal.Summary, _ = json.Marshal(map[string]any{"assignment_id": f.input.AssignmentID, "summary": "A patch is available for independent draft-PR publication.", "validation": []any{check}})
	digest := sha256.Sum256(f.proposal.Summary)
	f.proposal.Result, _ = json.Marshal(map[string]any{"contract_version": 1, "assignment_id": f.input.AssignmentID, "operation": "assignment", "profile_id": "codex-thorough", "profile_revision": f.input.ProfileRevision, "policy_revision": f.input.PolicyRevision, "base_sha": f.input.BaseSHA, "head_sha": f.input.BaseSHA, "run_id": 100, "run_attempt": 1, "outcome": "ready", "reason_code": "PROPOSAL_READY", "message": "A patch is available for independent draft-PR publication.", "next_action": "Review the original workflow attempt and its verified artifacts.", "policy": map[string]any{"requested_model": "gpt-6.1-sol", "effective_model": "gpt-6.1-sol", "requested_reasoning_effort": "high", "effective_reasoning_effort": "high", "cli_version": "codex-cli 0.159.3", "authority": "branch-draft-pr", "sandbox": "workspace-write", "network": false, "isolation": "verified"}, "validation": []any{check}, "proposal": map[string]any{"patch": "proposal.patch", "patch_sha256": patchDigest(f.proposal), "summary": "summary.json", "summary_sha256": hex.EncodeToString(digest[:])}})
	f.archives[50] = executionArchive(t, map[string][]byte{"invocation.json": serialized})
	f.archives[51] = executionArchive(t, map[string][]byte{"result.json": f.proposal.Result, "proposal.patch": f.proposal.Patch, "summary.json": f.proposal.Summary})
	return f
}

func (f *publicationFixture) replyPost(w http.ResponseWriter, stage string, value any) {
	f.posts[stage]++
	if f.lose == stage {
		w.WriteHeader(503)
		return
	}
	_ = json.NewEncoder(w).Encode(value)
}

func (f *publicationFixture) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("Authorization") != "Bearer synthetic-workflow-token" {
		f.mu.Unlock()
		f.t.Error("missing workflow token")
		w.WriteHeader(401)
		return
	}
	path := r.URL.Path
	stage := ""
	if r.Method == "POST" {
		switch path {
		case "/repos/octo/demo/git/refs":
			stage = "branch"
		case "/repos/octo/demo/pulls":
			stage = "pr"
		case "/repos/octo/demo/check-runs":
			stage = "check"
		case "/repos/octo/demo/issues/3/comments":
			if f.input.AssignmentID != "" {
				stage = "comment"
			}
		}
	}
	if stage != "" && f.fail == stage {
		f.posts[stage]++
		f.mu.Unlock()
		w.WriteHeader(503)
		return
	}
	if f.workflowRequest(w, r) || f.gitRequest(w, r) || f.objectsRequest(w, r) {
		f.mu.Unlock()
		return
	}
	f.mu.Unlock()
	f.base.handler(w, r)
}

func (f *publicationFixture) value() domain.Publication {
	return domain.Publication{Branch: domain.PublicationBranch(f.input), HeadSHA: strings.Repeat("4", 40), PRNumber: 12, PRURL: "https://github.com/octo/demo/pull/12"}
}
