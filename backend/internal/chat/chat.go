package chat

import (
	"context"
	"errors"
	"time"
)

const (
	RoomTypeDM    = "dm"
	RoomTypeGroup = "group"

	MessageTypeText  = "text"
	MessageTypeImage = "image"
	MessageTypeVideo = "video"
	MessageTypeFile  = "file"
)

var (
	ErrNotFound  = errors.New("chat: not found")
	ErrForbidden = errors.New("chat: forbidden")
)

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
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	PeerID      string          `json:"peer_id,omitempty"`
	PeerName    string          `json:"peer_name,omitempty"`
	LastMessage *MessageSummary `json:"last_message"`
	UnreadCount int             `json:"unread_count"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Message is the full representation of a chat message including sender info.
type Message struct {
	ID         string     `json:"id"`
	RoomID     string     `json:"room_id"`
	SenderID   string     `json:"sender_id"`
	SenderName string     `json:"sender_name"`
	Type       string     `json:"type"`
	Content    string     `json:"content"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	ViewOnce   bool       `json:"view_once"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Store is the persistence contract for the chat package.
type Store interface {
	GetOrCreateDM(ctx context.Context, userID, peerID string) (*Room, error)
	CreateGroup(ctx context.Context, creatorID, name string, memberIDs []string) (*Room, error)
	IsMember(ctx context.Context, roomID, userID string) (bool, error)
	ListMembers(ctx context.Context, roomID string) ([]string, error)
	ListRooms(ctx context.Context, userID string) ([]RoomSummary, error)
	SaveMessage(ctx context.Context, roomID, senderID, msgType, content string) (*Message, error)
	ListMessages(ctx context.Context, roomID string, before *time.Time, limit int) ([]Message, error)
	MarkRead(ctx context.Context, roomID, userID string) error
}

// Manager is the interface used by HTTP and WebSocket handlers.
type Manager interface {
	GetOrCreateDM(ctx context.Context, userID, peerID string) (*Room, error)
	CreateGroup(ctx context.Context, creatorID, name string, memberIDs []string) (*Room, error)
	IsMember(ctx context.Context, roomID, userID string) (bool, error)
	ListMembers(ctx context.Context, roomID string) ([]string, error)
	ListRooms(ctx context.Context, userID string) ([]RoomSummary, error)
	SaveMessage(ctx context.Context, roomID, senderID, msgType, content string) (*Message, error)
	ListMessages(ctx context.Context, roomID string, before *time.Time, limit int) ([]Message, error)
	MarkRead(ctx context.Context, roomID, userID string) error
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

func (s *Service) SaveMessage(ctx context.Context, roomID, senderID, msgType, content string) (*Message, error) {
	return s.store.SaveMessage(ctx, roomID, senderID, msgType, content)
}

func (s *Service) ListMessages(ctx context.Context, roomID string, before *time.Time, limit int) ([]Message, error) {
	return s.store.ListMessages(ctx, roomID, before, limit)
}

func (s *Service) MarkRead(ctx context.Context, roomID, userID string) error {
	return s.store.MarkRead(ctx, roomID, userID)
}
