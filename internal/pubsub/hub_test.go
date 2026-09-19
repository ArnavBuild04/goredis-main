package pubsub

import "testing"

func TestSubscribeUnsubscribeCounts(t *testing.T) {
	h := NewHub()
	sub := h.NewSubscriber()

	if n := h.Subscribe(sub, "a"); n != 1 {
		t.Fatalf("Subscribe(a) = %d, want 1", n)
	}
	if n := h.Subscribe(sub, "b"); n != 2 {
		t.Fatalf("Subscribe(b) = %d, want 2", n)
	}
	if n := h.Unsubscribe(sub, "a"); n != 1 {
		t.Fatalf("Unsubscribe(a) = %d, want 1", n)
	}
	if got := sub.Channels(); len(got) != 1 || got[0] != "b" {
		t.Fatalf("Channels() = %v, want [b]", got)
	}
}

func TestPublishDeliversToAllSubscribers(t *testing.T) {
	h := NewHub()
	s1, s2 := h.NewSubscriber(), h.NewSubscriber()
	h.Subscribe(s1, "news")
	h.Subscribe(s2, "news")

	n := h.Publish("news", "hello")
	if n != 2 {
		t.Fatalf("Publish delivered to %d subscribers, want 2", n)
	}

	for _, s := range []*Subscriber{s1, s2} {
		select {
		case msg := <-s.Messages():
			if msg.Channel != "news" || msg.Payload != "hello" {
				t.Fatalf("got %#v", msg)
			}
		default:
			t.Fatal("expected a message in the inbox")
		}
	}
}

func TestPublishOnlyReachesSubscribersOfThatChannel(t *testing.T) {
	h := NewHub()
	sub := h.NewSubscriber()
	h.Subscribe(sub, "sports")

	n := h.Publish("news", "hello")
	if n != 0 {
		t.Fatalf("Publish to an unsubscribed channel delivered to %d, want 0", n)
	}
}

func TestUnsubscribeAll(t *testing.T) {
	h := NewHub()
	sub := h.NewSubscriber()
	h.Subscribe(sub, "a")
	h.Subscribe(sub, "b")

	h.UnsubscribeAll(sub)

	if n := sub.ChannelCount(); n != 0 {
		t.Fatalf("ChannelCount() after UnsubscribeAll = %d, want 0", n)
	}
	if n := h.Publish("a", "x"); n != 0 {
		t.Fatalf("Publish after UnsubscribeAll reached %d subscribers, want 0", n)
	}
}

func TestPublishDropsWhenInboxFull(t *testing.T) {
	h := NewHub()
	sub := h.NewSubscriber()
	h.Subscribe(sub, "ch")

	// Fill the inbox past capacity; Publish must never block on a full
	// subscriber, so this loop has to terminate on its own.
	for i := 0; i < inboxSize+10; i++ {
		h.Publish("ch", "msg")
	}

	if len(sub.messages) != inboxSize {
		t.Fatalf("inbox len = %d, want it capped at %d", len(sub.messages), inboxSize)
	}
}
