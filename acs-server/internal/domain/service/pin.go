package service

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	pinLength = 4
	pinTTL    = 5 * time.Minute
)

// PINService handles generation and verification of card PIN codes for second factor auth.
type PINService struct{}

func NewPINService() *PINService {
	return &PINService{}
}

// Generate returns a random 4-digit PIN, its bcrypt hash, and expiry time.
func (s *PINService) Generate() (pin, hash string, expiry time.Time, err error) {
	digits := make([]byte, pinLength)
	for i := range digits {
		n, e := rand.Int(rand.Reader, big.NewInt(10))
		if e != nil {
			return "", "", time.Time{}, fmt.Errorf("pin generation failed: %w", e)
		}
		digits[i] = byte('0') + byte(n.Int64())
	}
	pin = string(digits)

	b, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("pin hashing failed: %w", err)
	}
	hash = base64.StdEncoding.EncodeToString(b)
	expiry = time.Now().UTC().Add(pinTTL)
	return pin, hash, expiry, nil
}

// Verify compares a submitted PIN against the stored hash.
func (s *PINService) Verify(submitted, storedHash string, expiry time.Time) bool {
	if time.Now().UTC().After(expiry) {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(storedHash)
	if err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword(decoded, []byte(submitted)) == nil
}
