// Package github implements GitHub App user authorization with oauth2 and go-github.
package github

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
	"golang.org/x/oauth2"
	oauthgithub "golang.org/x/oauth2/github"
)

type Provider struct {
	appID  int64
	oauth  oauth2.Config
	client *http.Client
	api    *gh.Client
	app    *gh.Client
}

func New(appID int64, clientID, secret, callback string, client *http.Client) *Provider {
	basic := &gh.BasicAuthTransport{Username: clientID, Password: secret, Transport: client.Transport}
	endpoint := oauthgithub.Endpoint
	endpoint.AuthStyle = oauth2.AuthStyleInParams

	return &Provider{appID: appID, oauth: oauth2.Config{ClientID: clientID, ClientSecret: secret, RedirectURL: callback, Endpoint: endpoint},
		client: client, api: gh.NewClient(client), app: gh.NewClient(&http.Client{Timeout: client.Timeout, Transport: basic})}
}

func (p *Provider) AuthorizationURL(state, verifier string) string {
	return p.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("allow_signup", "false"))
}

func (p *Provider) Exchange(ctx context.Context, code, verifier string) (domain.Credentials, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	token, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return domain.Credentials{}, oauthError(err)
	}

	return credentials(token)
}

func (p *Provider) Refresh(ctx context.Context, previous domain.Credentials) (domain.Credentials, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	// An expired token forces oauth2's standard refresh exchange; no custom OAuth HTTP client.
	token, err := p.oauth.TokenSource(ctx, &oauth2.Token{AccessToken: previous.Access, RefreshToken: previous.Refresh, Expiry: time.Now().Add(-time.Hour)}).Token()
	if err != nil {
		return domain.Credentials{}, oauthError(err)
	}

	value, err := credentials(token)
	if err == nil && (value.Refresh == "" || value.Refresh == previous.Refresh) {
		return domain.Credentials{}, domain.ErrReconnect
	}
	return value, err
}

func credentials(token *oauth2.Token) (domain.Credentials, error) {
	if !strings.HasPrefix(token.AccessToken, "ghu_") || !strings.EqualFold(token.TokenType, "bearer") {
		return domain.Credentials{}, domain.ErrReconnect
	}
	if scope, ok := token.Extra("scope").(string); ok && scope != "" {
		return domain.Credentials{}, domain.ErrReconnect
	}

	value := domain.Credentials{Access: token.AccessToken, Refresh: token.RefreshToken, AccessExpires: token.Expiry}
	if value.Refresh != "" {
		seconds := expirySeconds(token.Extra("refresh_token_expires_in"))
		if !(seconds > 0 && seconds <= 366*24*60*60) || !strings.HasPrefix(value.Refresh, "ghr_") || value.AccessExpires.IsZero() {
			return domain.Credentials{}, domain.ErrReconnect
		}
		value.RefreshExpires = time.Now().Add(time.Duration(seconds) * time.Second)
	} else if !value.AccessExpires.IsZero() {
		return domain.Credentials{}, domain.ErrReconnect
	}

	return value, nil
}

func expirySeconds(value any) float64 {
	switch value := value.(type) {
	case float64:
		return value
	case int64:
		return float64(value)
	case string:
		seconds, _ := strconv.ParseFloat(value, 64)
		return seconds
	default:
		return 0
	}
}

func (p *Provider) User(ctx context.Context, token string) (domain.User, error) {
	if !strings.HasPrefix(token, "ghu_") {
		return domain.User{}, domain.ErrReconnect
	}

	authorization, response, err := p.app.Authorizations.Check(ctx, p.oauth.ClientID, token)
	if err != nil {
		return domain.User{}, apiError(response, true)
	}
	if authorization.GetApp().GetClientID() != p.oauth.ClientID || authorization.GetUser().GetID() <= 0 {
		return domain.User{}, domain.ErrReconnect
	}

	user, response, err := p.api.WithAuthToken(token).Users.Get(ctx, "")
	if err != nil {
		return domain.User{}, apiError(response, false)
	}
	if user.GetID() != authorization.GetUser().GetID() || user.GetType() != "User" || user.GetLogin() == "" {
		return domain.User{}, domain.ErrReconnect
	}

	return domain.User{ID: user.GetID(), Login: user.GetLogin()}, nil
}

func (p *Provider) Revoke(ctx context.Context, token string) error {
	response, err := p.app.Authorizations.Revoke(ctx, p.oauth.ClientID, token)
	if err != nil {
		return apiError(response, true)
	}
	return nil
}
