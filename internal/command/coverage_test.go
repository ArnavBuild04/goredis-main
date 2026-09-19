package command

import (
	"testing"

	"github.com/newpath/goredis/internal/resp"
)

// This file rounds out command_test.go's coverage with the handlers not
// already exercised by the core round-trip and transaction/pubsub tests:
// the remaining string, hash, list, set, sorted-set, and generic commands.

func TestStringExtras(t *testing.T) {
	h := newHarness(t)

	if got := h.run("APPEND", "s", "hello"); got.Num != 5 {
		t.Fatalf("APPEND (create) = %#v, want 5", got)
	}
	if got := h.run("APPEND", "s", " world"); got.Num != 11 {
		t.Fatalf("APPEND (extend) = %#v, want 11", got)
	}
	if got := h.run("STRLEN", "s"); got.Num != 11 {
		t.Fatalf("STRLEN = %#v, want 11", got)
	}
	if got := h.run("STRLEN", "missing"); got.Num != 0 {
		t.Fatalf("STRLEN on a missing key = %#v, want 0", got)
	}

	if got := h.run("SETEX", "ex", "10", "v"); got.Str != "OK" {
		t.Fatalf("SETEX = %#v", got)
	}
	if ttl := h.run("TTL", "ex"); ttl.Num <= 0 || ttl.Num > 10 {
		t.Fatalf("TTL after SETEX = %#v, want (0,10]", ttl)
	}

	h.run("MSET", "a", "1", "b", "2")
	got := h.run("MGET", "a", "b", "missing")
	if len(got.Elems) != 3 || got.Elems[0].Str != "1" || got.Elems[1].Str != "2" || !got.Elems[2].Null {
		t.Fatalf("MGET = %#v", got)
	}

	if got := h.run("GETSET", "a", "99"); got.Str != "1" {
		t.Fatalf("GETSET = %#v, want old value 1", got)
	}
	if got := h.run("GET", "a"); got.Str != "99" {
		t.Fatalf("GET after GETSET = %#v", got)
	}
}

func TestGenericExtras(t *testing.T) {
	h := newHarness(t)
	h.run("SET", "k1", "v1")
	h.run("SET", "k2", "v2")

	if got := h.run("EXISTS", "k1", "k2", "missing"); got.Num != 2 {
		t.Fatalf("EXISTS = %#v, want 2", got)
	}
	if got := h.run("DEL", "k1", "missing"); got.Num != 1 {
		t.Fatalf("DEL = %#v, want 1", got)
	}
	if got := h.run("RENAME", "k2", "k3"); got.Str != "OK" {
		t.Fatalf("RENAME = %#v", got)
	}
	if got := h.run("GET", "k3"); got.Str != "v2" {
		t.Fatalf("GET after RENAME = %#v", got)
	}
	if got := h.run("RENAME", "missing", "x"); got.Type != resp.TypeError {
		t.Fatalf("RENAME on a missing key = %#v, want an error", got)
	}

	h.run("SET", "p", "v")
	if got := h.run("PERSIST", "p"); got.Num != 0 {
		t.Fatalf("PERSIST on a key with no TTL = %#v, want 0", got)
	}
	h.run("EXPIRE", "p", "100")
	if got := h.run("PERSIST", "p"); got.Num != 1 {
		t.Fatalf("PERSIST on a key with a TTL = %#v, want 1", got)
	}
}

func TestServerCommands(t *testing.T) {
	h := newHarness(t)
	h.run("SET", "a", "1")
	h.run("SET", "b", "2")

	if got := h.run("DBSIZE"); got.Num != 2 {
		t.Fatalf("DBSIZE = %#v, want 2", got)
	}
	if got := h.run("FLUSHALL"); got.Str != "OK" {
		t.Fatalf("FLUSHALL = %#v", got)
	}
	if got := h.run("DBSIZE"); got.Num != 0 {
		t.Fatalf("DBSIZE after FLUSHALL = %#v, want 0", got)
	}
	if got := h.run("INFO"); got.Type != resp.TypeBulk {
		t.Fatalf("INFO reply type = %v, want bulk", got.Type)
	}
}

func TestHashExtras(t *testing.T) {
	h := newHarness(t)
	h.run("HSET", "h", "a", "1", "b", "2")

	if got := h.run("HEXISTS", "h", "a"); got.Num != 1 {
		t.Fatalf("HEXISTS = %#v, want 1", got)
	}
	if got := h.run("HEXISTS", "h", "missing"); got.Num != 0 {
		t.Fatalf("HEXISTS missing = %#v, want 0", got)
	}

	keys := h.run("HKEYS", "h")
	if len(keys.Elems) != 2 {
		t.Fatalf("HKEYS = %#v, want 2 elements", keys)
	}
	vals := h.run("HVALS", "h")
	if len(vals.Elems) != 2 {
		t.Fatalf("HVALS = %#v, want 2 elements", vals)
	}

	got := h.run("HMGET", "h", "a", "missing")
	if got.Elems[0].Str != "1" || !got.Elems[1].Null {
		t.Fatalf("HMGET = %#v", got)
	}

	if got := h.run("HINCRBY", "h", "c", "5"); got.Num != 5 {
		t.Fatalf("HINCRBY (new field) = %#v, want 5", got)
	}
	if got := h.run("HINCRBY", "h", "c", "5"); got.Num != 10 {
		t.Fatalf("HINCRBY (existing field) = %#v, want 10", got)
	}
}

