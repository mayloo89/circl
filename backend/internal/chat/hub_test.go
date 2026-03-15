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
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	return hub, cancel
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

	hub.register <- client
	time.Sleep(20 * time.Millisecond) // allow Run to process

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

	hub.register <- c1
	hub.register <- c2
	time.Sleep(20 * time.Millisecond)

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

	hub.register <- client
	time.Sleep(20 * time.Millisecond)

	hub.unregister <- client
	time.Sleep(20 * time.Millisecond)

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

	hub.register <- c1
	hub.register <- c2
	time.Sleep(20 * time.Millisecond)

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

	hub.register <- client
	time.Sleep(20 * time.Millisecond)

	// Pre-fill the buffer so the next delivery has no room.
	client.send <- []byte("pre-fill")

	// Directly inject a broadcast without going through Redis to avoid
	// timing non-determinism in the pub/sub pipeline.
	hub.broadcast <- broadcastMsg{roomID: "room-slow", data: []byte(`{"type":"msg"}`)}

	// Wait for the Run loop to process the broadcast and call deliver.
	// deliver will find the buffer full and close the send channel.
	time.Sleep(50 * time.Millisecond)

	// The channel has at most one buffered item ("pre-fill") and is closed.
	// Reading it should return (value, true) for any buffered items, then
	// (nil, false) once empty.
	for i := 0; i < 2; i++ {
		select {
		case _, open := <-client.send:
			if !open {
				return // channel closed — eviction confirmed
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timeout reading from send channel")
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
	hub.register <- client
	time.Sleep(20 * time.Millisecond)

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
