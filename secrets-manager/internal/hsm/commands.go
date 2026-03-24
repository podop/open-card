package hsm

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// CommandBuilder translates high-level API requests into PayShield host command data.
type CommandBuilder struct{}

// BuildDiagnostics builds an NC (Perform Diagnostics) command.
func (b *CommandBuilder) BuildDiagnostics() (string, []byte, error) {
	return CmdNC, nil, nil
}

// BuildGenerateKey builds an A0 (Generate Key) command.
// PayShield A0 format: [mode][key_type][key_scheme][derive_key_mode][...optional fields]
func (b *CommandBuilder) BuildGenerateKey(req KeyGenerateRequest) (string, []byte, error) {
	mode := "0" // generate
	keyType := fmt.Sprintf("%03d", req.LMKKeyType)
	scheme := req.Scheme
	if scheme == "" {
		scheme = "U" // default variant
	}

	data := []byte(mode + keyType + scheme)
	return CmdA0, data, nil
}

// BuildGenerateRSA builds an EI (Generate RSA Key Pair) command.
// PayShield EI format: [key_length(4)][key_type(2)][public_exponent_flag(1)][...optional]
func (b *CommandBuilder) BuildGenerateRSA(req RSAGenerateRequest) (string, []byte, error) {
	keyLen := fmt.Sprintf("%04d", req.KeyLength)
	keyType := fmt.Sprintf("%02d", req.KeyType)
	pubExp := "01" // 65537 default
	if req.PublicExponent == "3" {
		pubExp = "00"
	}

	data := []byte(keyLen + keyType + pubExp)
	return CmdEI, data, nil
}

// BuildImportKey builds an A6 (Import Key) command.
func (b *CommandBuilder) BuildImportKey(req KeyImportRequest) (string, []byte, error) {
	keyType := fmt.Sprintf("%03d", req.KeyType)
	scheme := req.Scheme
	if scheme == "" {
		scheme = "U"
	}
	data := []byte(keyType + req.ZMKKey + req.ImportedKey + scheme)
	return CmdA6, data, nil
}

// BuildExportKey builds an A8 (Export Key) command.
func (b *CommandBuilder) BuildExportKey(req KeyExportRequest) (string, []byte, error) {
	keyType := fmt.Sprintf("%03d", req.KeyType)
	scheme := req.Scheme
	if scheme == "" {
		scheme = "U"
	}
	data := []byte(keyType + req.ZMKKey + req.KeyToExport + scheme)
	return CmdA8, data, nil
}

// BuildTranslateKey builds a B0 (Translate Key Scheme) command.
func (b *CommandBuilder) BuildTranslateKey(req KeyTranslateRequest) (string, []byte, error) {
	keyType := fmt.Sprintf("%03d", req.KeyType)
	data := []byte(keyType + req.SourceScheme + req.Key + req.DestScheme)
	return CmdB0, data, nil
}

// BuildCheckValue builds a BU (Generate Key Check Value) command.
func (b *CommandBuilder) BuildCheckValue(req CheckValueRequest) (string, []byte, error) {
	keyType := fmt.Sprintf("%03d", req.KeyType)
	checkLen := "0"
	if req.CheckLength > 0 {
		checkLen = fmt.Sprintf("%d", req.CheckLength)
	}
	data := []byte(keyType + req.Key + ";") // delimiter marks optional fields
	data = append(data, []byte(checkLen)...)
	return CmdBU, data, nil
}

// BuildVerifyPIN builds a DA (Verify Terminal PIN using IBM method) command.
func (b *CommandBuilder) BuildVerifyPIN(req PINVerifyRequest) (string, []byte, error) {
	data := []byte(req.TPK + req.PINBlock + req.PINBlockFormat +
		fmt.Sprintf("%02d", req.CheckLength) + req.PAN +
		req.DecimalisationTable + req.ValidationData + req.Offset)
	return CmdDA, data, nil
}

