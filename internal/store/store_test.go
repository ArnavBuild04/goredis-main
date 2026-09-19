package store

import (
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s := New()
	t.Cleanup(s.Close)
	return s
}

func TestSetGet(t *testing.T) {
	s := newTestStore(t)

	if _, found, _ := s.Get("missing"); found {
		t.Fatal("expected missing key to be not found")
	}

	if _, err := s.Set("k", "v", SetOptions{}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	v, found, err := s.Get("k")
	if err != nil || !found || v != "v" {
		t.Fatalf("Get() = %q, %v, %v; want v, true, nil", v, found, err)
	}
}

func TestSetNXXX(t *testing.T) {
	s := newTestStore(t)

	ok, _ := s.Set("k", "v1", SetOptions{OnlyIfOld: true})
	if ok {
		t.Fatal("XX on missing key should not write")
	}

	ok, _ = s.Set("k", "v1", SetOptions{OnlyIfNew: true})
	if !ok {
		t.Fatal("NX on missing key should write")
	}

	ok, _ = s.Set("k", "v2", SetOptions{OnlyIfNew: true})
	if ok {
		t.Fatal("NX on existing key should not write")
	}
	v, _, _ := s.Get("k")
	if v != "v1" {
		t.Fatalf("value changed under failed NX: got %q", v)
	}

	ok, _ = s.Set("k", "v2", SetOptions{OnlyIfOld: true})
	if !ok {
		t.Fatal("XX on existing key should write")
	}
	v, _, _ = s.Get("k")
	if v != "v2" {
		t.Fatalf("Get() = %q, want v2", v)
	}
}

func TestWrongType(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.LPush("k", "a"); err != nil {
		t.Fatalf("LPush: %v", err)
	}
	if _, _, err := s.Get("k"); err != ErrWrongType {
		t.Fatalf("Get on a list key: err = %v, want ErrWrongType", err)
	}
	if _, err := s.Set("k", "v", SetOptions{}); err != nil {
		t.Fatalf("Set should overwrite a list key outright: %v", err)
	}
}

func TestExpiry(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Set("k", "v", SetOptions{TTL: 20 * time.Millisecond, HasTTL: true}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if _, found, _ := s.Get("k"); !found {
		t.Fatal("key should still be alive immediately after Set")
	}

	time.Sleep(120 * time.Millisecond)

	if _, found, _ := s.Get("k"); found {
		t.Fatal("key should have expired")
	}
	if n := s.Exists("k"); n != 0 {
		t.Fatalf("Exists() = %d, want 0 for an expired key", n)
	}
}

func TestPersist(t *testing.T) {
	s := newTestStore(t)
	s.Set("k", "v", SetOptions{TTL: time.Minute, HasTTL: true})
	if ok := s.Persist("k"); !ok {
		t.Fatal("Persist should succeed on a key with a TTL")
	}
	if ttl := s.TTLSeconds("k"); ttl != -1 {
		t.Fatalf("TTLSeconds() = %d, want -1 after Persist", ttl)
	}
}

func TestIncrDecr(t *testing.T) {
	s := newTestStore(t)
	n, err := s.IncrBy("counter", 1)
	if err != nil || n != 1 {
		t.Fatalf("IncrBy() = %d, %v; want 1, nil", n, err)
	}
	n, err = s.IncrBy("counter", 4)
	if err != nil || n != 5 {
		t.Fatalf("IncrBy() = %d, %v; want 5, nil", n, err)
	}
	n, err = s.IncrBy("counter", -10)
	if err != nil || n != -5 {
		t.Fatalf("IncrBy() = %d, %v; want -5, nil", n, err)
	}

	s.Set("notanumber", "abc", SetOptions{})
	if _, err := s.IncrBy("notanumber", 1); err != errNotAnInteger {
		t.Fatalf("IncrBy on non-numeric string: err = %v, want errNotAnInteger", err)
	}
}

func TestListOps(t *testing.T) {
	s := newTestStore(t)
	s.RPush("l", "a", "b", "c")
	s.LPush("l", "z") // list is now [z a b c]

	got, err := s.LRange("l", 0, -1)
	if err != nil {
		t.Fatalf("LRange: %v", err)
	}
	want := []string{"z", "a", "b", "c"}
	if !equalSlices(got, want) {
		t.Fatalf("LRange(0,-1) = %v, want %v", got, want)
	}

	v, ok, _ := s.LIndex("l", -1)
	if !ok || v != "c" {
		t.Fatalf("LIndex(-1) = %q, %v; want c, true", v, ok)
	}

	v, ok, _ = s.LPop("l")
	if !ok || v != "z" {
		t.Fatalf("LPop() = %q, %v; want z, true", v, ok)
	}

	v, ok, _ = s.RPop("l")
	if !ok || v != "c" {
		t.Fatalf("RPop() = %q, %v; want c, true", v, ok)
	}

	n, _ := s.LLen("l")
	if n != 2 {
		t.Fatalf("LLen() = %d, want 2", n)
	}
}

func TestListEmptiedKeyRemoved(t *testing.T) {
	s := newTestStore(t)
	s.RPush("l", "only")
	s.LPop("l")
	if n := s.Exists("l"); n != 0 {
		t.Fatalf("emptied list should be removed from the keyspace, Exists() = %d", n)
	}
}

func TestHashOps(t *testing.T) {
	s := newTestStore(t)
	created, err := s.HSet("h", map[string]string{"a": "1", "b": "2"})
	if err != nil || created != 2 {
		t.Fatalf("HSet() = %d, %v; want 2, nil", created, err)
	}
	created, err = s.HSet("h", map[string]string{"a": "10", "c": "3"})
	if err != nil || created != 1 {
		t.Fatalf("HSet() (partial overwrite) = %d, %v; want 1, nil", created, err)
	}

	v, ok, _ := s.HGet("h", "a")
	if !ok || v != "10" {
		t.Fatalf("HGet(a) = %q, %v; want 10, true", v, ok)
	}

	n, _ := s.HDel("h", "a", "missing")
	if n != 1 {
		t.Fatalf("HDel() = %d, want 1", n)
	}

	n, _ = s.HLen("h")
	if n != 2 {
		t.Fatalf("HLen() = %d, want 2", n)
	}
}

func TestSetOps(t *testing.T) {
	s := newTestStore(t)
	n, _ := s.SAdd("s", "a", "b", "a")
	if n != 2 {
		t.Fatalf("SAdd() = %d, want 2 (duplicate in the same call shouldn't double count)", n)
	}
	ok, _ := s.SIsMember("s", "a")
	if !ok {
		t.Fatal("SIsMember(a) = false, want true")
	}
	n, _ = s.SRem("s", "a", "missing")
	if n != 1 {
		t.Fatalf("SRem() = %d, want 1", n)
	}
	n, _ = s.SCard("s")
	if n != 1 {
		t.Fatalf("SCard() = %d, want 1", n)
	}
}

func TestZSetOps(t *testing.T) {
	s := newTestStore(t)
	s.ZAdd("z", map[string]float64{"a": 3, "b": 1, "c": 2})

	members, err := s.ZRange("z", 0, -1)
	if err != nil {
		t.Fatalf("ZRange: %v", err)
	}
	wantOrder := []string{"b", "c", "a"}
	for i, m := range members {
		if m.Member != wantOrder[i] {
			t.Fatalf("ZRange order[%d] = %s, want %s (full: %v)", i, m.Member, wantOrder[i], members)
		}
	}

	rank, ok, _ := s.ZRank("z", "a")
	if !ok || rank != 2 {
		t.Fatalf("ZRank(a) = %d, %v; want 2, true", rank, ok)
	}

	sc, ok, _ := s.ZScore("z", "b")
	if !ok || sc != 1 {
		t.Fatalf("ZScore(b) = %v, %v; want 1, true", sc, ok)
	}

	n, _ := s.ZRem("z", "a")
	if n != 1 {
		t.Fatalf("ZRem() = %d, want 1", n)
	}
	card, _ := s.ZCard("z")
	if card != 2 {
		t.Fatalf("ZCard() = %d, want 2", card)
	}
}

func TestKeysGlob(t *testing.T) {
	s := newTestStore(t)
	s.Set("user:1", "a", SetOptions{})
	s.Set("user:2", "b", SetOptions{})
	s.Set("order:1", "c", SetOptions{})

	got := s.Keys("user:*")
	if len(got) != 2 {
		t.Fatalf("Keys(user:*) = %v, want 2 matches", got)
	}
}

func TestConcurrentIncr(t *testing.T) {
	s := newTestStore(t)
	const n = 200
	done := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			s.IncrBy("counter", 1)
			done <- struct{}{}
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}
	v, _, _ := s.Get("counter")
	if v != "200" {
		t.Fatalf("counter = %s, want 200 after %d concurrent increments", v, n)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
