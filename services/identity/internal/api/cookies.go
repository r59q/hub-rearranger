package api

import (
	"net/http"
	"time"
)

func (h *Handler) cookieName(kind string) string {
	if h.secure {
		return "__Host-hub_" + kind
	}
	return "hub_" + kind
}

func (h *Handler) cookie(r *http.Request, kind string) string {
	// Reject ambiguous duplicate cookies rather than choosing one by header order.
	cookies := r.CookiesNamed(h.cookieName(kind))
	if len(cookies) != 1 {
		return ""
	}
	return cookies[0].Value
}

func (h *Handler) setCookie(w http.ResponseWriter, kind, value string, expires time.Time, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: h.cookieName(kind), Value: value, Path: "/", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: maxAge})
}

func (h *Handler) clearCookie(w http.ResponseWriter, kind string) {
	h.setCookie(w, kind, "", time.Unix(1, 0), -1)
}
