package api

import (
	"context"
	"net/http"
	"time"

	"github.com/r59q/hub-rearranger/services/agents/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

type bootstrapUseCases interface {
	RepositoryBootstrap(context.Context, domain.Repository) (domain.BootstrapPreview, error)
}

func (h *Handler) GetRepositoryBootstrap(w http.ResponseWriter, r *http.Request, owner, repo string) {
	h.repositoryBootstrap(w, r, owner, repo, false)
}

func (h *Handler) HeadRepositoryBootstrap(w http.ResponseWriter, r *http.Request, owner, repo string) {
	h.repositoryBootstrap(w, r, owner, repo, true)
}

func (h *Handler) repositoryBootstrap(w http.ResponseWriter, r *http.Request, owner, repo string, head bool) {
	respond := func(status int, value any) {
		if head {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(status)
			return
		}
		writeJSON(w, status, value)
	}
	if !ownerName.MatchString(owner) || !repositoryName.MatchString(repo) || repo == "." || repo == ".." {
		respond(400, contract.Error{Code: contract.ErrorCodeInvalidRequest, Message: "Choose a valid GitHub repository."})
		return
	}
	service, ok := h.service.(bootstrapUseCases)
	if !ok {
		respond(500, contract.Error{Code: contract.ErrorCodeInternalError, Message: "Bootstrap planning is unavailable."})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	preview, err := service.RepositoryBootstrap(ctx, domain.Repository{Owner: owner, Name: repo})
	if err != nil {
		status, problem := profileError(err)
		if status == 404 {
			problem.Message = "Check repository access and initialize its default branch. Existing bootstrap paths must be bounded regular UTF-8 files with directory parents."
		} else if status == 502 {
			problem.Message = "The complete bootstrap snapshot or canonical templates are unavailable. Check the Agents installation and retry."
		}
		respond(status, problem)
		return
	}
	dto := contract.BootstrapPreview{Repository: preview.Repository, DefaultBranch: preview.DefaultBranch,
		BaseRevision: preview.BaseRevision, Digest: preview.Digest, State: contract.BootstrapPreviewState(preview.State),
		Private: preview.Private, Diff: preview.Diff, Files: []contract.BootstrapPreviewFile{}, Diagnostics: []contract.BootstrapDiagnostic{}}
	for _, file := range preview.Files {
		var base, digest *string
		if file.BaseSHA != "" {
			base = &file.BaseSHA
		}
		if file.SHA256 != "" {
			digest = &file.SHA256
		}
		dto.Files = append(dto.Files, contract.BootstrapPreviewFile{Path: file.Path, Status: contract.BootstrapPreviewFileStatus(file.Status), BaseSha: base, Sha256: digest, Content: file.Content})
	}
	for _, issue := range preview.Diagnostics {
		dto.Diagnostics = append(dto.Diagnostics, contract.BootstrapDiagnostic{Code: issue.Code, Path: issue.Path, Message: issue.Message})
	}
	respond(200, dto)
}
