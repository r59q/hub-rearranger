package domain

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSessionRefreshPreservesIdentityAndRotatedPair(t *testing.T) {
	service, store, provider, id, session := setup()
	session.Credentials = Credentials{Access: "old-synthetic-token", Refresh: "old-synthetic-refresh", AccessExpires: service.now(), RefreshExpires: service.now().Add(time.Hour)}
	store.sessions[Hash(id)] = session
	provider.refreshed = Credentials{Access: "new-synthetic-token", Refresh: "new-synthetic-refresh", AccessExpires: service.now().Add(time.Hour), RefreshExpires: service.now().Add(2 * time.Hour)}

	result, err := service.Session(context.Background(), id)

	if err != nil || result.User != session.User || result.Credentials != provider.refreshed || provider.refreshes != 1 {
		t.Fatal("token rotation did not preserve verified identity")
	}

	_, err = service.Session(context.Background(), id)

	if err != nil || provider.refreshes != 1 {
		t.Fatal("fresh pair was not persisted")
	}
}

func TestExpiredRevokedOrChangedIdentityDeletesSession(t *testing.T) {
	for _, variant := range []string{"session-expired", "refresh-expired", "no-refresh", "revoked", "different-user"} {
		t.Run(variant, func(t *testing.T) {
			service, store, provider, id, session := setup()
			switch variant {
			case "session-expired":
				session.Expires = service.now()
			case "refresh-expired":
				session.Credentials.AccessExpires = service.now()
				session.Credentials.Refresh = "expired"
				session.Credentials.RefreshExpires = service.now()
			case "no-refresh":
				session.Credentials.AccessExpires = service.now()
			case "revoked":
				provider.userError = ErrReconnect
			case "different-user":
				provider.user.ID++
			}
			store.sessions[Hash(id)] = session

			_, err := service.Session(context.Background(), id)

			if !errors.Is(err, ErrReconnect) {
				t.Fatal("invalid session was accepted")
			}
			if _, exists := store.sessions[Hash(id)]; exists {
				t.Fatal("invalid session was retained")
			}
		})
	}
}

func TestTemporaryFailurePreservesSessionWithoutClaimingIdentity(t *testing.T) {
	service, store, provider, id, _ := setup()
	provider.userError = ErrUnavailable

	_, err := service.Session(context.Background(), id)

	if !errors.Is(err, ErrUnavailable) || len(store.sessions) != 1 {
		t.Fatal("temporary failure changed authority state incorrectly")
	}
}

func TestSignOutDeletesSessionBeforeRemoteRevocationFailure(t *testing.T) {
	service, store, provider, id, session := setup()
	provider.revokeError = ErrUnavailable

	revoked, err := service.SignOut(context.Background(), id, session.CSRF)

	if err != nil || revoked || len(store.sessions) != 0 || provider.revokes != 1 {
		t.Fatal("remote failure left Hub session active")
	}
}

func TestSignOutOfExpiredAccessRevokesCurrentRefreshPair(t *testing.T) {
	service, store, provider, id, session := setup()
	session.Credentials = Credentials{Access: "expired-synthetic-access", Refresh: "usable-synthetic-refresh", AccessExpires: service.now(), RefreshExpires: service.now().Add(time.Hour)}
	store.sessions[Hash(id)] = session
	provider.refreshed = Credentials{Access: "fresh-synthetic-access", Refresh: "fresh-synthetic-refresh", AccessExpires: service.now().Add(time.Hour), RefreshExpires: service.now().Add(2 * time.Hour)}

	revoked, err := service.SignOut(context.Background(), id, session.CSRF)

	if err != nil || !revoked || provider.refreshes != 1 || provider.revokedToken != provider.refreshed.Access || len(store.sessions) != 0 {
		t.Fatal("expired access left a usable refresh credential without revocation")
	}
}

func TestSignOutRefreshFailureStillRemovesLocalAuthority(t *testing.T) {
	service, store, provider, id, session := setup()
	session.Credentials = Credentials{Access: "expired-synthetic-access", Refresh: "usable-synthetic-refresh", AccessExpires: service.now(), RefreshExpires: service.now().Add(time.Hour)}
	store.sessions[Hash(id)] = session
	provider.refreshError = ErrUnavailable

	revoked, err := service.SignOut(context.Background(), id, session.CSRF)

	if err != nil || revoked || len(store.sessions) != 0 || provider.refreshes != 1 || provider.revokes != 0 {
		t.Fatal("failed refresh restored local authority or claimed remote revocation")
	}
}

func TestSignOutMissingAccessDoesNotClaimUsableRefreshWasRevoked(t *testing.T) {
	service, store, provider, id, session := setup()
	session.Credentials.Refresh = "usable-synthetic-refresh"
	session.Credentials.RefreshExpires = service.now().Add(time.Hour)
	store.sessions[Hash(id)] = session
	provider.revokeError = ErrReconnect

	revoked, err := service.SignOut(context.Background(), id, session.CSRF)

	if err != nil || revoked || len(store.sessions) != 0 {
		t.Fatal("missing access token incorrectly proved refresh-token revocation")
	}
}
