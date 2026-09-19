// Package command translates RESP requests into calls against the store
// engine. Each handler is a pure function of (Store, connection context,
// args) -> resp.Value; nothing here talks to the network directly, which
// is what lets the same registry serve live connections and AOF replay.
package command

import (
	"strings"

	"github.com/newpath/goredis/internal/resp"
	"github.com/newpath/goredis/internal/store"
)

// Handler executes one command against st, given the arguments that
// followed the command name, and returns the reply to send the client.
type Handler func(st *store.Store, ctx *Context, args []resp.Value) resp.Value

// Spec describes one command: how to run it, how many arguments it takes,
// and whether it mutates the keyspace (which decides whether it's appended
// to the AOF and whether MULTI is allowed to queue it for later replay).
type Spec struct {
	Name    string
	Handler Handler
	// Arity is the exact argument count required (excluding the command
	// name itself), or -1 for "at least MinArity".
	Arity    int
	MinArity int
	Write    bool
}

// Registry is the set of commands a server understands, keyed by
// upper-cased command name.
type Registry struct {
	specs map[string]Spec
}

// NewRegistry builds the registry with every command this server supports.
func NewRegistry() *Registry {
	r := &Registry{specs: make(map[string]Spec)}
	r.register(connectionCommands()...)
	r.register(genericCommands()...)
	r.register(stringCommands()...)
	r.register(hashCommands()...)
	r.register(listCommands()...)
	r.register(setCommands()...)
	r.register(zsetCommands()...)
	r.register(serverCommands()...)
	r.register(pubsubCommands()...)
	r.register(transactionCommands()...)
	return r
}

func (r *Registry) register(specs ...Spec) {
	for _, s := range specs {
		r.specs[s.Name] = s
	}
}

// Lookup returns the Spec for a command name (case-insensitive).
func (r *Registry) Lookup(name string) (Spec, bool) {
	s, ok := r.specs[strings.ToUpper(name)]
	return s, ok
}

// CheckArity reports whether argc (the number of arguments after the
// command name) satisfies spec's declared arity.
func (spec Spec) CheckArity(argc int) bool {
	if spec.Arity >= 0 {
		return argc == spec.Arity
	}
	return argc >= spec.MinArity
}

// validate looks up name and checks its arity, returning either a runnable
// Spec or the protocol error Value to send back for an unknown command or a
// bad argument count.
func (r *Registry) validate(name string, args []resp.Value) (Spec, resp.Value, bool) {
	spec, ok := r.Lookup(name)
	if !ok {
		return Spec{}, unknownCommand(name, args), false
	}
	if !spec.CheckArity(len(args)) {
		return Spec{}, resp.Errorf("ERR wrong number of arguments for '%s' command", strings.ToLower(name)), false
	}
	return spec, resp.Value{}, true
}

// commandsBypassingQueue never get queued by MULTI — they control the
// transaction itself rather than mutating the keyspace.
var commandsBypassingQueue = map[string]bool{"MULTI": true, "EXEC": true, "DISCARD": true}

// Dispatch runs name against st, or — if this connection is inside a
// MULTI/EXEC block — queues it for EXEC to run later. It returns the
// protocol error Value directly for an unknown command or a bad argument
// count, so callers can send the result straight back to the client
// without further checks.
func (r *Registry) Dispatch(st *store.Store, ctx *Context, name string, args []resp.Value) resp.Value {
	upper := strings.ToUpper(name)
	if ctx.inTx && !commandsBypassingQueue[upper] {
		return ctx.queueCommand(upper, args)
	}

	spec, errVal, ok := r.validate(upper, args)
	if !ok {
		return errVal
	}
	result := spec.Handler(st, ctx, args)

	if spec.Write && result.Type != resp.TypeError && ctx.OnWrite != nil {
		full := make([]resp.Value, 0, len(args)+1)
		full = append(full, resp.Bulk(upper))
		full = append(full, args...)
		ctx.OnWrite(full)
	}

	return result
}

func unknownCommand(name string, args []resp.Value) resp.Value {
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte('\'')
		b.WriteString(a.Str)
		b.WriteByte('\'')
	}
	return resp.Errorf("ERR unknown command '%s', with args beginning with: %s", name, b.String())
}
