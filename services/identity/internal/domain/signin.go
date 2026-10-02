package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

const SessionLifetime = 7 * 24 * time.Hour
const IntentLifetime = 10 * time.Minute

type Service struct {
	provider Provider
	store    Store
	now      func() time.Time
}

func NewService(provider Provider, store Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{provider: provider, store: store, now: now}
}

func (s *Service) Enabled() bool { return s.provider != nil && s.store != nil }

func randomSecret() string {
	value := make([]byte, 32)
	// crypto/rand.Read cannot fail on supported Go platforms.
	_, _ = rand.Read(value)
	return base64.RawURLEncoding.EncodeToString(value)
}

func Hash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func (s *Service) Begin(ctx context.Context) (url, binding string, err error) {
	if !s.Enabled() {
		return "", "", ErrUnavailable
	}

	state, binding, verifier := randomSecret(), randomSecret(), randomSecret()
	intent := Intent{Binding: Hash(binding), Verifier: verifier, Expires: s.now().Add(IntentLifetime)}
	if err := s.store.PutIntent(ctx, Hash(state), intent); err != nil {
		return "", "", ErrUnavailable
	}

	return s.provider.AuthorizationURL(state, verifier), binding, nil
}

func (s *Service) Complete(ctx context.Context, state, binding, code, previous string) (string, Session, error) {
	if !s.Enabled() {
		return "", Session{}, ErrUnavailable
	}
	if len(state) != 43 || len(binding) != 43 || code == "" || len(code) > 1024 {
		return "", Session{}, ErrForgery
	}

	intent, err := s.store.TakeIntent(ctx, Hash(state), Hash(binding))
	if err != nil {
		return "", Session{}, err
	}
	if !s.now().Before(intent.Expires) {
		return "", Session{}, ErrForgery
	}

	credentials, err := s.provider.Exchange(ctx, code, intent.Verifier)
	if err != nil {
		return "", Session{}, err
	}

	user, err := s.provider.User(ctx, credentials.Access)
	if err != nil {
		_ = s.provider.Revoke(ctx, credentials.Access)
		return "", Session{}, err
	}
	if user.ID <= 0 || user.Login == "" {
		return "", Session{}, ErrReconnect
	}

	if len(previous) == 43 {
		err := s.store.WithSession(ctx, Hash(previous), func(session *Session) error {
			session.Credentials = Credentials{}
			return nil
		})
		if err != nil && !errors.Is(err, ErrReconnect) {
			_ = s.provider.Revoke(ctx, credentials.Access)
			return "", Session{}, ErrUnavailable
		}
	}

	session := Session{User: user, Credentials: credentials, CSRF: randomSecret(), Expires: s.now().Add(SessionLifetime)}
	id := randomSecret()
	if err := s.store.PutSession(ctx, Hash(id), session); err != nil {
		_ = s.provider.Revoke(ctx, credentials.Access)
		return "", Session{}, ErrUnavailable
	}

	return id, session, nil
}
