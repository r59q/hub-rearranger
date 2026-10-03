package domain

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type previewFixture struct {
	templates map[string]string
	snapshot  BootstrapSnapshot
	err       error
}

func (f *previewFixture) ReadTemplates(context.Context) (map[string]string, error) {
	return f.templates, nil
}

func (f *previewFixture) ReadBootstrap(ctx context.Context, _ Repository, _ map[string]string) (BootstrapSnapshot, error) {
	if ctx.Err() != nil {
		return BootstrapSnapshot{}, ctx.Err()
	}
	return f.snapshot, f.err
}

func (f *previewFixture) Render(context.Context, BootstrapPlan, map[string]string) (string, error) {
	return "reviewed diff", nil
}

func TestBootstrapPreviewDigestBindsBaseOriginalBlobAndProposedBytes(t *testing.T) {
	f := &previewFixture{templates: map[string]string{"docs/agent-workflows.md": "setup"}, snapshot: BootstrapSnapshot{DefaultBranch: "main", Revision: strings.Repeat("a", 40), Files: map[string]string{}, Blobs: map[string]string{}}}
	service := NewBootstrapService(f, f, bootstrapMerger{}, f)
	first, err := service.Preview(context.Background(), Repository{"octo", "demo"})
	if err != nil || first.State != "ready" || first.Diff != "reviewed diff" {
		t.Fatal(first, err)
	}
	repeat, _ := service.Preview(context.Background(), Repository{"octo", "demo"})
	if repeat.Digest != first.Digest {
		t.Fatal("unstable review identity")
	}
	for _, change := range []func(){func() { f.snapshot.Revision = strings.Repeat("b", 40) }, func() { f.snapshot.Blobs["docs/agent-workflows.md"] = strings.Repeat("c", 40) }, func() { f.templates["docs/agent-workflows.md"] = "new setup" }} {
		change()
		next, _ := service.Preview(context.Background(), Repository{"octo", "demo"})
		if next.Digest == repeat.Digest {
			t.Fatal("review identity ignored changed input")
		}
		repeat = next
	}
}

func TestBootstrapPreviewReportsUnchangedConflictAndCancelledReads(t *testing.T) {
	f := &previewFixture{templates: map[string]string{"AGENTS.md": BootstrapInstructions}, snapshot: BootstrapSnapshot{DefaultBranch: "main", Revision: strings.Repeat("a", 40), Files: map[string]string{"AGENTS.md": BootstrapInstructions}, Blobs: map[string]string{"AGENTS.md": strings.Repeat("b", 40)}}}
	service := NewBootstrapService(f, f, bootstrapMerger{}, f)
	preview, err := service.Preview(context.Background(), Repository{"octo", "demo"})
	if err != nil || preview.State != "unchanged" {
		t.Fatal(preview, err)
	}
	f.snapshot.Files["AGENTS.md"] = "<!-- hub-agent-bootstrap:v1:begin -->"
	preview, err = service.Preview(context.Background(), Repository{"octo", "demo"})
	if err != nil || preview.State != "conflict" || len(preview.Diagnostics) == 0 {
		t.Fatal(preview, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = service.Preview(ctx, Repository{"octo", "demo"})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored")
	}
}
