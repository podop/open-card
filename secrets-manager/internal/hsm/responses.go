package hsm

import (
	"encoding/json"
	"fmt"
)

// ResponseParser parses PayShield wire responses into structured types.
type ResponseParser struct{}

// ParseDiagnostics parses an ND response.
func (p *ResponseParser) ParseDiagnostics(respCode string, data []byte) (json.RawMessage, error) {
	if err := ErrorForCode(respCode); err != nil {
		return nil, err
	}
	result := map[string]interface{}{
		"status":      "ok",
		"lmkCheckVal": "",
		"firmware":    "",
	}
	if len(data) >= 16 {
		result["lmkCheckVal"] = string(data[:16])
	}
	if len(data) >= 25 {
		result["firmware"] = string(data[16:25])
	}
	return json.Marshal(result)
}

// ParseGenerateKey parses an A1 response.
func (p *ResponseParser) ParseGenerateKey(respCode string, data []byte) (*KeyGenerateResponse, error) {
	if err := ErrorForCode(respCode); err != nil {
		return nil, err
	}
	resp := &KeyGenerateResponse{ErrorCode: respCode}
	// A1 response: [key_under_lmk][key_check_value(6H)]
	if len(data) >= 6 {
		resp.CheckValue = string(data[len(data)-6:])
		resp.Key = string(data[:len(data)-6])
	}
	return resp, nil
}

// ParseGenerateRSA parses an EJ response.
func (p *ResponseParser) ParseGenerateRSA(respCode string, data []byte) (*RSAGenerateResponse, error) {
	if err := ErrorForCode(respCode); err != nil {
		return nil, err
	}
	resp := &RSAGenerateResponse{ErrorCode: respCode}
	// EJ response format varies; simplified parsing
	if len(data) > 0 {
		resp.PublicKey = string(data)
	}
	return resp, nil
}

// ParseCVVVerify parses a CZ response.
func (p *ResponseParser) ParseCVVVerify(respCode string, _ []byte) (json.RawMessage, error) {
	result := map[string]interface{}{
		"verified":  respCode == RespOK,
		"errorCode": respCode,
	}
	if respCode != RespOK {
		msg := ResponseCodeMessage[respCode]
		if msg == "" {
			msg = "unknown error"
		}
		result["errorMessage"] = msg
	}
	return json.Marshal(result)
}

// ParseCVVGenerate parses a CX response.
func (p *ResponseParser) ParseCVVGenerate(respCode string, data []byte) (json.RawMessage, error) {
	if err := ErrorForCode(respCode); err != nil {
		return nil, err
	}
	cvv := ""
	if len(data) >= 3 {
		cvv = string(data[:3])
	}
	return json.Marshal(map[string]interface{}{
		"cvv": cvv,
	})
}

// ParsePINVerify parses a DB/DD response.
func (p *ResponseParser) ParsePINVerify(respCode string, _ []byte) (json.RawMessage, error) {
	return json.Marshal(map[string]interface{}{
		"verified":  respCode == RespOK,
		"errorCode": respCode,
	})
}

// ParsePINTranslate parses a CB/CP response.
func (p *ResponseParser) ParsePINTranslate(respCode string, data []byte) (json.RawMessage, error) {
	if err := ErrorForCode(respCode); err != nil {
		return nil, err
	}
	pinBlock := ""
	if len(data) >= 16 {
		pinBlock = string(data[:16])
	}
	return json.Marshal(map[string]interface{}{
		"pinBlock": pinBlock,
	})
}

// ParsePINOffset parses a DF response.
func (p *ResponseParser) ParsePINOffset(respCode string, data []byte) (json.RawMessage, error) {
	if err := ErrorForCode(respCode); err != nil {
		return nil, err
	}
	offset := ""
	if len(data) > 0 {
		offset = string(data)
	}
	return json.Marshal(map[string]interface{}{
		"offset": offset,
	})
}

// ParseARQCValidate parses a KR response (ARQC validation).
func (p *ResponseParser) ParseARQCValidate(respCode string, data []byte) (json.RawMessage, error) {
	result := map[string]interface{}{
		"verified":  respCode == RespOK,
		"errorCode": respCode,
	}
	if respCode == RespOK && len(data) >= 16 {
		result["arpc"] = string(data[:16])
	}
	return json.Marshal(result)
}

// ParseEMVMAC parses an M7 response.
func (p *ResponseParser) ParseEMVMAC(respCode string, data []byte) (json.RawMessage, error) {
	if err := ErrorForCode(respCode); err != nil {
		return nil, err
	}
	mac := ""
	if len(data) >= 16 {
		mac = string(data[:16])
	}
	return json.Marshal(map[string]interface{}{
		"mac": mac,
	})
}

// ParseCheckValue parses a BV response.
func (p *ResponseParser) ParseCheckValue(respCode string, data []byte) (json.RawMessage, error) {
	if err := ErrorForCode(respCode); err != nil {
		return nil, err
	}
	kcv := ""
	if len(data) >= 6 {
		kcv = string(data[:6])
	}
	return json.Marshal(map[string]interface{}{
		"checkValue": kcv,
	})
}

// ParseKeyImport parses an A7 response.
func (p *ResponseParser) ParseKeyImport(respCode string, data []byte) (json.RawMessage, error) {
	if err := ErrorForCode(respCode); err != nil {
		return nil, err
	}
	result := map[string]interface{}{
		"errorCode": respCode,
	}
	if len(data) >= 6 {
		result["checkValue"] = string(data[len(data)-6:])
		result["key"] = string(data[:len(data)-6])
	}
	return json.Marshal(result)
}

// ParseKeyExport parses an A9 response.
func (p *ResponseParser) ParseKeyExport(respCode string, data []byte) (json.RawMessage, error) {
	if err := ErrorForCode(respCode); err != nil {
		return nil, err
	}
	result := map[string]interface{}{
		"errorCode": respCode,
	}
	if len(data) >= 6 {
		result["checkValue"] = string(data[len(data)-6:])
		result["key"] = string(data[:len(data)-6])
	}
	return json.Marshal(result)
}

// ParseRawResponse returns the raw response as hex + ASCII.
func (p *ResponseParser) ParseRawResponse(respCode string, data []byte) (json.RawMessage, error) {
	return json.Marshal(map[string]interface{}{
		"errorCode": respCode,
		"dataHex":   fmt.Sprintf("%x", data),
		"dataAscii": string(data),
	})
}
