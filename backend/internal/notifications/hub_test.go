package notifications_test

import (
	"testing"
	"time"

	"github.com/mayloo89/circl/backend/internal/notifications"
)

func TestHub_Subscribe_ReceivesEvent(t *testing.T) {
	hub := notifications.NewHub()
	ch, unsub := hub.Subscribe("user-1")
	defer unsub()

	want := notifications.Event{Type: "contact_request"}
	hub.Notify("user-1", want)

	select {
	case got := <-ch:
		if got.Type != want.Type {
			t.Errorf("type = %q, want %q", got.Type, want.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHub_Notify_NoSubscribers(t *testing.T) {
	hub := notifications.NewHub()
	// must not panic
	hub.Notify("unknown-user", notifications.Event{Type: "contact_request"})
}

func TestHub_Unsubscribe_NoLongerReceives(t *testing.T) {
	hub := notifications.NewHub()
	ch, unsub := hub.Subscribe("user-1")
	unsub()

	hub.Notify("user-1", notifications.Event{Type: "contact_request"})

	// The channel is closed by unsub. Reading it returns zero value with ok=false.
	// A real event (ok=true) would mean the subscriber was not properly removed.
	select {
	case e, ok := <-ch:
		if ok {
			t.Errorf("received real event (type=%q) after unsubscribe", e.Type)
		}
		// ok==false: channel was closed cleanly — expected
	default:
		// no data on channel — also acceptable
	}
}

func TestHub_MultipleSubscribers_SameUser(t *testing.T) {
	hub := notifications.NewHub()
	ch1, unsub1 := hub.Subscribe("user-1")
	ch2, unsub2 := hub.Subscribe("user-1")
	defer unsub1()
	defer unsub2()

	hub.Notify("user-1", notifications.Event{Type: "ping"})

	for _, ch := range []<-chan notifications.Event{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Type != "ping" {
				t.Errorf("type = %q, want ping", got.Type)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event on subscriber")
		}
	}
}

func TestHub_FullChannel_DropsEvent(t *testing.T) {
	hub := notifications.NewHub()
	ch, unsub := hub.Subscribe("user-1")
	defer unsub()

	// Fill channel (capacity 8) without reading.
	for range 8 {
		hub.Notify("user-1", notifications.Event{Type: "ping"})
	}

	// The 9th call must not block.
	done := make(chan struct{})
	go func() {
		hub.Notify("user-1", notifications.Event{Type: "ping"})
		close(done)
	}()

	select {
	case <-done:
		// correct: returned without blocking
	case <-time.After(time.Second):
		t.Fatal("Notify blocked on full channel")
	}
	_ = ch
}

func TestHub_Notify_OtherUserNotReceive(t *testing.T) {
	hub := notifications.NewHub()
	ch, unsub := hub.Subscribe("user-1")
	defer unsub()

	hub.Notify("user-2", notifications.Event{Type: "ping"})

	select {
	case <-ch:
		t.Error("user-1 received event meant for user-2")
	default:
		// correct
	}
}

func TestHub_Unsubscribe_CleansUpMap(t *testing.T) {
	hub := notifications.NewHub()
	_, unsub1 := hub.Subscribe("user-1")
	_, unsub2 := hub.Subscribe("user-1")
	unsub1()
	unsub2()

	// After all subscribers removed, notifying must not panic.
	hub.Notify("user-1", notifications.Event{Type: "ping"})
}
