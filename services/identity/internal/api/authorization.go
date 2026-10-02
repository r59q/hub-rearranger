package api

import (
	"net/http"

	"github.com/r59q/hub-rearranger/services/identity/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func (h *Handler) AuthorizeRepository(w http.ResponseWriter, r *http.Request, owner, repo string, _ contract.AuthorizeRepositoryParams) {
	var body contract.AuthorizationRequest
	if err := readJSON(w, r, &body); err != nil {
		h.failure(w, r, err)
		return
	}

	result, err := h.service.WithAuthorization(r.Context(), h.cookie(r, "session"), body.Csrf, domain.Repository{Owner: owner, Name: repo}, domain.Action(body.Action), nil)
	if err != nil {
		h.failure(w, r, err)
		return
	}

	writeJSON(w, 200, contract.Authorization{User: contract.User{Id: result.User.ID, Login: result.User.Login}, RepositoryId: result.RepositoryID, Repository: result.Repository, Action: contract.Action(result.Action), CheckedAt: result.CheckedAt})
}
