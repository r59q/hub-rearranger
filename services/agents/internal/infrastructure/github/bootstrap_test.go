package github

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

func TestBootstrapSnapshotPinsReadsAndRejectsUnsafeTreesAndBlobs(t *testing.T) {
	for _, variant := range []string{"matching", "changed", "truncated", "symlink", "parent", "duplicate", "bad-blob", "directory", "commit-mismatch", "tree-mismatch"} {
		t.Run(variant, func(t *testing.T) {
			original := "setup"
			if variant != "matching" {
				original = "existing setup"
			}
			sha := fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(original), original))))
			blobReads := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var value any
				switch r.URL.Path {
				case "/repos/octo/demo":
					value = map[string]any{"full_name": "octo/demo", "default_branch": "main", "private": true}
				case "/repos/octo/demo/branches/main":
					value = map[string]any{"commit": map[string]string{"sha": testSHA}}
				case "/repos/octo/demo/git/commits/" + testSHA:
					commit := &gh.Commit{SHA: gh.Ptr(testSHA), Tree: &gh.Tree{SHA: gh.Ptr(strings.Repeat("b", 40))}}
					if variant == "commit-mismatch" {
						commit.SHA = gh.Ptr(strings.Repeat("c", 40))
					}
					value = commit
				case "/repos/octo/demo/git/trees/" + strings.Repeat("b", 40):
					if r.URL.Query().Get("recursive") != "1" {
						t.Error("snapshot not complete")
					}
					entry := &gh.TreeEntry{Path: gh.Ptr("docs/agent-workflows.md"), Type: gh.Ptr("blob"), Mode: gh.Ptr("100644"), SHA: gh.Ptr(sha), Size: gh.Ptr(len(original))}
					if variant == "symlink" {
						entry.Mode = gh.Ptr("120000")
					}
					if variant == "parent" {
						entry.Path = gh.Ptr("docs")
					}
					if variant == "directory" {
						entry.Type = gh.Ptr("tree")
						entry.Mode = gh.Ptr("040000")
					}
					tree := &gh.Tree{SHA: gh.Ptr(strings.Repeat("b", 40)), Truncated: gh.Ptr(variant == "truncated"), Entries: []*gh.TreeEntry{entry}}
					if variant == "tree-mismatch" {
						tree.SHA = gh.Ptr(strings.Repeat("c", 40))
					}
					if variant == "duplicate" {
						tree.Entries = append(tree.Entries, entry)
					}
					value = tree
				case "/repos/octo/demo/git/blobs/" + sha:
					blobReads++
					content := original
					if variant == "bad-blob" {
						content = "unbound data"
					}
					value = map[string]any{"sha": sha, "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(content))}
				default:
					t.Errorf("unexpected read %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode(value)
			}))
			defer server.Close()
			client := gh.NewClient(server.Client())
			client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")
			snapshot, err := NewProfileReader(client).ReadBootstrap(context.Background(), domain.Repository{Owner: "octo", Name: "demo"}, map[string]string{"docs/agent-workflows.md": "setup"})
			if variant == "matching" || variant == "changed" {
				if err != nil || snapshot.Revision != testSHA || snapshot.Files["docs/agent-workflows.md"] != original || snapshot.Blobs["docs/agent-workflows.md"] != sha {
					t.Fatal(snapshot, err)
				}
				if variant == "matching" && blobReads != 0 {
					t.Fatal("matching bytes unnecessarily downloaded")
				}
			} else if err == nil {
				t.Fatal("unsafe or incomplete snapshot accepted")
			}
		})
	}
}
