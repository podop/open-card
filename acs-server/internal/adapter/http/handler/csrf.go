package handler

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// csrfStore manages per-transaction CSRF tokens with one-time use semantics.
// Each token is bound to an acsTransID and can only be used once, preventing
// both CSRF attacks and duplicate form submissions.
type csrfStore struct {
	mu     sync.Mutex
	tokens map[string]*csrfEntry // key: acsTransID
}

type csrfEntry struct {
	token    string
	used     bool
	expireAt time.Time
}

var globalCSRF = &csrfStore{
	tokens: make(map[string]*csrfEntry),
}

func init() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			globalCSRF.cleanup()
		}
	}()
}

// Generate creates a new CSRF token for the given acsTransID.
// If a valid unused token already exists, it is returned.
func (s *csrfStore) Generate(acsTransID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e, ok := s.tokens[acsTransID]; ok && !e.used && time.Now().Before(e.expireAt) {
		return e.token
	}

	token := generateCSRFToken()
	s.tokens[acsTransID] = &csrfEntry{
		token:    token,
		expireAt: time.Now().Add(15 * time.Minute),
	}
	return token
}

// Validate checks the token and marks it as consumed (one-time use).
// Returns true if valid and unused, false if invalid, expired, or already consumed.
func (s *csrfStore) Validate(acsTransID, token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.tokens[acsTransID]
	if !ok || e.token != token || time.Now().After(e.expireAt) {
		return false
	}
	if e.used {
		return false // duplicate request
	}
	e.used = true
	return true
}

// Refresh replaces the token for an acsTransID (e.g. after a failed OTP attempt
// when the page is re-rendered and a fresh token is needed).
func (s *csrfStore) Refresh(acsTransID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	token := generateCSRFToken()
	s.tokens[acsTransID] = &csrfEntry{
		token:    token,
		expireAt: time.Now().Add(15 * time.Minute),
	}
	return token
}

func (s *csrfStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, e := range s.tokens {
		if now.After(e.expireAt) {
			delete(s.tokens, k)
		}
	}
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("csrf: failed to generate random token")
	}
	return hex.EncodeToString(b)
}