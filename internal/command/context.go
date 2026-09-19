package command

import (
	"github.com/newpath/goredis/internal/pubsub"
	"github.com/newpath/goredis/internal/resp"
)

// QueuedCommand is one command captured between MULTI and EXEC.
type QueuedCommand struct {
	Name string
	Args []resp.Value
}

// Context carries the per-connection state a handler needs beyond the
// keyspace itself: which registry to recurse into for EXEC, this
// connection's transaction queue, and its pub/sub subscription.
//
// A Context is owned by exactly one connection and is never touched by more
// than one goroutine at a time, so it needs no locking of its own.
type Context struct {
	Registry *Registry
	Hub      *pubsub.Hub

	// OnWrite, if set, is called after every successful write command —
	// including each one dispatched from inside EXEC — with the full
	// command line (name as args[0], its own arguments after). The server
	// wires this to the AOF; it stays nil during AOF replay so replaying
	// the log doesn't re-append what it just read.
	OnWrite func(args []resp.Value)

	sub *pubsub.Subscriber

	inTx    bool
	queue   []QueuedCommand
	txError bool // set when a queued command fails arity/lookup, aborting EXEC
}

// NewContext creates the per-connection state for one client.
func NewContext(reg *Registry, hub *pubsub.Hub) *Context {
	return &Context{Registry: reg, Hub: hub}
}

// Subscriber returns this connection's pub/sub inbox, creating it on first
// use so connections that never subscribe pay nothing for it.
func (c *Context) Subscriber() *pubsub.Subscriber {
	if c.sub == nil {
		c.sub = c.Hub.NewSubscriber()
	}
	return c.sub
}

// InSubscriberMode reports whether this connection has an active
// subscription, which — as in real Redis — restricts it to a small set of
// allowed commands.
func (c *Context) InSubscriberMode() bool {
	return c.sub != nil && c.sub.ChannelCount() > 0
}

// Close releases any pub/sub subscriptions held by this connection. Called
// when the connection closes.
func (c *Context) Close() {
	if c.sub != nil {
		c.Hub.UnsubscribeAll(c.sub)
	}
}

// queueCommand records one command for a later EXEC. A command that fails
// validation up front (unknown name, wrong arity) marks the whole
// transaction dirty — real Redis aborts EXEC entirely in that case, rather
// than silently skipping just the bad command.
func (c *Context) queueCommand(name string, args []resp.Value) resp.Value {
	_, errVal, ok := c.Registry.validate(name, args)
	if !ok {
		c.txError = true
		return errVal
	}
	c.queue = append(c.queue, QueuedCommand{Name: name, Args: args})
	return resp.Str("QUEUED")
}
