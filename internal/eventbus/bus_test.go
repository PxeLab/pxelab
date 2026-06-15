package eventbus

import (
	"testing"
)

func TestBus(t *testing.T) {
	b := New()

	received := make(chan Event, 1)
	id := b.Subscribe("test", func(e Event) {
		received <- e
	})

	b.Publish("test", "hello")

	select {
	case e := <-received:
		if e.Topic != "test" || e.Payload != "hello" {
			t.Fatalf("unexpected event: %+v", e)
		}
	default:
		t.Fatal("expected event")
	}

	b.Unsubscribe("test", id)
	b.Publish("test", "world")

	select {
	case <-received:
		t.Fatal("expected no event after unsubscribe")
	default:
	}
}
