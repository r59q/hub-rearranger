package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/r59q/hub-rearranger/services/repositories/internal/domain"
)

const maxRequestBytes = 1 << 20

type repositoryUseCases interface {
	ListAvailable(context.Context) ([]domain.Repository, error)
	ListSelected(context.Context) ([]domain.Repository, error)
	ReplaceSelection(context.Context, []int64) ([]domain.Repository, error)
}

type Handler struct {
	service repositoryUseCases
	logger  *slog.Logger
}

type repositoryDTO struct {
	ID            int64  `json:"id"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	Selected      bool   `json:"selected"`
}

type repositoryListDTO struct {
	Repositories []repositoryDTO `json:"repositories"`
}

type replaceSelectionRequest struct {
	RepositoryIDs []int64 `json:"repository_ids"`
}

type errorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewHandler(service repositoryUseCases, logger *slog.Logger) http.Handler {
	handler := &Handler{service: service, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.health)
	mux.HandleFunc("GET /v1/repositories", handler.listSelected)
	mux.HandleFunc("GET /v1/repositories/available", handler.listAvailable)
	mux.HandleFunc("PUT /v1/repository-selection", handler.replaceSelection)
	return mux
}

func (h *Handler) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listSelected(response http.ResponseWriter, request *http.Request) {
	repositories, err := h.service.ListSelected(request.Context())
	if err != nil {
		h.writeServiceError(response, request, err)
		return
	}
	writeJSON(response, http.StatusOK, toRepositoryListDTO(repositories))
}

func (h *Handler) listAvailable(response http.ResponseWriter, request *http.Request) {
	repositories, err := h.service.ListAvailable(request.Context())
	if err != nil {
		h.writeServiceError(response, request, err)
		return
	}
	writeJSON(response, http.StatusOK, toRepositoryListDTO(repositories))
}

func (h *Handler) replaceSelection(response http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(response, request.Body, maxRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input replaceSelectionRequest
	if err := decoder.Decode(&input); err != nil {
		writeJSON(response, http.StatusBadRequest, errorDTO{Code: "invalid_request", Message: "Request body must contain repository_ids as an array of numbers."})
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(response, http.StatusBadRequest, errorDTO{Code: "invalid_request", Message: "Request body must contain one JSON object."})
		return
	}
	if input.RepositoryIDs == nil {
		writeJSON(response, http.StatusBadRequest, errorDTO{Code: "invalid_request", Message: "repository_ids is required."})
		return
	}
	if len(input.RepositoryIDs) > 1000 {
		writeJSON(response, http.StatusBadRequest, errorDTO{Code: "invalid_request", Message: "At most 1000 repositories can be selected."})
		return
	}
	if hasDuplicate(input.RepositoryIDs) {
		writeJSON(response, http.StatusBadRequest, errorDTO{Code: "invalid_request", Message: "repository_ids must not contain duplicates."})
		return
	}

	repositories, err := h.service.ReplaceSelection(request.Context(), input.RepositoryIDs)
	if err != nil {
		h.writeServiceError(response, request, err)
		return
	}
	writeJSON(response, http.StatusOK, toRepositoryListDTO(repositories))
}

func hasDuplicate(ids []int64) bool {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			return true
		}
		seen[id] = struct{}{}
	}
	return false
}

func (h *Handler) writeServiceError(response http.ResponseWriter, request *http.Request, err error) {
	var unknown *domain.UnknownRepositoriesError
	switch {
	case errors.As(err, &unknown):
		writeJSON(response, http.StatusUnprocessableEntity, errorDTO{
			Code:    "repositories_unavailable",
			Message: fmt.Sprintf("Some selected repositories are no longer available: %v.", unknown.IDs),
		})
	case errors.Is(err, domain.ErrCatalogUnavailable):
		h.logger.Warn("GitHub repository catalog unavailable", "method", request.Method, "path", request.URL.Path, "error", err)
		writeJSON(response, http.StatusBadGateway, errorDTO{
			Code:    "github_unavailable",
			Message: "GitHub repositories are temporarily unavailable. Check the service credentials and try again.",
		})
	default:
		h.logger.Error("repository request failed", "method", request.Method, "path", request.URL.Path, "error", err)
		writeJSON(response, http.StatusInternalServerError, errorDTO{
			Code:    "internal_error",
			Message: "The repository request could not be completed.",
		})
	}
}

func toRepositoryListDTO(repositories []domain.Repository) repositoryListDTO {
	items := make([]repositoryDTO, 0, len(repositories))
	for _, repository := range repositories {
		items = append(items, repositoryDTO{
			ID: repository.ID, Owner: repository.Owner, Name: repository.Name,
			FullName: repository.FullName, HTMLURL: repository.HTMLURL,
			Description: repository.Description, Private: repository.Private,
			DefaultBranch: repository.DefaultBranch, Selected: repository.Selected,
		})
	}
	return repositoryListDTO{Repositories: items}
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
