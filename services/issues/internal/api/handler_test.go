package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/r59q/hub-rearranger/services/issues/internal/domain"
)

type fakeIssueService struct {
	page         domain.Page
	err          error
	repositories []domain.Repository
	pageNumber   int
	perPage      int
}

func (f *fakeIssueService) ListRecent(_ context.Context, repositories []domain.Repository, page, perPage int) (domain.Page, error) {
	f.repositories = repositories
	f.pageNumber = page
	f.perPage = perPage
	return f.page, f.err
}

func testHandler(service issueUseCases) http.Handler {
	return NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestListIssuesReturnsContractShape(t *testing.T) {
	// Arrange.
	service := &fakeIssueService{page: domain.Page{Issues: []domain.Issue{{
		ID: 1, Number: 12, Repository: "octo/demo", Title: "A bug", State: "open",
		HTMLURL: "https://github.com/octo/demo/issues/12", UpdatedAt: time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC),
		Labels: []domain.Label{{Name: "bug", Color: "d73a4a"}}, Subtasks: []domain.RelatedIssue{},
		LinkedIssues: []domain.RelatedIssue{}, LinkedPullRequests: []domain.RelatedIssue{}, DetailsAvailable: true,
	}}, Page: 2, PerPage: 6, HasNext: true}}
	handler := testHandler(service)
	request := httptest.NewRequest(http.MethodGet, "/v1/issues?repository=octo%2Fdemo&page=2&per_page=6", nil)
	response := httptest.NewRecorder()

	// Act.
	handler.ServeHTTP(response, request)

	// Assert.
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body issuePageDTO
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Page != 2 || body.Issues[0].UpdatedAt != "2026-09-27T10:00:00Z" || !body.Issues[0].DetailsAvailable {
		t.Fatalf("body = %#v", body)
	}
	if !reflect.DeepEqual(service.repositories, []domain.Repository{{Owner: "octo", Name: "demo"}}) || service.pageNumber != 2 || service.perPage != 6 {
		t.Fatalf("arguments = %#v, %d, %d", service.repositories, service.pageNumber, service.perPage)
	}
}

func TestListIssuesRejectsInvalidRepository(t *testing.T) {
	// Arrange.
	service := &fakeIssueService{}
	handler := testHandler(service)
	request := httptest.NewRequest(http.MethodGet, "/v1/issues?repository=not-a-repository", nil)
	response := httptest.NewRecorder()

	// Act.
	handler.ServeHTTP(response, request)

	// Assert.
	if response.Code != http.StatusBadRequest || service.repositories != nil {
		t.Fatalf("status = %d, repositories = %#v", response.Code, service.repositories)
	}
}

func TestListIssuesMapsCatalogFailureWithoutLeakingDetails(t *testing.T) {
	// Arrange.
	service := &fakeIssueService{err: errors.Join(domain.ErrIssuesUnavailable, errors.New("secret upstream detail"))}
	handler := testHandler(service)
	request := httptest.NewRequest(http.MethodGet, "/v1/issues?repository=octo%2Fdemo", nil)
	response := httptest.NewRecorder()

	// Act.
	handler.ServeHTTP(response, request)

	// Assert.
	if response.Code != http.StatusBadGateway || strings.Contains(response.Body.String(), "secret upstream detail") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
