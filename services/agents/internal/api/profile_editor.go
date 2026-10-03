package api

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/r59q/hub-rearranger/services/agents/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

type editorUseCases interface {
	RepositoryProfileEditor(context.Context, domain.Repository) (domain.ProfileEditorState, error)
	PreviewProfileDraft(context.Context, domain.Repository, domain.ProfileDraft) (domain.BootstrapPreview, error)
}

func (h *Handler) editorContext(w http.ResponseWriter, r *http.Request, owner, repo string) (editorUseCases, context.Context, context.CancelFunc, bool) {
	if !ownerName.MatchString(owner) || !repositoryName.MatchString(repo) || repo == "." || repo == ".." {
		writeJSON(w, 400, contract.Error{Code: contract.ErrorCodeInvalidRequest, Message: "Choose a valid GitHub repository."})
		return nil, nil, nil, false
	}
	service, ok := h.service.(editorUseCases)
	if !ok {
		writeJSON(w, 502, contract.Error{Code: contract.ErrorCodeGithubUnavailable, Message: "Profile authoring is unavailable."})
		return nil, nil, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	return service, ctx, cancel, true
}

func (h *Handler) GetRepositoryProfileEditor(w http.ResponseWriter, r *http.Request, owner, repo string) {
	service, ctx, cancel, ok := h.editorContext(w, r, owner, repo)
	if !ok {
		return
	}
	defer cancel()
	state, err := service.RepositoryProfileEditor(ctx, domain.Repository{Owner: owner, Name: repo})
	if err != nil {
		status, problem := profileError(err)
		writeJSON(w, status, problem)
		return
	}
	dto := contract.ProfileEditor{BaseRevision: state.BaseRevision, Diagnostics: []contract.BootstrapDiagnostic{}}
	if len(state.Diagnostics) == 0 {
		dto.Draft = draftDTO(state.Draft)
	}
	for _, issue := range state.Diagnostics {
		dto.Diagnostics = append(dto.Diagnostics, contract.BootstrapDiagnostic{Code: issue.Code, Path: issue.Path, Message: issue.Message})
	}
	writeJSON(w, 200, dto)
}

func (h *Handler) PreviewRepositoryProfile(w http.ResponseWriter, r *http.Request, owner, repo string) {
	service, ctx, cancel, ok := h.editorContext(w, r, owner, repo)
	if !ok {
		return
	}
	defer cancel()
	var draft contract.ProfileDraft
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	// Required false booleans must remain distinguishable from missing fields.
	var fields map[string]json.RawMessage
	if err != nil || kind != "application/json" || decoder.Decode(&fields) != nil || decoder.Decode(new(any)) != io.EOF || len(fields) != 5 {
		invalidDraft(w)
		return
	}
	for _, key := range []string{"name", "description", "enabled", "context_sources", "review_comments"} {
		if len(fields[key]) == 0 || string(fields[key]) == "null" {
			invalidDraft(w)
			return
		}
	}
	encoded, _ := json.Marshal(fields)
	if json.Unmarshal(encoded, &draft) != nil {
		invalidDraft(w)
		return
	}
	sources := make([]string, len(draft.ContextSources))
	for i, source := range draft.ContextSources {
		sources[i] = string(source)
	}
	preview, err := service.PreviewProfileDraft(ctx, domain.Repository{Owner: owner, Name: repo}, domain.ProfileDraft{Name: draft.Name, Description: draft.Description, Enabled: draft.Enabled, ContextSources: sources, ReviewComments: draft.ReviewComments})
	if err != nil {
		status, problem := profileError(err)
		writeJSON(w, status, problem)
		return
	}
	writeJSON(w, 200, bootstrapDTO(preview))
}

func draftDTO(draft domain.ProfileDraft) *contract.ProfileDraft {
	result := &contract.ProfileDraft{Name: draft.Name, Description: draft.Description, Enabled: draft.Enabled, ReviewComments: draft.ReviewComments, ContextSources: []contract.ProfileDraftContextSources{}}
	for _, source := range draft.ContextSources {
		result.ContextSources = append(result.ContextSources, contract.ProfileDraftContextSources(source))
	}
	return result
}

func invalidDraft(w http.ResponseWriter) {
	writeJSON(w, 400, contract.Error{Code: contract.ErrorCodeInvalidRequest, Message: "Submit only the five structured profile choices. Files, credentials and runtime overrides are not accepted."})
}
