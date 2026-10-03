package domain

import (
	"context"
	"errors"
	"sync"
	"time"
)

type fakeStore struct {
	mu       sync.Mutex
	intents  map[string]Intent
	sessions map[string]Session
}

func newStore() *fakeStore {
	return &fakeStore{intents: map[string]Intent{}, sessions: map[string]Session{}}
}

func (f *fakeStore) PutIntent(_ context.Context, id string, value Intent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.intents[id] = value
	return nil
}

func (f *fakeStore) TakeIntent(_ context.Context, id, binding string) (Intent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.intents[id]
	delete(f.intents, id)
	if !ok || value.Binding != binding {
		return Intent{}, ErrForgery
	}
	return value, nil
}

func (f *fakeStore) PutSession(_ context.Context, id string, value Session) error {
	f.sessions[id] = value
	return nil
}

func (f *fakeStore) WithSession(_ context.Context, id string, operation func(*Session) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.sessions[id]
	if !ok {
		return ErrReconnect
	}
	err := operation(&value)
	if errors.Is(err, ErrReconnect) || value.Credentials.Access == "" {
		delete(f.sessions, id)
	} else {
		f.sessions[id] = value
	}
	return err
}

type fakeProvider struct {
	state, verifier                                   string
	user                                              User
	access                                            Access
	userError, accessError, refreshError, revokeError error
	exchanges, refreshes, checks, revokes             int
	refreshed                                         Credentials
	revokedToken                                      string
}

func (f *fakeProvider) AuthorizationURL(state, verifier string) string {
	f.state, f.verifier = state, verifier
	return "https://github.com/login/oauth/authorize"
}

func (f *fakeProvider) Exchange(context.Context, string, string) (Credentials, error) {
	f.exchanges++
	return Credentials{Access: "synthetic-user-token"}, nil
}

func (f *fakeProvider) Refresh(context.Context, Credentials) (Credentials, error) {
	f.refreshes++
	return f.refreshed, f.refreshError
}

func (f *fakeProvider) User(context.Context, string) (User, error) {
	f.checks++
	return f.user, f.userError
}

func (f *fakeProvider) RepositoryAccess(context.Context, string, User, Repository) (Access, error) {
	return f.access, f.accessError
}

func (f *fakeProvider) Revoke(_ context.Context, token string) error {
	f.revokes++
	f.revokedToken = token
	return f.revokeError
}

func setup() (*Service, *fakeStore, *fakeProvider, string, Session) {
	store := newStore()
	provider := &fakeProvider{user: User{ID: 7, Login: "octocat"}, access: Access{RepositoryID: 42, FullName: "octo/demo", Role: "maintain", IssuesWrite: true, ContentsWrite: true, PullRequestsWrite: true, WorkflowsWrite: true}}
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	service := NewService(provider, store, func() time.Time { return now })
	id := randomSecret()
	session := Session{User: provider.user, Credentials: Credentials{Access: "synthetic-user-token"}, CSRF: randomSecret(), Expires: now.Add(SessionLifetime)}
	store.sessions[Hash(id)] = session
	return service, store, provider, id, session
}
