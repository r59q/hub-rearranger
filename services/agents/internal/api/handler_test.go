package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

func TestReadEndpointsHonorOpenAPIWithoutAuthentication(t *testing.T) {
	for _, path := range []string{"/health", "/v1/assignment-convention"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(method+path, func(t *testing.T) {
				// Arrange. No token or repository access is involved.
				handler := NewHandler(domain.NewService(nil), slog.New(slog.NewTextHandler(io.Discard, nil)))
				request := httptest.NewRequest(method, path, nil)
				response := httptest.NewRecorder()

				// Act.
				handler.ServeHTTP(response, request)

				// Assert.
				if response.Code != http.StatusOK {
					t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
				}
				assertContractResponse(t, request, response)
				if method == http.MethodHead && response.Body.Len() != 0 {
					t.Fatal("HEAD returned a response body")
				}
			})
		}
	}
}

type failingService struct {
	ctx context.Context
}

func (s *failingService) AssignmentConvention(ctx context.Context) (domain.Convention, error) {
	s.ctx = ctx
	return domain.Convention{}, errors.New("private upstream detail")
}

func TestConventionFailureIsContractValidAndSafeInResponseAndLogs(t *testing.T) {
	// Arrange.
	service := &failingService{}
	var logs bytes.Buffer
	handler := NewHandler(service, slog.New(slog.NewTextHandler(&logs, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/v1/assignment-convention", nil).WithContext(ctx)
	response := httptest.NewRecorder()

	// Act.
	handler.ServeHTTP(response, request)

	// Assert.
	if response.Code != http.StatusInternalServerError || service.ctx != ctx {
		t.Fatalf("status = %d, request context was passed = %t", response.Code, service.ctx == ctx)
	}
	if strings.Contains(response.Body.String()+logs.String(), "private upstream detail") {
		t.Fatal("upstream error was exposed")
	}
	assertContractResponse(t, request, response)
}

func TestWriteMethodsAreRejectedWithoutCallingTheDomain(t *testing.T) {
	for _, path := range []string{"/health", "/v1/assignment-convention"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
			t.Run(method+path, func(t *testing.T) {
				// Arrange.
				service := &failingService{}
				handler := NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
				request := httptest.NewRequest(method, path, nil)
				response := httptest.NewRecorder()

				// Act.
				handler.ServeHTTP(response, request)

				// Assert.
				if response.Code != http.StatusMethodNotAllowed || service.ctx != nil || response.Header().Get("Allow") != "GET, HEAD" {
					t.Fatalf("status = %d, Allow = %q, domain called = %t", response.Code, response.Header().Get("Allow"), service.ctx != nil)
				}
				// OpenAPI describes the 405 response on the endpoint's GET operation.
				assertContractResponse(t, httptest.NewRequest(http.MethodGet, path, nil), response)
			})
		}
	}
}

func TestUnknownEndpointsAreStructuredAndDoNotEchoRequestData(t *testing.T) {
	// Arrange.
	handler := NewHandler(domain.NewService(nil), slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/private-sentinel", nil)
	response := httptest.NewRecorder()

	// Act.
	handler.ServeHTTP(response, request)

	// Assert.
	if response.Code != http.StatusNotFound || response.Header().Get("Content-Type") != "application/json" || strings.Contains(response.Body.String(), "private-sentinel") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func assertContractResponse(t *testing.T, request *http.Request, response *httptest.ResponseRecorder) {
	t.Helper()
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("load OpenAPI: %v", err)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI: %v", err)
	}
	// Service addresses are deployment configuration, not a test constraint.
	document.Servers = nil
	router, err := legacy.NewRouter(document)
	if err != nil {
		t.Fatalf("create OpenAPI router: %v", err)
	}
	route, parameters, err := router.FindRoute(request)
	if err != nil {
		t.Fatalf("find OpenAPI route: %v", err)
	}
	input := &openapi3filter.RequestValidationInput{Request: request, PathParams: parameters, Route: route}
	if err := openapi3filter.ValidateRequest(request.Context(), input); err != nil {
		t.Fatalf("request violates OpenAPI: %v", err)
	}
	output := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: input, Status: response.Code, Header: response.Header(),
		Options: &openapi3filter.Options{IncludeResponseStatus: true},
	}
	output.SetBodyBytes(response.Body.Bytes())
	if err := openapi3filter.ValidateResponse(request.Context(), output); err != nil {
		t.Fatalf("response violates OpenAPI: %v", err)
	}
}

func (s *failingService) RepositoryProfiles(ctx context.Context, _ domain.Repository) (domain.ProfileCatalog, error) {
	s.ctx = ctx
	return domain.ProfileCatalog{}, errors.New("private upstream detail")
}
