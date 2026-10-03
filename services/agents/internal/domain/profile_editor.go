package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
)

// ProfileDraft contains authoring choices, never file paths, commands or source.
// The installed v1 adapter's fixed policy comes from canonical templates.
type ProfileDraft struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Enabled        bool     `json:"enabled"`
	ContextSources []string `json:"context_sources"`
	ReviewComments bool     `json:"review_comments"`
}

type ProfileEditorState struct {
	BaseRevision string
	Draft        ProfileDraft
	Diagnostics  []Diagnostic
}

type ProfileEditor interface {
	ReadProfileDraft(existing, canonical []byte) (ProfileDraft, []Diagnostic)
	MergeProfileDraft(existing, canonical []byte, draft ProfileDraft) ([]byte, []Diagnostic)
}

func (s *BootstrapService) Editor(ctx context.Context, repository Repository) (ProfileEditorState, error) {
	templates, err := s.templates.ReadTemplates(ctx)
	if err != nil {
		return ProfileEditorState{}, ErrGitHubUnavailable
	}
	snapshot, err := s.reader.ReadBootstrap(ctx, repository, templates)
	if err != nil {
		return ProfileEditorState{}, err
	}
	editor, ok := s.merger.(ProfileEditor)
	if !ok {
		return ProfileEditorState{}, ErrGitHubUnavailable
	}
	draft, diagnostics := editor.ReadProfileDraft([]byte(snapshot.Files[ProfileCatalogPath]), []byte(templates[ProfileCatalogPath]))
	return ProfileEditorState{BaseRevision: snapshot.Revision, Draft: draft, Diagnostics: diagnostics}, nil
}

func (s *Service) RepositoryProfileEditor(ctx context.Context, repository Repository) (ProfileEditorState, error) {
	if s.bootstrap == nil {
		return ProfileEditorState{}, ErrGitHubUnavailable
	}
	return s.bootstrap.Editor(ctx, repository)
}

func (s *Service) PreviewProfileDraft(ctx context.Context, repository Repository, draft ProfileDraft) (BootstrapPreview, error) {
	if s.bootstrap == nil {
		return BootstrapPreview{}, ErrGitHubUnavailable
	}
	return s.bootstrap.preview(ctx, repository, &draft)
}

func (s *BootstrapService) editPlan(plan BootstrapPlan, snapshot BootstrapSnapshot, templates map[string]string, draft ProfileDraft) (BootstrapPlan, error) {
	editor, ok := s.merger.(ProfileEditor)
	if !ok {
		return BootstrapPlan{}, ErrGitHubUnavailable
	}
	content, diagnostics := editor.MergeProfileDraft([]byte(snapshot.Files[ProfileCatalogPath]), []byte(templates[ProfileCatalogPath]), draft)
	retained := []Diagnostic{}
	for _, issue := range plan.Diagnostics {
		if issue.Path != ProfileCatalogPath {
			retained = append(retained, issue)
		}
	}
	for i := range diagnostics {
		diagnostics[i].Path = ProfileCatalogPath + diagnostics[i].Path
	}
	plan.Diagnostics = append(retained, diagnostics...)
	for i := range plan.Files {
		if plan.Files[i].Path != ProfileCatalogPath {
			continue
		}
		file := &plan.Files[i]
		*file = BootstrapFile{Path: ProfileCatalogPath, Status: "conflict"}
		if len(diagnostics) > 0 {
			continue
		}
		file.Content = string(content)
		hash := sha256.Sum256(content)
		file.SHA256 = hex.EncodeToString(hash[:])
		file.Status = "create"
		if previous, exists := snapshot.Files[ProfileCatalogPath]; exists {
			file.Status = "update"
			if previous == file.Content {
				file.Status = "unchanged"
			}
		}
	}
	return plan, nil
}
