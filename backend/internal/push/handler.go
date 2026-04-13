package push

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
)

// NewHandler returns an http.Handler with all push routes.
// All routes require authentication via requireAuth middleware applied by the
// caller (server.go).
func NewHandler(svc *Service) http.Handler {
	r := chi.NewRouter()
	r.Get("/vapid-public-key", vapidPublicKeyHandler(svc))
	r.Post("/subscribe", subscribeHandler(svc))
	r.Delete("/subscribe", unsubscribeHandler(svc))
	return r
}

// vapidPublicKeyHandler returns the VAPID public key for browser subscription.
// Returns 503 when push notifications are not configured.
func vapidPublicKeyHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !svc.Enabled() {
			apierror.Write(w, http.StatusServiceUnavailable, apierror.CodeServiceUnavailable, "push notifications not configured")
			return
		}
		apierror.WriteJSON(w, http.StatusOK, map[string]string{"public_key": svc.VAPIDPublicKey()})
	}
}

// subscribeHandler saves a push subscription for the authenticated user.
func subscribeHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		var req struct {
			Endpoint string `json:"endpoint"`
			P256DH   string `json:"p256dh"`
			Auth     string `json:"auth"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		if req.Endpoint == "" || req.P256DH == "" || req.Auth == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "endpoint, p256dh, and auth are required")
			return
		}

		if err := svc.Subscribe(r.Context(), userID, req.Endpoint, req.P256DH, req.Auth); err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// unsubscribeHandler removes a push subscription for the authenticated user.
func unsubscribeHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		var req struct {
			Endpoint string `json:"endpoint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		if req.Endpoint == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "endpoint is required")
			return
		}

		if err := svc.Unsubscribe(r.Context(), userID, req.Endpoint); err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
