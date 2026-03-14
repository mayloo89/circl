package contacts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mayloo89/circl/backend/internal/middleware"
	"github.com/mayloo89/circl/backend/internal/notifications"
)

// Manager is the interface the handlers depend on.
// *Service satisfies this interface.
type Manager interface {
	SendRequest(ctx context.Context, requesterID, addresseeID string) (*Contact, error)
	Accept(ctx context.Context, contactID, userID string) (*Contact, error)
	Delete(ctx context.Context, contactID, userID string) error
	ListAccepted(ctx context.Context, userID string) ([]AcceptedContact, error)
	ListPending(ctx context.Context, userID string) ([]PendingRequest, error)
	ListSent(ctx context.Context, userID string) ([]SentRequest, error)
	SearchUsers(ctx context.Context, query, userID string) ([]UserSummary, error)
}

// handlerConfig holds optional dependencies for the contacts handler.
type handlerConfig struct {
	notifier notifications.Notifier
}

// HandlerOption configures the contacts handler.
type HandlerOption func(*handlerConfig)

// WithNotifier sets a Notifier that receives events when contact requests are
// sent or accepted.
func WithNotifier(n notifications.Notifier) HandlerOption {
	return func(cfg *handlerConfig) { cfg.notifier = n }
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
	r.Put("/contacts/{id}/accept", acceptHandler(svc, cfg))
	r.Delete("/contacts/{id}", deleteHandler(svc))

	return r
}

func searchUsersHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
			return
		}
		q := r.URL.Query().Get("q")
		results, err := svc.SearchUsers(r.Context(), q, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errResp("internal server error"))
			return
		}
		writeJSON(w, http.StatusOK, results)
	}
}

func sendRequestHandler(svc Manager, cfg *handlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
			return
		}

		var body struct {
			AddresseeID string `json:"addressee_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, errResp("invalid request body"))
			return
		}
		if body.AddresseeID == "" {
			writeJSON(w, http.StatusBadRequest, errResp("addressee_id is required"))
			return
		}

		contact, err := svc.SendRequest(r.Context(), userID, body.AddresseeID)
		if err != nil {
			switch {
			case errors.Is(err, ErrSelfContact):
				writeJSON(w, http.StatusBadRequest, errResp("cannot add yourself as a contact"))
			case errors.Is(err, ErrAlreadyExists):
				writeJSON(w, http.StatusConflict, errResp("contact request already exists"))
			default:
				writeJSON(w, http.StatusInternalServerError, errResp("internal server error"))
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

		writeJSON(w, http.StatusCreated, contact)
	}
}

func listAcceptedHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
			return
		}
		contacts, err := svc.ListAccepted(r.Context(), userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errResp("internal server error"))
			return
		}
		writeJSON(w, http.StatusOK, contacts)
	}
}

func listPendingHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
			return
		}
		contacts, err := svc.ListPending(r.Context(), userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errResp("internal server error"))
			return
		}
		writeJSON(w, http.StatusOK, contacts)
	}
}

func listSentHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
			return
		}
		sent, err := svc.ListSent(r.Context(), userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errResp("internal server error"))
			return
		}
		writeJSON(w, http.StatusOK, sent)
	}
}

func acceptHandler(svc Manager, cfg *handlerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
			return
		}
		contactID := chi.URLParam(r, "id")
		contact, err := svc.Accept(r.Context(), contactID, userID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeJSON(w, http.StatusNotFound, errResp("contact not found"))
				return
			}
			writeJSON(w, http.StatusInternalServerError, errResp("internal server error"))
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

		writeJSON(w, http.StatusOK, contact)
	}
}

func deleteHandler(svc Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errResp("unauthorized"))
			return
		}
		contactID := chi.URLParam(r, "id")
		if err := svc.Delete(r.Context(), contactID, userID); err != nil {
			if errors.Is(err, ErrNotFound) {
				writeJSON(w, http.StatusNotFound, errResp("contact not found"))
				return
			}
			writeJSON(w, http.StatusInternalServerError, errResp("internal server error"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type errorResponse struct {
	Error string `json:"error"`
}

func errResp(msg string) errorResponse { return errorResponse{Error: msg} }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
