package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/r59q/hub-rearranger/services/repositories/internal/domain"
)

type fakeRepositoryService struct {
	available  []domain.Repository
	selected   []domain.Repository
	replaceErr error
	replaced   []int64
	listErr    error
}

func (f *fakeRepositoryService) ListAvailable(context.Context) ([]domain.Repository, error) {
	return f.available, f.listErr
}

func (f *fakeRepositoryService) ListSelected(context.Context) ([]domain.Repository, error) {
	return f.selected, f.listErr
}

func (f *fakeRepositoryService) ReplaceSelection(_ context.Context, ids []int64) ([]domain.Repository, error) {
	f.replaced = append([]int64(nil), ids...)
	return f.selected, f.replaceErr
}

func testHandler(service repositoryUseCases) http.Handler {
	return NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestListAvailableReturnsContractShape(t *testing.T) {
	// Arrange.
	handler := testHandler(&fakeRepositoryService{available: []domain.Repository{{
		ID: 1, Owner: "octo", Name: "demo", FullName: "octo/demo",
		HTMLURL: "https://github.com/octo/demo", DefaultBranch: "main", Selected: true,
	}}})
	request := httptest.NewRequest(http.MethodGet, "/v1/repositories/available", nil)
	response := httptest.NewRecorder()

	// Act.
	handler.ServeHTTP(response, request)

	// Assert.
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body repositoryListDTO
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Repositories) != 1 || body.Repositories[0].FullName != "octo/demo" || !body.Repositories[0].Selected {
		t.Fatalf("body = %#v", body)
	}
}

func TestReplaceSelectionPassesIDsToUseCase(t *testing.T) {
	// Arrange.
	service := &fakeRepositoryService{}
	handler := testHandler(service)
	request := httptest.NewRequest(http.MethodPut, "/v1/repository-selection", strings.NewReader(`{"repository_ids":[3,5]}`))
	response := httptest.NewRecorder()

	// Act.
	handler.ServeHTTP(response, request)

	// Assert.
	if response.Code != http.StatusOK || len(service.replaced) != 2 || service.replaced[0] != 3 || service.replaced[1] != 5 {
		t.Fatalf("status = %d, replaced = %v", response.Code, service.replaced)
	}
}

func TestReplaceSelectionMapsUnknownRepositories(t *testing.T) {
	// Arrange.
	service := &fakeRepositoryService{replaceErr: &domain.UnknownRepositoriesError{IDs: []int64{9}}}
	handler := testHandler(service)
	request := httptest.NewRequest(http.MethodPut, "/v1/repository-selection", strings.NewReader(`{"repository_ids":[9]}`))
	response := httptest.NewRecorder()

	// Act.
	handler.ServeHTTP(response, request)

	// Assert.
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestReplaceSelectionRejectsDuplicateIDs(t *testing.T) {
	// Arrange.
	service := &fakeRepositoryService{}
	handler := testHandler(service)
	request := httptest.NewRequest(http.MethodPut, "/v1/repository-selection", strings.NewReader(`{"repository_ids":[3,3]}`))
	response := httptest.NewRecorder()

	// Act.
	handler.ServeHTTP(response, request)

	// Assert.
	if response.Code != http.StatusBadRequest || service.replaced != nil {
		t.Fatalf("status = %d, replaced = %v", response.Code, service.replaced)
	}
}

func TestListMapsCatalogFailureWithoutLeakingDetails(t *testing.T) {
	// Arrange.
	service := &fakeRepositoryService{listErr: errors.Join(domain.ErrCatalogUnavailable, errors.New("secret upstream detail"))}
	handler := testHandler(service)
	request := httptest.NewRequest(http.MethodGet, "/v1/repositories", nil)
	response := httptest.NewRecorder()

	// Act.
	handler.ServeHTTP(response, request)

	// Assert.
	if response.Code != http.StatusBadGateway || strings.Contains(response.Body.String(), "secret upstream detail") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
