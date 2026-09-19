package store

// setOrWrongType returns the set at key, creating it via create if
// requested and absent. Callers hold s.mu.
func (s *Store) setOrWrongType(key string, create bool) (map[string]struct{}, error) {
	e := s.getLocked(key)
	if e == nil {
		if !create {
			return nil, nil
		}
		e = &entry{kind: KindSet, set: map[string]struct{}{}}
		s.data[key] = e
		return e.set, nil
	}
	if e.kind != KindSet {
		return nil, ErrWrongType
	}
	return e.set, nil
}

// SAdd adds members to the set at key (created if absent) and returns how
// many were newly added.
func (s *Store) SAdd(key string, members ...string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, err := s.setOrWrongType(key, true)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, m := range members {
		if _, exists := set[m]; !exists {
			set[m] = struct{}{}
			n++
		}
	}
	return n, nil
}

// SRem removes members from the set at key and returns how many were
// actually present. An emptied set is removed from the keyspace.
func (s *Store) SRem(key string, members ...string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, err := s.setOrWrongType(key, false)
	if err != nil || set == nil {
		return 0, err
	}
	n := 0
	for _, m := range members {
		if _, ok := set[m]; ok {
			delete(set, m)
			n++
		}
	}
	if len(set) == 0 {
		delete(s.data, key)
	}
	return n, nil
}

// SMembers returns every member of the set at key.
func (s *Store) SMembers(key string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set, err := s.setOrWrongType(key, false)
	if err != nil || set == nil {
		return nil, err
	}
	out := make([]string, 0, len(set))
	for m := range set {
		out = append(out, m)
	}
	return out, nil
}

// SIsMember reports whether member is in the set at key.
func (s *Store) SIsMember(key, member string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set, err := s.setOrWrongType(key, false)
	if err != nil || set == nil {
		return false, err
	}
	_, ok := set[member]
	return ok, nil
}

// SCard returns the number of members in the set at key, 0 if absent.
func (s *Store) SCard(key string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set, err := s.setOrWrongType(key, false)
	if err != nil || set == nil {
		return 0, err
	}
	return len(set), nil
}
