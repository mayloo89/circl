package chat

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	RoomTypeDM    = "dm"
	RoomTypeGroup = "group"

	MessageTypeText  = "text"
	MessageTypeImage = "image"
	MessageTypeVideo = "video"
	MessageTypeFile  = "file"

	// TTL label constants for ephemeral messages.
	TTL1Hour  = "1h"
	TTL24Hour = "24h"
	TTL7Days  = "7d"
)

var (
	ErrNotFound  = errors.New("chat: not found")
	ErrForbidden = errors.New("chat: forbidden")
)

var ttlDurations = map[string]time.Duration{
	TTL1Hour:  time.Hour,
	TTL24Hour: 24 * time.Hour,
	TTL7Days:  7 * 24 * time.Hour,
}

// ParseTTL returns the Duration for a TTL label.
// Valid labels are "1h", "24h", "7d".
func ParseTTL(ttl string) (time.Duration, error) {
	d, ok := ttlDurations[ttl]
	if !ok {
		return 0, fmt.Errorf("chat: invalid ttl %q; valid values: 1h, 24h, 7d", ttl)
	}
	return d, nil
}

type Room struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	DMKey     string    `json:"dm_key,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MessageSummary is a lightweight view of the most recent message in a room,
// used when listing rooms without fetching full message history.
type MessageSummary struct {
	SenderID  string    `json:"sender_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// RoomSummary is returned by ListRooms and contains everything the UI needs
// to render a conversation list entry without extra round-trips.
type RoomSummary struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Name          string          `json:"name"`
	PeerID        string          `json:"peer_id,omitempty"`
	PeerName      string          `json:"peer_name,omitempty"`
	PeerAvatarURL string          `json:"peer_avatar_url,omitempty"`
	LastMessage   *MessageSummary `json:"last_message"`
	UnreadCount   int             `json:"unread_count"`
	CreatedAt     time.Time       `json:"created_at"`
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
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	ViewOnce        bool       `json:"view_once"`
	CreatedAt       time.Time  `json:"created_at"`
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
	IsMember(ctx context.Context, roomID, userID string) (bool, error)
	ListMembers(ctx context.Context, roomID string) ([]string, error)
	ListRooms(ctx context.Context, userID string) ([]RoomSummary, error)
	SaveMessage(ctx context.Context, p SaveMessageParams) (*Message, error)
	ListMessages(ctx context.Context, roomID string, before *time.Time, limit int) ([]Message, error)
	MarkRead(ctx context.Context, roomID, userID string) error
	// ViewOnceMessage atomically records that viewerID has seen the message and,
	// if all non-sender members have now viewed it, deletes the message from the
	// database and returns the storage keys to clean up from object storage.
	ViewOnceMessage(ctx context.Context, messageID, roomID, viewerID string) (msg *Message, storageKeys []string, err error)
	// DeleteMessage deletes a message and its linked uploads from the database
	// and returns the room ID and any storage keys to remove from object storage.
	DeleteMessage(ctx context.Context, messageID string) (roomID string, storageKeys []string, err error)
	// ListExpiredMessages returns the IDs of messages whose expires_at has passed.
	ListExpiredMessages(ctx context.Context) ([]string, error)
}

// Manager is the interface used by HTTP and WebSocket handlers.
type Manager interface {
	GetOrCreateDM(ctx context.Context, userID, peerID string) (*Room, error)
	CreateGroup(ctx context.Context, creatorID, name string, memberIDs []string) (*Room, error)
	IsMember(ctx context.Context, roomID, userID string) (bool, error)
	ListMembers(ctx context.Context, roomID string) ([]string, error)
	ListRooms(ctx context.Context, userID string) ([]RoomSummary, error)
	SaveMessage(ctx context.Context, p SaveMessageParams) (*Message, error)
	ListMessages(ctx context.Context, roomID string, before *time.Time, limit int) ([]Message, error)
	MarkRead(ctx context.Context, roomID, userID string) error
	ViewOnceMessage(ctx context.Context, messageID, roomID, viewerID string) (msg *Message, storageKeys []string, err error)
	DeleteMessage(ctx context.Context, messageID string) (roomID string, storageKeys []string, err error)
	ListExpiredMessages(ctx context.Context) ([]string, error)
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

func (s *Service) IsMember(ctx context.Context, roomID, userID string) (bool, error) {
	return s.store.IsMember(ctx, roomID, userID)
}

func (s *Service) ListMembers(ctx context.Context, roomID string) ([]string, error) {
	return s.store.ListMembers(ctx, roomID)
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

func (s *Service) MarkRead(ctx context.Context, roomID, userID string) error {
	return s.store.MarkRead(ctx, roomID, userID)
}

func (s *Service) ViewOnceMessage(ctx context.Context, messageID, roomID, viewerID string) (*Message, []string, error) {
	return s.store.ViewOnceMessage(ctx, messageID, roomID, viewerID)
}

func (s *Service) DeleteMessage(ctx context.Context, messageID string) (string, []string, error) {
	return s.store.DeleteMessage(ctx, messageID)
}

func (s *Service) ListExpiredMessages(ctx context.Context) ([]string, error) {
	return s.store.ListExpiredMessages(ctx)
}
