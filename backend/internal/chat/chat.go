package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mayloo89/circl/backend/internal/redact"
)

const (
	RoomTypeDM      = "dm"
	RoomTypeGroup   = "group"
	RoomTypeChannel = "channel"
	RoomTypePublic  = "public"

	RoomVisibilityPublic = "public"

	MessageTypeText       = "text"
	MessageTypeImage      = "image"
	MessageTypeVideo      = "video"
	MessageTypeFile       = "file"
	MessageTypeAlbumShare = "album_share"
	MessageTypeSystem     = "system"

	// TTL label constants for ephemeral messages.
	TTL15Min   = "15m"
	TTL30Min   = "30m"
	TTL1Hour   = "1h"
	TTL6Hours  = "6h"
	TTL12Hours = "12h"
	TTL24Hour  = "24h"
)

var (
	ErrNotFound  = errors.New("chat: not found")
	ErrForbidden = errors.New("chat: forbidden")
	// ErrUploadNotApproved is returned when a message references an image
	// attachment that has not cleared moderation (still pending, rejected, or
	// quarantined). The image must never reach a recipient.
	ErrUploadNotApproved = errors.New("chat: attachment not approved")
)

// RetentionDurations maps room types to their data-retention window. Messages
// older than the window are hard-deleted by the retention sweeper (the row is
// removed; no tombstone is left). This map is the single source of truth —
// ListRetentionEligibleMessages builds its query from it.
//
// DM and group rooms retain for ~3 months — both are relationship spaces, so
// they share the window. Public guest rooms retain for 24h — they are
// high-volume, low-value transient spaces. Channels are absent because they
// are broadcast-only — their messages are never persisted (see the WS send
// loop in handler.go), so there is nothing to retain or sweep.
var RetentionDurations = map[string]time.Duration{
	RoomTypeDM:     3 * 30 * 24 * time.Hour, // ~3 months
	RoomTypeGroup:  3 * 30 * 24 * time.Hour, // ~3 months
	RoomTypePublic: 24 * time.Hour,          // 24 hours
}

var ttlDurations = map[string]time.Duration{
	TTL15Min:   15 * time.Minute,
	TTL30Min:   30 * time.Minute,
	TTL1Hour:   time.Hour,
	TTL6Hours:  6 * time.Hour,
	TTL12Hours: 12 * time.Hour,
	TTL24Hour:  24 * time.Hour,
}

// ParseTTL returns the Duration for a TTL label.
// Valid labels are "15m", "30m", "1h", "6h", "12h", "24h".
func ParseTTL(ttl string) (time.Duration, error) {
	d, ok := ttlDurations[ttl]
	if !ok {
		return 0, fmt.Errorf("chat: invalid ttl %q; valid values: 15m, 30m, 1h, 6h, 12h, 24h", ttl)
	}
	return d, nil
}

type Room struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	DMKey       string    `json:"dm_key,omitempty"`
	CreatorID   string    `json:"creator_id,omitempty"`
	Description string    `json:"description,omitempty"`
	Visibility  string    `json:"visibility,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ChannelSummary is returned by ListChannels for the public channel directory.
// ActiveCount is populated by the handler from the Hub (not stored in the DB)
// because channel membership is ephemeral — a user is "in" a channel only while
// their WebSocket connection is open.
type ChannelSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatorID   string    `json:"creator_id,omitempty"`
	ActiveCount int       `json:"active_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// PublicRoomSummary is returned by ListPublicRooms for the guest-facing
// directory. ActiveCount is populated by the handler from the Hub.
type PublicRoomSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ActiveCount int       `json:"active_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// MemberProfile is a lightweight view of a room member returned by ListMemberProfiles.
type MemberProfile struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	IsAdmin     bool      `json:"is_admin"`
	JoinedAt    time.Time `json:"joined_at"`
}

// MessageSummary is a lightweight view of the most recent message in a room,
// used when listing rooms without fetching full message history.
type MessageSummary struct {
	SenderID  string    `json:"sender_id"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// RoomSummary is returned by ListRooms and contains everything the UI needs
