package store

// listOrWrongType returns the list at key, creating it via create if
// requested and absent. Callers hold s.mu.
func (s *Store) listOrWrongType(key string, create bool) (*entry, error) {
	e := s.getLocked(key)
	if e == nil {
		if !create {
			return nil, nil
		}
		e = &entry{kind: KindList}
		s.data[key] = e
		return e, nil
	}
	if e.kind != KindList {
		return nil, ErrWrongType
	}
	return e, nil
}

// LPush prepends values to the list at key (created if absent), each
// pushed one at a time so the last argument ends up at the head, matching
// Redis. Returns the resulting list length.
func (s *Store) LPush(key string, values ...string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.listOrWrongType(key, true)
	if err != nil {
		return 0, err
	}
	for _, v := range values {
		e.list = append([]string{v}, e.list...)
	}
	return len(e.list), nil
}

// RPush appends values to the list at key (created if absent). Returns the
// resulting list length.
func (s *Store) RPush(key string, values ...string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.listOrWrongType(key, true)
	if err != nil {
		return 0, err
	}
	e.list = append(e.list, values...)
	return len(e.list), nil
}

// LPop removes and returns the head of the list at key. An emptied list is
// removed from the keyspace.
func (s *Store) LPop(key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.listOrWrongType(key, false)
	if err != nil || e == nil || len(e.list) == 0 {
		return "", false, err
	}
	v := e.list[0]
	e.list = e.list[1:]
	if len(e.list) == 0 {
		delete(s.data, key)
	}
	return v, true, nil
}

// RPop removes and returns the tail of the list at key.
func (s *Store) RPop(key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.listOrWrongType(key, false)
	if err != nil || e == nil || len(e.list) == 0 {
		return "", false, err
	}
	n := len(e.list)
	v := e.list[n-1]
	e.list = e.list[:n-1]
	if len(e.list) == 0 {
		delete(s.data, key)
	}
	return v, true, nil
}

// LLen returns the length of the list at key, 0 if absent.
func (s *Store) LLen(key string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, err := s.listOrWrongType(key, false)
	if err != nil || e == nil {
		return 0, err
	}
	return len(e.list), nil
}

// LRange returns list[start:stop] inclusive, with Redis's negative-index
// (from the tail) and out-of-range clamping semantics.
func (s *Store) LRange(key string, start, stop int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, err := s.listOrWrongType(key, false)
	if err != nil || e == nil {
		return nil, err
	}

	n := len(e.list)
	start = clampIndex(start, n)
	stop = clampIndex(stop, n)
	if stop >= n {
		stop = n - 1
	}
	if start > stop || start >= n || n == 0 {
		return []string{}, nil
	}

	out := make([]string, stop-start+1)
	copy(out, e.list[start:stop+1])
	return out, nil
}

// LIndex returns the element at idx (Redis-style negative indexing).
func (s *Store) LIndex(key string, idx int) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, err := s.listOrWrongType(key, false)
	if err != nil || e == nil {
		return "", false, err
	}
	n := len(e.list)
	if idx < 0 {
		idx += n
	}
	if idx < 0 || idx >= n {
		return "", false, nil
	}
	return e.list[idx], true, nil
}

// clampIndex converts a possibly-negative Redis-style index (counted from
// the tail) into a non-negative one, floored at 0.
func clampIndex(idx, n int) int {
	if idx < 0 {
		idx += n
		if idx < 0 {
			idx = 0
		}
	}
	return idx
}
