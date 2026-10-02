package domain

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"
)

func (s *Service) current(ctx context.Context, session *Session) error {
	if ctx.Err() != nil {
		return ErrUnavailable
	}
	if !s.now().Before(session.Expires) {
		return ErrReconnect
	}

	credential := session.Credentials
	if !credential.AccessExpires.IsZero() && !s.now().Add(30*time.Second).Before(credential.AccessExpires) {
		if credential.Refresh == "" || !s.now().Before(credential.RefreshExpires) {
			return ErrReconnect
		}
		refreshed, err := s.provider.Refresh(ctx, credential)
		if err != nil {
			return err
		}
		session.Credentials = refreshed
	}

	user, err := s.provider.User(ctx, session.Credentials.Access)
	if err != nil {
		return err
	}
	if user.ID != session.User.ID {
		return ErrReconnect
	}

	session.User = user
	return nil
}

func (s *Service) Session(ctx context.Context, id string) (Session, error) {
	if !s.Enabled() {
		return Session{}, ErrUnavailable
	}
	if len(id) != 43 {
		return Session{}, ErrReconnect
	}

	var result Session
	err := s.store.WithSession(ctx, Hash(id), func(session *Session) error {
		if err := s.current(ctx, session); err != nil {
			return err
		}

		result = *session
		return nil
	})

	return result, err
}

func checkCSRF(session *Session, csrf string) error {
	if len(csrf) != 43 || subtle.ConstantTimeCompare([]byte(session.CSRF), []byte(csrf)) != 1 {
		return ErrForgery
	}
	return nil
}

// SignOut removes the local session before revocation, so remote failures never
// leave Hub authority active. The caller offers GitHub settings recovery if revocation fails.
func (s *Service) SignOut(ctx context.Context, id, csrf string) (bool, error) {
	if !s.Enabled() {
		return false, ErrUnavailable
	}
	if len(id) != 43 {
		return false, ErrReconnect
	}

	var credentials Credentials
	err := s.store.WithSession(ctx, Hash(id), func(session *Session) error {
		if err := checkCSRF(session, csrf); err != nil {
			return err
		}
		credentials = session.Credentials
		session.Credentials = Credentials{}
		return nil
	})
	if err != nil {
		return false, err
	}

	// An expired access token cannot revoke a still-usable refresh token. Rotate
	// after deleting the local session, then revoke the current pair on GitHub.
	if !credentials.AccessExpires.IsZero() && !s.now().Before(credentials.AccessExpires) &&
		credentials.Refresh != "" && s.now().Before(credentials.RefreshExpires) {
		credentials, err = s.provider.Refresh(ctx, credentials)
		if err != nil {
			return false, nil
		}
	}

	err = s.provider.Revoke(ctx, credentials.Access)
	refreshMayBeValid := credentials.Refresh != "" && s.now().Before(credentials.RefreshExpires)
	return err == nil || (errors.Is(err, ErrReconnect) && !refreshMayBeValid), nil
}
