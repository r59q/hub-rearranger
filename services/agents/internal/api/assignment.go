package api

import (
	"context"
	"net/http"
	"time"

	"github.com/r59q/hub-rearranger/services/agents/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

type assignmentUseCases interface {
	IssueAssignment(context.Context, domain.Repository, int64) (domain.IssueAssignment, error)
}

func (h *Handler) GetIssueAssignment(w http.ResponseWriter, r *http.Request, owner, repo string, number int) {
	if !ownerName.MatchString(owner) || !repositoryName.MatchString(repo) || repo == "." || repo == ".." || number < 1 || number > 2147483647 {
		writeJSON(w, 400, contract.Error{Code: contract.ErrorCodeInvalidRequest, Message: "Use a valid repository and issue number."})
		return
	}
	service, ok := h.service.(assignmentUseCases)
	if !ok {
		writeJSON(w, 502, contract.Error{Code: contract.ErrorCodeGithubUnavailable, Message: "Assignment context is unavailable."})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	result, err := service.IssueAssignment(ctx, domain.Repository{Owner: owner, Name: repo}, int64(number))
	if err != nil {
		status, failure := profileError(err)
		writeJSON(w, status, failure)
		return
	}
	writeJSON(w, 200, assignmentDTO(result))
}

func assignmentDTO(value domain.IssueAssignment) contract.IssueAssignment {
	dto := contract.IssueAssignment{Repository: value.Repository, RepositoryId: value.RepositoryID, Number: value.Number, Title: value.Title, Body: value.Body, Url: value.URL, IssueState: contract.IssueAssignmentIssueState(value.IssueState), ProfileRevision: value.ProfileRevision, Assignable: value.Assignable, Reason: value.Reason, Command: value.Command, LastCommentId: value.LastCommentID, Requests: []contract.AssignmentRequest{}}
	for _, request := range value.Requests {
		row := contract.AssignmentRequest{CommentId: request.CommentID, Url: request.URL, RequesterId: request.RequesterID, Requester: request.Requester, ProfileRevision: request.ProfileRevision, CreatedAt: request.CreatedAt, Edited: request.Edited, State: contract.AssignmentRequestState(request.State), Summary: request.Summary}
		if run := request.Run; run != nil {
			row.Run = &contract.AssignmentRun{Id: run.ID, Attempt: run.Attempt, Url: run.URL, Status: run.Status, Conclusion: run.Conclusion}
		}
		if proposal := request.Proposal; proposal != nil {
			row.Proposal = &contract.AssignmentProposal{Number: proposal.Number, Url: proposal.URL, State: proposal.State, Draft: proposal.Draft, Branch: proposal.Branch, BranchUrl: proposal.BranchURL, HeadSha: proposal.HeadSHA}
			if check := proposal.Check; check != nil {
				row.Proposal.Check = &contract.AssignmentCheck{Url: check.URL, Status: check.Status, Conclusion: check.Conclusion, Summary: check.Summary}
			}
		}
		dto.Requests = append(dto.Requests, row)
	}
	return dto
}
