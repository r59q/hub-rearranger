package domain

import (
	"context"
	"errors"
	"testing"
)

func TestSignInBindsAndConsumesStateAndRotatesSession(t *testing.T) {
	service, store, provider, previous, _ := setup()
	_, binding, err := service.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	id, session, err := service.Complete(context.Background(), provider.state, binding, "synthetic-code", previous)

	if err != nil || id == previous || len(id) != 43 || session.User != provider.user || len(session.CSRF) != 43 {
		t.Fatal("fresh verified session was not created")
	}
	if _, exists := store.sessions[Hash(previous)]; exists {
		t.Fatal("previous session remained authorized")
	}

	if _, _, err := service.Complete(context.Background(), provider.state, binding, "synthetic-code", ""); !errors.Is(err, ErrForgery) || provider.exchanges != 1 {
		t.Fatal("state replay reached the provider")
	}
}

func TestWrongBrowserOrExpiredIntentFailsBeforeExchange(t *testing.T) {
	for _, variant := range []string{"browser", "expired", "unknown"} {
		t.Run(variant, func(t *testing.T) {
			service, store, provider, _, _ := setup()
			_, binding, _ := service.Begin(context.Background())
			state := provider.state
			switch variant {
			case "browser":
				binding = randomSecret()
			case "expired":
				value := store.intents[Hash(state)]
				value.Expires = service.now()
				store.intents[Hash(state)] = value
			case "unknown":
				state = randomSecret()
			}

			_, _, err := service.Complete(context.Background(), state, binding, "synthetic-code", "")

			if !errors.Is(err, ErrForgery) || provider.exchanges != 0 {
				t.Fatal("untrusted callback reached token exchange")
			}
		})
	}
}

func TestUnverifiedUserCannotCreateSession(t *testing.T) {
	service, store, provider, _, _ := setup()
	provider.userError = ErrReconnect
	_, binding, _ := service.Begin(context.Background())

	_, _, err := service.Complete(context.Background(), provider.state, binding, "synthetic-code", "")

	if !errors.Is(err, ErrReconnect) || len(store.sessions) != 1 || provider.revokes != 1 {
		t.Fatal("invalid identity created a session or leaked a new credential")
	}
}
