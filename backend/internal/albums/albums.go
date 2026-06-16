// Package albums implements the private-albums feature: owners group
// uploads into named albums and explicitly grant access to specific
// contacts. Grants flow through three entry points (owner-invite,
// viewer-request, chat-share) and surface to the recipient via the
// notifications hub.
package albums

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/storage"
	"github.com/mayloo89/circl/backend/internal/uploads"
)

// Public errors. Handlers map these to apierror codes.
var (
	ErrNotFound       = errors.New("albums: not found")
	ErrForbidden      = errors.New("albums: forbidden")
	ErrInvalidRequest = errors.New("albums: invalid request")
	// ErrUploadNotApproved is returned when a photo's upload has not cleared
	// moderation (still pending, rejected, or quarantined).
	ErrUploadNotApproved = errors.New("albums: upload not approved")
	ErrGrantExists    = errors.New("albums: open grant already exists")
	ErrSelfGrant      = errors.New("albums: cannot grant access to yourself")
)

// Grant lifecycle constants. Mirrors the CHECK constraint in migration 000036.
const (
	GrantPending  = "pending"
	GrantActive   = "active"
	GrantDenied   = "denied"
	GrantRevoked  = "revoked"
	SourceInvite  = "invite"
	SourceRequest = "request"
	SourceChat    = "chat"
)

// Album is the owner-facing model. Counts are denormalised on the row to
// keep the list query cheap.
type Album struct {
	ID            string    `json:"id"`
	OwnerID       string    `json:"owner_id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	CoverUploadID *string   `json:"cover_upload_id,omitempty"`
	PhotoCount    int       `json:"photo_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	// CoverURL is the public URL of the cover thumbnail (when set). The
	// album listing endpoint fills it in for both owner and viewer; the
	// owner sees the original cover, the viewer sees the proxied
	// authenticated URL the frontend renders via the same Bearer-blob
	// dance used by the admin moderation page.
	CoverURL string `json:"cover_url,omitempty"`
	// Role reflects the caller's relationship with the album: "owner",
	// "viewer" (active grant), or "" (no relationship — only relevant for
	// the request-access flow).
	Role string `json:"role,omitempty"`
	// OwnerName and OwnerAvatarURL are populated on the shared-with-me
	// listing so the viewer can see whose album it is without a second
	// request.
	OwnerName      string `json:"owner_name,omitempty"`
	OwnerAvatarURL string `json:"owner_avatar_url,omitempty"`
}

