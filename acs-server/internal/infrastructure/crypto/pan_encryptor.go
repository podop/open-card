package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// PANEncryptor provides AES-256-GCM encryption for PANs stored at rest.
// In production, delegate to an HSM or cloud KMS rather than an in-process key.
type PANEncryptor struct {
	key []byte
}

// NewPANEncryptor creates a PANEncryptor from a hex-encoded 32-byte key.
func NewPANEncryptor(keyHex string) (*PANEncryptor, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("decode PAN key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("PAN encryption key must be 32 bytes (AES-256), got %d", len(key))
	}
	return &PANEncryptor{key: key}, nil
}

// Encrypt encrypts a plaintext PAN and returns a hex-encoded ciphertext (nonce || ciphertext || tag).
func (e *PANEncryptor) Encrypt(pan string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(pan), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a hex-encoded ciphertext back to a PAN string.
func (e *PANEncryptor) Decrypt(ciphertextHex string) (string, error) {
	data, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, data := data[:nonceSize], data[nonceSize:]

	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}
