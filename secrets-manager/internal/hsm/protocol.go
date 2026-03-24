package hsm

import (
	"encoding/binary"
	"fmt"
)

// PayShield host command codes
const (
	CmdNC = "NC" // Perform Diagnostics
	CmdND = "ND" // Diagnostics response
	CmdA0 = "A0" // Generate Key
	CmdA1 = "A1" // Generate Key response
	CmdA6 = "A6" // Import Key
	CmdA7 = "A7" // Import Key response
	CmdA8 = "A8" // Export Key
	CmdA9 = "A9" // Export Key response
	CmdB0 = "B0" // Translate Key Scheme
	CmdB1 = "B1" // Translate Key Scheme response
	CmdBU = "BU" // Generate Key Check Value
	CmdBV = "BV" // Generate KCV response
	CmdBW = "BW" // Verify CVV (also used for CVV check)
	CmdBX = "BX" // Verify CVV response
	CmdCA = "CA" // Translate PIN (ZPK to ZPK)
	CmdCB = "CB" // Translate PIN response
	CmdCO = "CO" // Translate PIN (LMK to ZPK)
	CmdCP = "CP" // Translate PIN response
	CmdCU = "CU" // Generate PIN Change Script
	CmdCV = "CV" // Generate PIN Change Script response
	CmdCW = "CW" // Generate CVV
	CmdCX = "CX" // Generate CVV response
	CmdCY = "CY" // Verify CVV
	CmdCZ = "CZ" // Verify CVV response
	CmdDA = "DA" // Verify Terminal PIN (IBM)
	CmdDB = "DB" // Verify Terminal PIN response
	CmdDC = "DC" // Verify Terminal PIN (VISA)
	CmdDD = "DD" // Verify Terminal PIN response
	CmdDE = "DE" // Generate PIN Offset (IBM)
	CmdDF = "DF" // Generate PIN Offset response
	CmdEI = "EI" // Generate RSA Key Pair
	CmdEJ = "EJ" // Generate RSA Key Pair response
	CmdFA = "FA" // Translate PIN Block
	CmdFB = "FB" // Translate PIN Block response
	CmdKQ = "KQ" // Verify ARQC and/or Generate ARPC (EMV)
	CmdKR = "KR" // ARQC/ARPC response
	CmdM6 = "M6" // Generate EMV MAC
	CmdM7 = "M7" // Generate EMV MAC response
)

// PayShield response error codes
const (
	RespOK                  = "00"
	RespVerificationFailure = "01"
	RespInvalidKeyType      = "02"
	RespInvalidKeyLength    = "03"
	RespInvalidMessageLen   = "05"
	RespKeyParityError      = "10"
	RespKeyNotFound         = "11"
	RespNoKeysLoaded        = "13"
	RespPINBlockError       = "20"
	RespCommandDisabled     = "68"
	RespHSMNotAuth          = "90"
)

// ResponseCodeMessage maps HSM error codes to human-readable descriptions.
var ResponseCodeMessage = map[string]string{
	"00": "No error",
	"01": "Verification failure",
	"02": "Inappropriate key type or algorithm",
	"03": "Invalid key length",
	"04": "Invalid source/dest PIN key type",
	"05": "Invalid message length",
	"10": "Source/destination PIN key parity error",
	"11": "Key not found in key store",
	"12": "Invalid number of components",
	"13": "No keys loaded",
	"15": "Invalid LMK identifier",
	"20": "PIN block does not contain valid PIN",
	"21": "Invalid PIN pad character",
	"23": "No room in key store",
	"24": "PIN too long",
	"25": "PIN too short",
	"26": "Decimalization table error",
	"27": "Invalid key scheme",
	"40": "Invalid firmware",
	"41": "Internal error",
	"42": "PIN length not supported",
	"68": "Command disabled",
	"90": "HSM not authorized / data parity error in LMK",
}

// EncodeCommand builds a PayShield wire message: [header][commandCode][data].
// headerLen is 2 for PS9K or 4 for PS10K.
func EncodeCommand(headerLen int, commandCode string, data []byte) []byte {
	cmdBytes := []byte(commandCode)
	bodyLen := len(cmdBytes) + len(data)
	msg := make([]byte, headerLen+bodyLen)

	switch headerLen {
	case 2:
		binary.BigEndian.PutUint16(msg[0:2], uint16(bodyLen))
	case 4:
		binary.BigEndian.PutUint32(msg[0:4], uint32(bodyLen))
	default:
		binary.BigEndian.PutUint32(msg[0:headerLen], uint32(bodyLen))
	}

	copy(msg[headerLen:], cmdBytes)
	copy(msg[headerLen+len(cmdBytes):], data)
	return msg
}

// DecodeResponse parses a PayShield wire response.
// Returns the response command code (e.g. "A1"), the 2-char error code, the data portion, and any error.
func DecodeResponse(headerLen int, raw []byte) (commandCode string, responseCode string, data []byte, err error) {
	if len(raw) < headerLen+4 { // minimum: header + 2 cmd + 2 resp
		return "", "", nil, fmt.Errorf("response too short: %d bytes", len(raw))
	}

	body := raw[headerLen:]
	if len(body) < 4 {
		return "", "", nil, fmt.Errorf("response body too short: %d bytes", len(body))
	}

	commandCode = string(body[0:2])
	responseCode = string(body[2:4])
	if len(body) > 4 {
		data = body[4:]
	}
	return commandCode, responseCode, data, nil
}

// ReadMessageLength reads the message length from the header bytes.
func ReadMessageLength(header []byte, headerLen int) int {
	switch headerLen {
	case 2:
		return int(binary.BigEndian.Uint16(header))
	case 4:
		return int(binary.BigEndian.Uint32(header))
	default:
		return int(binary.BigEndian.Uint32(header))
	}
}

// ErrorForCode returns an error for a non-00 response code, or nil for success.
func ErrorForCode(code string) error {
	if code == RespOK {
		return nil
	}
	msg, ok := ResponseCodeMessage[code]
	if !ok {
		msg = "unknown error"
	}
	return fmt.Errorf("HSM error %s: %s", code, msg)
}
