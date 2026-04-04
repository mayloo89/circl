package chat

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	RoomTypeDM      = "dm"
	RoomTypeGroup   = "group"
	RoomTypeChannel = "channel"

	MessageTypeText  = "text"
	MessageTypeImage = "image"
	MessageTypeVideo = "video"
	MessageTypeFile  = "file"

	// TTL label constants for ephemeral messages.
	TTL15Min  = "15m"
	TTL30Min  = "30m"
	TTL1Hour  = "1h"
	TTL6Hours = "6h"
	TTL12Hours = "12h"
	TTL24Hour = "24h"
)

var (
	ErrNotFound  = errors.New("chat: not found")
	ErrForbidden = errors.New("chat: forbidden")
)

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
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ChannelSummary is returned by ListChannels for the public channel directory.
type ChannelSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatorID   string    `json:"creator_id,omitempty"`
	MemberCount int       `json:"member_count"`
	IsMember    bool      `json:"is_member"`
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
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Name          string          `json:"name"`
	Description   string          `json:"description,omitempty"`
	CreatorID     string          `json:"creator_id,omitempty"`
	PeerID        string          `json:"peer_id,omitempty"`
	PeerUsername  string          `json:"peer_username,omitempty"`
	PeerName      string          `json:"peer_name,omitempty"`
	PeerAvatarURL string          `json:"peer_avatar_url,omitempty"`
	// PeerLastReadAt is the peer's last_read_at timestamp for DM rooms.
	// Used to seed the initial read-receipt state without a round-trip.
	PeerLastReadAt *time.Time      `json:"peer_last_read_at,omitempty"`
	LastMessage    *MessageSummary `json:"last_message"`
	UnreadCount    int             `json:"unread_count"`
	CreatedAt      time.Time       `json:"created_at"`
}

// Message is the full representation of a chat message including sender info.
type Message struct {
	ID              string     `json:"id"`
	RoomID          string     `json:"room_id"`
	SenderID        string     `json:"sender_id"`
	SenderName      string     `json:"sender_name"`
	SenderAvatarURL string     `json:"sender_avatar_url"`
	Type            string     `json:"type"`
	Content         string     `json:"content"`
	// ThumbnailURL is the public URL of the image thumbnail, generated
	// asynchronously after upload. Empty for non-image messages.
	ThumbnailURL    string     `json:"thumbnail_url,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	ViewOnce        bool       `json:"view_once"`
	// Tombstone is true when the message content has been permanently erased
	// (view-once viewed or TTL expired). The record is kept so the chat
	// history can show a placeholder where the message used to be.
	Tombstone bool `json:"tombstone,omitempty"`
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
}

// Store is the persistence contract for the chat package.
type Store interface {
	GetOrCreateDM(ctx context.Context, userID, peerID string) (*Room, error)
	CreateGroup(ctx context.Context, creatorID, name string, memberIDs []string) (*Room, error)
	// CreateChannel creates a public channel room and auto-joins the creator.
	CreateChannel(ctx context.Context, creatorID, name, description string) (*Room, error)
	// ListChannels returns all public channels with member counts and membership status for userID.
	ListChannels(ctx context.Context, userID string) ([]ChannelSummary, error)
	// JoinChannel adds userID to a channel room.
	JoinChannel(ctx context.Context, roomID, userID string) error
	// LeaveChannel removes userID from a channel room. Anyone may leave, including the creator.
	LeaveChannel(ctx context.Context, roomID, userID string) error
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
}

// Manager is the interface used by HTTP and WebSocket handlers.
type Manager interface {
	GetOrCreateDM(ctx context.Context, userID, peerID string) (*Room, error)
	CreateGroup(ctx context.Context, creatorID, name string, memberIDs []string) (*Room, error)
	CreateChannel(ctx context.Context, creatorID, name, description string) (*Room, error)
	ListChannels(ctx context.Context, userID string) ([]ChannelSummary, error)
	JoinChannel(ctx context.Context, roomID, userID string) error
	LeaveChannel(ctx context.Context, roomID, userID string) error
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
}

// Service is the application-layer implementation of Manager.
// It is a thin delegation layer over Store — business rules live here when
// they grow beyond simple persistence calls.
type Service struct {
	store Store
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

func (s *Service) ListChannels(ctx context.Context, userID string) ([]ChannelSummary, error) {
	return s.store.ListChannels(ctx, userID)
}

func (s *Service) JoinChannel(ctx context.Context, roomID, userID string) error {
	return s.store.JoinChannel(ctx, roomID, userID)
}

func (s *Service) LeaveChannel(ctx context.Context, roomID, userID string) error {
	return s.store.LeaveChannel(ctx, roomID, userID)
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
	return s.store.SaveMessage(ctx, p)
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
