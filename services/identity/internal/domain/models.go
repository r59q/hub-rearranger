// Package domain owns Hub sign-in, credential lifecycle, and repository write policy.
package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUnavailable = errors.New("identity unavailable")
	ErrReconnect   = errors.New("sign in again")
	ErrForbidden   = errors.New("repository access denied")
	ErrForgery     = errors.New("request could not be verified")
	ErrInvalid     = errors.New("invalid request")
)

type User struct {
	ID    int64
	Login string
}

// Credentials never cross the public API boundary. Only the identity service stores them.
type Credentials struct {
	Access         string
	Refresh        string
	AccessExpires  time.Time
	RefreshExpires time.Time
}

type Session struct {
	User        User
	Credentials Credentials
	CSRF        string
	Expires     time.Time
}

type Intent struct {
	Binding  string
	Verifier string
	Expires  time.Time
}

type Action string

const (
	AssignComment        Action = "assign_comment"
	BootstrapPullRequest Action = "bootstrap_pull_request"
)

type Repository struct {
	Owner string
	Name  string
}

type Access struct {
	RepositoryID      int64
	FullName          string
	Role              string
	IssuesWrite       bool
	ContentsWrite     bool
	PullRequestsWrite bool
	WorkflowsWrite    bool
}

type Authorization struct {
	User         User
	RepositoryID int64
	Repository   string
	Action       Action
	CheckedAt    time.Time
}

type Provider interface {
	AuthorizationURL(state, verifier string) string
	Exchange(context.Context, string, string) (Credentials, error)
	Refresh(context.Context, Credentials) (Credentials, error)
	User(context.Context, string) (User, error)
	RepositoryAccess(context.Context, string, User, Repository) (Access, error)
	Revoke(context.Context, string) error
}

type Store interface {
	PutIntent(context.Context, string, Intent) error
	// TakeIntent atomically consumes a browser-bound intent, including expired intents.
	TakeIntent(context.Context, string, string) (Intent, error)
	PutSession(context.Context, string, Session) error
	// WithSession serializes refresh, revocation, and authorized operations. ErrReconnect
	// deletes the record. An empty access token also deletes it after successful sign-out.
	WithSession(context.Context, string, func(*Session) error) error
}
