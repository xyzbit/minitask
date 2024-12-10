package queue

type (
	empty             struct{}
	t                 interface{}
	set[t comparable] map[t]empty
)

func (s set[t]) has(item t) bool {
	key := s.getKey(item)
	_, exists := s[key]
	return exists
}

func (s set[t]) insert(item t) {
	key := s.getKey(item)
	s[key] = empty{}
}

func (s set[t]) delete(item t) {
	key := s.getKey(item)
	delete(s, key)
}

func (s set[t]) len() int {
	return len(s)
}

func (s set[t]) getKey(item t) t {
	if uniKeyer, ok := any(item).(UniKey[t]); ok {
		return uniKeyer.GetUniKey()
	}
	return item
}
