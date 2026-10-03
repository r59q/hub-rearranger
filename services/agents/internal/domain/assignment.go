package domain

import (
	"context"
	"fmt"
	"time"
)

type AssignmentCheck struct{ URL, Status, Conclusion, Summary string }

type AssignmentProposal struct {
	Number                                 int64
	URL, State, Branch, BranchURL, HeadSHA string
	Draft                                  bool
	Check                                  *AssignmentCheck
}

type AssignmentRun struct {
	ID                      int64
	Attempt                 int64
	URL, Status, Conclusion string
}

type AssignmentRequest struct {
	IssueNumber                                     int64
	CommentID, RequesterID                          int64
	URL, Requester, ProfileRevision, State, Summary string
	CreatedAt                                       time.Time
	Edited                                          bool
	Run                                             *AssignmentRun
	Proposal                                        *AssignmentProposal
}

type IssueAssignment struct {
	Repository                                                     string
	RepositoryID                                                   int64
	Number                                                         int64
	Title, Body, URL, IssueState, ProfileRevision, Reason, Command string
	Assignable                                                     bool
	IssueLocked                                                    bool
	LastCommentID                                                  int64
	Requests                                                       []AssignmentRequest
}

type AssignmentReader interface {
	ReadAssignment(context.Context, Repository, int64) (IssueAssignment, error)
}

func (s *Service) WithAssignments(reader AssignmentReader) *Service {
	s.assignments = reader
	return s
}

func (s *Service) IssueAssignment(ctx context.Context, repo Repository, number int64) (IssueAssignment, error) {
	if s.assignments == nil || number < 1 || number > 2147483647 {
		return IssueAssignment{}, ErrRepositoryUnavailable
	}
	issue, err := s.assignments.ReadAssignment(ctx, repo, number)
	if err != nil {
		return issue, err
	}
	catalog, err := s.RepositoryProfiles(ctx, repo)
	if err != nil {
		return issue, err
	}
	issue.ProfileRevision = catalog.Revision
	issue.Command = fmt.Sprintf("/agent assign codex-thorough@%s authority=branch-draft-pr", catalog.Revision)
	issue.Reason = "Configure and enable codex-thorough on the default branch before assigning."
	for _, profile := range catalog.Profiles {
		if profile.ID == "codex-thorough" && profile.Configuration["enabled"] == true && catalog.State == "valid" {
			issue.Assignable = issue.IssueState == "open" && !issue.IssueLocked
			issue.Reason = "Intake independently checks current roles, profile policy and runner readiness."
		}
	}
	if issue.IssueLocked {
		issue.Reason = "Unlock the GitHub issue before assigning."
	}
	if issue.IssueState != "open" {
		issue.Reason = "Reopen the GitHub issue before assigning."
	}
	if len(issue.Requests) > 0 {
		issue.Assignable = false
		issue.Reason = "Review the existing request and its original workflow before making another assignment on GitHub. A new comment creates a new assignment."
	}
	return issue, nil
}
