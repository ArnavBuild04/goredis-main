package command

import (
	"testing"

	"github.com/newpath/goredis/internal/pubsub"
	"github.com/newpath/goredis/internal/resp"
	"github.com/newpath/goredis/internal/store"
)

// harness bundles everything a test needs to fire a command and inspect
// the reply, mirroring how a real connection drives the registry.
type harness struct {
	t   *testing.T
	st  *store.Store
	reg *Registry
	ctx *Context
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	st := store.New()
	t.Cleanup(st.Close)
	reg := NewRegistry()
	hub := pubsub.NewHub()
	return &harness{t: t, st: st, reg: reg, ctx: NewContext(reg, hub)}
}

func (h *harness) run(name string, args ...string) resp.Value {
	h.t.Helper()
	vals := make([]resp.Value, len(args))
	for i, a := range args {
		vals[i] = resp.Bulk(a)
	}
	return h.reg.Dispatch(h.st, h.ctx, name, vals)
}

func TestPing(t *testing.T) {
	h := newHarness(t)
	got := h.run("PING")
	if got.Type != resp.TypeSimpleString || got.Str != "PONG" {
		t.Fatalf("PING = %#v", got)
	}
}

func TestSetGetRoundtrip(t *testing.T) {
	h := newHarness(t)
	if got := h.run("SET", "k", "v"); got.Str != "OK" {
		t.Fatalf("SET = %#v", got)
	}
	got := h.run("GET", "k")
	if got.Type != resp.TypeBulk || got.Str != "v" {
		t.Fatalf("GET = %#v", got)
	}
}

func TestGetMissingIsNullBulk(t *testing.T) {
	h := newHarness(t)
	got := h.run("GET", "missing")
	if got.Type != resp.TypeBulk || !got.Null {
		t.Fatalf("GET missing = %#v, want null bulk", got)
	}
}

func TestUnknownCommand(t *testing.T) {
	h := newHarness(t)
	got := h.run("NOPE", "a")
	if got.Type != resp.TypeError {
		t.Fatalf("NOPE = %#v, want an error", got)
	}
}

func TestWrongArity(t *testing.T) {
	h := newHarness(t)
	got := h.run("GET")
	if got.Type != resp.TypeError {
		t.Fatalf("GET with no args = %#v, want an error", got)
	}
}

func TestSetNXOverExisting(t *testing.T) {
	h := newHarness(t)
	h.run("SET", "k", "v1")
	got := h.run("SET", "k", "v2", "NX")
	if got.Type != resp.TypeBulk || !got.Null {
		t.Fatalf("SET NX over an existing key = %#v, want null bulk", got)
	}
	got = h.run("GET", "k")
	if got.Str != "v1" {
		t.Fatalf("value changed under a failed NX: %q", got.Str)
	}
}

func TestExpireAndTTL(t *testing.T) {
	h := newHarness(t)
	h.run("SET", "k", "v")
	if got := h.run("EXPIRE", "k", "100"); got.Num != 1 {
		t.Fatalf("EXPIRE = %#v", got)
	}
	got := h.run("TTL", "k")
	if got.Num <= 0 || got.Num > 100 {
		t.Fatalf("TTL = %d, want (0, 100]", got.Num)
	}
}

func TestTypeCommand(t *testing.T) {
	h := newHarness(t)
	h.run("LPUSH", "l", "a")
	got := h.run("TYPE", "l")
	if got.Str != "list" {
		t.Fatalf("TYPE = %#v, want list", got)
	}
	got = h.run("TYPE", "missing")
	if got.Str != "none" {
		t.Fatalf("TYPE on missing key = %#v, want none", got)
	}
}

func TestHashCommands(t *testing.T) {
	h := newHarness(t)
	h.run("HSET", "h", "a", "1", "b", "2")
	got := h.run("HGET", "h", "a")
	if got.Str != "1" {
		t.Fatalf("HGET = %#v", got)
	}
	got = h.run("HLEN", "h")
	if got.Num != 2 {
		t.Fatalf("HLEN = %#v", got)
	}
}

func TestListCommands(t *testing.T) {
	h := newHarness(t)
	h.run("RPUSH", "l", "a", "b", "c")
	got := h.run("LRANGE", "l", "0", "-1")
	if len(got.Elems) != 3 || got.Elems[0].Str != "a" || got.Elems[2].Str != "c" {
		t.Fatalf("LRANGE = %#v", got)
	}
}

