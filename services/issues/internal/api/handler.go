package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/r59q/hub-rearranger/services/issues/internal/domain"
)

const maxRepositories = 100

type issueUseCases interface {
	ListRecent(context.Context, []domain.Repository, int, int) (domain.Page, error)
}

type Handler struct {
	service issueUseCases
	logger  *slog.Logger
}

type labelDTO struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type relatedIssueDTO struct {
	Number     int    `json:"number"`
	Repository string `json:"repository"`
	Title      string `json:"title"`
	State      string `json:"state"`
	HTMLURL    string `json:"html_url"`
}

type issueDTO struct {
	ID                 int64             `json:"id"`
	Number             int               `json:"number"`
	Repository         string            `json:"repository"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	State              string            `json:"state"`
	HTMLURL            string            `json:"html_url"`
	UpdatedAt          string            `json:"updated_at"`
	Labels             []labelDTO        `json:"labels"`
	Subtasks           []relatedIssueDTO `json:"subtasks"`
	LinkedIssues       []relatedIssueDTO `json:"linked_issues"`
	LinkedPullRequests []relatedIssueDTO `json:"linked_pull_requests"`
	DetailsAvailable   bool              `json:"details_available"`
}

type issuePageDTO struct {
	Issues                  []issueDTO `json:"issues"`
	Page                    int        `json:"page"`
	PerPage                 int        `json:"per_page"`
	HasNext                 bool       `json:"has_next"`
	UnavailableRepositories []string   `json:"unavailable_repositories"`
	IncompleteDetails       int        `json:"incomplete_details"`
}

type errorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewHandler(service issueUseCases, logger *slog.Logger) http.Handler {
	handler := &Handler{service: service, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.health)
	mux.HandleFunc("GET /v1/issues", handler.listIssues)
	return mux
}

func (h *Handler) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listIssues(response http.ResponseWriter, request *http.Request) {
	repositories, valid := parseRepositories(request.URL.Query()["repository"])
	if !valid {
		writeJSON(response, http.StatusBadRequest, errorDTO{Code: "invalid_request", Message: "Each repository must use the owner/name format, with at most 100 repositories."})
		return
	}
	page, valid := positiveInt(request.URL.Query().Get("page"), 1, 1000)
	if !valid {
		writeJSON(response, http.StatusBadRequest, errorDTO{Code: "invalid_request", Message: "page must be an integer from 1 to 1000."})
		return
	}
	perPage, valid := positiveInt(request.URL.Query().Get("per_page"), 6, 24)
	if !valid {
		writeJSON(response, http.StatusBadRequest, errorDTO{Code: "invalid_request", Message: "per_page must be an integer from 1 to 24."})
		return
	}

	result, err := h.service.ListRecent(request.Context(), repositories, page, perPage)
	if err != nil {
		h.writeServiceError(response, request, err)
		return
	}
	if len(result.UnavailableRepositories) > 0 {
		h.logger.Warn("some repository issues unavailable", "count", len(result.UnavailableRepositories))
	}
	if result.IncompleteDetails > 0 {
		h.logger.Warn("some issue relationship details unavailable", "count", result.IncompleteDetails)
	}
	writeJSON(response, http.StatusOK, toIssuePageDTO(result))
}

func parseRepositories(values []string) ([]domain.Repository, bool) {
	if len(values) > maxRepositories {
		return nil, false
	}
	repositories := make([]domain.Repository, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		parts := strings.Split(value, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || len(parts[0]) > 39 || len(parts[1]) > 100 {
			return nil, false
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		repositories = append(repositories, domain.Repository{Owner: parts[0], Name: parts[1]})
	}
	return repositories, true
}

func positiveInt(value string, fallback, maximum int) (int, bool) {
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed > 0 && parsed <= maximum
}

func (h *Handler) writeServiceError(response http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, domain.ErrIssuesUnavailable) {
		h.logger.Warn("GitHub issues unavailable", "method", request.Method, "path", request.URL.Path, "error", err)
		writeJSON(response, http.StatusBadGateway, errorDTO{Code: "github_unavailable", Message: "GitHub issues are temporarily unavailable. Check the service credentials and try again."})
		return
	}
	h.logger.Error("issue request failed", "method", request.Method, "path", request.URL.Path, "error", err)
	writeJSON(response, http.StatusInternalServerError, errorDTO{Code: "internal_error", Message: "The issue request could not be completed."})
}

func toIssuePageDTO(page domain.Page) issuePageDTO {
	issues := make([]issueDTO, 0, len(page.Issues))
	for _, issue := range page.Issues {
		labels := make([]labelDTO, 0, len(issue.Labels))
		for _, label := range issue.Labels {
			labels = append(labels, labelDTO{Name: label.Name, Color: label.Color})
		}
		issues = append(issues, issueDTO{
			ID: issue.ID, Number: issue.Number, Repository: issue.Repository, Title: issue.Title,
			Description: issue.Description, State: issue.State, HTMLURL: issue.HTMLURL,
			UpdatedAt: issue.UpdatedAt.UTC().Format(time.RFC3339), Labels: labels,
			Subtasks: relatedDTOs(issue.Subtasks), LinkedIssues: relatedDTOs(issue.LinkedIssues),
			LinkedPullRequests: relatedDTOs(issue.LinkedPullRequests), DetailsAvailable: issue.DetailsAvailable,
		})
	}
	return issuePageDTO{
		Issues: issues, Page: page.Page, PerPage: page.PerPage, HasNext: page.HasNext,
		UnavailableRepositories: nonNilStrings(page.UnavailableRepositories),
		IncompleteDetails:       page.IncompleteDetails,
	}
}

func relatedDTOs(issues []domain.RelatedIssue) []relatedIssueDTO {
	result := make([]relatedIssueDTO, 0, len(issues))
	for _, issue := range issues {
		result = append(result, relatedIssueDTO{
			Number: issue.Number, Repository: issue.Repository, Title: issue.Title,
			State: issue.State, HTMLURL: issue.HTMLURL,
		})
	}
	return result
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
