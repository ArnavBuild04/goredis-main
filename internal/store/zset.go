package store

import "sort"

// zset holds a sorted set's member->score mapping. Real Redis backs this
// with a skip list + hash table so rank/range queries run in O(log N);
// this implementation trades that for a plain map and sorts on read, which
// is O(N log N) per ZRANGE/ZRANK call. That's the right trade for a
// learning project's command surface and dataset sizes — the skip list is
// noted as a roadmap item precisely because it's the interesting part.
type zset struct {
	scores map[string]float64
}

// ZMember is one (member, score) pair, as returned by ZRange.
type ZMember struct {
	Member string
	Score  float64
}

// sorted returns every member ordered by score ascending, ties broken
// lexicographically by member name (matching Redis's ordering rule).
func (z *zset) sorted() []ZMember {
	out := make([]ZMember, 0, len(z.scores))
	for m, sc := range z.scores {
		out = append(out, ZMember{m, sc})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score < out[j].Score
		}
		return out[i].Member < out[j].Member
	})
	return out
}

// zsetOrWrongType returns the sorted set at key, creating it via create if
// requested and absent. Callers hold s.mu.
func (s *Store) zsetOrWrongType(key string, create bool) (*zset, error) {
	e := s.getLocked(key)
	if e == nil {
		if !create {
			return nil, nil
		}
		e = &entry{kind: KindZSet, zset: &zset{scores: map[string]float64{}}}
		s.data[key] = e
		return e.zset, nil
	}
	if e.kind != KindZSet {
		return nil, ErrWrongType
	}
	return e.zset, nil
}

// ZAdd sets each member's score (created if absent) and returns how many
// members were newly added, as opposed to merely re-scored.
func (s *Store) ZAdd(key string, members map[string]float64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	z, err := s.zsetOrWrongType(key, true)
	if err != nil {
		return 0, err
	}
	added := 0
	for m, sc := range members {
		if _, exists := z.scores[m]; !exists {
			added++
		}
		z.scores[m] = sc
	}
	return added, nil
}

// ZScore returns member's score.
func (s *Store) ZScore(key, member string) (float64, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	z, err := s.zsetOrWrongType(key, false)
	if err != nil || z == nil {
		return 0, false, err
	}
	sc, ok := z.scores[member]
	return sc, ok, nil
}

// ZRem removes members and returns how many were actually present. An
// emptied sorted set is removed from the keyspace.
func (s *Store) ZRem(key string, members ...string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	z, err := s.zsetOrWrongType(key, false)
	if err != nil || z == nil {
		return 0, err
	}
	n := 0
	for _, m := range members {
		if _, ok := z.scores[m]; ok {
			delete(z.scores, m)
			n++
		}
	}
	if len(z.scores) == 0 {
		delete(s.data, key)
	}
	return n, nil
}

// ZCard returns the number of members in the sorted set at key.
func (s *Store) ZCard(key string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	z, err := s.zsetOrWrongType(key, false)
	if err != nil || z == nil {
		return 0, err
	}
	return len(z.scores), nil
}

// ZRank returns member's 0-based rank in ascending score order.
func (s *Store) ZRank(key, member string) (int, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	z, err := s.zsetOrWrongType(key, false)
	if err != nil || z == nil {
		return 0, false, err
	}
	if _, ok := z.scores[member]; !ok {
		return 0, false, nil
	}
	for i, sm := range z.sorted() {
		if sm.Member == member {
			return i, true, nil
		}
	}
	return 0, false, nil
}

// ZRange returns members[start:stop] inclusive in ascending score order,
// with Redis's negative-index and out-of-range clamping semantics.
func (s *Store) ZRange(key string, start, stop int) ([]ZMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	z, err := s.zsetOrWrongType(key, false)
	if err != nil || z == nil {
		return nil, err
	}

	all := z.sorted()
	n := len(all)
	start = clampIndex(start, n)
	stop = clampIndex(stop, n)
	if stop >= n {
		stop = n - 1
	}
	if start > stop || start >= n || n == 0 {
		return []ZMember{}, nil
	}
	return all[start : stop+1], nil
}