func TestWrongTypeSurfacesAsError(t *testing.T) {
	h := newHarness(t)
	h.run("LPUSH", "l", "a")
	got := h.run("GET", "l")
	if got.Type != resp.TypeError {
		t.Fatalf("GET on a list key = %#v, want error", got)
	}
}

func TestZSetCommands(t *testing.T) {
	h := newHarness(t)
	h.run("ZADD", "z", "3", "a", "1", "b", "2", "c")
	got := h.run("ZRANGE", "z", "0", "-1")
	want := []string{"b", "c", "a"}
	for i, w := range want {
		if got.Elems[i].Str != w {
			t.Fatalf("ZRANGE[%d] = %q, want %q", i, got.Elems[i].Str, w)
		}
	}

	withScores := h.run("ZRANGE", "z", "0", "-1", "WITHSCORES")
	if len(withScores.Elems) != 6 {
		t.Fatalf("ZRANGE WITHSCORES len = %d, want 6", len(withScores.Elems))
	}
}

func TestMultiExecQueuesAndRuns(t *testing.T) {
	h := newHarness(t)
	if got := h.run("MULTI"); got.Str != "OK" {
		t.Fatalf("MULTI = %#v", got)
	}
	if got := h.run("SET", "k", "v"); got.Str != "QUEUED" {
		t.Fatalf("queued SET = %#v, want QUEUED", got)
	}
	if got := h.run("INCR", "n"); got.Str != "QUEUED" {
		t.Fatalf("queued INCR = %#v, want QUEUED", got)
	}

	// Nothing should be visible until EXEC runs — checked directly against
	// the store, since issuing EXISTS through h.run here would itself just
	// get queued as a third command.
	if n := h.st.Exists("k"); n != 0 {
		t.Fatalf("store.Exists before EXEC = %d, want 0 (write still queued)", n)
	}

	got := h.run("EXEC")
	if len(got.Elems) != 2 {
		t.Fatalf("EXEC reply = %#v, want 2 elements", got)
	}
	if got.Elems[0].Str != "OK" {
		t.Fatalf("EXEC[0] = %#v, want OK", got.Elems[0])
	}
	if got.Elems[1].Num != 1 {
		t.Fatalf("EXEC[1] = %#v, want 1", got.Elems[1])
	}

	v := h.run("GET", "k")
	if v.Str != "v" {
		t.Fatalf("GET after EXEC = %#v", v)
	}
}

func TestMultiAbortsOnBadQueuedCommand(t *testing.T) {
	h := newHarness(t)
	h.run("MULTI")
	h.run("SET", "k", "v")
	got := h.run("NOSUCHCOMMAND")
	if got.Type != resp.TypeError {
		t.Fatalf("queueing an unknown command = %#v, want an immediate error", got)
	}
	exec := h.run("EXEC")
	if exec.Type != resp.TypeError {
		t.Fatalf("EXEC after a dirty queue = %#v, want EXECABORT error", exec)
	}
	// k must not have been set — the whole transaction was aborted.
	if got := h.run("EXISTS", "k"); got.Num != 0 {
		t.Fatalf("EXISTS after aborted EXEC = %#v, want 0", got)
	}
}

func TestDiscard(t *testing.T) {
	h := newHarness(t)
	h.run("MULTI")
	h.run("SET", "k", "v")
	got := h.run("DISCARD")
	if got.Str != "OK" {
		t.Fatalf("DISCARD = %#v", got)
	}
	if got := h.run("EXISTS", "k"); got.Num != 0 {
		t.Fatalf("EXISTS after DISCARD = %#v, want 0", got)
	}
}

func TestPubSubSubscribePublish(t *testing.T) {
	h := newHarness(t)
	got := h.run("SUBSCRIBE", "news", "sports")
	if !got.Multi || len(got.Elems) != 2 {
		t.Fatalf("SUBSCRIBE reply = %#v, want a 2-frame multi", got)
	}
	if got.Elems[0].Elems[2].Num != 1 {
		t.Fatalf("first subscribe count = %#v, want 1", got.Elems[0])
	}
	if got.Elems[1].Elems[2].Num != 2 {
		t.Fatalf("second subscribe count = %#v, want 2", got.Elems[1])
	}

	n := h.run("PUBLISH", "news", "hello")
	if n.Num != 1 {
		t.Fatalf("PUBLISH receiver count = %#v, want 1", n)
	}

	select {
	case msg := <-h.ctx.Subscriber().Messages():
		if msg.Channel != "news" || msg.Payload != "hello" {
			t.Fatalf("delivered message = %#v", msg)
		}
	default:
		t.Fatal("expected a message to be waiting in the subscriber's inbox")
	}
}
