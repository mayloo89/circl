// Package exports implements the per-user data export flow (Habeas Data /
// GDPR Art. 20). The user requests an export, an asynq worker builds a zip
// (machine-readable JSON + the user's own media), and we email a one-time
// download link with a 14-day TTL.
//
// The export schema is versioned (`schema_version` in data.json) so future
// changes to the JSON shape can be detected by downstream tooling.
package exports

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

// SchemaVersion identifies the JSON shape inside data.json. Bump on any
// breaking change to the snapshot structure.
const SchemaVersion = "1"

// TTL is how long the download link in the export-ready email is valid.
const TTL = 14 * 24 * time.Hour

// RequestRateLimit is the per-user cool-down between export builds. Exports
// are expensive (zip every message + every media file) so one per day is the
// strict upper bound; existing in-flight builds are also blocked by the
// unique partial index on the table.
const RequestRateLimit = 24 * time.Hour

// Valid export statuses.
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusReady      = "ready"
	StatusFailed     = "failed"
	StatusExpired    = "expired"
)

// Sentinel errors.
var (
	ErrNotFound       = errors.New("export request not found")
	ErrAlreadyPending = errors.New("an export is already being prepared")
	ErrRateLimited    = errors.New("an export was requested recently; please try again later")
	ErrInvalidToken   = errors.New("invalid or expired download token")
	ErrNotReady       = errors.New("export not ready")
)

// Request is the persisted state of an export request.
type Request struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	Status       string     `json:"status"`
	StorageKey   *string    `json:"storage_key,omitempty"`
	Error        string     `json:"error,omitempty"`
	RequestedAt  time.Time  `json:"requested_at"`
	CompletedAt  *time.Time `json:"completed_at,omitzero"`
	ExpiresAt    *time.Time `json:"expires_at,omitzero"`
	DownloadedAt *time.Time `json:"downloaded_at,omitzero"`
}

// PublicStatus is the subset of a request returned to the user; the
// storage_key and token_hash are deliberately omitted.
type PublicStatus struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	RequestedAt time.Time  `json:"requested_at"`
	CompletedAt *time.Time `json:"completed_at,omitzero"`
	ExpiresAt   *time.Time `json:"expires_at,omitzero"`
}

// Snapshot is the full JSON payload written to data.json inside the zip.
type Snapshot struct {
	SchemaVersion   string            `json:"schema_version"`
	GeneratedAt     time.Time         `json:"generated_at"`
	Account         Account           `json:"account"`
	Profile         *Profile          `json:"profile,omitempty"`
	ProfilePhotos   []ProfilePhoto    `json:"profile_photos"`
	Preferences     *Preferences      `json:"preferences,omitempty"`
	Contacts        []Contact         `json:"contacts"`
	Blocks          []Block           `json:"blocks"`
	Rooms           []Room            `json:"rooms"`
	Messages        []Message         `json:"messages"`
	ReportsFiled    []ReportFiled     `json:"reports_filed"`
	AgeAttestations []AgeAttestation  `json:"age_attestations"`
	Uploads         []Upload          `json:"uploads"`
}

// MediaItem describes a single file inside the export zip and where to fetch
// it from in storage.
type MediaItem struct {
	StorageKey  string
	ContentType string
	ArchivePath string
}

// Bundle is what Source returns: the JSON snapshot plus the list of media
// files the builder should fetch and zip alongside data.json.
type Bundle struct {
	Snapshot Snapshot
	Media    []MediaItem
}

// Account is the user's row in the users table (PII columns only).
type Account struct {
	ID                    string     `json:"id"`
	Email                 string     `json:"email"`
	Status                string     `json:"status"`
	Role                  string     `json:"role"`
	CreatedAt             time.Time  `json:"created_at"`
	EmailVerifiedAt       *time.Time `json:"email_verified_at,omitzero"`
	TermsAcceptedAt       *time.Time `json:"terms_accepted_at,omitzero"`
	PrivacyAcceptedAt     *time.Time `json:"privacy_accepted_at,omitzero"`
	AcceptedPolicyVersion *string    `json:"accepted_policy_version,omitempty"`
}

// Profile is the profile row.
type Profile struct {
	Username         string     `json:"username"`
	DisplayName      string     `json:"display_name"`
	Bio              string     `json:"bio"`
	DateOfBirth      *time.Time `json:"date_of_birth,omitzero"`
	Gender           string     `json:"gender"`
	Location         string     `json:"location"`
	Latitude         *float64   `json:"latitude,omitempty"`
	Longitude        *float64   `json:"longitude,omitempty"`
	AvatarURL        string     `json:"avatar_url"`
	Interests        []string   `json:"interests"`
	Locale           string     `json:"locale"`
	LookingForGender []string   `json:"looking_for_gender"`
	LookingForAgeMin *int       `json:"looking_for_age_min,omitempty"`
	LookingForAgeMax *int       `json:"looking_for_age_max,omitempty"`
	OnboardedAt      *time.Time `json:"onboarded_at,omitzero"`
}

