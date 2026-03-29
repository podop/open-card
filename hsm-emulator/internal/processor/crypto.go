package processor

import (
	"crypto/cipher"
	"crypto/des"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// handleCW — Generate CVV (CW → CX)
// Request:  cvk(32H) + pan(variable, ';'-delimited) + expiry(4) + service_code(3)
// Response: cvv(3)
func (p *Processor) handleCW(data []byte) (string, []byte) {
	s := string(data)

	pos := 0
	cvkHex := readKeyHex(s, &pos)

	// PAN is delimited by ';'
	rest := s[pos:]
	semiIdx := strings.Index(rest, ";")
	if semiIdx < 0 {
		return ErrBadMsgLen, nil
	}
	pan := strings.TrimRight(rest[:semiIdx], " ")
	after := rest[semiIdx+1:]

	if len(after) < 7 {
		return ErrBadMsgLen, nil
	}
	expiry := after[:4]
	svcCode := after[4:7]

	// Decrypt CVK under LMK
	clearCVK, err := p.lmk.DecryptUnderLMK(cvkHex, 0x00A, 4)
	if err != nil {
		return ErrParityError, nil
	}

	cvv := computeCVV(clearCVK, pan, expiry, svcCode)
	return ErrOK, []byte(cvv)
}

// handleCY — Verify CVV (CY → CZ)
// Request:  cvk(32H) + cvv(3) + pan(variable, ';'-delimited) + expiry(4) + service_code(3)
// Response: (empty on success)
func (p *Processor) handleCY(data []byte) (string, []byte) {
	s := string(data)

	pos := 0
	cvkHex := readKeyHex(s, &pos)

	if pos+3 > len(s) {
		return ErrBadMsgLen, nil
	}
	cvv := s[pos : pos+3]
	pos += 3

	rest := s[pos:]
	semiIdx := strings.Index(rest, ";")
	if semiIdx < 0 {
		// PAN might be fixed 16 chars
		if len(rest) < 16+4+3 {
			return ErrBadMsgLen, nil
		}
		pan := strings.TrimRight(rest[:16], " ")
		expiry := rest[16:20]
		svcCode := rest[20:23]

		clearCVK, err := p.lmk.DecryptUnderLMK(cvkHex, 0x00A, 4)
		if err != nil {
			return ErrParityError, nil
		}

		computed := computeCVV(clearCVK, pan, expiry, svcCode)
		if computed != cvv {
			return ErrVerifyFail, nil
		}
		return ErrOK, nil
	}

	pan := strings.TrimRight(rest[:semiIdx], " ")
	after := rest[semiIdx+1:]
	if len(after) < 7 {
		return ErrBadMsgLen, nil
	}
	expiry := after[:4]
	svcCode := after[4:7]

	clearCVK, err := p.lmk.DecryptUnderLMK(cvkHex, 0x00A, 4)
	if err != nil {
		return ErrParityError, nil
	}

	computed := computeCVV(clearCVK, pan, expiry, svcCode)
	if computed != cvv {
		return ErrVerifyFail, nil
	}
	return ErrOK, nil
}

// handleM0 — Encrypt Data Block (M0 → M1)
// Request:  mode(2) + input_format(2) + output_format(2) + key_type(3) + key + data_len(4H) + data
// Response: encrypted_data
func (p *Processor) handleM0(data []byte) (string, []byte) {
	if len(data) < 2+2+2+3 {
		return ErrBadMsgLen, nil
	}
	mode, _ := safeString(data, 0, 2)
	_, _ = safeString(data, 2, 2) // input format
	_, _ = safeString(data, 4, 2) // output format
	keyTypeStr, _ := safeString(data, 6, 3)
	keyType, _ := strconv.Atoi(keyTypeStr)

	pos := 9
	keyHex := readKeyHex(string(data[pos:]), &pos)
	pos += 9 // adjust for readKeyHex reading from substring

	// Recalculate position
	fullStr := string(data)
	keyStart := 9
	if keyStart < len(fullStr) && (fullStr[keyStart] == 'U' || fullStr[keyStart] == 'X' || fullStr[keyStart] == 'T' || fullStr[keyStart] == 'Z') {
		keyStart++
	}
	keyEnd := keyStart + len(keyHex)
	pos = keyEnd

	if pos+4 > len(data) {
		return ErrBadMsgLen, nil
	}
	dataLenStr, _ := safeString(data, pos, 4)
	dataLen, _ := strconv.ParseInt(dataLenStr, 16, 32)
	pos += 4

	if pos+int(dataLen)*2 > len(data) {
		return ErrBadMsgLen, nil
	}
	plainHex, _ := safeString(data, pos, int(dataLen)*2)

	// Decrypt key
	clearKey, err := p.lmk.DecryptUnderLMK(keyHex, keyType, 0)
	if err != nil {
		return ErrParityError, nil
	}

	plainBytes, _ := hex.DecodeString(plainHex)

	var result []byte
	switch mode {
	case "00": // ECB
		result, err = tripleDESEncryptRaw(clearKey, plainBytes)
	case "01": // CBC with zero IV
		iv := make([]byte, 8)
		result, err = tripleDESEncryptCBC(clearKey, iv, plainBytes)
	default:
		result, err = tripleDESEncryptRaw(clearKey, plainBytes)
	}
	if err != nil {
		return ErrInternalErr, nil
	}

	return ErrOK, []byte(strings.ToUpper(hex.EncodeToString(result)))
}

// handleM2 — Decrypt Data Block (M2 → M3)
func (p *Processor) handleM2(data []byte) (string, []byte) {
	if len(data) < 2+2+2+3 {
		return ErrBadMsgLen, nil
	}
	mode, _ := safeString(data, 0, 2)
	keyTypeStr, _ := safeString(data, 6, 3)
	keyType, _ := strconv.Atoi(keyTypeStr)

	pos := 9
	keyHex := readKeyHex(string(data[pos:]), &pos)
	fullStr := string(data)
	keyStart := 9
	if keyStart < len(fullStr) && (fullStr[keyStart] == 'U' || fullStr[keyStart] == 'X') {
		keyStart++
	}
	pos = keyStart + len(keyHex)

	if pos+4 > len(data) {
		return ErrBadMsgLen, nil
	}
	dataLenStr, _ := safeString(data, pos, 4)
	dataLen, _ := strconv.ParseInt(dataLenStr, 16, 32)
	pos += 4

	cipherHex, _ := safeString(data, pos, int(dataLen)*2)
	cipherBytes, _ := hex.DecodeString(cipherHex)

	clearKey, err := p.lmk.DecryptUnderLMK(keyHex, keyType, 0)
	if err != nil {
		return ErrParityError, nil
	}

	var result []byte
	switch mode {
	case "00":
		result, err = tripleDESDecryptECB(clearKey, cipherBytes)
	default:
		result, err = tripleDESDecryptECB(clearKey, cipherBytes)
	}
	if err != nil {
		return ErrInternalErr, nil
	}

	return ErrOK, []byte(strings.ToUpper(hex.EncodeToString(result)))
}

// handleMS — Generate MAC (MS → MT)
// Request:  mode(1) + key_type(3) + key_len(1) + mac_algo(2) + mac_pad(2) + key + data_len(4H) + data
// Response: mac(16H for 8-byte MAC)
func (p *Processor) handleMS(data []byte) (string, []byte) {
	if len(data) < 1+3+1 {
		return ErrBadMsgLen, nil
	}

	_, _ = safeString(data, 0, 1) // mode
	keyTypeStr, _ := safeString(data, 1, 3)
	keyType, _ := strconv.Atoi(keyTypeStr)

	pos := 4
	// key_len_flag + mac_algo + mac_pad — simplified parsing
	if pos+5 > len(data) {
		return ErrBadMsgLen, nil
	}
	pos += 5 // skip key_len(1) + mac_algo(2) + mac_pad(2)

	keyHex := readKeyHex(string(data[pos:]), &pos)
	fullStr := string(data)
	keyStart := 9
	if keyStart < len(fullStr) && (fullStr[keyStart] == 'U' || fullStr[keyStart] == 'X') {
		keyStart++
	}
	pos = keyStart + len(keyHex)

	if pos+4 > len(data) {
		return ErrBadMsgLen, nil
	}
	dataLenStr, _ := safeString(data, pos, 4)
	dataLen, _ := strconv.ParseInt(dataLenStr, 16, 32)
	pos += 4

	msgHex, _ := safeString(data, pos, int(dataLen)*2)
	msgBytes, _ := hex.DecodeString(msgHex)

	clearKey, err := p.lmk.DecryptUnderLMK(keyHex, keyType, 0)
	if err != nil {
		return ErrParityError, nil
	}

	mac, err := computeMAC(clearKey, msgBytes)
	if err != nil {
		return ErrInternalErr, nil
	}

	return ErrOK, []byte(strings.ToUpper(hex.EncodeToString(mac)))
}

// handleKQ — Verify ARQC and/or Generate ARPC (KQ → KR)
// Simplified EMV ARQC verification.
func (p *Processor) handleKQ(data []byte) (string, []byte) {
	if len(data) < 1 {
		return ErrBadMsgLen, nil
	}

	// Parse mode/scheme from first byte
	s := string(data)
	pos := 0

	// SchemeID(1) + MK-AC + PAN(16) + PANSeqNo(2) + ATC(4) + UN(8) + TransData(variable) + ARQC(16)
	schemeID, _ := safeString(data, pos, 1)
	pos += 1
	_ = schemeID

	mkHex := readKeyHex(s[pos:], &pos)
	pos += 0 // readKeyHex already advanced

	// For emulator: just verify the command is well-formed and return success + dummy ARPC
	clearMK, err := p.lmk.DecryptUnderLMK(mkHex, 0x002, 0)
	if err != nil {
		return ErrParityError, nil
	}
	_ = clearMK

	// Return success with a dummy ARPC (8 bytes = 16 hex chars)
	arpc := make([]byte, 8)
	rand.Read(arpc)
	return ErrOK, []byte(strings.ToUpper(hex.EncodeToString(arpc)))
}

// handleN0 — Generate Random Data (N0 → N1)
// Request:  length(4, decimal string = number of random bytes)
// Response: random_data(hex)
func (p *Processor) handleN0(data []byte) (string, []byte) {
	if len(data) < 4 {
		return ErrBadMsgLen, nil
	}

	lenStr, _ := safeString(data, 0, 4)
	n, err := strconv.Atoi(lenStr)
	if err != nil || n <= 0 || n > 4096 {
		return ErrBadInput, nil
	}

	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return ErrInternalErr, nil
	}

	return ErrOK, []byte(strings.ToUpper(hex.EncodeToString(buf)))
}

