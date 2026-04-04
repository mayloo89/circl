package notifications

import (
	"slices"
	"sync"
)

// Event is a notification pushed to a connected client.
type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

// Notifier is the interface used by other packages to push events to users.
// *Hub satisfies this interface.
type Notifier interface {
	Notify(userID string, e Event)
}

// Hub manages active SSE subscriber channels keyed by user ID.
// A single user may have multiple concurrent connections (e.g. two tabs).
type Hub struct {
	mu          sync.RWMutex
	subscribers map[string][]chan Event
}

// NewHub creates a ready-to-use Hub.
func NewHub() *Hub {
	return &Hub{subscribers: make(map[string][]chan Event)}
}

// Subscribe registers a new channel for the given user and returns it along
// with an unsubscribe function that must be called when the connection closes.
// The channel is buffered (capacity 8); events are dropped if it fills up.
func (h *Hub) Subscribe(userID string) (<-chan Event, func()) {
	ch := make(chan Event, 8)

	h.mu.Lock()
	h.subscribers[userID] = append(h.subscribers[userID], ch)
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		subs := h.subscribers[userID]
		if idx := slices.Index(subs, ch); idx >= 0 {
			h.subscribers[userID] = slices.Delete(subs, idx, idx+1)
		}
		if len(h.subscribers[userID]) == 0 {
			delete(h.subscribers, userID)
		}
		close(ch)
	}
}

// IsConnected reports whether the user has at least one active SSE connection.
// Used to decide whether a web push notification is needed.
func (h *Hub) IsConnected(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers[userID]) > 0
}

// Notify sends an event to all active connections for the given user.
// If a subscriber's channel is full the event is dropped for that subscriber
// rather than blocking the caller.
func (h *Hub) Notify(userID string, e Event) {
	h.mu.RLock()
	subs := h.subscribers[userID]
	h.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- e:
		default:
			// channel full — drop rather than block
		}
	}
}