// ProfilePhoto is one row from profile_photos.
type ProfilePhoto struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// Preferences is the user's profile_preferences row.
type Preferences struct {
	MinAge                       *int    `json:"min_age,omitempty"`
	MaxAge                       *int    `json:"max_age,omitempty"`
	MaxDistanceKm                *int    `json:"max_distance_km,omitempty"`
	GenderPreference             *string `json:"gender_preference,omitempty"`
	RequirePhoto                 bool    `json:"require_photo"`
	HideDistanceFromNonContacts  bool    `json:"hide_distance_from_non_contacts"`
	HidePresence                 bool    `json:"hide_presence"`
	HideReadReceipts             bool    `json:"hide_read_receipts"`
	HideTypingIndicator          bool    `json:"hide_typing_indicator"`
	NotifyChatMessages           bool    `json:"notify_chat_messages"`
	NotifyContactRequests        bool    `json:"notify_contact_requests"`
	NotifyChannelMentions        bool    `json:"notify_channel_mentions"`
	NotifySystem                 bool    `json:"notify_system"`
	Locale                       string  `json:"locale"`
}

// Contact is an accepted-contact link.
type Contact struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Block records that this user has blocked another user.
type Block struct {
	BlockedUserID   string    `json:"blocked_user_id"`
	BlockedUsername string    `json:"blocked_username"`
	CreatedAt       time.Time `json:"created_at"`
}

// Room is a room (DM, group, channel) the user belongs to.
type Room struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Message is one chat message authored by this user.
type Message struct {
	ID          string    `json:"id"`
	RoomID      string    `json:"room_id"`
	Body        string    `json:"body"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
}

// ReportFiled is a moderation report the user filed against someone else.
type ReportFiled struct {
	ID             string    `json:"id"`
	ReportedUserID string    `json:"reported_user_id"`
	Reason         string    `json:"reason"`
	Priority       string    `json:"priority"`
	Description    string    `json:"description"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// AgeAttestation is an `age_verification_audit` row for this user.
type AgeAttestation struct {
	AttestedAge   int        `json:"attested_age"`
	IP            *string    `json:"ip,omitempty"`
	UserAgent     string     `json:"user_agent"`
	DateOfBirth   *time.Time `json:"date_of_birth,omitzero"`
	PolicyVersion string     `json:"policy_version"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Upload is one row from the uploads table — a committed media item the user
// owns (chat attachment, profile photo source, etc).
type Upload struct {
	ID          string    `json:"id"`
	StorageKey  string    `json:"storage_key"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// Source builds the in-memory bundle for a user. The pg implementation
// fans out queries across every package's table; tests stub this directly.
type Source interface {
	BuildBundle(ctx context.Context, userID string) (*Bundle, error)
}

// Store is the persistence interface for export requests.
type Store interface {
	// Create attempts to insert a new pending request. Returns ErrAlreadyPending
	// when the unique partial index trips (a build is already in flight) and
	// ErrRateLimited when the user's previous request was inside the cool-down.
	Create(ctx context.Context, userID string) (*Request, error)
	// LatestForUser returns the user's most recent request, or ErrNotFound.
	LatestForUser(ctx context.Context, userID string) (*Request, error)
	// GetByID returns a request by ID.
	GetByID(ctx context.Context, id string) (*Request, error)
	// GetByTokenHash looks up a request via its SHA-256 token hash; returns
	// ErrNotFound when no row matches.
	GetByTokenHash(ctx context.Context, tokenHash string) (*Request, error)
	// MarkProcessing flips a row from pending to processing.
	MarkProcessing(ctx context.Context, id string) error
	// MarkReady stamps the storage key, hashed token, expiry, and flips to ready.
	MarkReady(ctx context.Context, id, storageKey, tokenHash string, expiresAt time.Time) error
	// MarkFailed flips a row to failed and records the error message.
	MarkFailed(ctx context.Context, id, errMsg string) error
	// MarkDownloaded stamps downloaded_at; idempotent.
	MarkDownloaded(ctx context.Context, id string) error
}

// HashToken returns the hex-encoded SHA-256 of plain. Exported so the worker
// and the download handler share one definition.
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// GenerateToken returns a random 32-byte token (hex-encoded) and its hash.
func GenerateToken() (plain, hash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return
	}
	plain = hex.EncodeToString(raw)
	hash = HashToken(plain)
	return
}
