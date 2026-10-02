package api

import (
	"net/http"
	"time"

	"github.com/r59q/hub-rearranger/services/identity/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

func (h *Handler) StartSignIn(w http.ResponseWriter, r *http.Request, _ contract.StartSignInParams) {
	url, binding, err := h.service.Begin(r.Context())
	if err != nil {
		h.failure(w, r, err)
		return
	}

	h.setCookie(w, "login", binding, time.Now().Add(domain.IntentLifetime), int(domain.IntentLifetime.Seconds()))
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func (h *Handler) CompleteSignIn(w http.ResponseWriter, r *http.Request, params contract.CompleteSignInParams) {
	h.clearCookie(w, "login")
	if len(r.URL.Query()["state"]) != 1 || len(r.URL.Query()["code"]) != 1 {
		h.failure(w, r, domain.ErrForgery)
		return
	}

	id, session, err := h.service.Complete(r.Context(), params.State, h.cookie(r, "login"), params.Code, h.cookie(r, "session"))
	if err != nil {
		h.failure(w, r, err)
		return
	}

	h.setCookie(w, "session", id, session.Expires, int(domain.SessionLifetime.Seconds()))
	http.Redirect(w, r, h.origin+"/account", http.StatusSeeOther)
}

func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	result := contract.Session{State: contract.SignedOut}
	if !h.service.Enabled() {
		result.State = contract.Disabled
		writeJSON(w, 200, result)
		return
	}

	id := h.cookie(r, "session")
	if id == "" {
		writeJSON(w, 200, result)
		return
	}

	session, err := h.service.Session(r.Context(), id)
	if err != nil {
		h.failure(w, r, err)
		return
	}

	user := contract.User{Id: session.User.ID, Login: session.User.Login}
	result = contract.Session{State: contract.Authenticated, User: &user, Csrf: &session.CSRF, ExpiresAt: &session.Expires}
	writeJSON(w, 200, result)
}

func (h *Handler) SignOut(w http.ResponseWriter, r *http.Request, _ contract.SignOutParams) {
	var body contract.ProtectedRequest
	if err := readJSON(w, r, &body); err != nil {
		h.failure(w, r, err)
		return
	}

	revoked, err := h.service.SignOut(r.Context(), h.cookie(r, "session"), body.Csrf)
	if err != nil {
		h.failure(w, r, err)
		return
	}

	h.clearCookie(w, "session")
	message := "Signed out. The GitHub user token has been revoked."
	if !revoked {
		message = "Signed out of Hub. GitHub token revocation could not be confirmed; revoke the App authorization in GitHub account settings."
	}

	writeJSON(w, 200, contract.SignOutResult{Revoked: revoked, Message: message})
}
