package chat

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestHub(t *testing.T) (*Hub, context.CancelFunc) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	hub := NewHub(rdb)
	ctx, cancel := context.WithCancel(t.Context())
	go hub.Run(ctx)
	return hub, cancel
}

// mustRegister sends client to hub.register and polls RoomParticipants until
// the registration is confirmed, replacing fragile time.Sleep calls.
func mustRegister(t *testing.T, hub *Hub, client *Client) {
	t.Helper()
	hub.register <- client
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	for {
		for _, p := range hub.RoomParticipants(ctx, client.roomID) {
			if p.UserID == client.userID {
				return
			}
		}
		if ctx.Err() != nil {
			t.Fatal("timeout waiting for client to be registered in hub")
		}
	}
}

func receiveWithTimeout(t *testing.T, ch <-chan []byte, timeout time.Duration) []byte {
	t.Helper()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(timeout):
		t.Fatal("timeout waiting for message delivery")
		return nil
	}
}

func TestHub_LocalDelivery(t *testing.T) {
	hub, cancel := newTestHub(t)
	defer cancel()

	client := &Client{
		hub:    hub,
		send:   make(chan []byte, 8),
		userID: "u-1",
		roomID: "room-delivery",
	}

	mustRegister(t, hub, client)

	payload := []byte(`{"type":"message","content":"hello"}`)
	if err := hub.Publish(context.Background(), "room-delivery", payload); err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	got := receiveWithTimeout(t, client.send, 500*time.Millisecond)
	if string(got) != string(payload) {
		t.Errorf("got %s, want %s", got, payload)
	}
}

func TestHub_MultipleClientsInRoom(t *testing.T) {
	hub, cancel := newTestHub(t)
	defer cancel()

	c1 := &Client{hub: hub, send: make(chan []byte, 8), userID: "u-1", roomID: "room-multi"}
	c2 := &Client{hub: hub, send: make(chan []byte, 8), userID: "u-2", roomID: "room-multi"}

	mustRegister(t, hub, c1)
	mustRegister(t, hub, c2)

	payload := []byte(`{"type":"message"}`)
	if err := hub.Publish(context.Background(), "room-multi", payload); err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	receiveWithTimeout(t, c1.send, 500*time.Millisecond)
	receiveWithTimeout(t, c2.send, 500*time.Millisecond)
}

func TestHub_UnregisterStopsDelivery(t *testing.T) {
	hub, cancel := newTestHub(t)
	defer cancel()

	client := &Client{
		hub:    hub,
		send:   make(chan []byte, 8),
		userID: "u-1",
		roomID: "room-unsub",
	}

	mustRegister(t, hub, client)

	hub.unregister <- client

	// send channel should be closed by Run
	select {
	case _, open := <-client.send:
		if open {
			t.Error("send channel should be closed after unregister")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("send channel was not closed after unregister")
	}
}

func TestHub_ClientsInDifferentRoomsAreIsolated(t *testing.T) {
	hub, cancel := newTestHub(t)
	defer cancel()

	c1 := &Client{hub: hub, send: make(chan []byte, 8), userID: "u-1", roomID: "room-A"}
	c2 := &Client{hub: hub, send: make(chan []byte, 8), userID: "u-2", roomID: "room-B"}

	mustRegister(t, hub, c1)
	mustRegister(t, hub, c2)

	// Publish only to room-A.
	if err := hub.Publish(context.Background(), "room-A", []byte(`{"room":"A"}`)); err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	receiveWithTimeout(t, c1.send, 500*time.Millisecond)

	// c2 (in room-B) must NOT receive the message.
	select {
	case msg := <-c2.send:
		t.Errorf("room-B client received unexpected message: %s", msg)
	case <-time.After(100 * time.Millisecond):
		// expected — no message delivered to the other room
	}
}

func TestHub_SlowClientIsEvicted(t *testing.T) {
	hub, cancel := newTestHub(t)
	defer cancel()

	// Buffer of 1 so it can be overfilled easily.
	client := &Client{
		hub:    hub,
		send:   make(chan []byte, 1),
		userID: "u-slow",
		roomID: "room-slow",
	}

	mustRegister(t, hub, client)

	// Pre-fill the buffer so the next delivery has no room.
	client.send <- []byte("pre-fill")

	// Inject a broadcast directly (no Redis round-trip).
	hub.broadcast <- broadcastMsg{roomID: "room-slow", data: []byte(`{"type":"msg"}`)}

	// Poll RoomParticipants until the client disappears from the room —
	// that is the deterministic signal that deliver() closed the send channel.
	evictCtx, evictCancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer evictCancel()
	for {
		evicted := true
		for _, p := range hub.RoomParticipants(evictCtx, client.roomID) {
			if p.UserID == client.userID {
				evicted = false
				break
			}
		}
		if evicted {
			break
		}
		if evictCtx.Err() != nil {
			t.Fatal("timeout waiting for slow-client eviction")
		}
	}

	// Drain the pre-fill item, then confirm the channel is closed.
	for range 2 {
		_, open := <-client.send
		if !open {
			return // channel closed — eviction confirmed
		}
	}
	t.Error("send channel was not closed after slow-client eviction")
}

func TestHub_CancelStopsRun(t *testing.T) {
	hub, cancel := newTestHub(t)

	// A client registered before cancel should have its send channel closed.
	client := &Client{
		hub:    hub,
		send:   make(chan []byte, 8),
		userID: "u-1",
		roomID: "room-cancel",
	}
	mustRegister(t, hub, client)

	cancel()

	select {
	case _, open := <-client.send:
		if open {
			t.Error("send channel should be closed after hub shutdown")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("send channel was not closed after hub shutdown")
	}
}