func TestListExtras(t *testing.T) {
	h := newHarness(t)
	h.run("RPUSH", "l", "a", "b", "c")

	if got := h.run("LINDEX", "l", "0"); got.Str != "a" {
		t.Fatalf("LINDEX 0 = %#v", got)
	}
	if got := h.run("LINDEX", "l", "-1"); got.Str != "c" {
		t.Fatalf("LINDEX -1 = %#v", got)
	}
	if got := h.run("LINDEX", "l", "99"); !got.Null {
		t.Fatalf("LINDEX out of range = %#v, want null", got)
	}

	if got := h.run("LPOP", "l"); got.Str != "a" {
		t.Fatalf("LPOP = %#v", got)
	}
	if got := h.run("RPOP", "l"); got.Str != "c" {
		t.Fatalf("RPOP = %#v", got)
	}
	if got := h.run("LPOP", "missing"); !got.Null {
		t.Fatalf("LPOP on a missing key = %#v, want null", got)
	}
}

func TestSetExtras(t *testing.T) {
	h := newHarness(t)
	h.run("SADD", "s", "a", "b", "c")

	members := h.run("SMEMBERS", "s")
	if len(members.Elems) != 3 {
		t.Fatalf("SMEMBERS = %#v, want 3 elements", members)
	}
	if got := h.run("SISMEMBER", "s", "a"); got.Num != 1 {
		t.Fatalf("SISMEMBER = %#v, want 1", got)
	}
	if got := h.run("SISMEMBER", "s", "z"); got.Num != 0 {
		t.Fatalf("SISMEMBER missing = %#v, want 0", got)
	}
	if got := h.run("SCARD", "s"); got.Num != 3 {
		t.Fatalf("SCARD = %#v, want 3", got)
	}
}

func TestZSetExtras(t *testing.T) {
	h := newHarness(t)
	h.run("ZADD", "z", "1", "a", "2", "b")

	if got := h.run("ZSCORE", "z", "a"); got.Str != "1" {
		t.Fatalf("ZSCORE = %#v, want 1", got)
	}
	if got := h.run("ZSCORE", "z", "missing"); !got.Null {
		t.Fatalf("ZSCORE missing = %#v, want null", got)
	}
	if got := h.run("ZRANK", "z", "b"); got.Num != 1 {
		t.Fatalf("ZRANK = %#v, want 1", got)
	}
	if got := h.run("ZREM", "z", "a"); got.Num != 1 {
		t.Fatalf("ZREM = %#v, want 1", got)
	}
	if got := h.run("ZCARD", "z"); got.Num != 1 {
		t.Fatalf("ZCARD = %#v, want 1", got)
	}
}

func TestUnsubscribeExplicitChannel(t *testing.T) {
	h := newHarness(t)
	h.run("SUBSCRIBE", "a", "b")

	got := h.run("UNSUBSCRIBE", "a")
	if !got.Multi || len(got.Elems) != 1 {
		t.Fatalf("UNSUBSCRIBE a = %#v", got)
	}
	if got.Elems[0].Elems[2].Num != 1 {
		t.Fatalf("remaining channel count = %#v, want 1", got.Elems[0])
	}
}

func TestUnsubscribeWithNoChannelsSubscribed(t *testing.T) {
	h := newHarness(t)
	got := h.run("UNSUBSCRIBE")
	if !got.Multi || len(got.Elems) != 1 {
		t.Fatalf("UNSUBSCRIBE with nothing subscribed = %#v", got)
	}
	if !got.Elems[0].Elems[1].Null || got.Elems[0].Elems[2].Num != 0 {
		t.Fatalf("frame = %#v, want (nil channel, 0)", got.Elems[0])
	}
}

func TestPingWithMessage(t *testing.T) {
	h := newHarness(t)
	if got := h.run("PING", "hello"); got.Str != "hello" {
		t.Fatalf("PING hello = %#v", got)
	}
	if got := h.run("ECHO", "hi"); got.Str != "hi" {
		t.Fatalf("ECHO = %#v", got)
	}
}

func TestKeysGlobThroughDispatch(t *testing.T) {
	h := newHarness(t)
	h.run("SET", "user:1", "a")
	h.run("SET", "user:2", "b")
	h.run("SET", "order:1", "c")

	got := h.run("KEYS", "user:*")
	if len(got.Elems) != 2 {
		t.Fatalf("KEYS user:* = %#v, want 2 matches", got)
	}
}
