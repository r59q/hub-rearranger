package api

import (
	"errors"
	"net/http"

	"github.com/r59q/hub-rearranger/services/identity/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func (h *Handler) failure(w http.ResponseWriter, _ *http.Request, err error) {
	status, code, message := 503, contract.IdentityUnavailable, "GitHub sign-in is unavailable. Check identity service configuration or try again."
	switch {
	case errors.Is(err, domain.ErrBootstrapStale):
		status, code, message = 409, contract.BootstrapStale, "The default branch or bootstrap files changed. Review a fresh preview before creating the PR."
	case errors.Is(err, domain.ErrBootstrapConflict):
		status, code, message = 409, contract.BootstrapConflict, "The bootstrap branch or PR conflicts with the reviewed changes. Inspect its GitHub state before retrying; existing refs are never overwritten."
	case errors.Is(err, domain.ErrBootstrapIncomplete):
		status, code, message = 409, contract.BootstrapIncomplete, "Bootstrap publication is incomplete. Review GitHub for the dedicated branch and PR, then retry the same reviewed request to reconcile them."
	case errors.Is(err, domain.ErrInvalid):
		status, code, message = 400, contract.InvalidRequest, "Use a valid repository, action, and request body."
	case errors.Is(err, domain.ErrForgery):
		status, code, message = 403, contract.ForgeryRejected, "The request could not be verified. Reload the account page and try again."
	case errors.Is(err, domain.ErrReconnect):
		h.clearCookie(w, "session")
		status, code, message = 401, contract.ReconnectRequired, "Your GitHub connection expired or was revoked. Sign in again."
	case errors.Is(err, domain.ErrForbidden):
		status, code, message = 403, contract.Forbidden, "Check your repository role, GitHub App installation access, and the permissions required by this action."
	}

	writeJSON(w, status, contract.Error{Code: code, Message: message})
}
