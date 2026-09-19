package store

import (
	"errors"
	"strconv"
	"time"
)

// SetOptions mirrors the option flags SET accepts: an optional expiry and
// existence-conditioned write (NX / XX).
type SetOptions struct {
	TTL       time.Duration // zero means no expiry
	HasTTL    bool
	OnlyIfNew bool // NX
	OnlyIfOld bool // XX
}

// Set writes key=value, unconditionally replacing whatever kind of value
// (if any) previously lived there — like real Redis, SET is not subject to
// WRONGTYPE. It returns ok=false without writing when an NX/XX condition in
// opts isn't satisfied.
func (s *Store) Set(key, value string, opts SetOptions) (ok bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	exists := s.getLocked(key) != nil
	if opts.OnlyIfNew && exists {
		return false, nil
	}
	if opts.OnlyIfOld && !exists {
		return false, nil
	}

	e := &entry{kind: KindString, str: value}
	if opts.HasTTL {
		e.expireAt = time.Now().Add(opts.TTL)
	}
	s.data[key] = e
	return true, nil
}

// stringOrWrongType returns the live entry at key if it exists, requiring
// it to be a string; nil, nil if the key is absent.
func (s *Store) stringOrWrongType(key string) (*entry, error) {
	e := s.getLocked(key)
	if e == nil {
		return nil, nil
	}
	if e.kind != KindString {
		return nil, ErrWrongType
	}
	return e, nil
}

// Get returns the string at key. found is false if the key doesn't exist.
func (s *Store) Get(key string) (value string, found bool, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, err := s.stringOrWrongType(key)
	if err != nil || e == nil {
		return "", false, err
	}
	return e.str, true, nil
}

// GetSet atomically sets key to value and returns the previous value.
func (s *Store) GetSet(key, value string) (old string, found bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.stringOrWrongType(key)
	if err != nil {
		return "", false, err
	}
	if e != nil {
		old, found = e.str, true
	}
	s.data[key] = &entry{kind: KindString, str: value}
	return old, found, nil
}

// MSet writes every key/value pair unconditionally.
func (s *Store) MSet(pairs map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range pairs {
		s.data[k] = &entry{kind: KindString, str: v}
	}
}

// MGet returns one *string per requested key: nil for a missing key or a
// key holding a non-string value (matching Redis's MGET semantics, which
// treats a type mismatch there as "no value" rather than an error).
func (s *Store) MGet(keys []string) []*string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*string, len(keys))
	for i, k := range keys {
		e := s.getLocked(k)
		if e == nil || e.kind != KindString {
			continue
		}
		v := e.str
		out[i] = &v
	}
	return out
}

var errNotAnInteger = errors.New("ERR value is not an integer or out of range")

// IncrBy adds delta to the integer stored at key (default 0 if absent) and
// returns the new value.
func (s *Store) IncrBy(key string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, err := s.stringOrWrongType(key)
	if err != nil {
		return 0, err
	}

	var cur int64
	if e != nil {
		cur, err = strconv.ParseInt(e.str, 10, 64)
		if err != nil {
			return 0, errNotAnInteger
		}
	}

	next := cur + delta
	if (delta > 0 && next < cur) || (delta < 0 && next > cur) {
		return 0, errors.New("ERR increment or decrement would overflow")
	}

	if e != nil {
		e.str = strconv.FormatInt(next, 10)
	} else {
		s.data[key] = &entry{kind: KindString, str: strconv.FormatInt(next, 10)}
	}
	return next, nil
}

// Append concatenates value onto the string at key (creating it if absent)
// and returns the resulting length.
func (s *Store) Append(key, value string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.stringOrWrongType(key)
	if err != nil {
		return 0, err
	}
	if e == nil {
		s.data[key] = &entry{kind: KindString, str: value}
		return len(value), nil
	}
	e.str += value
	return len(e.str), nil
}

// StrLen returns the length of the string at key, 0 if absent.
func (s *Store) StrLen(key string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, err := s.stringOrWrongType(key)
	if err != nil || e == nil {
		return 0, err
	}
	return len(e.str), nil
}
