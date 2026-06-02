package contacts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mayloo89/circl/backend/internal/apierror"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/notifications"
	"github.com/rs/zerolog"
)

// RateLimiter is satisfied by *ratelimit.RedisLimiter.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// Manager is the interface the handlers depend on.
// *Service satisfies this interface.
type Manager interface {
	SendRequest(ctx context.Context, requesterID, addresseeID string) (*Contact, error)
	Accept(ctx context.Context, contactID, userID string) (*Contact, error)
	Delete(ctx context.Context, contactID, userID string) (*Contact, error)
	ListAccepted(ctx context.Context, userID string) ([]AcceptedContact, error)
	ListPending(ctx context.Context, userID string) ([]PendingRequest, error)
	ListSent(ctx context.Context, userID string) ([]SentRequest, error)
	SearchUsers(ctx context.Context, query, userID string) ([]UserSummary, error)
	Block(ctx context.Context, blockerID, targetID string) error
	Unblock(ctx context.Context, blockerID, targetID string) error
	ListBlocked(ctx context.Context, userID string) ([]BlockedUser, error)
}

// handlerConfig holds optional dependencies for the contacts handler.
type handlerConfig struct {
	notifier notifications.Notifier
	limiter  RateLimiter
}

// HandlerOption configures the contacts handler.
type HandlerOption func(*handlerConfig)

// WithNotifier sets a Notifier that receives events when contact requests are
// sent or accepted.
func WithNotifier(n notifications.Notifier) HandlerOption {
	return func(cfg *handlerConfig) { cfg.notifier = n }
}

// WithLimiter sets a rate limiter for outgoing contact requests (100/day per user).
func WithLimiter(l RateLimiter) HandlerOption {
	return func(cfg *handlerConfig) { cfg.limiter = l }
}

// NewHandler returns a chi router with all contacts and user-search routes.
// All routes require a valid JWT (enforced by the caller via middleware.RequireAuth).
func NewHandler(svc Manager, opts ...HandlerOption) http.Handler {
	cfg := &handlerConfig{}
	for _, o := range opts {
		o(cfg)
	}

	r := chi.NewRouter()

	r.Get("/users/search", searchUsersHandler(svc))
	r.Post("/contacts", sendRequestHandler(svc, cfg))
	r.Get("/contacts", listAcceptedHandler(svc))
	r.Get("/contacts/pending", listPendingHandler(svc))
	r.Get("/contacts/sent", listSentHandler(svc))
	r.Get("/contacts/blocked", listBlockedHandler(svc))
	r.Put("/contacts/{id}/accept", acceptHandler(svc, cfg))
	r.Delete("/contacts/{id}", deleteHandler(svc, cfg))
	r.Post("/contacts/{id}/block", blockHandler(svc))
	r.Delete("/contacts/{id}/block", unblockHandler(svc))

	return r
}

func searchUsersHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		q := r.URL.Query().Get("q")
		results, err := svc.SearchUsers(r.Context(), q, userID)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		apierror.WriteJSON(w, http.StatusOK, results)
	}
}

func sendRequestHandler(svc Manager, cfg *handlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}

		if cfg.limiter != nil {
			key := fmt.Sprintf("contacts:send:%s:%s", userID, time.Now().UTC().Format("2006-01-02"))
			allowed, err := cfg.limiter.Allow(r.Context(), key, 100, 24*time.Hour)
			if err != nil {
				zerolog.Ctx(r.Context()).Warn().Err(err).Msg("contacts: rate limiter error")
			} else if !allowed {
				apierror.Write(w, http.StatusTooManyRequests, apierror.CodeRateLimited, "too many contact requests")
				return
			}
		}

		var body struct {
			AddresseeID string `json:"addressee_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "invalid request body")
			return
		}
		if body.AddresseeID == "" {
			apierror.Write(w, http.StatusBadRequest, apierror.CodeInvalidRequest, "addressee_id is required")
			return
		}

		contact, err := svc.SendRequest(r.Context(), userID, body.AddresseeID)
		if err != nil {
			switch {
			case errors.Is(err, ErrSelfContact):
				apierror.Write(w, http.StatusBadRequest, apierror.CodeSelfContact, "cannot add yourself as a contact")
			case errors.Is(err, ErrAlreadyExists):
				apierror.Write(w, http.StatusConflict, apierror.CodeContactExists, "contact request already exists")
			default:
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			}
			return
		}

		if cfg.notifier != nil {
			cfg.notifier.Notify(contact.AddresseeID, notifications.Event{
				Type: "contact_request",
				Payload: map[string]string{
					"contact_id":   contact.ID,
					"requester_id": contact.RequesterID,
				},
			})
		}

		apierror.WriteJSON(w, http.StatusCreated, contact)
	}
}

func listAcceptedHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		contacts, err := svc.ListAccepted(r.Context(), userID)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		apierror.WriteJSON(w, http.StatusOK, contacts)
	}
}

func listPendingHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		contacts, err := svc.ListPending(r.Context(), userID)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		apierror.WriteJSON(w, http.StatusOK, contacts)
	}
}

func listSentHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		sent, err := svc.ListSent(r.Context(), userID)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		apierror.WriteJSON(w, http.StatusOK, sent)
	}
}

func acceptHandler(svc Manager, cfg *handlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		contactID := chi.URLParam(r, "id")
		contact, err := svc.Accept(r.Context(), contactID, userID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apierror.Write(w, http.StatusNotFound, apierror.CodeContactNotFound, "contact not found")
				return
			}
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		if cfg.notifier != nil {
			cfg.notifier.Notify(contact.RequesterID, notifications.Event{
				Type: "contact_accepted",
				Payload: map[string]string{
					"contact_id":   contact.ID,
					"addressee_id": contact.AddresseeID,
				},
			})
		}

		apierror.WriteJSON(w, http.StatusOK, contact)
	}
}

func deleteHandler(svc Manager, cfg *handlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		contactID := chi.URLParam(r, "id")
		contact, err := svc.Delete(r.Context(), contactID, userID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apierror.Write(w, http.StatusNotFound, apierror.CodeContactNotFound, "contact not found")
				return
			}
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}

		if cfg.notifier != nil {
			otherUserID := contact.RequesterID
			if contact.RequesterID == userID {
				otherUserID = contact.AddresseeID
			}
			cfg.notifier.Notify(otherUserID, notifications.Event{
				Type:    "contact_removed",
				Payload: map[string]string{"contact_id": contact.ID},
			})
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func listBlockedHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		blocked, err := svc.ListBlocked(r.Context(), userID)
		if err != nil {
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		apierror.WriteJSON(w, http.StatusOK, blocked)
	}
}

func blockHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		targetID := chi.URLParam(r, "id")
		err := svc.Block(r.Context(), userID, targetID)
		if err != nil {
			switch {
			case errors.Is(err, ErrSelfContact):
				apierror.Write(w, http.StatusBadRequest, apierror.CodeSelfBlock, "cannot block yourself")
			case errors.Is(err, ErrAlreadyBlocked):
				apierror.Write(w, http.StatusConflict, apierror.CodeAlreadyBlocked, "user already blocked")
			default:
				apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func unblockHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			apierror.Write(w, http.StatusUnauthorized, apierror.CodeUnauthorized, "unauthorized")
			return
		}
		targetID := chi.URLParam(r, "id")
		err := svc.Unblock(r.Context(), userID, targetID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apierror.Write(w, http.StatusNotFound, apierror.CodeBlockNotFound, "block not found")
				return
			}
			apierror.Write(w, http.StatusInternalServerError, apierror.CodeInternalError, "internal server error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
