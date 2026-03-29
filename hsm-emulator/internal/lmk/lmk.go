package lmk

import (
	"crypto/cipher"
	"crypto/des"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
)

// PayShield 10K has 20 LMK pairs (00-01, 02-03, ..., 38-39).
// Each key in a pair is a double-length DES key (16 bytes).
// The two keys in a pair are concatenated: LMK_LEFT(16) + LMK_RIGHT(16) = 32 bytes per pair.

const (
	NumPairs    = 20
	KeyLen      = 16 // double-length DES key
	PairLen     = 32 // both halves
	TotalLMKLen = NumPairs * PairLen
)

// Variant values XORed with the first byte of the LMK left half.
var Variants = [6]byte{0x00, 0xA6, 0x5A, 0x6A, 0xDE, 0x2B}

// KeyTypeLMKPair maps PayShield key type codes to the LMK pair index used to encrypt them.
var KeyTypeLMKPair = map[int]int{
	0x000: 4,  // ZMK          → LMK 04-05
	0x001: 6,  // ZPK          → LMK 06-07
	0x002: 14, // PVK/TMK/TPK  → LMK 14-15
	0x003: 16, // TAK          → LMK 16-17
	0x004: 14, // PVK (alt)    → LMK 14-15
	0x005: 20, // (reserved)   → LMK 20-21
	0x006: 26, // ZAK          → LMK 26-27
	0x007: 28, // ZEK          → LMK 28-29
	0x008: 30, // DEK          → LMK 30-31
	0x009: 22, // TEK          → LMK 22-23
	0x00A: 14, // CVK          → LMK 14-15 (variant 4)
	0x00B: 4,  // CSK          → LMK 04-05 (variant 2)
}

// Store holds the LMK key material in memory.
type Store struct {
	mu     sync.RWMutex
	pairs  [NumPairs][PairLen]byte // 20 pairs × 32 bytes each
	loaded bool
}

func New() *Store {
	return &Store{}
}

// GenerateRandom generates a fresh random LMK set.
func (s *Store) GenerateRandom() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i < NumPairs; i++ {
		if _, err := rand.Read(s.pairs[i][:]); err != nil {
			return fmt.Errorf("generate LMK pair %d: %w", i, err)
		}
		// Enforce odd parity on each DES key byte
		for j := 0; j < PairLen; j++ {
			s.pairs[i][j] = setOddParity(s.pairs[i][j])
		}
	}
	s.loaded = true
	return nil
}

// LoadTestLMK loads the well-known Thales test LMK set.
// These are deterministically derived so every emulator instance produces identical results.
func (s *Store) LoadTestLMK() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i < NumPairs; i++ {
		seed := sha256.Sum256([]byte(fmt.Sprintf("THALES-TEST-LMK-PAIR-%02d", i)))
		copy(s.pairs[i][:], seed[:PairLen])
		for j := 0; j < PairLen; j++ {
			s.pairs[i][j] = setOddParity(s.pairs[i][j])
		}
	}
	s.loaded = true
}

// LoadFromFile reads LMK key material from a binary file.
func (s *Store) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < TotalLMKLen {
		return fmt.Errorf("LMK file too short: need %d bytes, got %d", TotalLMKLen, len(data))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i < NumPairs; i++ {
		copy(s.pairs[i][:], data[i*PairLen:(i+1)*PairLen])
	}
	s.loaded = true
	return nil
}

// SaveToFile persists the LMK keys to a binary file.
func (s *Store) SaveToFile(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	buf := make([]byte, TotalLMKLen)
	for i := 0; i < NumPairs; i++ {
		copy(buf[i*PairLen:], s.pairs[i][:])
	}
	return os.WriteFile(path, buf, 0600)
}

// LoadFromComponents loads LMK from 3 key custodian components (XORed together).
// Each component is TotalLMKLen bytes.
func (s *Store) LoadFromComponents(c1, c2, c3 []byte) error {
	if len(c1) < TotalLMKLen || len(c2) < TotalLMKLen || len(c3) < TotalLMKLen {
		return fmt.Errorf("each component must be at least %d bytes", TotalLMKLen)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	buf := make([]byte, TotalLMKLen)
	for i := 0; i < TotalLMKLen; i++ {
		buf[i] = c1[i] ^ c2[i] ^ c3[i]
	}
	for i := 0; i < NumPairs; i++ {
		copy(s.pairs[i][:], buf[i*PairLen:(i+1)*PairLen])
		for j := 0; j < PairLen; j++ {
			s.pairs[i][j] = setOddParity(s.pairs[i][j])
		}
	}
	s.loaded = true
	return nil
}

func (s *Store) IsLoaded() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loaded
}

// GetPair returns the raw 32-byte LMK pair (left 16 + right 16).
// pairIndex is the LMK number: 0=pair 00-01, 2=pair 02-03, etc.
func (s *Store) GetPair(pairIndex int) ([PairLen]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx := pairIndex / 2
	if idx < 0 || idx >= NumPairs {
		return [PairLen]byte{}, fmt.Errorf("invalid LMK pair index: %d", pairIndex)
	}
	return s.pairs[idx], nil
}

// CheckValues returns the KCV (key check value) for each LMK pair.
// KCV = first 3 bytes of 3DES-ECB encrypt of 8 zero bytes using the key.
func (s *Store) CheckValues() ([][6]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.loaded {
		return nil, fmt.Errorf("LMK not loaded")
	}
	result := make([][6]byte, NumPairs)
	for i := 0; i < NumPairs; i++ {
		// Use the left half (16 bytes) to compute KCV
		kcv, err := computeKCV(s.pairs[i][:KeyLen])
		if err != nil {
			return nil, fmt.Errorf("KCV for pair %02d-%02d: %w", i*2, i*2+1, err)
		}
		copy(result[i][:], []byte(kcv))
	}
	return result, nil
}

