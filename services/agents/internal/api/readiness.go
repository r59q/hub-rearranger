package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/r59q/hub-rearranger/services/agents/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

func (h *Handler) GetRepositoryReadiness(response http.ResponseWriter, request *http.Request, owner, repo string) {
	h.repositoryReadiness(response, request, owner, repo, false)
}

func (h *Handler) HeadRepositoryReadiness(response http.ResponseWriter, request *http.Request, owner, repo string) {
	h.repositoryReadiness(response, request, owner, repo, true)
}

func (h *Handler) repositoryReadiness(response http.ResponseWriter, request *http.Request, owner, repo string, head bool) {
	respond := func(status int, value any) {
		if head {
			response.Header().Set("Cache-Control", "no-store")
			response.WriteHeader(status)
			return
		}
		writeJSON(response, status, value)
	}

	if !ownerName.MatchString(owner) || !repositoryName.MatchString(repo) || repo == "." || repo == ".." {
		respond(http.StatusBadRequest, contract.Error{Code: contract.ErrorCodeInvalidRequest, Message: "Use a valid GitHub owner and repository name."})
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), 20*time.Second)
	defer cancel()
	result, err := h.service.RepositoryReadiness(ctx, domain.Repository{Owner: owner, Name: repo})
	if err != nil {
		status, failure := profileError(err)
		if status == http.StatusForbidden {
			failure.Message = "Grant the server-side GitHub token read-only Metadata, Contents, and Actions access to this repository, then retry."
		}
		h.logger.Warn("agents readiness read failed", "code", failure.Code)
		respond(status, failure)
		return
	}

	dto, err := toReadinessDTO(result)
	if err != nil {
		respond(http.StatusInternalServerError, contract.Error{Code: contract.ErrorCodeInternalError, Message: "Repository readiness could not be loaded. Try again."})
		return
	}

	respond(http.StatusOK, dto)
}

func toReadinessDTO(result domain.RepositoryReadiness) (contract.RepositoryReadiness, error) {
	dto := contract.RepositoryReadiness{Repository: result.Repository, DefaultBranch: result.DefaultBranch, Revision: result.Revision, CatalogState: contract.RepositoryReadinessCatalogState(result.CatalogState), Profiles: []contract.ProfileReadiness{}, Diagnostics: []contract.ProfileDiagnostic{}}
	for _, issue := range result.Diagnostics {
		dto.Diagnostics = append(dto.Diagnostics, contract.ProfileDiagnostic{Code: contract.ProfileDiagnosticCode(issue.Code), Path: issue.Path, Message: issue.Message})
	}

	for _, profile := range result.Profiles {
		row := contract.ProfileReadiness{Id: profile.ID, Revision: profile.Revision, State: contract.ProfileReadinessState(profile.State), RunnerLabel: profile.RunnerLabel, NextAction: profile.NextAction, Diagnostics: []contract.ReadinessDiagnostic{}}
		for _, issue := range profile.Diagnostics {
			row.Diagnostics = append(row.Diagnostics, contract.ReadinessDiagnostic{Code: contract.ReadinessDiagnosticCode(issue.Code), Path: issue.Path, Message: issue.Message})
		}

		if profile.Evidence != nil {
			content, err := json.Marshal(profile.Evidence)
			if err != nil {
				return dto, err
			}
			row.Evidence = new(contract.RuntimeEvidence)
			if err := json.Unmarshal(content, row.Evidence); err != nil {
				return dto, err
			}
		}

		dto.Profiles = append(dto.Profiles, row)
	}
	return dto, nil
}