// Photo is one upload pinned to an album. URL is the gated URL the
// frontend uses to render the bytes (see handler.go: it routes through
// /albums/{id}/photos/{upload_id}/file with the caller's Bearer token).
type Photo struct {
	UploadID    string    `json:"upload_id"`
	AlbumID     string    `json:"album_id"`
	Position    int       `json:"position"`
	AddedAt     time.Time `json:"added_at"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	URL         string    `json:"url"`
}

// Grant is one access record. Sent to the owner with full visibility
// (granter/grantee identities, status, source) and to the grantee with
// the same shape so the UI can render "this album was shared with you".
type Grant struct {
	ID          string     `json:"id"`
	AlbumID     string     `json:"album_id"`
	GranterID   string     `json:"granter_id"`
	GranteeID   string     `json:"grantee_id"`
	Status      string     `json:"status"`
	Source      string     `json:"source"`
	RequestedAt time.Time  `json:"requested_at"`
	GrantedAt   *time.Time `json:"granted_at,omitzero"`
	RevokedAt   *time.Time `json:"revoked_at,omitzero"`
	ExpiresAt   *time.Time `json:"expires_at,omitzero"`
	// Counterparty is the OTHER user's id from the caller's perspective —
	// for the owner this is the grantee, for the grantee this is the
	// granter. Saves the frontend a join when rendering "members" lists.
	Counterparty string `json:"counterparty,omitempty"`
}

// MediaResolver is the seam the service uses to confirm an upload belongs
// to the calling owner and to read its storage key for the gated stream.
// Implemented by the existing uploads.Store wrapper in main.go.
type MediaResolver interface {
	GetForOwner(ctx context.Context, uploadID, ownerID string) (*uploads.Upload, error)
}

// Store is the persistence contract.
type Store interface {
	CreateAlbum(ctx context.Context, ownerID, name, description string) (*Album, error)
	GetAlbum(ctx context.Context, albumID string) (*Album, error)
	ListAlbumsByOwner(ctx context.Context, ownerID string) ([]Album, error)
	ListAlbumsSharedWith(ctx context.Context, granteeID string) ([]Album, error)
	UpdateAlbum(ctx context.Context, albumID, name, description string) (*Album, error)
	DeleteAlbum(ctx context.Context, albumID string) error

	AddPhoto(ctx context.Context, albumID, uploadID string, position int) error
	RemovePhoto(ctx context.Context, albumID, uploadID string) error
	ListPhotos(ctx context.Context, albumID string) ([]Photo, error)
	GetPhoto(ctx context.Context, albumID, uploadID string) (*Photo, error)

	CreateGrant(ctx context.Context, albumID, granterID, granteeID, status, source string, expiresAt *time.Time) (*Grant, error)
	GetGrant(ctx context.Context, grantID string) (*Grant, error)
	UpdateGrantStatus(ctx context.Context, grantID, status string) (*Grant, error)
	ListGrantsByAlbum(ctx context.Context, albumID string) ([]Grant, error)
	FindOpenGrant(ctx context.Context, albumID, granteeID string) (*Grant, error)
	HasActiveGrant(ctx context.Context, albumID, viewerID string) (bool, error)
	ExpireAlbumGrants(ctx context.Context) error

}

// ChatBridge persists and broadcasts a chat message of type 'album_share'
// in the given room. Implemented by an adapter in main.go that talks to
// chat.Service + the chat hub, so this package stays decoupled from the
// chat internals.
type ChatBridge interface {
	SaveAlbumShareMessage(ctx context.Context, roomID, senderID, payload string) error
	// RoomPeer resolves the other participant of a DM room from the
	// caller's perspective. Returns ErrInvalidRequest for non-DM rooms or
	// when the caller is not a participant.
	RoomPeer(ctx context.Context, roomID, callerID string) (string, error)
}

// Notifier is how the albums service signals grant lifecycle events to the
// other party. Implementations push to the notifications hub + web push.
type Notifier interface {
	// AlbumInvited fires when the owner invites a contact (source='invite').
	AlbumInvited(ctx context.Context, granteeID, ownerID, albumID, albumName string)
	// AlbumRequested fires when a viewer requests access (source='request').
	AlbumRequested(ctx context.Context, ownerID, requesterID, albumID, albumName string)
	// AlbumGrantAccepted fires when the other party accepts a pending grant.
	AlbumGrantAccepted(ctx context.Context, targetID, actorID, albumID, albumName string)
	// AlbumGrantRevoked fires when either party revokes an active grant.
	AlbumGrantRevoked(ctx context.Context, targetID, actorID, albumID, albumName string)
}

// Service is the application-level orchestration.
type Service struct {
	store    Store
	storage  storage.Storage
	media    MediaResolver
	notifier Notifier
	chat     ChatBridge
	log      zerolog.Logger
}

// NewService returns a ready-to-use albums service.
func NewService(store Store, st storage.Storage, media MediaResolver, log zerolog.Logger) *Service {
	return &Service{
		store:   store,
		storage: st,
		media:   media,
		log:     log.With().Str("component", "albums").Logger(),
	}
}

// SetNotifier wires the notifications hub. Optional — if unset, grant
// events are silently logged and never push out.
func (s *Service) SetNotifier(n Notifier) { s.notifier = n }

// SetChatBridge wires the chat persist+publish bridge. Required for the
// share-in-chat flow; absent, that endpoint returns ErrInvalidRequest.
func (s *Service) SetChatBridge(b ChatBridge) { s.chat = b }

// CreateAlbum returns the new row. Name and description are trimmed; an
// empty name after trim is rejected.
func (s *Service) CreateAlbum(ctx context.Context, ownerID, name, description string) (*Album, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return nil, ErrInvalidRequest
	}
	return s.store.CreateAlbum(ctx, ownerID, name, description)
}

// GetAlbum returns the album for any caller with at least an active grant
// (or the owner). The Role field reflects the caller's relationship.
func (s *Service) GetAlbum(ctx context.Context, callerID, albumID string) (*Album, error) {
	a, err := s.store.GetAlbum(ctx, albumID)
	if err != nil {
		return nil, err
	}
	role, err := s.roleFor(ctx, callerID, a)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, ErrForbidden
	}
	a.Role = role
	return a, nil
}

// ListMyAlbums returns albums the caller owns.
func (s *Service) ListMyAlbums(ctx context.Context, ownerID string) ([]Album, error) {
	rows, err := s.store.ListAlbumsByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].Role = "owner"
	}
	return rows, nil
}

// ListSharedWithMe returns albums other users have actively granted to the caller.
func (s *Service) ListSharedWithMe(ctx context.Context, granteeID string) ([]Album, error) {
	rows, err := s.store.ListAlbumsSharedWith(ctx, granteeID)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].Role = "viewer"
	}
	return rows, nil
}

// AlbumPatch is the partial update for an album. Only the owner may update.
type AlbumPatch struct {
	Name        *string
	Description *string
}

func (s *Service) UpdateAlbum(ctx context.Context, callerID, albumID string, patch AlbumPatch) (*Album, error) {
	a, err := s.store.GetAlbum(ctx, albumID)
	if err != nil {
		return nil, err
	}
	if a.OwnerID != callerID {
		return nil, ErrForbidden
	}
	name := a.Name
	if patch.Name != nil {
		n := strings.TrimSpace(*patch.Name)
		if n == "" {
			return nil, ErrInvalidRequest
		}
		name = n
	}
	desc := a.Description
	if patch.Description != nil {
		desc = strings.TrimSpace(*patch.Description)
	}
	return s.store.UpdateAlbum(ctx, albumID, name, desc)
}

// DeleteAlbum removes the album and cascades to photos + grants.
// Only the owner may delete.
func (s *Service) DeleteAlbum(ctx context.Context, callerID, albumID string) error {
	a, err := s.store.GetAlbum(ctx, albumID)
	if err != nil {
		return err
	}
	if a.OwnerID != callerID {
		return ErrForbidden
	}
	return s.store.DeleteAlbum(ctx, albumID)
}

// AddPhoto adds an existing upload owned by the caller to the album.
// The upload must be in category 'album-private', already moderated to
// 'approved'/'skipped', and not already attached to this album.
func (s *Service) AddPhoto(ctx context.Context, callerID, albumID, uploadID string) error {
	a, err := s.store.GetAlbum(ctx, albumID)
	if err != nil {
		return err
	}
	if a.OwnerID != callerID {
		return ErrForbidden
	}
	u, err := s.media.GetForOwner(ctx, uploadID, callerID)
	if err != nil {
		return ErrInvalidRequest
	}
	if u.Category != string(storage.CategoryAlbumPrivate) {
		return ErrInvalidRequest
	}
	// Attach-time moderation gate: only an approved image may join an album.
	// This blocks pending (not-yet-scanned), rejected, and quarantined uploads
	// alike — not just rejected ones.
	if !uploadServable(u) {
		return ErrUploadNotApproved
	}
	return s.store.AddPhoto(ctx, albumID, uploadID, a.PhotoCount)
}

// RemovePhoto detaches a photo from the album. The storage object is not
// deleted — the owner can re-add it or use it elsewhere.
func (s *Service) RemovePhoto(ctx context.Context, callerID, albumID, uploadID string) error {
	a, err := s.store.GetAlbum(ctx, albumID)
	if err != nil {
		return err
	}
	if a.OwnerID != callerID {
		return ErrForbidden
	}
	return s.store.RemovePhoto(ctx, albumID, uploadID)
}

// ListPhotos returns photos for any caller with access (owner or active grant).
// Owner gets the raw URL via the same gated path so the URL shape stays
// uniform; viewer always goes through the gated path.
func (s *Service) ListPhotos(ctx context.Context, callerID, albumID string) ([]Photo, error) {
	a, err := s.store.GetAlbum(ctx, albumID)
	if err != nil {
		return nil, err
	}
	role, err := s.roleFor(ctx, callerID, a)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, ErrForbidden
	}
	photos, err := s.store.ListPhotos(ctx, albumID)
	if err != nil {
		return nil, err
	}
	for i := range photos {
		photos[i].URL = "/albums/" + albumID + "/photos/" + photos[i].UploadID + "/file"
	}
	return photos, nil
}

// StreamPhoto returns the storage key, content type, and caller role for a
// photo if the caller has access. The handler streams the bytes and applies
// a deterrence watermark for viewer-role callers.
func (s *Service) StreamPhoto(ctx context.Context, callerID, albumID, uploadID string) (key, contentType, role string, err error) {
	a, err := s.store.GetAlbum(ctx, albumID)
	if err != nil {
		return "", "", "", err
	}
	role, err = s.roleFor(ctx, callerID, a)
	if err != nil {
		return "", "", "", err
	}
	if role == "" {
		return "", "", "", ErrForbidden
	}
	u, err := s.media.GetForOwner(ctx, uploadID, a.OwnerID)
	if err != nil {
		return "", "", "", ErrNotFound
	}
	// Serve-time moderation gate: never stream an image that has not cleared
	// the pipeline, even if it slipped into the album before scanning finished.
	// Opaque 404 so the status isn't leaked.
	if !uploadServable(u) {
		return "", "", "", ErrNotFound
	}
	photo, err := s.store.GetPhoto(ctx, albumID, uploadID)
	if err != nil {
		return "", "", "", err
	}
	_ = photo
	return u.StorageKey, u.ContentType, role, nil
}

// uploadServable reports whether an upload may be exposed to other users:
// image uploads must be moderation-approved; non-image types are not scanned
// by the image pipeline and are cleared on confirm.
func uploadServable(u *uploads.Upload) bool {
	if !strings.HasPrefix(u.ContentType, "image/") {
		return true
	}
	return u.ModerationStatus == "approved"
}

// InviteUser opens a 'pending' grant for grantee (owner → push). Returns
// ErrGrantExists if another open grant for the pair already exists; the
// caller can choose to revive that one via accept-from-grantee instead.
// expiresAt is optional; nil means no expiry.
func (s *Service) InviteUser(ctx context.Context, ownerID, albumID, granteeID string, expiresAt *time.Time) (*Grant, error) {
	if ownerID == granteeID {
		return nil, ErrSelfGrant
	}
	a, err := s.store.GetAlbum(ctx, albumID)
	if err != nil {
		return nil, err
	}
	if a.OwnerID != ownerID {
		return nil, ErrForbidden
	}
	if existing, err := s.store.FindOpenGrant(ctx, albumID, granteeID); err == nil && existing != nil {
		return nil, ErrGrantExists
	}
	g, err := s.store.CreateGrant(ctx, albumID, ownerID, granteeID, GrantActive, SourceInvite, expiresAt)
	if err != nil {
		return nil, err
	}
	if s.notifier != nil {
		s.notifier.AlbumInvited(ctx, granteeID, ownerID, albumID, a.Name)
	}
	g.Counterparty = granteeID
	return g, nil
}

// RequestAccess opens a 'pending' grant for the caller (viewer → pull).
// The owner approves via AcceptGrant.
func (s *Service) RequestAccess(ctx context.Context, requesterID, albumID string) (*Grant, error) {
	a, err := s.store.GetAlbum(ctx, albumID)
	if err != nil {
		return nil, err
	}
	if a.OwnerID == requesterID {
		return nil, ErrSelfGrant
	}
	if existing, err := s.store.FindOpenGrant(ctx, albumID, requesterID); err == nil && existing != nil {
		return nil, ErrGrantExists
	}
	g, err := s.store.CreateGrant(ctx, albumID, requesterID, requesterID, GrantPending, SourceRequest, nil)
	if err != nil {
		return nil, err
	}
	if s.notifier != nil {
		s.notifier.AlbumRequested(ctx, a.OwnerID, requesterID, albumID, a.Name)
	}
	g.Counterparty = a.OwnerID
	return g, nil
}

// ShareInChatRoom resolves the DM room's peer, ensures the peer has an
// active grant, and persists + broadcasts an 'album_share' chat message
// containing the album reference payload. expiresAt is optional.
func (s *Service) ShareInChatRoom(ctx context.Context, ownerID, albumID, roomID string, expiresAt *time.Time) (*Album, *Grant, error) {
	if s.chat == nil {
		return nil, nil, ErrInvalidRequest
	}
	peerID, err := s.chat.RoomPeer(ctx, roomID, ownerID)
	if err != nil {
		return nil, nil, err
	}
	a, g, err := s.ShareInChat(ctx, ownerID, albumID, peerID, expiresAt)
	if err != nil {
		return nil, nil, err
	}
	payload := buildAlbumSharePayload(a)
	if err := s.chat.SaveAlbumShareMessage(ctx, roomID, ownerID, payload); err != nil {
		return nil, nil, err
	}
	return a, g, nil
}

// buildAlbumSharePayload returns the JSON body the chat message carries
// in its content field. The frontend parses it to render the rich card.
func buildAlbumSharePayload(a *Album) string {
	out, _ := json.Marshal(struct {
		AlbumID    string `json:"album_id"`
		Name       string `json:"name"`
		PhotoCount int    `json:"photo_count"`
		OwnerID    string `json:"owner_id"`
	}{
		AlbumID:    a.ID,
		Name:       a.Name,
		PhotoCount: a.PhotoCount,
		OwnerID:    a.OwnerID,
	})
	return string(out)
}

// ShareInChat opens (or reuses) an active grant for grantee with
// source='chat'. expiresAt is optional; nil means permanent access.
// Used by ShareInChatRoom and by tests.
func (s *Service) ShareInChat(ctx context.Context, ownerID, albumID, granteeID string, expiresAt *time.Time) (*Album, *Grant, error) {
	if ownerID == granteeID {
		return nil, nil, ErrSelfGrant
	}
	a, err := s.store.GetAlbum(ctx, albumID)
	if err != nil {
		return nil, nil, err
	}
	if a.OwnerID != ownerID {
		return nil, nil, ErrForbidden
	}
	// If there's already an active grant, reuse it; if there's a pending
	// one, promote it to active; otherwise create a new active grant.
	existing, _ := s.store.FindOpenGrant(ctx, albumID, granteeID)
	var g *Grant
	if existing != nil {
		if existing.Status == GrantActive {
			g = existing
		} else {
			updated, err := s.store.UpdateGrantStatus(ctx, existing.ID, GrantActive)
			if err != nil {
				return nil, nil, err
			}
			g = updated
		}
	} else {
		created, err := s.store.CreateGrant(ctx, albumID, ownerID, granteeID, GrantActive, SourceChat, expiresAt)
		if err != nil {
			return nil, nil, err
		}
		g = created
	}
	return a, g, nil
}

// AcceptGrant moves a 'pending' request-access grant to 'active'. Only
// the album owner may accept; source must be 'request'.
func (s *Service) AcceptGrant(ctx context.Context, callerID, grantID string) (*Grant, error) {
	g, err := s.store.GetGrant(ctx, grantID)
	if err != nil {
		return nil, err
	}
	if g.Status != GrantPending || g.Source != SourceRequest {
		return nil, ErrInvalidRequest
	}
	a, err := s.store.GetAlbum(ctx, g.AlbumID)
	if err != nil {
		return nil, err
	}
	if callerID != a.OwnerID {
		return nil, ErrForbidden
	}
	updated, err := s.store.UpdateGrantStatus(ctx, grantID, GrantActive)
	if err != nil {
		return nil, err
	}
	if s.notifier != nil {
		s.notifier.AlbumGrantAccepted(ctx, g.GranteeID, callerID, g.AlbumID, a.Name)
	}
	return updated, nil
}

// DenyGrant closes a 'pending' request-access grant with status='denied'.
// Only the album owner may deny; source must be 'request'.
func (s *Service) DenyGrant(ctx context.Context, callerID, grantID string) (*Grant, error) {
	g, err := s.store.GetGrant(ctx, grantID)
	if err != nil {
		return nil, err
	}
	if g.Status != GrantPending || g.Source != SourceRequest {
		return nil, ErrInvalidRequest
	}
	a, err := s.store.GetAlbum(ctx, g.AlbumID)
	if err != nil {
		return nil, err
	}
	if callerID != a.OwnerID {
		return nil, ErrForbidden
	}
	return s.store.UpdateGrantStatus(ctx, grantID, GrantDenied)
}

// RevokeGrant closes an 'active' grant with status='revoked'. Either party
// may revoke — owner cuts access for the grantee, grantee opts out of an
// album they no longer want to see.
func (s *Service) RevokeGrant(ctx context.Context, callerID, grantID string) (*Grant, error) {
	g, err := s.store.GetGrant(ctx, grantID)
	if err != nil {
		return nil, err
	}
	if g.Status != GrantActive {
		return nil, ErrInvalidRequest
	}
	a, err := s.store.GetAlbum(ctx, g.AlbumID)
	if err != nil {
		return nil, err
	}
	if callerID != a.OwnerID && callerID != g.GranteeID {
		return nil, ErrForbidden
	}
	updated, err := s.store.UpdateGrantStatus(ctx, grantID, GrantRevoked)
	if err != nil {
		return nil, err
	}
	if s.notifier != nil {
		other := g.GranteeID
		if callerID == g.GranteeID {
			other = a.OwnerID
		}
		s.notifier.AlbumGrantRevoked(ctx, other, callerID, g.AlbumID, a.Name)
	}
	return updated, nil
}

// ListGrants returns grants for an album. Owner gets every row; a grantee
// gets only their own grant entry (useful so the viewer can see / revoke
// their own access without exposing other invitees).
func (s *Service) ListGrants(ctx context.Context, callerID, albumID string) ([]Grant, error) {
	a, err := s.store.GetAlbum(ctx, albumID)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListGrantsByAlbum(ctx, albumID)
	if err != nil {
		return nil, err
	}
	if callerID == a.OwnerID {
		for i := range rows {
			rows[i].Counterparty = rows[i].GranteeID
		}
		return rows, nil
	}
	// Viewer: only own rows, with counterparty = owner.
	filtered := rows[:0]
	for _, g := range rows {
		if g.GranteeID == callerID {
			g.Counterparty = a.OwnerID
			filtered = append(filtered, g)
		}
	}
	if len(filtered) == 0 {
		return nil, ErrForbidden
	}
	return filtered, nil
}

// roleFor returns "owner", "viewer", or "" depending on the caller's
// relationship with the album. Used as the single gate for read access.
func (s *Service) roleFor(ctx context.Context, callerID string, a *Album) (string, error) {
	if a.OwnerID == callerID {
		return "owner", nil
	}
	ok, err := s.store.HasActiveGrant(ctx, a.ID, callerID)
	if err != nil {
		return "", err
	}
	if ok {
		return "viewer", nil
	}
	return "", nil
}
