// Package event extracts only the issue-comment identity needed by intake.
package event

import (
	"strconv"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/ops/agent-intake/internal/domain"
)

func Parse(payload []byte, environment map[string]string) (domain.Event, error) {
	if len(payload) > 2*1024*1024 || environment["GITHUB_EVENT_NAME"] != "issue_comment" || environment["GITHUB_SERVER_URL"] != "https://github.com" {
		return domain.Event{}, domain.Invalid
	}

	value, err := gh.ParseWebHook("issue_comment", payload)
	if err != nil {
		return domain.Event{}, domain.Invalid
	}
	event, ok := value.(*gh.IssueCommentEvent)
	if !ok || event.GetAction() != "created" {
		return domain.Event{}, domain.Invalid
	}

	runID, err := strconv.ParseInt(environment["GITHUB_RUN_ID"], 10, 64)
	if err != nil {
		return domain.Event{}, domain.Invalid
	}
	attempt, err := strconv.Atoi(environment["GITHUB_RUN_ATTEMPT"])
	if err != nil {
		return domain.Event{}, domain.Invalid
	}

	repo, comment := event.GetRepo(), event.GetComment()
	if repo.GetFullName() != environment["GITHUB_REPOSITORY"] || event.GetSender().GetID() != comment.GetUser().GetID() ||
		comment.GetUser().GetType() != "User" || repo.GetOwner().GetLogin()+"/"+repo.GetName() != repo.GetFullName() {
		return domain.Event{}, domain.Invalid
	}

	return domain.Event{Repository: domain.Repository{ID: repo.GetID(), Owner: repo.GetOwner().GetLogin(), Name: repo.GetName()},
		IssueNumber: event.GetIssue().GetNumber(), CommentID: comment.GetID(), Body: comment.GetBody(),
		Requester: domain.User{ID: comment.GetUser().GetID(), Login: comment.GetUser().GetLogin()},
		Actor:     environment["GITHUB_ACTOR"], RerunActor: environment["GITHUB_TRIGGERING_ACTOR"],
		RunID: runID, Attempt: attempt, WorkflowSHA: environment["GITHUB_WORKFLOW_SHA"]}, nil
}
