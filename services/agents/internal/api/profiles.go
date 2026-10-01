package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/r59q/hub-rearranger/services/agents/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

var ownerName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
var repositoryName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)

func (h *Handler) GetRepositoryProfiles(response http.ResponseWriter, request *http.Request, owner, repo string) {
	h.repositoryProfiles(response, request, owner, repo, false)
}
func (h *Handler) HeadRepositoryProfiles(response http.ResponseWriter, request *http.Request, owner, repo string) {
	h.repositoryProfiles(response, request, owner, repo, true)
}

func (h *Handler) repositoryProfiles(response http.ResponseWriter, request *http.Request, owner, repo string, head bool) {
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
	ctx, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	catalog, err := h.service.RepositoryProfiles(ctx, domain.Repository{Owner: owner, Name: repo})
	if err != nil {
		status, failure := profileError(err)
		// Never log upstream errors, source values, or repository identities.
		h.logger.Warn("agents profile read failed", "code", failure.Code)
		respond(status, failure)
		return
	}
	dto, err := toProfilesDTO(catalog)
	if err != nil {
		h.logger.Error("agents profile mapping failed", "code", "internal_error")
		respond(http.StatusInternalServerError, contract.Error{Code: contract.ErrorCodeInternalError, Message: "The repository profiles could not be loaded. Try again."})
		return
	}
	respond(http.StatusOK, dto)
}

func profileError(err error) (int, contract.Error) {
	switch {
	case errors.Is(err, domain.ErrRepositoryUnavailable):
		return http.StatusNotFound, contract.Error{Code: contract.ErrorCodeRepositoryUnavailable, Message: "Check the repository name, default branch, and read-token access, then retry."}
	case errors.Is(err, domain.ErrAccessDenied):
		return http.StatusForbidden, contract.Error{Code: contract.ErrorCodeAccessDenied, Message: "Grant the server-side GitHub token read-only Metadata and Contents access to this repository, then retry."}
	case errors.Is(err, domain.ErrRateLimited):
		return http.StatusTooManyRequests, contract.Error{Code: contract.ErrorCodeRateLimited, Message: "GitHub has rate-limited profile reads. Wait for the rate limit to reset, then retry."}
	case errors.Is(err, domain.ErrGitHubUnavailable), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return http.StatusBadGateway, contract.Error{Code: contract.ErrorCodeGithubUnavailable, Message: "GitHub profiles are temporarily unavailable. Check connectivity and retry."}
	default:
		return http.StatusInternalServerError, contract.Error{Code: contract.ErrorCodeInternalError, Message: "The repository profiles could not be loaded. Try again."}
	}
}

func toProfilesDTO(catalog domain.ProfileCatalog) (contract.RepositoryProfiles, error) {
	result := contract.RepositoryProfiles{Repository: catalog.Repository, CatalogPath: contract.RepositoryProfilesCatalogPath(domain.ProfileCatalogPath),
		DefaultBranch: catalog.DefaultBranch, Revision: catalog.Revision, State: contract.RepositoryProfilesState(catalog.State),
		Profiles: []contract.RepositoryProfile{}, Diagnostics: []contract.ProfileDiagnostic{}}
	for _, profile := range catalog.Profiles {
		// The infrastructure validates the complete configuration before it crosses
		// the domain boundary. Map its JSON-shaped data into generated transport DTOs.
		encoded, err := json.Marshal(profile.Configuration)
		if err != nil {
			return result, err
		}
		var configuration contract.CatalogProfile
		if err := json.Unmarshal(encoded, &configuration); err != nil {
			return result, err
		}
		result.Profiles = append(result.Profiles, contract.RepositoryProfile{Id: profile.ID, Revision: profile.Revision, Configuration: configuration})
	}
	for _, issue := range catalog.Diagnostics {
		result.Diagnostics = append(result.Diagnostics, contract.ProfileDiagnostic{Code: contract.ProfileDiagnosticCode(issue.Code), Path: issue.Path, Message: issue.Message})
	}
	return result, nil
}
