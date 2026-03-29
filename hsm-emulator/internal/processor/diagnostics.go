package processor

import "fmt"

// handleNC — Perform Diagnostics (NC → ND)
// Production response: error_code + LMK_check_value + firmware_version
func (p *Processor) handleNC(data []byte) (string, []byte) {
	// NC always responds even when not authorized
	kcvs, err := p.lmk.CheckValues()
	lmkKCV := "000000"
	if err == nil && len(kcvs) > 0 {
		lmkKCV = string(kcvs[0][:])
	}

	// PayShield ND response format:
	// error_code(2) already handled by caller
	// + LMK_check(6) + firmware(9)
	resp := fmt.Sprintf("%s%s", lmkKCV, p.cfg.FirmwareVersion)
	return ErrOK, []byte(resp)
}

// handleNO — HSM Status (NO → NP)
// Returns detailed HSM status information.
func (p *Processor) handleNO(data []byte) (string, []byte) {
	status := "1" // 0=not ready, 1=ready
	if !p.cfg.IsAuthorized() {
		status = "0"
	}

	// PayShield NP response:
	// status(1) + serial(14) + model(30, padded)
	resp := fmt.Sprintf("%s%-14s%-30s",
		status,
		p.cfg.SerialNumber,
		p.cfg.Model,
	)
	return ErrOK, []byte(resp)
}
