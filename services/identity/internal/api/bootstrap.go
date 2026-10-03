package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/r59q/hub-rearranger/services/identity/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func (h *Handler) CreateBootstrapPullRequest(w http.ResponseWriter, r *http.Request, owner, repo string, _ contract.CreateBootstrapPullRequestParams) {
	var raw map[string]json.RawMessage
	if err := readJSON(w, r, &raw); err != nil {
		h.failure(w, r, err)
		return
	}
	encoded, _ := json.Marshal(raw)
	var body contract.BootstrapRequest
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF {
		h.failure(w, r, domain.ErrInvalid)
		return
	}
	review := domain.BootstrapReview{BaseRevision: body.BaseRevision, Digest: body.Digest}
	if body.ProfileDraft != nil {
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw["profile_draft"], &fields) != nil || len(fields) != 5 {
			h.failure(w, r, domain.ErrInvalid)
			return
		}
		for _, key := range []string{"name", "description", "enabled", "context_sources", "review_comments"} {
			if len(fields[key]) == 0 || string(fields[key]) == "null" {
				h.failure(w, r, domain.ErrInvalid)
				return
			}
		}
		draft := body.ProfileDraft
		sources := make([]string, len(draft.ContextSources))
		for i, source := range draft.ContextSources {
			sources[i] = string(source)
		}
		review.Draft = &domain.ProfileDraft{Name: draft.Name, Description: draft.Description, Enabled: draft.Enabled, ContextSources: sources, ReviewComments: draft.ReviewComments}
	} else if _, exists := raw["profile_draft"]; exists {
		h.failure(w, r, domain.ErrInvalid)
		return
	}
	result, err := h.service.CreateBootstrap(r.Context(), h.cookie(r, "session"), body.Csrf,
		domain.Repository{Owner: owner, Name: repo}, review)
	if err != nil {
		h.failure(w, r, err)
		return
	}
	writeJSON(w, 200, contract.BootstrapResult{Repository: result.Repository, Branch: result.Branch, HeadSha: result.HeadSHA,
		PullRequestNumber: result.PullRequestNumber, PullRequestUrl: result.PullRequestURL})
}