// ── Cryptographic helpers ────────────────────────────────────

// computeCVV generates a 3-digit CVV using the Visa/Mastercard algorithm.
// CVV = decimalize( 3DES_ECB(CVK_A, data) XOR 3DES_ECB(CVK_B, ...) )
func computeCVV(cvk []byte, pan, expiry, serviceCode string) string {
	// CVK = CVK_A(8) + CVK_B(8)
	if len(cvk) < 16 {
		return "000"
	}
	cvkA := cvk[:8]

	// Build data block 1: PAN + expiry + service_code, right-padded with zeros to 16 chars
	block1Data := pan + expiry + serviceCode
	for len(block1Data) < 32 {
		block1Data += "0"
	}
	// Take first 16 hex chars (8 bytes) as block 1, next 16 as block 2
	b1, _ := hex.DecodeString(block1Data[:16])
	b2, _ := hex.DecodeString(block1Data[16:32])

	// Step 1: DES encrypt block1 with CVK_A
	desBlock, _ := des.NewCipher(cvkA)
	enc1 := make([]byte, 8)
	desBlock.Encrypt(enc1, b1)

	// Step 2: XOR result with block2
	for i := range enc1 {
		enc1[i] ^= b2[i]
	}

	// Step 3: 3DES encrypt with full CVK
	tdesKey := expandTo24(cvk)
	tdesBlock, _ := des.NewTripleDESCipher(tdesKey)
	enc2 := make([]byte, 8)
	tdesBlock.Encrypt(enc2, enc1)

	// Step 4: Decimalize
	encHex := hex.EncodeToString(enc2)
	digits := ""
	// First pass: take decimal digits
	for _, c := range encHex {
		if c >= '0' && c <= '9' {
			digits += string(c)
		}
		if len(digits) >= 3 {
			break
		}
	}
	// Second pass: convert hex A-F to 0-5
	if len(digits) < 3 {
		for _, c := range encHex {
			if c >= 'a' && c <= 'f' {
				digits += string(rune(c - 'a' + '0'))
			}
			if len(digits) >= 3 {
				break
			}
		}
	}
	for len(digits) < 3 {
		digits += "0"
	}
	return digits[:3]
}

