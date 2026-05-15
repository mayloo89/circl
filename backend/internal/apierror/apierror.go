// Package apierror provides a shared HTTP error response type and writer
// used across all handlers. Every error response includes a stable machine-
// readable code alongside the human-readable message so clients can
// translate errors independently of the English message text.
package apierror

import (
	"encoding/json"
	"net/http"
)

// Response is the standard error envelope returned by all API handlers.
type Response struct {
	Code    string `json:"code"`
	Message string `json:"error"`
}

// Write serialises an error Response with the given HTTP status code.
func Write(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{Code: code, Message: message})
}

// WriteJSON serialises any value as JSON with the given HTTP status code.
// Use this for success responses; use Write for error responses.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Stable error codes referenced by frontend i18n message keys.
const (
	// Generic
	CodeUnauthorized    = "unauthorized"
	CodeForbidden       = "forbidden"
	CodeNotFound        = "not_found"
	CodeInvalidRequest  = "invalid_request"
	CodeInternalError   = "internal_error"
	CodeRateLimited     = "rate_limited"

	// Auth
	CodeAccountLocked      = "account_locked"
	CodeEmailNotVerified   = "email_not_verified"
	CodeInvalidCredentials = "invalid_credentials"
	CodeEmailTaken         = "email_taken"
	CodeUsernameTaken      = "username_taken"
	CodePasswordTooLong    = "password_too_long"
	CodeInvalidToken       = "invalid_token"
	CodeTermsNotAccepted   = "terms_not_accepted"

	// Contacts
	CodeSelfContact    = "self_contact"
	CodeContactExists  = "contact_exists"
	CodeContactNotFound = "contact_not_found"
	CodeSelfBlock      = "self_block"
	CodeAlreadyBlocked = "already_blocked"
	CodeBlockNotFound  = "block_not_found"

	// Chat
	CodeRoomNotFound   = "room_not_found"
	CodeNameRequired   = "name_required"

	// Profile
	CodeProfileNotFound = "profile_not_found"
	CodeInvalidDOB      = "invalid_dob"

	// Admin
	CodeChannelNotFound     = "channel_not_found"
	CodeChannelNameTaken    = "channel_name_taken"
	CodeUserNotFound        = "user_not_found"

	// Appeals
	CodeAppealAlreadyResolved = "appeal_already_resolved"

	// Push
	CodeServiceUnavailable = "service_unavailable"
)