// to render a conversation list entry without extra round-trips.
type RoomSummary struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	CreatorID     string `json:"creator_id,omitempty"`
	PeerID        string `json:"peer_id,omitempty"`
	PeerUsername  string `json:"peer_username,omitempty"`
	PeerName      string `json:"peer_name,omitempty"`
	PeerAvatarURL string `json:"peer_avatar_url,omitempty"`
	// PeerLastReadAt is the peer's last_read_at timestamp for DM rooms.
	// Used to seed the initial read-receipt state without a round-trip.
	PeerLastReadAt *time.Time      `json:"peer_last_read_at,omitzero"`
	LastMessage    *MessageSummary `json:"last_message"`
	UnreadCount    int             `json:"unread_count"`
	CreatedAt      time.Time       `json:"created_at"`
}

// Message is the full representation of a chat message including sender info.
type Message struct {
	ID              string `json:"id"`
	RoomID          string `json:"room_id"`
	SenderID        string `json:"sender_id"`
	SenderName      string `json:"sender_name"`
	SenderAvatarURL string `json:"sender_avatar_url"`
	Type            string `json:"type"`
	Content         string `json:"content"`
	// ThumbnailURL is the public URL of the image thumbnail, generated
	// asynchronously after upload. Empty for non-image messages.
	ThumbnailURL string     `json:"thumbnail_url,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitzero"`
	ViewOnce     bool       `json:"view_once"`
	// Tombstone is true when the message content has been permanently erased
	// (view-once viewed or TTL expired). The record is kept so the chat
	// history can show a placeholder where the message used to be.
	Tombstone bool `json:"tombstone,omitempty"`
	// Redacted is true when the message content contained external contact
	// info and was replaced with a redaction token before persistence.
	Redacted  bool      `json:"redacted,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// SaveMessageParams are the inputs for persisting a new message.
type SaveMessageParams struct {
	RoomID   string
	SenderID string
	Type     string
	Content  string
	// UploadID, when non-empty, links the upload record to this message
	// so attachments can be cleaned up when the message is deleted.
	UploadID string
	// ViewOnce marks the message as a view-once ephemeral message.
	ViewOnce bool
	// ExpiresAt, when non-nil, marks the message as TTL-based ephemeral.
	ExpiresAt *time.Time
	// Redacted is set to true when contact-info detection has replaced
	// the original content with a redaction token.
	Redacted bool
	// SenderIsGuest is true when the sender is an unregistered guest.
	// Triggers unconditional contact-info redaction in public rooms.
	SenderIsGuest bool
	// SenderName overrides the display name resolved from the DB (used
	// for guest nicknames that are not backed by a profiles row).
	SenderName string
}

// Store is the persistence contract for the chat package.
type Store interface {
	GetOrCreateDM(ctx context.Context, userID, peerID string) (*Room, error)
	CreateGroup(ctx context.Context, creatorID, name string, memberIDs []string) (*Room, error)
	// CreateChannel creates a public channel room.
	CreateChannel(ctx context.Context, creatorID, name, description string) (*Room, error)
	// ListChannels returns all public channels ordered by creation date.
	ListChannels(ctx context.Context) ([]ChannelSummary, error)
	// CreatePublicRoom creates a public guest-accessible room.
	CreatePublicRoom(ctx context.Context, creatorID, name, description string) (*Room, error)
	// ListPublicRooms returns all public rooms ordered by creation date.
	ListPublicRooms(ctx context.Context) ([]PublicRoomSummary, error)
	// NicknameTaken returns true when the nickname matches a registered user's
	// username or display name, preventing guest impersonation.
	NicknameTaken(ctx context.Context, nickname string) (bool, error)
	// GetRoom returns the room record for the given ID.
	GetRoom(ctx context.Context, roomID string) (*Room, error)
	IsMember(ctx context.Context, roomID, userID string) (bool, error)
	ListMembers(ctx context.Context, roomID string) ([]string, error)
	// ListMemberProfiles returns full profile data for every member of the room.
	ListMemberProfiles(ctx context.Context, roomID string) ([]MemberProfile, error)
	// AddGroupMember adds targetID to a group room. actorID must be the creator.
	AddGroupMember(ctx context.Context, roomID, actorID, targetID string) error
	// RemoveGroupMember removes targetID from a group room. actorID must be the
	// creator (to remove others) or the same as targetID (self-leave).
	// The creator cannot be removed.
	RemoveGroupMember(ctx context.Context, roomID, actorID, targetID string) error
	// UpdateGroupName renames a group room. actorID must be the creator.
	UpdateGroupName(ctx context.Context, roomID, actorID, name string) error
	ListRooms(ctx context.Context, userID string) ([]RoomSummary, error)
	SaveMessage(ctx context.Context, p SaveMessageParams) (*Message, error)
	ListMessages(ctx context.Context, roomID string, before *time.Time, limit int) ([]Message, error)
	// MarkRead updates the user's last_read_at for the room and returns the
	// timestamp that was written so callers can broadcast read-receipt events.
	MarkRead(ctx context.Context, roomID, userID string) (time.Time, error)
	// ViewOnceMessage atomically records that viewerID has seen the message and,
	// if all non-sender members have now viewed it, deletes the message from the
	// database and returns the storage keys to clean up from object storage.
	ViewOnceMessage(ctx context.Context, messageID, roomID, viewerID string) (msg *Message, storageKeys []string, err error)
	// DeleteMessage deletes a message and its linked uploads from the database
	// and returns the room ID and any storage keys to remove from object storage.
	DeleteMessage(ctx context.Context, messageID string) (roomID string, storageKeys []string, err error)
	// TombstoneMessage converts an expired TTL message into a tombstone: it
	// erases the content and marks the record as tombstone=true so the chat
	// history can show a placeholder. Returns the room ID and any storage keys
	// to remove from object storage.
	TombstoneMessage(ctx context.Context, messageID string) (roomID string, storageKeys []string, err error)
	// ListExpiredMessages returns the IDs of messages whose expires_at has passed.
	ListExpiredMessages(ctx context.Context) ([]string, error)
	// GetDisplayName returns the display_name for the given user from their profile.
	// Returns an empty string when no profile row exists.
	GetDisplayName(ctx context.Context, userID string) (string, error)
	// GetAvatarURL returns the avatar_url for the given user from their profile.
	// Returns an empty string when no profile row exists.
	GetAvatarURL(ctx context.Context, userID string) (string, error)
	// GetUsername returns the username for the given user.
	// Returns an empty string when the user does not exist.
	GetUsername(ctx context.Context, userID string) (string, error)
	// GetDMPeerID returns the other member's user ID in a DM room.
	GetDMPeerID(ctx context.Context, roomID, userID string) (string, error)
	// ListRetentionEligibleMessages returns the IDs of messages that have
	// exceeded the retention window for their room type. Only non-tombstoned
	// messages in room types with a defined retention policy are returned.
	ListRetentionEligibleMessages(ctx context.Context) ([]string, error)
}

// Manager is the interface used by HTTP and WebSocket handlers.
type Manager interface {
	GetOrCreateDM(ctx context.Context, userID, peerID string) (*Room, error)
	CreateGroup(ctx context.Context, creatorID, name string, memberIDs []string) (*Room, error)
	CreateChannel(ctx context.Context, creatorID, name, description string) (*Room, error)
	ListChannels(ctx context.Context) ([]ChannelSummary, error)
	CreatePublicRoom(ctx context.Context, creatorID, name, description string) (*Room, error)
	ListPublicRooms(ctx context.Context) ([]PublicRoomSummary, error)
	NicknameTaken(ctx context.Context, nickname string) (bool, error)
	GetRoom(ctx context.Context, roomID string) (*Room, error)
	IsMember(ctx context.Context, roomID, userID string) (bool, error)
	ListMembers(ctx context.Context, roomID string) ([]string, error)
	ListMemberProfiles(ctx context.Context, roomID string) ([]MemberProfile, error)
	AddGroupMember(ctx context.Context, roomID, actorID, targetID string) error
	RemoveGroupMember(ctx context.Context, roomID, actorID, targetID string) error
	UpdateGroupName(ctx context.Context, roomID, actorID, name string) error
	ListRooms(ctx context.Context, userID string) ([]RoomSummary, error)
	SaveMessage(ctx context.Context, p SaveMessageParams) (*Message, error)
	ListMessages(ctx context.Context, roomID string, before *time.Time, limit int) ([]Message, error)
	// MarkRead updates the user's last_read_at for the room and returns the
	// timestamp that was written so callers can broadcast read-receipt events.
	MarkRead(ctx context.Context, roomID, userID string) (time.Time, error)
	ViewOnceMessage(ctx context.Context, messageID, roomID, viewerID string) (msg *Message, storageKeys []string, err error)
	DeleteMessage(ctx context.Context, messageID string) (roomID string, storageKeys []string, err error)
	TombstoneMessage(ctx context.Context, messageID string) (roomID string, storageKeys []string, err error)
	ListExpiredMessages(ctx context.Context) ([]string, error)
	// GetDisplayName returns the display_name for the given user from their profile.
	// Returns an empty string when no profile row exists.
	GetDisplayName(ctx context.Context, userID string) (string, error)
	// GetAvatarURL returns the avatar_url for the given user from their profile.
	// Returns an empty string when no profile row exists.
	GetAvatarURL(ctx context.Context, userID string) (string, error)
	// GetUsername returns the username for the given user.
	// Returns an empty string when the user does not exist.
	GetUsername(ctx context.Context, userID string) (string, error)
	// GetDMPeerID returns the other member's user ID in a DM room.
	GetDMPeerID(ctx context.Context, roomID, userID string) (string, error)
	// ListRetentionEligibleMessages returns the IDs of messages that have
	// exceeded the retention window for their room type.
	ListRetentionEligibleMessages(ctx context.Context) ([]string, error)
}

// Service is the application-layer implementation of Manager.
// It is a thin delegation layer over Store — business rules live here when
// they grow beyond simple persistence calls.
type Service struct {
	store          Store
	AreContacts    func(ctx context.Context, userA, userB string) (bool, error)
	IsExemptSender func(ctx context.Context, senderID string) bool
	// UploadApproved, when set, reports whether the image upload backing a
	// message attachment (owned by senderID) has cleared moderation.
	// SaveMessage refuses to persist a message whose attachment is not yet
	// approved, so an un-moderated or rejected image never reaches a recipient.
	UploadApproved func(ctx context.Context, uploadID, senderID string) (bool, error)
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) GetOrCreateDM(ctx context.Context, userID, peerID string) (*Room, error) {
	return s.store.GetOrCreateDM(ctx, userID, peerID)
}

func (s *Service) CreateGroup(ctx context.Context, creatorID, name string, memberIDs []string) (*Room, error) {
	return s.store.CreateGroup(ctx, creatorID, name, memberIDs)
}

func (s *Service) CreateChannel(ctx context.Context, creatorID, name, description string) (*Room, error) {
	return s.store.CreateChannel(ctx, creatorID, name, description)
}

func (s *Service) ListChannels(ctx context.Context) ([]ChannelSummary, error) {
	return s.store.ListChannels(ctx)
}

func (s *Service) CreatePublicRoom(ctx context.Context, creatorID, name, description string) (*Room, error) {
	return s.store.CreatePublicRoom(ctx, creatorID, name, description)
}

func (s *Service) ListPublicRooms(ctx context.Context) ([]PublicRoomSummary, error) {
	return s.store.ListPublicRooms(ctx)
}

func (s *Service) NicknameTaken(ctx context.Context, nickname string) (bool, error) {
	return s.store.NicknameTaken(ctx, nickname)
}

func (s *Service) GetRoom(ctx context.Context, roomID string) (*Room, error) {
	return s.store.GetRoom(ctx, roomID)
}

func (s *Service) IsMember(ctx context.Context, roomID, userID string) (bool, error) {
	return s.store.IsMember(ctx, roomID, userID)
}

func (s *Service) ListMembers(ctx context.Context, roomID string) ([]string, error) {
	return s.store.ListMembers(ctx, roomID)
}

func (s *Service) ListMemberProfiles(ctx context.Context, roomID string) ([]MemberProfile, error) {
	return s.store.ListMemberProfiles(ctx, roomID)
}

func (s *Service) AddGroupMember(ctx context.Context, roomID, actorID, targetID string) error {
	return s.store.AddGroupMember(ctx, roomID, actorID, targetID)
}

func (s *Service) RemoveGroupMember(ctx context.Context, roomID, actorID, targetID string) error {
	return s.store.RemoveGroupMember(ctx, roomID, actorID, targetID)
}

func (s *Service) UpdateGroupName(ctx context.Context, roomID, actorID, name string) error {
	return s.store.UpdateGroupName(ctx, roomID, actorID, name)
}

func (s *Service) ListRooms(ctx context.Context, userID string) ([]RoomSummary, error) {
	return s.store.ListRooms(ctx, userID)
}

func (s *Service) SaveMessage(ctx context.Context, p SaveMessageParams) (*Message, error) {
	// Attach-time moderation gate: an image attachment must have cleared the
	// pipeline before its message can be persisted and broadcast. This is what
	// keeps a pending/rejected/quarantined image from ever reaching a recipient.
	if p.UploadID != "" && s.UploadApproved != nil {
		approved, err := s.UploadApproved(ctx, p.UploadID, p.SenderID)
		if err != nil {
			return nil, err
		}
		if !approved {
			return nil, ErrUploadNotApproved
		}
	}
	s.maybeRedact(ctx, &p)
	return s.store.SaveMessage(ctx, p)
}

func (s *Service) maybeRedact(ctx context.Context, p *SaveMessageParams) {
	if p.Type != MessageTypeText {
		return
	}
	if s.AreContacts == nil || s.IsExemptSender == nil {
		return
	}
	room, err := s.store.GetRoom(ctx, p.RoomID)
	if err != nil {
		return
	}
	// Guest messages in public rooms are always redacted for contact info.
	if room.Type == RoomTypePublic && p.SenderIsGuest {
		res := redact.Redact(p.Content)
		if res.Redacted {
			p.Content = res.Content
			p.Redacted = true
		}
		return
	}
	if room.Type != RoomTypeDM {
		return
	}
	if s.IsExemptSender(ctx, p.SenderID) {
		return
	}
	members, err := s.store.ListMembers(ctx, p.RoomID)
	if err != nil {
		return
	}
	var peerID string
	for _, m := range members {
		if m != p.SenderID {
			peerID = m
			break
		}
	}
	if peerID == "" {
		return
	}
	accepted, err := s.AreContacts(ctx, p.SenderID, peerID)
	if err != nil || accepted {
		return
	}
	res := redact.Redact(p.Content)
	if res.Redacted {
		p.Content = res.Content
		p.Redacted = true
	}
}

func (s *Service) ListMessages(ctx context.Context, roomID string, before *time.Time, limit int) ([]Message, error) {
	return s.store.ListMessages(ctx, roomID, before, limit)
}

func (s *Service) MarkRead(ctx context.Context, roomID, userID string) (time.Time, error) {
	return s.store.MarkRead(ctx, roomID, userID)
}

func (s *Service) ViewOnceMessage(ctx context.Context, messageID, roomID, viewerID string) (*Message, []string, error) {
	return s.store.ViewOnceMessage(ctx, messageID, roomID, viewerID)
}

func (s *Service) DeleteMessage(ctx context.Context, messageID string) (string, []string, error) {
	return s.store.DeleteMessage(ctx, messageID)
}

func (s *Service) TombstoneMessage(ctx context.Context, messageID string) (string, []string, error) {
	return s.store.TombstoneMessage(ctx, messageID)
}

func (s *Service) ListExpiredMessages(ctx context.Context) ([]string, error) {
	return s.store.ListExpiredMessages(ctx)
}

func (s *Service) GetDisplayName(ctx context.Context, userID string) (string, error) {
	return s.store.GetDisplayName(ctx, userID)
}

func (s *Service) GetAvatarURL(ctx context.Context, userID string) (string, error) {
	return s.store.GetAvatarURL(ctx, userID)
}

func (s *Service) GetUsername(ctx context.Context, userID string) (string, error) {
	return s.store.GetUsername(ctx, userID)
}

func (s *Service) GetDMPeerID(ctx context.Context, roomID, userID string) (string, error) {
	return s.store.GetDMPeerID(ctx, roomID, userID)
}

func (s *Service) ListRetentionEligibleMessages(ctx context.Context) ([]string, error) {
	return s.store.ListRetentionEligibleMessages(ctx)
}
