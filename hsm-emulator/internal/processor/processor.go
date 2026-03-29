package processor

import (
	"fmt"
	"log"
	"strings"

	"github.com/your-org/hsm-emulator/internal/config"
	"github.com/your-org/hsm-emulator/internal/lmk"
)

// Response error codes (PayShield 10K compatible).
const (
	ErrOK            = "00"
	ErrVerifyFail    = "01"
	ErrBadKeyType    = "02"
	ErrBadKeyLen     = "03"
	ErrBadSrcKey     = "04"
	ErrBadMsgLen     = "05"
	ErrBadKeyBlock   = "06"
	ErrParityError   = "10"
	ErrKeyNotFound   = "11"
	ErrBadComponents = "12"
	ErrNoLMK         = "13"
	ErrBadInput      = "15"
	ErrPINBlockErr   = "20"
	ErrBadPINPad     = "21"
	ErrBadDecTable   = "26"
	ErrBadScheme     = "27"
	ErrInternalErr   = "41"
	ErrCmdDisabled   = "68"
	ErrNotAuthorized = "90"
)

// commandHandler is the signature for all command handlers.
type commandHandler func(data []byte) (errCode string, respData []byte)

// Processor routes incoming PayShield commands to their handlers.
type Processor struct {
	cfg      *config.Config
	lmk      *lmk.Store
	handlers map[string]commandHandler
}

func New(cfg *config.Config, lmkStore *lmk.Store) *Processor {
	p := &Processor{
		cfg:      cfg,
		lmk:      lmkStore,
		handlers: make(map[string]commandHandler),
	}
	p.registerAll()
	return p
}

// Execute processes a command and returns (responseCode, errorCode, responseData).
func (p *Processor) Execute(cmdCode string, data []byte) (respCode string, errCode string, respData []byte) {
	cmdCode = strings.ToUpper(cmdCode)
	respCode = nextCode(cmdCode)

	// Check if HSM is authorized (except NC which always works)
	if cmdCode != "NC" && cmdCode != "NO" {
		if !p.cfg.IsAuthorized() {
			return respCode, ErrNotAuthorized, nil
		}
		if !p.lmk.IsLoaded() {
			return respCode, ErrNoLMK, nil
		}
	}

	// Check if command is disabled
	if p.cfg.IsCommandDisabled(cmdCode) {
		return respCode, ErrCmdDisabled, nil
	}

	handler, ok := p.handlers[cmdCode]
	if !ok {
		log.Printf("[HSM] Unknown command: %s", cmdCode)
		return respCode, ErrCmdDisabled, nil
	}

	errCode, respData = handler(data)
	return respCode, errCode, respData
}

func (p *Processor) registerAll() {
	// Diagnostics
	p.handlers["NC"] = p.handleNC
	p.handlers["NO"] = p.handleNO

	// Key management
	p.handlers["A0"] = p.handleA0 // Generate Key
	p.handlers["A6"] = p.handleA6 // Import Key
	p.handlers["A8"] = p.handleA8 // Export Key
	p.handlers["BU"] = p.handleBU // Key Check Value
	p.handlers["B0"] = p.handleB0 // Translate Key Scheme
	p.handlers["EI"] = p.handleEI // Generate RSA Key Pair

	// PIN operations
	p.handlers["BA"] = p.handleBA // Encrypt Clear PIN
	p.handlers["DA"] = p.handleDA // Verify Terminal PIN (IBM)
	p.handlers["DC"] = p.handleDC // Verify Terminal PIN (Visa PVV)
	p.handlers["DE"] = p.handleDE // Generate PIN Offset (IBM)
	p.handlers["DG"] = p.handleDG // Generate PVV
	p.handlers["CA"] = p.handleCA // Translate PIN (ZPK to ZPK)
	p.handlers["FA"] = p.handleFA // Translate PIN Block

	// CVV
	p.handlers["CW"] = p.handleCW // Generate CVV
	p.handlers["CY"] = p.handleCY // Verify CVV

	// Crypto
	p.handlers["M0"] = p.handleM0 // Encrypt Data Block
	p.handlers["M2"] = p.handleM2 // Decrypt Data Block
	p.handlers["MS"] = p.handleMS // Generate MAC

	// EMV
	p.handlers["KQ"] = p.handleKQ // Verify ARQC / Generate ARPC

	// Random
	p.handlers["N0"] = p.handleN0 // Generate Random Data
}

// nextCode computes the response command code.
// PayShield convention: response code = request code with last char incremented.
func nextCode(cmd string) string {
	if len(cmd) != 2 {
		return cmd
	}
	b := []byte(cmd)
	b[1]++
	return string(b)
}

// safeString extracts a substring with bounds checking.
func safeString(data []byte, offset, length int) (string, bool) {
	if offset+length > len(data) {
		return "", false
	}
	return string(data[offset : offset+length]), true
}

func hexToDisplay(s string) string {
	return fmt.Sprintf("%s...(%d chars)", s[:min(12, len(s))], len(s))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
