package server

import (
	"testing"
	"time"
)

func TestBlock403Store(t *testing.T) {
	s := &block403Store{
		blocked: make(map[string]time.Time),
	}

	userHash := "abc12345"

	// ブロック前はfalse
	if s.IsBlocked(userHash) {
		t.Error("should not be blocked initially")
	}

	// ブロック後はtrue
	s.Block(userHash)
	if !s.IsBlocked(userHash) {
		t.Error("should be blocked after Block()")
	}

	// 別ユーザーはブロックされない
	if s.IsBlocked("other_user") {
		t.Error("other user should not be blocked")
	}
}

func TestBlock403StoreExpiry(t *testing.T) {
	s := &block403Store{
		blocked: make(map[string]time.Time),
	}

	userHash := "expired_user"

	// 過去の時刻でブロック（即期限切れ）
	s.mu.Lock()
	s.blocked[userHash] = time.Now().Add(-1 * time.Second)
	s.mu.Unlock()

	// 期限切れなのでブロックされない
	if s.IsBlocked(userHash) {
		t.Error("should not be blocked after expiry")
	}

	// エントリがクリーンアップされている
	s.mu.RLock()
	_, exists := s.blocked[userHash]
	s.mu.RUnlock()
	if exists {
		t.Error("expired entry should be cleaned up")
	}
}
