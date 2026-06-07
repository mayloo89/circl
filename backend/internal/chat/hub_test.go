package chat

import (
	"context"
	"encoding/json"
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
	if err := hub.Publish(t.Context(), "room-delivery", payload); err != nil {
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
	if err := hub.Publish(t.Context(), "room-multi", payload); err != nil {
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
	if err := hub.Publish(t.Context(), "room-A", []byte(`{"room":"A"}`)); err != nil {
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

func TestHub_TypingFrameSkippedForClientHidingTyping(t *testing.T) {
	hub, cancel := newTestHub(t)
	defer cancel()

	hider := &Client{hub: hub, send: make(chan []byte, 8), userID: "u-hider", roomID: "room-priv", hideTyping: true}
	listener := &Client{hub: hub, send: make(chan []byte, 8), userID: "u-listener", roomID: "room-priv"}

	mustRegister(t, hub, hider)
	mustRegister(t, hub, listener)

	typing := []byte(`{"event":"typing","user_id":"u-other","room_id":"room-priv","display_name":"Other"}`)
	if err := hub.Publish(t.Context(), "room-priv", typing); err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	// listener still receives the typing frame.
	got := receiveWithTimeout(t, listener.send, 500*time.Millisecond)
	if string(got) != string(typing) {
		t.Errorf("listener got %s, want %s", got, typing)
	}
	// hider does NOT receive the typing frame.
	select {
	case msg := <-hider.send:
		t.Errorf("hider unexpectedly received typing frame: %s", msg)
	case <-time.After(100 * time.Millisecond):
		// expected — frame suppressed for the hider
	}
}

func TestHub_ReadReceiptFrameSkippedForClientHidingReceipts(t *testing.T) {
	hub, cancel := newTestHub(t)
	defer cancel()

	hider := &Client{hub: hub, send: make(chan []byte, 8), userID: "u-hider", roomID: "room-rr", hideReadReceipts: true}
	listener := &Client{hub: hub, send: make(chan []byte, 8), userID: "u-listener", roomID: "room-rr"}

	mustRegister(t, hub, hider)
	mustRegister(t, hub, listener)

	receipt := []byte(`{"event":"read_receipt","room_id":"room-rr","user_id":"u-other","read_at":"2026-01-01T00:00:00Z"}`)
	if err := hub.Publish(t.Context(), "room-rr", receipt); err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	got := receiveWithTimeout(t, listener.send, 500*time.Millisecond)
	if string(got) != string(receipt) {
		t.Errorf("listener got %s, want %s", got, receipt)
	}
	select {
	case msg := <-hider.send:
		t.Errorf("hider unexpectedly received read_receipt frame: %s", msg)
	case <-time.After(100 * time.Millisecond):
		// expected — frame suppressed for the hider
	}
}

func TestHub_NewMessageFrameDeliveredToHidersRegardless(t *testing.T) {
	hub, cancel := newTestHub(t)
	defer cancel()

	// Both flags set on a single client; only typing/read_receipt should be
	// suppressed — content frames must still arrive.
	hider := &Client{
		hub: hub, send: make(chan []byte, 8), userID: "u-hider", roomID: "room-mix",
		hideTyping: true, hideReadReceipts: true,
	}
	mustRegister(t, hub, hider)

	msg := []byte(`{"event":"new_message","id":"m-1","room_id":"room-mix","sender_id":"u-other","content":"hi"}`)
	if err := hub.Publish(t.Context(), "room-mix", msg); err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	got := receiveWithTimeout(t, hider.send, 500*time.Millisecond)
	if string(got) != string(msg) {
		t.Errorf("hider got %s, want %s — content frames must not be gated", got, msg)
	}
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

func TestHub_KickFromRoom(t *testing.T) {
	hub, cancel := newTestHub(t)
	defer cancel()

	target := &Client{
		hub:    hub,
		send:   make(chan []byte, 8),
		userID: "guest:target",
		roomID: "room-1",
	}
	bystander := &Client{
		hub:    hub,
		send:   make(chan []byte, 8),
		userID: "u-bystander",
		roomID: "room-1",
	}
	mustRegister(t, hub, target)
	mustRegister(t, hub, bystander)

	hub.KickFromRoom("room-1", "guest:target")

	// target's send channel must receive a "kicked" frame then be closed.
	select {
	case frame, ok := <-target.send:
		if !ok {
			t.Fatal("received closed channel before kicked frame")
		}
		if !containsEvent(frame, "kicked") {
			t.Errorf("first frame event is not kicked: %s", frame)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("target did not receive kicked frame within 500ms")
	}
	// Next receive must close the channel.
	select {
	case _, open := <-target.send:
		if open {
			t.Error("send channel must be closed after kick")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("target send channel not closed after kick")
	}

	// Bystander must remain registered (not kicked).
	participants := hub.RoomParticipants(t.Context(), "room-1")
	for _, p := range participants {
		if p.UserID == "guest:target" {
			t.Error("kicked target still appears in participants")
		}
	}
	if len(participants) != 1 || participants[0].UserID != "u-bystander" {
		t.Errorf("expected only bystander in room, got %v", participants)
	}
}

func TestHub_SendToUser(t *testing.T) {
	hub, cancel := newTestHub(t)
	defer cancel()

	target := &Client{hub: hub, send: make(chan []byte, 8), userID: "u-target", roomID: "room-notify"}
	bystander := &Client{hub: hub, send: make(chan []byte, 8), userID: "u-bystander", roomID: "room-notify"}
	mustRegister(t, hub, target)
	mustRegister(t, hub, bystander)

	frame, _ := json.Marshal(map[string]any{"event": "you_are_unmuted", "room_id": "room-notify"})
	hub.SendToUser("room-notify", "u-target", frame)

	got := receiveWithTimeout(t, target.send, 500*time.Millisecond)
	if !containsEvent(got, "you_are_unmuted") {
		t.Errorf("target got unexpected frame: %s", got)
	}

	select {
	case msg := <-bystander.send:
		t.Errorf("bystander should not receive the notification: %s", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

// containsEvent checks whether a JSON frame has the given "event" field value.
func containsEvent(data []byte, event string) bool {
	var f struct{ Event string `json:"event"` }
	if err := json.Unmarshal(data, &f); err != nil {
		return false
	}
	return f.Event == event
}
