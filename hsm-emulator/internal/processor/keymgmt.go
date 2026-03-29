package processor

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strconv"
	"strings"

	"github.com/your-org/hsm-emulator/internal/lmk"
)

// handleA0 — Generate a Key (A0 → A1)
// Request:  mode(1) + key_type(3) + key_scheme(1) [+ optional fields]
// Response: key_under_lmk + key_check_value(6)
func (p *Processor) handleA0(data []byte) (string, []byte) {
	if len(data) < 5 {
		return ErrBadMsgLen, nil
	}

	mode := string(data[0])
	keyTypeStr, _ := safeString(data, 1, 3)
	scheme, _ := safeString(data, 4, 1)

	keyType, err := strconv.Atoi(keyTypeStr)
	if err != nil {
		return ErrBadKeyType, nil
	}

	_ = mode // mode 0=generate, 1=generate+export (simplified)

	// Determine key length from scheme
	var keyLen int
	switch scheme {
	case "Z": // single-length DES
		keyLen = 8
	case "U": // double-length 3DES
		keyLen = 16
	case "T": // triple-length 3DES
		keyLen = 24
	case "X": // double-length variant (key block)
		keyLen = 16
	default:
		keyLen = 16 // default double-length
	}

	// Generate random key material
	clearKey := make([]byte, keyLen)
	if _, err := rand.Read(clearKey); err != nil {
		return ErrInternalErr, nil
	}
	// Enforce odd parity
	for i := range clearKey {
		clearKey[i] = setOddParity(clearKey[i])
	}

	// Encrypt under LMK
	encKey, err := p.lmk.EncryptUnderLMK(clearKey, keyType, 0)
	if err != nil {
		return ErrInternalErr, nil
	}

	// Compute check value
	kcv, err := lmk.ComputeKCV(clearKey)
	if err != nil {
		return ErrInternalErr, nil
	}

	// Response: scheme_tag + encrypted_key + check_value
	resp := fmt.Sprintf("%s%s%s", scheme, encKey, kcv)
	return ErrOK, []byte(resp)
}

// handleA6 — Import Key under ZMK (A6 → A7)
// Request:  key_type(3) + zmk_under_lmk + key_under_zmk + key_scheme(1)
// Response: key_under_lmk + key_check_value(6)
func (p *Processor) handleA6(data []byte) (string, []byte) {
	if len(data) < 3 {
		return ErrBadMsgLen, nil
	}

	keyTypeStr, _ := safeString(data, 0, 3)
	keyType, err := strconv.Atoi(keyTypeStr)
	if err != nil {
		return ErrBadKeyType, nil
	}
	rest := string(data[3:])

	// Parse ZMK (under LMK) - could be 16 or 32 hex chars (single or double length)
	// Then the imported key, then scheme
	// For simplicity: assume both are double-length (32 hex chars each)
	zmkLen := 32
	if len(rest) > 65 && (rest[0] == 'U' || rest[0] == 'X') {
		rest = rest[1:] // skip scheme tag on ZMK
		zmkLen = 32
	} else if len(rest) > 49 && rest[0] == 'Z' {
		rest = rest[1:]
		zmkLen = 16
	}

	if len(rest) < zmkLen+zmkLen+1 {
		return ErrBadMsgLen, nil
	}

	zmkHex := rest[:zmkLen]
	importedHex := rest[zmkLen : zmkLen+zmkLen]
	scheme := rest[zmkLen+zmkLen : zmkLen+zmkLen+1]

	// Decrypt ZMK from under LMK (key type 000 = ZMK)
	clearZMK, err := p.lmk.DecryptUnderLMK(zmkHex, 0x000, 0)
	if err != nil {
		return ErrParityError, nil
	}

	// Decrypt imported key from under ZMK
	importedBytes, err := hex.DecodeString(importedHex)
	if err != nil {
		return ErrBadInput, nil
	}
	clearKey, err := tripleDESDecrypt(clearZMK, importedBytes)
	if err != nil {
		return ErrInternalErr, nil
	}

	// Re-encrypt under LMK
	encKey, err := p.lmk.EncryptUnderLMK(clearKey, keyType, 0)
	if err != nil {
		return ErrInternalErr, nil
	}

	kcv, err := lmk.ComputeKCV(clearKey)
	if err != nil {
		return ErrInternalErr, nil
	}

	_ = scheme
	resp := fmt.Sprintf("U%s%s", encKey, kcv)
	return ErrOK, []byte(resp)
}

