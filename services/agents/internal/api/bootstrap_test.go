package api

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

type bootstrapAPI struct {
	*domain.Service
	err   error
	calls int
}

func (s *bootstrapAPI) AssignmentConvention(context.Context) (domain.Convention, error) {
	return domain.Convention{}, nil
}

func (s *bootstrapAPI) RepositoryBootstrap(context.Context, domain.Repository) (domain.BootstrapPreview, error) {
	s.calls++
	return domain.BootstrapPreview{Repository: "octo/demo", DefaultBranch: "main", BaseRevision: strings.Repeat("a", 40), Digest: strings.Repeat("d", 64), State: "ready", Files: []domain.BootstrapPreviewFile{{BootstrapFile: domain.BootstrapFile{Path: "AGENTS.md", Status: "create", Content: "setup", SHA256: strings.Repeat("e", 64)}}}, Diagnostics: []domain.Diagnostic{}, Diff: "safe diff"}, s.err
}

func TestBootstrapReadContractAndSafeFailures(t *testing.T) {
	for _, variant := range []string{"success", "head", "invalid", "method", "unavailable", "forbidden"} {
		t.Run(variant, func(t *testing.T) {
			service := &bootstrapAPI{}
			path, method, status := "/v1/repositories/octo/demo/bootstrap", "GET", 200
			switch variant {
			case "head":
				method = "HEAD"
			case "invalid":
				path = "/v1/repositories/invalid!/demo/bootstrap"
				status = 400
			case "method":
				method = "POST"
				status = 405
			case "unavailable":
				service.err = domain.ErrGitHubUnavailable
				status = 502
			case "forbidden":
				service.err = domain.ErrAccessDenied
				status = 403
			}
			request := httptest.NewRequest(method, path, nil)
			response := httptest.NewRecorder()
			NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil))).ServeHTTP(response, request)
			if response.Code != status {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
			if variant == "head" {
				if response.Body.Len() != 0 {
					t.Fatal("HEAD exposed source")
				}
				assertContractResponse(t, request, response)
			} else {
				assertContractResponse(t, httptest.NewRequest("GET", "/v1/repositories/octo/demo/bootstrap", nil), response)
			}
			if variant == "method" && service.calls != 0 {
				t.Fatal("write reached read domain")
			}
		})
	}
}
