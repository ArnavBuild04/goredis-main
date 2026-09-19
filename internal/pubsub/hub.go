// Package pubsub implements Redis-style publish/subscribe: channels with no
// persistence — a message reaches only the subscribers connected at the
// instant it's published, and is dropped otherwise.
package pubsub

import "sync"

// Message is one published event, delivered to every subscriber of Channel.
type Message struct {
	Channel string
	Payload string
}

// inboxSize bounds how far a subscriber can lag before Publish starts
// dropping messages to it rather than blocking the publisher. A slow
// consumer in real Redis eventually gets disconnected for the same reason:
// unbounded pub/sub buffering turns one stuck client into a memory leak for
// everyone else.
const inboxSize = 256

// Subscriber is one connection's pub/sub inbox. The zero value is not
// usable — create one with Hub.NewSubscriber.
type Subscriber struct {
	id       int64
	messages chan Message

	mu       sync.Mutex
	channels map[string]struct{}
}

// Messages returns the channel the owning connection should range over to
// forward published events to its client.
func (s *Subscriber) Messages() <-chan Message { return s.messages }

// ChannelCount returns how many channels this subscriber currently listens
// to, the count Redis's SUBSCRIBE/UNSUBSCRIBE replies include.
func (s *Subscriber) ChannelCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.channels)
}

// Channels returns a snapshot of the channel names this subscriber
// currently listens to.
func (s *Subscriber) Channels() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.channels))
	for c := range s.channels {
		out = append(out, c)
	}
	return out
}

// Hub is the process-wide channel registry.
type Hub struct {
	mu     sync.Mutex
	subs   map[string]map[*Subscriber]struct{}
	nextID int64
}

func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[*Subscriber]struct{})}
}

// NewSubscriber allocates a Subscriber not yet listening to any channel.
func (h *Hub) NewSubscriber() *Subscriber {
	h.mu.Lock()
	h.nextID++
	id := h.nextID
	h.mu.Unlock()
	return &Subscriber{
		id:       id,
		messages: make(chan Message, inboxSize),
		channels: make(map[string]struct{}),
	}
}

// Subscribe registers sub for channel and returns sub's new total channel
// count.
func (h *Hub) Subscribe(sub *Subscriber, channel string) int {
	h.mu.Lock()
	if h.subs[channel] == nil {
		h.subs[channel] = make(map[*Subscriber]struct{})
	}
	h.subs[channel][sub] = struct{}{}
	h.mu.Unlock()

	sub.mu.Lock()
	sub.channels[channel] = struct{}{}
	n := len(sub.channels)
	sub.mu.Unlock()
	return n
}

// Unsubscribe removes sub from channel and returns sub's remaining channel
// count.
func (h *Hub) Unsubscribe(sub *Subscriber, channel string) int {
	h.mu.Lock()
	if set, ok := h.subs[channel]; ok {
		delete(set, sub)
		if len(set) == 0 {
			delete(h.subs, channel)
		}
	}
	h.mu.Unlock()

	sub.mu.Lock()
	delete(sub.channels, channel)
	n := len(sub.channels)
	sub.mu.Unlock()
	return n
}

// UnsubscribeAll removes sub from every channel it's listening to, for
// connection teardown.
func (h *Hub) UnsubscribeAll(sub *Subscriber) {
	sub.mu.Lock()
	channels := make([]string, 0, len(sub.channels))
	for c := range sub.channels {
		channels = append(channels, c)
	}
	sub.mu.Unlock()

	for _, c := range channels {
		h.Unsubscribe(sub, c)
	}
}

// Publish delivers payload to every current subscriber of channel and
// returns how many received it. A subscriber whose inbox is full is
// skipped rather than blocked on.
func (h *Hub) Publish(channel, payload string) int {
	h.mu.Lock()
	subs := make([]*Subscriber, 0, len(h.subs[channel]))
	for sub := range h.subs[channel] {
		subs = append(subs, sub)
	}
	h.mu.Unlock()

	msg := Message{Channel: channel, Payload: payload}
	delivered := 0
	for _, sub := range subs {
		select {
		case sub.messages <- msg:
			delivered++
		default:
		}
	}
	return delivered
}
