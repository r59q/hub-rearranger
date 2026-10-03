// Package domain implements trusted assignment intake without transport dependencies.
package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

const WorkflowPath = ".github/workflows/agent-assignment.yml"
const CatalogPath = ".github/agent-profiles.yml"

type Failure string

func (f Failure) Error() string { return string(f) }

const (
	Invalid            Failure = "INVALID_REQUEST"
	Denied             Failure = "AUTHORIZATION_DENIED"
	SourceChanged      Failure = "SOURCE_CHANGED"
	ProfileInvalid     Failure = "PROFILE_INVALID"
	ProfileDisabled    Failure = "PROFILE_DISABLED"
	ProfileChanged     Failure = "PROFILE_POLICY_CHANGED"
	RevisionInvalid    Failure = "REVISION_INVALID"
	Unavailable        Failure = "GITHUB_UNAVAILABLE"
	HistoryUnavailable Failure = "REPLAY_STATE_UNAVAILABLE"
)

type Repository struct {
	ID             int64  `json:"id"`
	Owner          string `json:"owner"`
	Name           string `json:"name"`
	DefaultBranch  string `json:"default_branch"`
	Fork, Archived bool   `json:"-"`
	Private        bool   `json:"-"`
}

type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

type Event struct {
	Repository        Repository
	IssueNumber       int
	CommentID         int64
	Body              string
	Requester         User
	Actor, RerunActor string
	RunID             int64
	Attempt           int
	WorkflowSHA       string
}

type Source struct {
	IssueNumber       int
	CommentID         int64
	Body              string
	Requester         User
	Created, Updated  time.Time
	Open, PullRequest bool
}

type Command struct{ ProfileID, Revision string }

type Run struct {
	ID                          int64
	Attempt                     int
	HeadSHA, Path, Event, Title string
	RepositoryID                int64
}

// PublicationIntent prevents blind retries of non-idempotent comment/check POSTs.
// It is recovery context only; live authorization and artifact verification remain
// required. Started with a missing object means an operator must reconcile.
type PublicationIntent struct {
	ArtifactID     int64  `json:"artifact_id"`
	Attempt        int    `json:"attempt"`
	HeadSHA        string `json:"head_sha"`
	CommentStarted bool   `json:"comment_started"`
	CheckStarted   bool   `json:"check_started"`
}

type Receipt struct {
	CommentID       int64              `json:"-"`
	Publication     *PublicationIntent `json:"publication,omitempty"`
	AssignmentID    string             `json:"assignment_id"`
	RunID           int64              `json:"run_id"`
	RunAttempt      int                `json:"run_attempt"`
	BaseSHA         string             `json:"base_sha"`
	ProfileID       string             `json:"profile_id"`
	ProfileRevision string             `json:"profile_revision"`
	RequesterID     int64              `json:"requester_id"`
}

type Invocation struct {
	ContractVersion  int             `json:"contract_version"`
	AssignmentID     string          `json:"assignment_id"`
	Operation        string          `json:"operation"`
	Repository       Repository      `json:"repository"`
	IssueNumber      int             `json:"issue_number"`
	SourceURL        string          `json:"source_url"`
	RequestCommentID int64           `json:"request_comment_id"`
	RequestURL       string          `json:"request_url"`
	Requester        User            `json:"requester"`
	RequesterRole    string          `json:"requester_role"`
	Authority        string          `json:"authority"`
	ProfileID        string          `json:"profile_id"`
	ProfileRevision  string          `json:"profile_revision"`
	PolicyRevision   string          `json:"policy_revision"`
	Profile          json.RawMessage `json:"profile"`
	BaseSHA          string          `json:"base_sha"`
	RunID            int64           `json:"run_id"`
	RunAttempt       int             `json:"run_attempt"`
	RunURL           string          `json:"run_url"`
	ReceiptCommentID int64           `json:"receipt_comment_id"`
}

type Decision struct {
	Disposition    string      `json:"disposition"`
	CanonicalRunID int64       `json:"canonical_run_id"`
	Invocation     *Invocation `json:"invocation,omitempty"`
}

type RepositoryPort interface {
	Repository(context.Context, Event) (Repository, error)
	Source(context.Context, Repository, Event) (Source, error)
	Role(context.Context, Repository, string) (User, string, error)
	Head(context.Context, Repository) (string, error)
	Ancestor(context.Context, Repository, string, string) (bool, error)
	Catalog(context.Context, Repository, string) ([]byte, error)
	CanonicalRun(context.Context, Repository, Event, Source) (Run, error)
	Receipt(context.Context, Repository, Source, string) (*Receipt, error)
	Accept(context.Context, Repository, Source, Receipt, Invocation) (int64, error)
}

type PolicyPort interface {
	Verify(context.Context, []byte, []byte, string) (json.RawMessage, error)
}

var commandPattern = regexp.MustCompile(`^/agent assign ([a-z][a-z0-9]*(?:-[a-z0-9]+)*)@([0-9a-f]{40}) authority=branch-draft-pr$`)
var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var loginPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,38}$`)
var repoPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,100}$`)

func Parse(body string) (Command, error) {
	match := commandPattern.FindStringSubmatch(body)
	if match == nil || len(match[1]) > 64 {
		return Command{}, Invalid
	}
	return Command{ProfileID: match[1], Revision: match[2]}, nil
}

func ValidSHA(value string) bool { return shaPattern.MatchString(value) }

func AssignmentID(repositoryID, commentID int64) string {
	return fmt.Sprintf("%d:%d", repositoryID, commentID)
}

func RunTitle(repositoryID, commentID int64) string {
	return "Agent assignment " + AssignmentID(repositoryID, commentID)
}

func RepositoryURL(repo Repository) string {
	return "https://github.com/" + repo.Owner + "/" + repo.Name
}
