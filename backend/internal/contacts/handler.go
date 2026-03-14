package contacts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mayloo89/circl/backend/internal/middleware"
)

// Manager is the interface the handlers depend on.
// *Service satisfies this interface.
type Manager interface {
	SendRequest(ctx context.Context, requesterID, addresseeID string) (*Contact, error)
	Accept(ctx context.Context, contactID, userID string) (*Contact, error)
	Delete(ctx context.Context, contactID, userID string) error
	ListAccepted(ctx context.Context, userID string) ([]UserSummary, error)
	ListPending(ctx context.Context, userID string) ([]UserSummary, error)
	SearchUsers(ctx context.Context, query, userID string) ([]UserSummary, error)
}

// NewHandler returns a chi router with all contacts and user-search routes.
// All routes require a valid JWT (enforced by the caller via middleware.RequireAuth).
func NewHandler(svc Manager) http.Handler {
	r := chi.NewRouter()

	r.Get("/users/search", searchUsersHandler(svc))
	r.Post("/contacts", sendRequestHandler(svc))
	r.Get("/contacts", listAcceptedHandler(svc))
	r.Get("/contacts/pending", listPendingHandler(svc))
	r.Put("/contacts/{id}/accept", acceptHandler(svc))
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

func sendRequestHandler(svc Manager) http.HandlerFunc {
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

func acceptHandler(svc Manager) http.HandlerFunc {
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