// handleA8 — Export Key under ZMK (A8 → A9)
// Request:  key_type(3) + zmk_under_lmk + key_under_lmk + scheme(1)
// Response: key_under_zmk + check_value(6)
func (p *Processor) handleA8(data []byte) (string, []byte) {
	if len(data) < 3 {
		return ErrBadMsgLen, nil
	}

	keyTypeStr, _ := safeString(data, 0, 3)
	keyType, err := strconv.Atoi(keyTypeStr)
	if err != nil {
		return ErrBadKeyType, nil
	}
	rest := string(data[3:])

	zmkLen := 32
	if len(rest) > 0 && (rest[0] == 'U' || rest[0] == 'X') {
		rest = rest[1:]
	}

	if len(rest) < zmkLen+zmkLen+1 {
		return ErrBadMsgLen, nil
	}

	zmkHex := rest[:zmkLen]
	keyHex := rest[zmkLen : zmkLen+zmkLen]
	if keyHex[0] == 'U' || keyHex[0] == 'X' {
		keyHex = keyHex[1 : zmkLen+1]
	}
	scheme := rest[len(rest)-1:]

	// Decrypt ZMK
	clearZMK, err := p.lmk.DecryptUnderLMK(zmkHex, 0x000, 0)
	if err != nil {
		return ErrParityError, nil
	}

	// Decrypt key under LMK
	clearKey, err := p.lmk.DecryptUnderLMK(keyHex, keyType, 0)
	if err != nil {
		return ErrParityError, nil
	}

	// Encrypt key under ZMK
	encKey, err := tripleDESEncrypt(clearZMK, clearKey)
	if err != nil {
		return ErrInternalErr, nil
	}

	kcv, err := lmk.ComputeKCV(clearKey)
	if err != nil {
		return ErrInternalErr, nil
	}

	_ = scheme
	resp := fmt.Sprintf("U%s%s", encKey, kcv)
	return ErrOK, []byte(resp)
}

// handleBU — Generate Key Check Value (BU → BV)
// Request:  key_type(3) + key_under_lmk + ";" + check_length(1)
// Response: check_value
func (p *Processor) handleBU(data []byte) (string, []byte) {
	if len(data) < 3 {
		return ErrBadMsgLen, nil
	}

	keyTypeStr, _ := safeString(data, 0, 3)
	keyType, err := strconv.Atoi(keyTypeStr)
	if err != nil {
		return ErrBadKeyType, nil
	}

	rest := string(data[3:])

	// Find the key hex (before the semicolon delimiter)
	parts := strings.SplitN(rest, ";", 2)
	keyHex := parts[0]
	// Strip scheme prefix
	if len(keyHex) > 0 && (keyHex[0] == 'U' || keyHex[0] == 'X' || keyHex[0] == 'T' || keyHex[0] == 'Z') {
		keyHex = keyHex[1:]
	}

	checkLen := 6 // default: 6 hex chars = 3 bytes
	if len(parts) > 1 && len(parts[1]) > 0 {
		if parts[1][0] == '1' {
			checkLen = 6
		}
	}

	// Decrypt key under LMK
	clearKey, err := p.lmk.DecryptUnderLMK(keyHex, keyType, 0)
	if err != nil {
		return ErrParityError, nil
	}

	kcv, err := lmk.ComputeKCV(clearKey)
	if err != nil {
		return ErrInternalErr, nil
	}

	_ = checkLen
	return ErrOK, []byte(kcv)
}

