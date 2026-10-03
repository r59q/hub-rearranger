package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type BootstrapSnapshot struct {
	DefaultBranch, Revision string
	Private                 bool
	Files                   map[string]string
	Blobs                   map[string]string
}

type BootstrapPreviewFile struct {
	BootstrapFile
	BaseSHA string
}

type BootstrapPreview struct {
	Repository, DefaultBranch, BaseRevision, Digest, State, Diff string
	Private                                                      bool
	Files                                                        []BootstrapPreviewFile
	Diagnostics                                                  []Diagnostic
}

type BootstrapTemplates interface {
	ReadTemplates(context.Context) (map[string]string, error)
}

type BootstrapRepositoryReader interface {
	ReadBootstrap(context.Context, Repository, map[string]string) (BootstrapSnapshot, error)
}

type BootstrapDiff interface {
	Render(context.Context, BootstrapPlan, map[string]string) (string, error)
}

type BootstrapService struct {
	templates BootstrapTemplates
	reader    BootstrapRepositoryReader
	merger    BootstrapCatalogMerger
	diff      BootstrapDiff
}

func NewBootstrapService(templates BootstrapTemplates, reader BootstrapRepositoryReader, merger BootstrapCatalogMerger, diff BootstrapDiff) *BootstrapService {
	return &BootstrapService{templates: templates, reader: reader, merger: merger, diff: diff}
}

func (s *BootstrapService) Preview(ctx context.Context, repository Repository) (BootstrapPreview, error) {
	templates, err := s.templates.ReadTemplates(ctx)
	if err != nil {
		return BootstrapPreview{}, ErrGitHubUnavailable
	}
	snapshot, err := s.reader.ReadBootstrap(ctx, repository, templates)
	if err != nil {
		return BootstrapPreview{}, err
	}
	plan := PlanBootstrap(templates, snapshot.Files, s.merger)
	preview := BootstrapPreview{Repository: repository.FullName(), DefaultBranch: snapshot.DefaultBranch,
		BaseRevision: snapshot.Revision, Private: snapshot.Private, State: "unchanged", Files: []BootstrapPreviewFile{}, Diagnostics: plan.Diagnostics}
	for _, file := range plan.Files {
		preview.Files = append(preview.Files, BootstrapPreviewFile{BootstrapFile: file, BaseSHA: snapshot.Blobs[file.Path]})
		if file.Status == "create" || file.Status == "update" {
			preview.State = "ready"
		}
	}
	if len(plan.Diagnostics) > 0 {
		preview.State = "conflict"
	}
	// The review identity binds the base, original blobs and every proposed byte.
	// No timestamp, cookie, token or durable Hub draft participates in this digest.
	encoded, err := json.Marshal(struct {
		Repository, Branch, Base string
		Files                    []BootstrapPreviewFile
	}{preview.Repository, preview.DefaultBranch, preview.BaseRevision, preview.Files})
	if err != nil {
		return BootstrapPreview{}, ErrGitHubUnavailable
	}
	digest := sha256.Sum256(encoded)
	preview.Digest = hex.EncodeToString(digest[:])
	preview.Diff, err = s.diff.Render(ctx, plan, snapshot.Files)
	return preview, err
}
