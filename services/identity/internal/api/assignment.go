package api

import (
	"net/http"

	"github.com/r59q/hub-rearranger/services/identity/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func (h *Handler) ReviewIssueAssignment(w http.ResponseWriter, r *http.Request, owner, repo string, number int, _ contract.ReviewIssueAssignmentParams) {
	var body contract.AssignmentReviewRequest
	if err := readJSON(w, r, &body); err != nil {
		h.failure(w, r, err)
		return
	}
	id := h.cookie(r, "session")
	review, err := h.service.ReviewAssignment(r.Context(), id, body.Csrf, domain.Repository{Owner: owner, Name: repo}, int64(number), body.ProfileRevision)
	if err != nil {
		h.failure(w, r, err)
		return
	}
	writeJSON(w, 200, contract.AssignmentReview{ReviewToken: review.Token, Repository: review.Plan.Repository, Number: review.Plan.Number, Title: review.Plan.Title, ProfileRevision: review.Plan.Revision, Command: review.Plan.Command, User: contract.User{Id: review.User.ID, Login: review.User.Login}, ExpiresAt: review.Expires})
}

func (h *Handler) CreateIssueAssignment(w http.ResponseWriter, r *http.Request, owner, repo string, number int, _ contract.CreateIssueAssignmentParams) {
	var body contract.AssignmentRequest
	if err := readJSON(w, r, &body); err != nil {
		h.failure(w, r, err)
		return
	}
	id := h.cookie(r, "session")
	result, err := h.service.CreateAssignment(r.Context(), id, body.Csrf, domain.Repository{Owner: owner, Name: repo}, int64(number), body.ReviewToken)
	if err != nil {
		h.failure(w, r, err)
		return
	}
	writeJSON(w, 200, contract.AssignmentResult{Repository: result.Repository, Number: result.Number, CommentId: result.CommentID, CommentUrl: result.CommentURL})
}
