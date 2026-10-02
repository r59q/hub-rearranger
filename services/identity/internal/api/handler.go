// Package api maps the versioned identity contract to domain use cases.
package api

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/r59q/hub-rearranger/services/identity/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
)

type Handler struct {
	service *domain.Service
	origin  string
	secure  bool
}

var _ contract.ServerInterface = (*Handler)(nil)

func NewHandler(service *domain.Service, origin string, secure bool) http.Handler {
	h := &Handler{service: service, origin: origin, secure: secure}
	mux := http.NewServeMux()
	contract.HandlerWithOptions(h, contract.StdHTTPServerOptions{BaseRouter: mux, ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) { h.failure(w, r, domain.ErrInvalid) }})

	for path, method := range map[string]string{"/health": "GET, HEAD", "/v1/session": "GET", "/v1/sign-in": "POST", "/v1/sign-in/callback": "GET", "/v1/sign-out": "POST", "/v1/repositories/{owner}/{repo}/authorization": "POST"} {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Allow", method)
			writeJSON(w, 405, contract.Error{Code: contract.MethodNotAllowed, Message: "Use the documented method for this endpoint."})
		})
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 404, contract.Error{Code: contract.NotFound, Message: "The identity endpoint was not found."})
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		if r.Method == http.MethodPost && (len(r.Header.Values("Origin")) != 1 || r.Header.Get("Origin") != origin) {
			h.failure(w, r, domain.ErrForgery)
			return
		}
		if r.Method == http.MethodHead && r.URL.Path != "/health" {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(405)
			return
		}

		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) GetHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, contract.Health{Status: contract.Ok})
}

func (h *Handler) HeadHealth(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func readJSON(w http.ResponseWriter, r *http.Request, value any) error {
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || kind != "application/json" {
		return domain.ErrInvalid
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil || decoder.Decode(new(any)) != io.EOF {
		return domain.ErrInvalid
	}
	return nil
}
