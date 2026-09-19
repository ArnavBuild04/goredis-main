package store

import (
	"strconv"
)

// hashOrWrongType returns the hash at key, creating it via create if
// requested and absent. Callers hold s.mu.
func (s *Store) hashOrWrongType(key string, create bool) (map[string]string, error) {
	e := s.getLocked(key)
	if e == nil {
		if !create {
			return nil, nil
		}
		e = &entry{kind: KindHash, hash: map[string]string{}}
		s.data[key] = e
		return e.hash, nil
	}
	if e.kind != KindHash {
		return nil, ErrWrongType
	}
	return e.hash, nil
}

// HSet sets the given fields on the hash at key (created if absent) and
// returns how many fields were newly created (not merely overwritten).
func (s *Store) HSet(key string, fields map[string]string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.hashOrWrongType(key, true)
	if err != nil {
		return 0, err
	}
	created := 0
	for f, v := range fields {
		if _, exists := h[f]; !exists {
			created++
		}
		h[f] = v
	}
	return created, nil
}

// HGet returns one field's value.
func (s *Store) HGet(key, field string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, err := s.hashOrWrongType(key, false)
	if err != nil || h == nil {
		return "", false, err
	}
	v, ok := h[field]
	return v, ok, nil
}

// HMGet returns one *string per requested field, nil where the field or
// the key itself is absent.
func (s *Store) HMGet(key string, fields []string) ([]*string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, err := s.hashOrWrongType(key, false)
	if err != nil {
		return nil, err
	}
	out := make([]*string, len(fields))
	for i, f := range fields {
		if v, ok := h[f]; ok {
			vv := v
			out[i] = &vv
		}
	}
	return out, nil
}

// HGetAll returns a copy of the whole hash at key.
func (s *Store) HGetAll(key string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, err := s.hashOrWrongType(key, false)
	if err != nil || h == nil {
		return nil, err
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out, nil
}

// HDel removes the given fields and returns how many were actually
// present. An emptied hash is removed from the keyspace entirely, matching
// Redis's "no empty containers" rule.
func (s *Store) HDel(key string, fields ...string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.hashOrWrongType(key, false)
	if err != nil || h == nil {
		return 0, err
	}
	n := 0
	for _, f := range fields {
		if _, ok := h[f]; ok {
			delete(h, f)
			n++
		}
	}
	if len(h) == 0 {
		delete(s.data, key)
	}
	return n, nil
}

// HExists reports whether field is present in the hash at key.
func (s *Store) HExists(key, field string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, err := s.hashOrWrongType(key, false)
	if err != nil || h == nil {
		return false, err
	}
	_, ok := h[field]
	return ok, nil
}

// HLen returns the number of fields in the hash at key.
func (s *Store) HLen(key string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, err := s.hashOrWrongType(key, false)
	if err != nil || h == nil {
		return 0, err
	}
	return len(h), nil
}

// HKeys returns every field name in the hash at key.
func (s *Store) HKeys(key string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, err := s.hashOrWrongType(key, false)
	if err != nil || h == nil {
		return nil, err
	}
	out := make([]string, 0, len(h))
	for k := range h {
		out = append(out, k)
	}
	return out, nil
}

// HVals returns every field value in the hash at key.
func (s *Store) HVals(key string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, err := s.hashOrWrongType(key, false)
	if err != nil || h == nil {
		return nil, err
	}
	out := make([]string, 0, len(h))
	for _, v := range h {
		out = append(out, v)
	}
	return out, nil
}

// HIncrBy adds delta to the integer stored in field (default 0) and returns
// the new value.
func (s *Store) HIncrBy(key, field string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.hashOrWrongType(key, true)
	if err != nil {
		return 0, err
	}
	var cur int64
	if v, ok := h[field]; ok {
		cur, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, errNotAnInteger
		}
	}
	next := cur + delta
	h[field] = strconv.FormatInt(next, 10)
	return next, nil
}