// computeMAC computes a MAC using DES CBC with zero IV (ISO 9797-1 Algorithm 3).
func computeMAC(key, data []byte) ([]byte, error) {
	if len(key) < 8 {
		return nil, fmt.Errorf("key too short for MAC")
	}

	// Pad data to 8-byte boundary
	padded := make([]byte, len(data))
	copy(padded, data)
	if len(padded)%8 != 0 {
		pad := make([]byte, 8-len(padded)%8)
		padded = append(padded, pad...)
	}
	if len(padded) == 0 {
		padded = make([]byte, 8)
	}

	// CBC-MAC with single DES using left half of key
	desBlock, err := des.NewCipher(key[:8])
	if err != nil {
		return nil, err
	}

	mac := make([]byte, 8)
	for i := 0; i < len(padded); i += 8 {
		for j := 0; j < 8; j++ {
			mac[j] ^= padded[i+j]
		}
		desBlock.Encrypt(mac, mac)
	}

	// Final 3DES encryption if key is double-length (Retail MAC / Algorithm 3)
	if len(key) >= 16 {
		desBlock2, err := des.NewCipher(key[8:16])
		if err != nil {
			return nil, err
		}
		temp := make([]byte, 8)
		desBlock2.Decrypt(temp, mac)
		desBlock.Encrypt(mac, temp)
	}

	return mac, nil
}

func tripleDESEncryptCBC(key, iv, plaintext []byte) ([]byte, error) {
	tdesKey := expandTo24(key)
	block, err := des.NewTripleDESCipher(tdesKey)
	if err != nil {
		return nil, err
	}
	// Pad to block size
	padLen := 8 - (len(plaintext) % 8)
	if padLen == 8 && len(plaintext) > 0 {
		padLen = 0
	}
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)

	ct := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ct, padded)
	return ct, nil
}

func tripleDESDecryptECB(key, ct []byte) ([]byte, error) {
	tdesKey := expandTo24(key)
	block, err := des.NewTripleDESCipher(tdesKey)
	if err != nil {
		return nil, err
	}
	result := make([]byte, len(ct))
	for i := 0; i < len(ct); i += 8 {
		end := i + 8
		if end > len(ct) {
			break
		}
		block.Decrypt(result[i:end], ct[i:end])
	}
	return result, nil
}

func newTripleDESCipher(key []byte) (cipher.Block, error) {
	return des.NewTripleDESCipher(key)
}