// BuildTranslatePIN builds a CA (Translate PIN ZPK to ZPK) command.
func (b *CommandBuilder) BuildTranslatePIN(req PINTranslateRequest) (string, []byte, error) {
	maxPIN := fmt.Sprintf("%02d", req.MaxPINLength)
	data := []byte(req.SourceTPK + req.DestTPK + maxPIN +
		req.SourcePINBlock + req.SourceFormat + req.DestFormat +
		rightPad(req.PAN, 12))
	return CmdCA, data, nil
}

// BuildPINOffset builds a DE (Generate PIN Offset) command.
func (b *CommandBuilder) BuildPINOffset(req PINOffsetRequest) (string, []byte, error) {
	data := []byte(req.PVK + req.PINBlock + req.PINBlockFormat +
		fmt.Sprintf("%02d", req.CheckLength) + req.PAN +
		req.DecimalisationTable + req.ValidationData)
	return CmdDE, data, nil
}

// BuildVerifyCVV builds a CY (Verify CVV) command.
func (b *CommandBuilder) BuildVerifyCVV(req CVVVerifyRequest) (string, []byte, error) {
	data := []byte(req.CVK + req.CVV + rightPad(req.PAN, 16) +
		";" + req.ExpDate + req.ServiceCode)
	return CmdCY, data, nil
}

// BuildGenerateCVV builds a CW (Generate CVV) command.
func (b *CommandBuilder) BuildGenerateCVV(req CVVGenerateRequest) (string, []byte, error) {
	data := []byte(req.CVK + rightPad(req.PAN, 16) +
		";" + req.ExpDate + req.ServiceCode)
	return CmdCW, data, nil
}

// BuildValidateARQC builds a KQ (Verify ARQC and/or Generate ARPC) command.
func (b *CommandBuilder) BuildValidateARQC(req ARQCValidateRequest) (string, []byte, error) {
	schemeID := fmt.Sprintf("%01d", req.SchemeID)
	data := []byte(schemeID + req.MKdac + rightPad(req.PAN, 16) + req.PANSeqNo +
		req.ATC + req.UN + req.TransData + req.ARQC)
	return CmdKQ, data, nil
}

// BuildGenerateARPC builds the ARPC portion of a KQ command.
func (b *CommandBuilder) BuildGenerateARPC(req ARPCGenerateRequest) (string, []byte, error) {
	schemeID := fmt.Sprintf("%01d", req.SchemeID)
	// KQ with mode 1 = verify ARQC + generate ARPC
	mode := "1"
	data := []byte(mode + schemeID + req.MKdac + rightPad(req.PAN, 16) + req.PANSeqNo +
		req.ATC + req.ARQC + req.ARC)
	return CmdKQ, data, nil
}

// BuildEMVMAC builds an M6 (Generate EMV MAC) command.
func (b *CommandBuilder) BuildEMVMAC(req EMVMACRequest) (string, []byte, error) {
	keyType := fmt.Sprintf("%01d", req.MACKeyType)
	dataHex := req.Data
	dataLen := fmt.Sprintf("%04d", len(dataHex)/2) // byte length
	data := []byte(keyType + req.MACKey + rightPad(req.PAN, 16) + req.PANSeqNo +
		req.ATC + dataLen + dataHex)
	return CmdM6, data, nil
}

// BuildGenerateRandom builds an N0 (Generate Random) command.
// PayShield N0 format: [length as 4-digit decimal string]
func (b *CommandBuilder) BuildGenerateRandom(length int) (string, []byte, error) {
	data := []byte(fmt.Sprintf("%04d", length))
	return "N0", data, nil
}

// BuildRawCommand builds a raw command from hex string.
func (b *CommandBuilder) BuildRawCommand(cmdHex string) (string, []byte, error) {
	if len(cmdHex) < 4 {
		return "", nil, fmt.Errorf("raw command too short (need at least 2-char command code)")
	}
	cmdCode := strings.ToUpper(cmdHex[:2])
	var data []byte
	if len(cmdHex) > 2 {
		var err error
		data, err = hex.DecodeString(cmdHex[2:])
		if err != nil {
			// Treat as ASCII if not valid hex
			data = []byte(cmdHex[2:])
		}
	}
	return cmdCode, data, nil
}

func rightPad(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	return s + strings.Repeat(" ", length-len(s))
}
