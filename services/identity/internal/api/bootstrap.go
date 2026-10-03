package api

import (
	"net/http"

	"github.com/r59q/hub-rearranger/services/identity/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func (h *Handler) CreateBootstrapPullRequest(w http.ResponseWriter, r *http.Request, owner, repo string, _ contract.CreateBootstrapPullRequestParams) {
	var body contract.BootstrapRequest
	if err := readJSON(w, r, &body); err != nil {
		h.failure(w, r, err)
		return
	}
	result, err := h.service.CreateBootstrap(r.Context(), h.cookie(r, "session"), body.Csrf,
		domain.Repository{Owner: owner, Name: repo}, domain.BootstrapReview{BaseRevision: body.BaseRevision, Digest: body.Digest})
	if err != nil {
		h.failure(w, r, err)
		return
	}
	writeJSON(w, 200, contract.BootstrapResult{Repository: result.Repository, Branch: result.Branch, HeadSha: result.HeadSHA,
		PullRequestNumber: result.PullRequestNumber, PullRequestUrl: result.PullRequestURL})
}
