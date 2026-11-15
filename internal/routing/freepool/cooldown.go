package freepool

import (
	"sync"
	"time"
)

// Store holds parked (provider, model) pairs until their deadline passes.
type Store struct {
	mu    sync.Mutex
	until map[string]time.Time
}

var (
	defaultOnce  sync.Once
	defaultStore *Store
)

// Default returns the process-wide parking store.
func Default() *Store {
	defaultOnce.Do(func() { defaultStore = &Store{until: make(map[string]time.Time)} })
	return defaultStore
}

func key(provider, model string) string { return provider + "\x00" + model }

// Park keeps a model out of the pool for d. A non-positive d is ignored, and a
// later Park only extends an existing deadline.
func (s *Store) Park(provider, model string, d time.Duration) {
	if d <= 0 {
		return
	}
	until := time.Now().Add(d)
	k := key(provider, model)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.until == nil {
		s.until = make(map[string]time.Time)
	}
	if existing, ok := s.until[k]; ok && !until.After(existing) {
		return
	}
	s.until[k] = until
}

// Parked reports whether the model is still parked. The entry is dropped once
// its deadline has passed.
func (s *Store) Parked(provider, model string) bool {
	k := key(provider, model)
	s.mu.Lock()
	defer s.mu.Unlock()
	until, ok := s.until[k]
	if !ok {
		return false
	}
	if !time.Now().Before(until) {
		delete(s.until, k)
		return false
	}
	return true
}

// Reset clears every parked entry. Tests only.
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.until = make(map[string]time.Time)
}
