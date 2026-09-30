package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/r59q/hub-rearranger/services/agents/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

type conventionUseCases interface {
	AssignmentConvention(context.Context) (domain.Convention, error)
}

type Handler struct {
	service conventionUseCases
	logger  *slog.Logger
}

var _ contract.ServerInterface = (*Handler)(nil)

func NewHandler(service conventionUseCases, logger *slog.Logger) http.Handler {
	handler := &Handler{service: service, logger: logger}
	mux := http.NewServeMux()
	contract.HandlerFromMux(handler, mux)
	mux.HandleFunc("/health", methodNotAllowed)
	mux.HandleFunc("/v1/assignment-convention", methodNotAllowed)
	mux.HandleFunc("/", func(response http.ResponseWriter, _ *http.Request) {
		writeJSON(response, http.StatusNotFound, contract.Error{
			Code: contract.ErrorCodeNotFound, Message: "The Agents endpoint was not found.",
		})
	})
	return mux
}

func (h *Handler) GetHealth(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, contract.Health{Status: contract.Ok})
}

func (h *Handler) HeadHealth(response http.ResponseWriter, _ *http.Request) {
	response.WriteHeader(http.StatusOK)
}

func (h *Handler) GetAssignmentConvention(response http.ResponseWriter, request *http.Request) {
	convention, err := h.service.AssignmentConvention(request.Context())
	if err != nil {
		h.logFailure(request)
		writeJSON(response, http.StatusInternalServerError, contract.Error{
			Code:    contract.ErrorCodeInternalError,
			Message: "The agent assignment convention could not be loaded. Try again.",
		})
		return
	}
	writeJSON(response, http.StatusOK, toConventionDTO(convention))
}

func (h *Handler) HeadAssignmentConvention(response http.ResponseWriter, request *http.Request) {
	if _, err := h.service.AssignmentConvention(request.Context()); err != nil {
		h.logFailure(request)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}
	response.WriteHeader(http.StatusOK)
}

func (h *Handler) logFailure(request *http.Request) {
	// Errors from future repository integrations may contain credentials or source
	// text. Log only a fixed classification and transport context here.
	h.logger.Error("agents convention request failed", "method", request.Method,
		"operation", "getAssignmentConvention", "code", "internal_error")
}

func methodNotAllowed(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Allow", "GET, HEAD")
	writeJSON(response, http.StatusMethodNotAllowed, contract.Error{
		Code: contract.ErrorCodeMethodNotAllowed, Message: "Use GET or HEAD for this endpoint.",
	})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