// handleB0 — Translate Key Scheme (B0 → B1)
// Re-encrypts a key from one scheme to another under LMK.
func (p *Processor) handleB0(data []byte) (string, []byte) {
	if len(data) < 5 {
		return ErrBadMsgLen, nil
	}

	keyTypeStr, _ := safeString(data, 0, 3)
	keyType, err := strconv.Atoi(keyTypeStr)
	if err != nil {
		return ErrBadKeyType, nil
	}

	rest := string(data[3:])
	srcScheme := rest[0:1]
	keyHex := rest[1:]
	// Find dest scheme at the end
	destScheme := keyHex[len(keyHex)-1:]
	keyHex = keyHex[:len(keyHex)-1]

	// Decrypt from LMK
	clearKey, err := p.lmk.DecryptUnderLMK(keyHex, keyType, 0)
	if err != nil {
		return ErrParityError, nil
	}

	// Re-encrypt (in emulator, just re-encrypt under same LMK)
	encKey, err := p.lmk.EncryptUnderLMK(clearKey, keyType, 0)
	if err != nil {
		return ErrInternalErr, nil
	}

	_ = srcScheme
	resp := fmt.Sprintf("%s%s", destScheme, encKey)
	return ErrOK, []byte(resp)
}

// handleEI — Generate RSA Key Pair (EI → EJ)
// Request:  key_length(4) + key_type(2) + public_exponent(2)
// Response: public_key(DER, length-prefixed) + private_key_under_lmk(length-prefixed)
func (p *Processor) handleEI(data []byte) (string, []byte) {
	if len(data) < 8 {
		return ErrBadMsgLen, nil
	}

	keyLenStr, _ := safeString(data, 0, 4)
	keyBits, err := strconv.Atoi(keyLenStr)
	if err != nil || keyBits < 512 || keyBits > 4096 {
		return ErrBadKeyLen, nil
	}

	// Generate RSA key pair
	privKey, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return ErrInternalErr, nil
	}

	// Encode public key as DER
	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return ErrInternalErr, nil
	}
	pubHex := strings.ToUpper(hex.EncodeToString(pubDER))

	// Encode private key as PKCS#8 DER, then encrypt under LMK 24-25
	privDER, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return ErrInternalErr, nil
	}
	privEncHex, err := p.lmk.EncryptUnderLMK(privDER, 0x002, 0)
	if err != nil {
		return ErrInternalErr, nil
	}

	// Also build PEM versions for convenience (non-standard but useful for testing)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	_ = pubPEM

	// Response: public_key_len(4) + public_key_hex + private_key_len(4) + private_key_hex
	resp := fmt.Sprintf("%04d%s%04d%s", len(pubHex), pubHex, len(privEncHex), privEncHex)
	return ErrOK, []byte(resp)
}

// ── DES helpers ──────────────────────────────────────────────

func tripleDESDecrypt(key, ciphertext []byte) ([]byte, error) {
	tdesKey := expandTo24(key)
	block, err := newTripleDESCipher(tdesKey)
	if err != nil {
		return nil, err
	}
	result := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += 8 {
		end := i + 8
		if end > len(ciphertext) {
			break
		}
		block.Decrypt(result[i:end], ciphertext[i:end])
	}
	return result, nil
}

func tripleDESEncrypt(key, plaintext []byte) (string, error) {
	tdesKey := expandTo24(key)
	block, err := newTripleDESCipher(tdesKey)
	if err != nil {
		return "", err
	}
	result := make([]byte, len(plaintext))
	for i := 0; i < len(plaintext); i += 8 {
		end := i + 8
		if end > len(plaintext) {
			padded := make([]byte, 8)
			copy(padded, plaintext[i:])
			block.Encrypt(result[i:end], padded)
		} else {
			block.Encrypt(result[i:end], plaintext[i:end])
		}
	}
	return strings.ToUpper(hex.EncodeToString(result)), nil
}

func setOddParity(b byte) byte {
	bits := 0
	v := b
	for i := 0; i < 8; i++ {
		bits += int(v & 1)
		v >>= 1
	}
	if bits%2 == 0 {
		return b ^ 1
	}
	return b
}

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
		out := make([]byte, 24)
		copy(out, key)
		if len(key) >= 8 {
			copy(out[16:], key[:8])
		}
		return out
	}
}
