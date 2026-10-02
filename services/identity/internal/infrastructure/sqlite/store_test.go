package sqlite

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func openStore(t *testing.T) (*Store, string, []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "identity.db")
	key := bytes.Repeat([]byte{9}, 32)
	store, err := Open(path, key)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, path, key
}

func session() domain.Session {
	return domain.Session{User: domain.User{ID: 7, Login: "octocat"}, Credentials: domain.Credentials{Access: "synthetic-access-never-live", Refresh: "synthetic-refresh-never-live"}, CSRF: "synthetic-csrf", Expires: time.Now().Add(time.Hour)}
}

func TestSessionsPersistEncryptedAndRejectAnotherKey(t *testing.T) {
	store, path, key := openStore(t)
	value := session()
	id := domain.Hash("opaque-browser-id")

	if err := store.PutSession(context.Background(), id, value); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(value.Credentials.Access)) || bytes.Contains(data, []byte(value.Credentials.Refresh)) || bytes.Contains(data, []byte(value.CSRF)) {
		t.Fatal("unencrypted credential data was persisted")
	}

	restarted, err := Open(path, key)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = restarted.Close() }()
	if err := restarted.WithSession(context.Background(), id, func(current *domain.Session) error {
		if current.Credentials != value.Credentials {
			t.Fatal("credentials did not survive restart")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	_ = restarted.Close()
	wrong, err := Open(path, bytes.Repeat([]byte{8}, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = wrong.Close() }()
	if err := wrong.WithSession(context.Background(), id, func(*domain.Session) error { t.Fatal("wrong key decrypted a session"); return nil }); !errors.Is(err, domain.ErrReconnect) {
		t.Fatal("wrong key did not fail closed")
	}
}

func TestConcurrentUseRotatesOnceAndPreservesRefreshOnDenial(t *testing.T) {
	store, _, _ := openStore(t)
	id := domain.Hash("opaque-browser-id")

	if err := store.PutSession(context.Background(), id, session()); err != nil {
		t.Fatal(err)
	}

	var group sync.WaitGroup
	rotations := 0
	for range 10 {
		group.Go(func() {
			err := store.WithSession(context.Background(), id, func(value *domain.Session) error {
				if value.Credentials.Access == "synthetic-access-never-live" {
					rotations++
					value.Credentials.Access = "rotated-synthetic-access"
				}
				return domain.ErrForbidden
			})
			if !errors.Is(err, domain.ErrForbidden) {
				t.Error("authorization denial lost or storage failed")
			}
		})
	}

	group.Wait()

	if rotations != 1 {
		t.Fatal("concurrent requests rotated more than once")
	}

	if err := store.WithSession(context.Background(), id, func(value *domain.Session) error {
		if value.Credentials.Access != "rotated-synthetic-access" {
			t.Fatal("denial rolled back the refreshed token")
		}
		return domain.ErrReconnect
	}); !errors.Is(err, domain.ErrReconnect) {
		t.Fatal(err)
	}

	if err := store.WithSession(context.Background(), id, func(*domain.Session) error { t.Fatal("deleted session was reused"); return nil }); !errors.Is(err, domain.ErrReconnect) {
		t.Fatal("revoked session remained")
	}
}

func TestIntentIsBoundAndConsumedOnce(t *testing.T) {
	for _, binding := range []string{"correct", "wrong"} {
		t.Run(binding, func(t *testing.T) {
			store, _, _ := openStore(t)
			intent := domain.Intent{Binding: domain.Hash("correct"), Verifier: "synthetic-verifier", Expires: time.Now().Add(time.Minute)}
			if err := store.PutIntent(context.Background(), domain.Hash("state"), intent); err != nil {
				t.Fatal(err)
			}

			result, err := store.TakeIntent(context.Background(), domain.Hash("state"), domain.Hash(binding))

			if binding == "correct" && (err != nil || result.Binding != intent.Binding || result.Verifier != intent.Verifier || !result.Expires.Equal(intent.Expires)) {
				t.Fatal("matching browser failed")
			}
			if binding == "wrong" && !errors.Is(err, domain.ErrForgery) {
				t.Fatal("wrong browser accepted")
			}

			if _, err := store.TakeIntent(context.Background(), domain.Hash("state"), domain.Hash("correct")); !errors.Is(err, domain.ErrForgery) {
				t.Fatal("intent replay succeeded")
			}
		})
	}
}