// EncryptUnderLMK encrypts a working key under the appropriate LMK pair with variant.
// Returns the encrypted key as uppercase hex string.
func (s *Store) EncryptUnderLMK(clearKey []byte, keyTypeCode int, variant int) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.loaded {
		return "", fmt.Errorf("LMK not loaded")
	}

	pairIdx, ok := KeyTypeLMKPair[keyTypeCode]
	if !ok {
		pairIdx = 4 // default to LMK 04-05
	}
	lmkPair := s.pairs[pairIdx/2]

	// Apply variant to left half
	lmkKey := make([]byte, KeyLen)
	copy(lmkKey, lmkPair[:KeyLen])
	if variant >= 0 && variant < len(Variants) && variant > 0 {
		lmkKey[0] ^= Variants[variant]
	}

	return tripleDESEncryptECB(lmkKey, clearKey)
}

// DecryptUnderLMK decrypts a working key encrypted under LMK.
func (s *Store) DecryptUnderLMK(encKeyHex string, keyTypeCode int, variant int) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.loaded {
		return nil, fmt.Errorf("LMK not loaded")
	}

	encKey, err := hex.DecodeString(strings.ToUpper(encKeyHex))
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	pairIdx, ok := KeyTypeLMKPair[keyTypeCode]
	if !ok {
		pairIdx = 4
	}
	lmkPair := s.pairs[pairIdx/2]

	lmkKey := make([]byte, KeyLen)
	copy(lmkKey, lmkPair[:KeyLen])
	if variant >= 0 && variant < len(Variants) && variant > 0 {
		lmkKey[0] ^= Variants[variant]
	}

	return tripleDESDecryptECB(lmkKey, encKey)
}

// ComputeKCV computes a Key Check Value: 3DES-ECB encrypt 8 zero bytes, return first 6 hex chars.
func ComputeKCV(key []byte) (string, error) {
	return computeKCV(key)
}

func computeKCV(key []byte) (string, error) {
	zeros := make([]byte, 8)
	result, err := tripleDESEncryptECBRaw(key, zeros)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(result[:3])), nil
}

// ── 3DES helpers ─────────────────────────────────────────────

func tripleDESEncryptECB(key, plaintext []byte) (string, error) {
	result, err := tripleDESEncryptECBRaw(key, plaintext)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(result)), nil
}

func tripleDESEncryptECBRaw(key, plaintext []byte) ([]byte, error) {
	tdesKey := expandTo24(key)
	block, err := des.NewTripleDESCipher(tdesKey)
	if err != nil {
		return nil, fmt.Errorf("3DES cipher: %w", err)
	}
	bs := block.BlockSize()
	// Round up result size to a full block boundary
	resultLen := len(plaintext)
	if resultLen%bs != 0 {
		resultLen += bs - (resultLen % bs)
	}
	result := make([]byte, resultLen)
	// ECB mode: encrypt each 8-byte block independently
	for i := 0; i < len(plaintext); i += bs {
		end := i + bs
		if end > len(plaintext) {
			// Pad last partial block with zeros
			padded := make([]byte, bs)
			copy(padded, plaintext[i:])
			block.Encrypt(result[i:i+bs], padded)
		} else {
			block.Encrypt(result[i:end], plaintext[i:end])
		}
	}
	return result, nil
}

func tripleDESDecryptECB(key, ciphertext []byte) ([]byte, error) {
	tdesKey := expandTo24(key)
	block, err := des.NewTripleDESCipher(tdesKey)
	if err != nil {
		return nil, fmt.Errorf("3DES cipher: %w", err)
	}
	result := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += block.BlockSize() {
		end := i + block.BlockSize()
		if end > len(ciphertext) {
			break
		}
		block.Decrypt(result[i:end], ciphertext[i:end])
	}
	return result, nil
}

// TripleDESEncryptCBC encrypts using 3DES-CBC with the given IV.
func TripleDESEncryptCBC(key, iv, plaintext []byte) ([]byte, error) {
	tdesKey := expandTo24(key)
	block, err := des.NewTripleDESCipher(tdesKey)
	if err != nil {
		return nil, err
	}
	// Pad to block size
	padLen := block.BlockSize() - (len(plaintext) % block.BlockSize())
	if padLen == block.BlockSize() && len(plaintext) > 0 {
		padLen = 0
	}
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)

	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)
	return ciphertext, nil
}

// expandTo24 takes a 8 or 16-byte key and returns a 24-byte key for 3DES (K1||K2||K1).
func expandTo24(key []byte) []byte {
	switch len(key) {
	case 24:
		return key
	case 16:
		out := make([]byte, 24)
		copy(out, key)
		copy(out[16:], key[:8])
		return out
	case 8:
		out := make([]byte, 24)
		copy(out[0:8], key)
		copy(out[8:16], key)
		copy(out[16:24], key)
		return out
	default:
		// best effort: pad or truncate
		out := make([]byte, 24)
		copy(out, key)
		if len(key) >= 8 {
			copy(out[16:], key[:8])
		}
		return out
	}
}

func setOddParity(b byte) byte {
	// Count bits
	bits := 0
	v := b
	for i := 0; i < 8; i++ {
		bits += int(v & 1)
		v >>= 1
	}
	if bits%2 == 0 {
		return b ^ 1 // flip LSB to make parity odd
	}
	return b
}
