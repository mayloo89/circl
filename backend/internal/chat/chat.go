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
	ID        string
	Type      string
	Name      string
	DMKey     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MessageSummary is a lightweight view of the most recent message in a room,
// used when listing rooms without fetching full message history.
type MessageSummary struct {
	SenderID  string
	Content   string
	CreatedAt time.Time
}

// RoomSummary is returned by ListRooms and contains everything the UI needs
// to render a conversation list entry without extra round-trips.
type RoomSummary struct {
	ID          string
	Type        string
	Name        string
	PeerID      string // non-empty for DMs
	PeerName    string // non-empty for DMs
	LastMessage *MessageSummary
	UnreadCount int
	CreatedAt   time.Time
}

// Message is the full representation of a chat message including sender info.
type Message struct {
	ID         string
	RoomID     string
	SenderID   string
	SenderName string
	Type       string
	Content    string
	ExpiresAt  *time.Time
	ViewOnce   bool
	CreatedAt  time.Time
}

// Store is the persistence contract for the chat package.
type Store interface {
	GetOrCreateDM(ctx context.Context, userID, peerID string) (*Room, error)
	CreateGroup(ctx context.Context, creatorID, name string, memberIDs []string) (*Room, error)
	IsMember(ctx context.Context, roomID, userID string) (bool, error)
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
