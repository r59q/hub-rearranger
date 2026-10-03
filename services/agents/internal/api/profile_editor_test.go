package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/profiles"
)

type editorFixture struct {
	canonical string
	snapshot  domain.BootstrapSnapshot
}

func (f *editorFixture) ReadTemplates(context.Context) (map[string]string, error) {
	return map[string]string{domain.ProfileCatalogPath: f.canonical, "AGENTS.md": domain.BootstrapInstructions}, nil
}

func (f *editorFixture) ReadBootstrap(context.Context, domain.Repository, map[string]string) (domain.BootstrapSnapshot, error) {
	return f.snapshot, nil
}

func (f *editorFixture) Render(context.Context, domain.BootstrapPlan, map[string]string) (string, error) {
	return "reviewed diff", nil
}

func TestProfileEditorContractValidatesStructuredChoicesAndFreshReview(t *testing.T) {
	canonical, err := os.ReadFile("../../../../.github/agent-profiles.yml")
	if err != nil {
		t.Fatal(err)
	}
	validator, err := profiles.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"valid", "missing", "null", "unknown", "type", "schema", "instructions", "oversize", "invalid-repository"} {
		t.Run(variant, func(t *testing.T) {
			fixture := &editorFixture{canonical: string(canonical), snapshot: domain.BootstrapSnapshot{DefaultBranch: "main", Revision: strings.Repeat("a", 40), Files: map[string]string{}, Blobs: map[string]string{}}}
			service := domain.NewService(nil).WithBootstrap(domain.NewBootstrapService(fixture, fixture, validator, fixture))
			handler := NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
			path := "/v1/repositories/octo/demo/profile-editor"
			request := httptest.NewRequest("GET", path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			assertContractResponse(t, request, response)
			var state struct {
				Draft domain.ProfileDraft `json:"draft"`
			}
			if json.Unmarshal(response.Body.Bytes(), &state) != nil || state.Draft.Name == "" {
				t.Fatal("editor defaults unavailable")
			}
			encoded, _ := json.Marshal(state.Draft)
			var body map[string]any
			_ = json.Unmarshal(encoded, &body)
			body["name"] = "Reviewed display name"
			status, previewState := 200, "ready"
			switch variant {
			case "missing":
				delete(body, "enabled")
				status = 400
			case "null":
				body["enabled"] = nil
				status = 400
			case "unknown":
				body["content"] = "private-sentinel"
				status = 400
			case "type":
				body["enabled"] = "false"
				status = 400
			case "schema":
				body["context_sources"] = []string{"repository"}
				previewState = "conflict"
			case "instructions":
				fixture.snapshot.Files["AGENTS.md"] = "<!-- hub-agent-bootstrap:v1:begin -->"
				previewState = "conflict"
			case "oversize":
				body["description"] = strings.Repeat("x", 5000)
				status = 400
			case "invalid-repository":
				path = "/v1/repositories/invalid!/demo/profile-editor"
				status = 400
			}
			encoded, _ = json.Marshal(body)
			request = httptest.NewRequest("POST", path, strings.NewReader(string(encoded)))
			request.Header.Set("Content-Type", "application/json")
			response = httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != status || strings.Contains(response.Body.String(), "private-sentinel") {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
			// Validate the response using a fresh valid request: the handler has
			// consumed its body, and error variants deliberately violate inputs.
			validBody, _ := json.Marshal(state.Draft)
			contractRequest := httptest.NewRequest("POST", "/v1/repositories/octo/demo/profile-editor", strings.NewReader(string(validBody)))
			contractRequest.Header.Set("Content-Type", "application/json")
			assertContractResponse(t, contractRequest, response)
			if status != 200 {
				return
			}
			var first struct{ State, Digest string }
			_ = json.Unmarshal(response.Body.Bytes(), &first)
			if first.State != previewState {
				t.Fatal("unsafe preview state", first.State)
			}
			fixture.snapshot.Revision = strings.Repeat("b", 40)
			request = httptest.NewRequest("POST", path, strings.NewReader(string(encoded)))
			request.Header.Set("Content-Type", "application/json")
			response = httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			var next struct{ Digest string }
			_ = json.Unmarshal(response.Body.Bytes(), &next)
			if next.Digest == first.Digest {
				t.Fatal("profile review ignored changed base")
			}
		})
	}
}
