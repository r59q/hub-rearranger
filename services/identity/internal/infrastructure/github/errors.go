package github

import (
	"errors"
	"net/http"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
	"golang.org/x/oauth2"
)

func oauthError(err error) error {
	var failure *oauth2.RetrieveError
	if errors.As(err, &failure) {
		switch failure.ErrorCode {
		case "bad_verification_code", "invalid_grant", "expired_refresh_token", "incorrect_code", "invalid_refresh_token":
			return domain.ErrReconnect
		}
	}
	return domain.ErrUnavailable
}

func apiError(response *gh.Response, tokenCheck bool) error {
	if response == nil {
		return domain.ErrUnavailable
	}

	switch response.StatusCode {
	case http.StatusUnauthorized:
		return domain.ErrReconnect
	case http.StatusNotFound:
		if tokenCheck {
			return domain.ErrReconnect
		}
		return domain.ErrForbidden
	case http.StatusForbidden:
		if tokenCheck {
			return domain.ErrUnavailable
		}
		// Rate limits are temporary, while permission denials require changed access.
		if response.Rate.Remaining == 0 && response.Header.Get("X-RateLimit-Limit") != "" || response.Header.Get("Retry-After") != "" {
			return domain.ErrUnavailable
		}
		return domain.ErrForbidden
	}
	return domain.ErrUnavailable
}
