// Package store is the in-memory data engine: a single sharded-by-nothing
// keyspace holding strings, hashes, lists, sets and sorted sets, each with
// an optional expiry. It has no knowledge of RESP or the network — command
// handlers translate protocol requests into calls against this package.
package store

import (
	"errors"
	"sync"
	"time"
)

// Kind identifies the type of value stored under a key, mirroring Redis's
// TYPE command output.
type Kind string

const (
	KindString Kind = "string"
	KindHash   Kind = "hash"
	KindList   Kind = "list"
	KindSet    Kind = "set"
	KindZSet   Kind = "zset"
)

// ErrWrongType is returned when a command targets a key holding a value of
// a different kind, e.g. LPUSH against a key created by SET.
var ErrWrongType = errors.New("WRONGTYPE Operation against a key holding the wrong kind of value")

// entry is one keyspace slot. Exactly one of the typed fields is populated,
// selected by kind. A zero expireAt means the key never expires.
type entry struct {
	kind     Kind
	str      string
	hash     map[string]string
	list     []string
	set      map[string]struct{}
	zset     *zset
	expireAt time.Time
}

func (e *entry) expired(now time.Time) bool {
	return !e.expireAt.IsZero() && !now.Before(e.expireAt)
}

// Store is the keyspace. All methods are safe for concurrent use.
type Store struct {
	mu   sync.RWMutex
	data map[string]*entry

	stopSweep chan struct{}
	sweepOnce sync.Once
}

// New returns an empty Store and starts its background expiry sweeper.
func New() *Store {
	s := &Store{
		data:      make(map[string]*entry),
		stopSweep: make(chan struct{}),
	}
	go s.sweepExpired(100 * time.Millisecond)
	return s
}

// Close stops the background expiry sweeper. Safe to call once.
func (s *Store) Close() {
	s.sweepOnce.Do(func() { close(s.stopSweep) })
}

// sweepExpired periodically scans for and removes expired keys, so idle
// keys don't survive forever just because nothing ever reads them again.
// Real Redis samples the keyspace rather than scanning it in full; a full
// scan is simpler and cheap enough at the scale this server targets.
func (s *Store) sweepExpired(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopSweep:
			return
		case now := <-ticker.C:
			s.mu.Lock()
			for k, e := range s.data {
				if e.expired(now) {
					delete(s.data, k)
				}
			}
			s.mu.Unlock()
		}
	}
}

// getLocked returns the live entry for key, or nil if it's absent or has
// expired. Callers must hold s.mu (read or write lock).
func (s *Store) getLocked(key string) *entry {
	e, ok := s.data[key]
	if !ok {
		return nil
	}
	if e.expired(time.Now()) {
		return nil
	}
	return e
}

// Type reports the Kind stored under key.
func (s *Store) Type(key string) (Kind, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e := s.getLocked(key)
	if e == nil {
		return "", false
	}
	return e.kind, true
}

// Del removes the given keys and returns how many actually existed.
func (s *Store) Del(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, k := range keys {
		if s.getLocked(k) != nil {
			delete(s.data, k)
			n++
		}
	}
	return n
}

// Exists returns how many of the given keys are present (a key repeated in
// the argument list is counted once per occurrence, matching Redis).
func (s *Store) Exists(keys ...string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, k := range keys {
		if s.getLocked(k) != nil {
			n++
		}
	}
	return n
}

// Expire sets key to expire after ttl. Returns false if key does not exist.
func (s *Store) Expire(key string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.getLocked(key)
	if e == nil {
		return false
	}
	e.expireAt = time.Now().Add(ttl)
	return true
}

// ExpireAt sets key to expire at the given absolute time.
func (s *Store) ExpireAt(key string, at time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.getLocked(key)
	if e == nil {
		return false
	}
	e.expireAt = at
	return true
}

// TTL returns the remaining time to live for key. It returns (0, false) if
// key does not exist, and (0, true) with a zero duration meaning "no
// expiry" signaled via the ok/hasTTL pair below — callers use TTLSeconds
// for the Redis-shaped -2/-1/N encoding instead of calling this directly.
func (s *Store) ttl(key string) (ttl time.Duration, exists, hasExpiry bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e := s.getLocked(key)
	if e == nil {
		return 0, false, false
	}
	if e.expireAt.IsZero() {
		return 0, true, false
	}
	remaining := max(time.Until(e.expireAt), 0)
	return remaining, true, true
}

// TTLSeconds returns the key's remaining lifetime the way Redis's TTL
// command reports it: -2 if the key doesn't exist, -1 if it exists but has
// no expiry, otherwise the remaining seconds (rounded up).
func (s *Store) TTLSeconds(key string) int64 {
	ttl, exists, hasExpiry := s.ttl(key)
	if !exists {
		return -2
	}
	if !hasExpiry {
		return -1
	}
	return int64((ttl + time.Second - 1) / time.Second)
}

// TTLMillis is TTLSeconds at millisecond resolution, for PTTL.
func (s *Store) TTLMillis(key string) int64 {
	ttl, exists, hasExpiry := s.ttl(key)
	if !exists {
		return -2
	}
	if !hasExpiry {
		return -1
	}
	return ttl.Milliseconds()
}

// Persist removes key's expiry. Returns false if key doesn't exist or had
// no expiry to remove.
func (s *Store) Persist(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.getLocked(key)
	if e == nil || e.expireAt.IsZero() {
		return false
	}
	e.expireAt = time.Time{}
	return true
}

// Keys returns every key whose name matches the glob-style pattern (per
// path.Match: '*', '?' and '[...]' classes — enough for interactive use,
// not the full Redis glob grammar). Keys is O(n) and, like real Redis,
// unsafe to run against a very large keyspace in production.
func (s *Store) Keys(pattern string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	var out []string
	for k, e := range s.data {
		if e.expired(now) {
			continue
		}
		if ok, _ := matchGlob(pattern, k); ok {
			out = append(out, k)
		}
	}
	return out
}

// FlushAll drops every key.
func (s *Store) FlushAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]*entry)
}

// DBSize returns the number of live (non-expired) keys.
func (s *Store) DBSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	n := 0
	for _, e := range s.data {
		if !e.expired(now) {
			n++
		}
	}
	return n
}

// Rename moves the value at src to dst, overwriting dst if present. It
// returns an error if src does not exist.
func (s *Store) Rename(src, dst string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.getLocked(src)
	if e == nil {
		return errors.New("ERR no such key")
	}
	delete(s.data, src)
	s.data[dst] = e
	return nil
}
