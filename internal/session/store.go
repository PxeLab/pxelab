package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Session struct {
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
}

func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	ss := &Store{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}
	go ss.cleanupLoop()
	return ss
}

func (s *Store) Create() *Session {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	now := time.Now()
	session := &Session{
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}
	s.mu.Lock()
	s.sessions[token] = session
	s.mu.Unlock()
	return session
}

func (s *Store) IsValid(token string) bool {
	s.mu.RLock()
	session, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(session.ExpiresAt) {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return false
	}
	return true
}

func (s *Store) Revoke(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

func (s *Store) RevokeAll() {
	s.mu.Lock()
	s.sessions = make(map[string]*Session)
	s.mu.Unlock()
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for token, session := range s.sessions {
			if now.After(session.ExpiresAt) {
				delete(s.sessions, token)
			}
		}
		s.mu.Unlock()
	}
}
