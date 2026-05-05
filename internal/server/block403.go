package server

import (
	"sync"
	"time"
)

const block403Duration = 10 * time.Minute

// block403Store は403を受けたユーザーを一時的にブロックするインメモリストア
type block403Store struct {
	mu      sync.RWMutex
	blocked map[string]time.Time // userHash -> blocked until
}

var forbidden403 = &block403Store{
	blocked: make(map[string]time.Time),
}

// Block はユーザーを10分間ブロックする
func (s *block403Store) Block(userHash string) {
	s.mu.Lock()
	s.blocked[userHash] = time.Now().Add(block403Duration)
	s.mu.Unlock()
}

// IsBlocked はユーザーがブロック中かを返す
func (s *block403Store) IsBlocked(userHash string) bool {
	s.mu.RLock()
	until, ok := s.blocked[userHash]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(until) {
		s.mu.Lock()
		delete(s.blocked, userHash)
		s.mu.Unlock()
		return false
	}
	return true
}
