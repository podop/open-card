package admin

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/your-org/hsm-emulator/internal/config"
	"github.com/your-org/hsm-emulator/internal/lmk"
)

// Handler serves the HTTP admin API for managing the HSM emulator from the UI.
type Handler struct {
	cfg     *config.Config
	lmk     *lmk.Store
	cfgFile string
	lmkFile string
}

func New(cfg *config.Config, lmkStore *lmk.Store, cfgFile, lmkFile string) *Handler {
	return &Handler{cfg: cfg, lmk: lmkStore, cfgFile: cfgFile, lmkFile: lmkFile}
}

// RegisterRoutes registers all admin HTTP endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/hsm-admin/status", h.cors(h.handleStatus))
	mux.HandleFunc("/hsm-admin/config", h.cors(h.handleConfig))
	mux.HandleFunc("/hsm-admin/lmk/load-test", h.cors(h.handleLoadTestLMK))
	mux.HandleFunc("/hsm-admin/lmk/generate", h.cors(h.handleGenerateLMK))
	mux.HandleFunc("/hsm-admin/lmk/check-values", h.cors(h.handleLMKCheckValues))
	mux.HandleFunc("/hsm-admin/authorize", h.cors(h.handleAuthorize))
	mux.HandleFunc("/hsm-admin/command", h.cors(h.handleCommand))
	mux.HandleFunc("/hsm-admin/commands/disabled", h.cors(h.handleDisabledCommands))
	mux.HandleFunc("/health", h.cors(h.handleHealth))
}

func (h *Handler) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next(w, r)
	}
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// GET /hsm-admin/status — full emulator status
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}

	var kcvs []string
	if h.cfg.IsLMKLoaded() {
		rawKCVs, err := h.lmk.CheckValues()
		if err == nil {
			for _, k := range rawKCVs {
				kcvs = append(kcvs, string(k[:]))
			}
		}
	}

	resp := map[string]interface{}{
		"hostAddress":      h.cfg.HostAddress,
		"hostPort":         h.cfg.HostPort,
		"consolePort":      h.cfg.ConsolePort,
		"headerLength":     h.cfg.HeaderLength,
		"maxConnections":   h.cfg.MaxConnections,
		"commandTimeout":   h.cfg.CommandTimeout,
		"firmwareVersion":  h.cfg.FirmwareVersion,
		"serialNumber":     h.cfg.SerialNumber,
		"model":            h.cfg.Model,
		"lmkIdentifier":    h.cfg.LMKIdentifier,
		"authorized":       h.cfg.IsAuthorized(),
		"lmkLoaded":        h.cfg.IsLMKLoaded(),
		"auditEnabled":     h.cfg.AuditEnabled,
		"disabledCommands": h.cfg.DisabledCommands,
		"lmkCheckValues":   kcvs,
	}
	writeJSON(w, 200, resp)
}

// GET/PUT /hsm-admin/config — read/update configuration
func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		h.handleStatus(w, r)
	case "PUT":
		var body struct {
			HeaderLength    *int    `json:"headerLength"`
			MaxConnections  *int    `json:"maxConnections"`
			CommandTimeout  *int    `json:"commandTimeout"`
			FirmwareVersion *string `json:"firmwareVersion"`
			SerialNumber    *string `json:"serialNumber"`
			Model           *string `json:"model"`
			LMKIdentifier   *int    `json:"lmkIdentifier"`
			AuditEnabled    *bool   `json:"auditEnabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if body.HeaderLength != nil {
			if *body.HeaderLength != 2 && *body.HeaderLength != 4 {
				writeJSON(w, 400, map[string]string{"error": "headerLength must be 2 or 4"})
				return
			}
			h.cfg.HeaderLength = *body.HeaderLength
		}
		if body.MaxConnections != nil {
			h.cfg.MaxConnections = *body.MaxConnections
		}
		if body.CommandTimeout != nil {
			h.cfg.CommandTimeout = *body.CommandTimeout
		}
		if body.FirmwareVersion != nil {
			h.cfg.FirmwareVersion = *body.FirmwareVersion
		}
		if body.SerialNumber != nil {
			h.cfg.SerialNumber = *body.SerialNumber
		}
		if body.Model != nil {
			h.cfg.Model = *body.Model
		}
		if body.LMKIdentifier != nil {
			h.cfg.LMKIdentifier = *body.LMKIdentifier
		}
		if body.AuditEnabled != nil {
			h.cfg.AuditEnabled = *body.AuditEnabled
		}
		// Persist
		if h.cfgFile != "" {
			h.cfg.Save(h.cfgFile)
		}
		writeJSON(w, 200, map[string]string{"status": "updated"})
	default:
		http.Error(w, "method not allowed", 405)
	}
}

// POST /hsm-admin/lmk/load-test — load test LMK set
func (h *Handler) handleLoadTestLMK(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	h.lmk.LoadTestLMK()
	h.cfg.SetLMKLoaded(true)
	if h.lmkFile != "" {
		h.lmk.SaveToFile(h.lmkFile)
	}
	log.Println("[HSM] Test LMK loaded via admin API")
	writeJSON(w, 200, map[string]string{"status": "test_lmk_loaded"})
}

// POST /hsm-admin/lmk/generate — generate random LMK set
func (h *Handler) handleGenerateLMK(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := h.lmk.GenerateRandom(); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	h.cfg.SetLMKLoaded(true)
	if h.lmkFile != "" {
		h.lmk.SaveToFile(h.lmkFile)
	}
	log.Println("[HSM] Random LMK generated via admin API")
	writeJSON(w, 200, map[string]string{"status": "random_lmk_generated"})
}

// GET /hsm-admin/lmk/check-values
func (h *Handler) handleLMKCheckValues(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	if !h.cfg.IsLMKLoaded() {
		writeJSON(w, 200, map[string]interface{}{"loaded": false, "checkValues": []string{}})
		return
	}
	kcvs, err := h.lmk.CheckValues()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	var pairs []map[string]string
	for i, k := range kcvs {
		pairs = append(pairs, map[string]string{
			"pair": fmt.Sprintf("%02d-%02d", i*2, i*2+1),
			"kcv":  string(k[:]),
		})
	}
	writeJSON(w, 200, map[string]interface{}{"loaded": true, "checkValues": pairs})
}

// POST /hsm-admin/authorize — authorize or deauthorize
func (h *Handler) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Authorized bool `json:"authorized"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	h.cfg.SetAuthorized(body.Authorized)
	if h.cfgFile != "" {
		h.cfg.Save(h.cfgFile)
	}
	log.Printf("[HSM] Authorization set to %v via admin API", body.Authorized)
	writeJSON(w, 200, map[string]string{"status": fmt.Sprintf("authorized=%v", body.Authorized)})
}

// POST /hsm-admin/command — send a raw host command to the emulator's own TCP port
func (h *Handler) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Command string `json:"command"` // e.g. "NC", "A00002U", "N00016"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if body.Command == "" {
		writeJSON(w, 400, map[string]string{"error": "command is required"})
		return
	}

	// Connect to ourselves (the TCP host-command port)
	addr := fmt.Sprintf("127.0.0.1:%d", h.cfg.HostPort)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": fmt.Sprintf("connect to HSM TCP: %v", err)})
		return
	}
	defer conn.Close()

	// Build frame
	cmdBytes := []byte(body.Command)
	frame := make([]byte, h.cfg.HeaderLength+len(cmdBytes))
	switch h.cfg.HeaderLength {
	case 2:
		binary.BigEndian.PutUint16(frame[:2], uint16(len(cmdBytes)))
	case 4:
		binary.BigEndian.PutUint32(frame[:4], uint32(len(cmdBytes)))
	}
	copy(frame[h.cfg.HeaderLength:], cmdBytes)

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(frame); err != nil {
		writeJSON(w, 500, map[string]string{"error": fmt.Sprintf("write: %v", err)})
		return
	}

	// Read response
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	header := make([]byte, h.cfg.HeaderLength)
	if _, err := io.ReadFull(conn, header); err != nil {
		writeJSON(w, 500, map[string]string{"error": fmt.Sprintf("read header: %v", err)})
		return
	}
	var respLen int
	switch h.cfg.HeaderLength {
	case 2:
		respLen = int(binary.BigEndian.Uint16(header))
	case 4:
		respLen = int(binary.BigEndian.Uint32(header))
	}
	if respLen <= 0 || respLen > 65536 {
		writeJSON(w, 500, map[string]string{"error": fmt.Sprintf("invalid response length: %d", respLen)})
		return
	}

	respBody := make([]byte, respLen)
	if _, err := io.ReadFull(conn, respBody); err != nil {
		writeJSON(w, 500, map[string]string{"error": fmt.Sprintf("read body: %v", err)})
		return
	}

	respStr := string(respBody)
	resp := map[string]interface{}{
		"raw": respStr,
	}
	// Parse standard response fields
	if len(respStr) >= 4 {
		resp["responseCode"] = respStr[:2]
		resp["errorCode"] = respStr[2:4]
		if respStr[2:4] == "00" {
			resp["success"] = true
		} else {
			resp["success"] = false
		}
		if len(respStr) > 4 {
			resp["data"] = respStr[4:]
		}
		resp["errorDescription"] = describeError(respStr[2:4])
	}
	resp["command"] = body.Command
	if len(body.Command) >= 2 {
		resp["commandCode"] = body.Command[:2]
		cmdCode := strings.ToUpper(body.Command[:2])
		resp["commandName"] = describeCommand(cmdCode)
	}

	writeJSON(w, 200, resp)
}

// POST/GET /hsm-admin/commands/disabled — manage disabled commands
func (h *Handler) handleDisabledCommands(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		writeJSON(w, 200, h.cfg.DisabledCommands)
	case "POST":
		var body struct {
			Command  string `json:"command"`
			Disabled bool   `json:"disabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if body.Command == "" {
			writeJSON(w, 400, map[string]string{"error": "command is required"})
			return
		}
		if h.cfg.DisabledCommands == nil {
			h.cfg.DisabledCommands = make(map[string]bool)
		}
		if body.Disabled {
			h.cfg.DisabledCommands[strings.ToUpper(body.Command)] = true
		} else {
			delete(h.cfg.DisabledCommands, strings.ToUpper(body.Command))
		}
		if h.cfgFile != "" {
			h.cfg.Save(h.cfgFile)
		}
		writeJSON(w, 200, map[string]string{"status": "updated"})
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func describeError(code string) string {
	switch code {
	case "00":
		return "No error"
	case "01":
		return "Verification failure"
	case "02":
		return "Bad key type"
	case "03":
		return "Bad key length"
	case "04":
		return "Bad source key"
	case "05":
		return "Bad message length"
	case "10":
		return "Parity error"
	case "13":
		return "No LMK loaded"
	case "15":
		return "Bad input"
	case "20":
		return "PIN block error"
	case "27":
		return "Bad scheme"
	case "41":
		return "Internal error"
	case "68":
		return "Command disabled"
	case "90":
		return "Not authorized"
	default:
		return "Unknown error"
	}
}

func describeCommand(code string) string {
	switch code {
	case "NC":
		return "Perform Diagnostics"
	case "NO":
		return "HSM Status"
	case "A0":
		return "Generate Key"
	case "A6":
		return "Import Key"
	case "A8":
		return "Export Key"
	case "BU":
		return "Key Check Value"
	case "B0":
		return "Translate Key Scheme"
	case "EI":
		return "Generate RSA Key Pair"
	case "BA":
		return "Encrypt Clear PIN"
	case "DA":
		return "Verify PIN (IBM)"
	case "DC":
		return "Verify PIN (Visa PVV)"
	case "DE":
		return "Generate PIN Offset"
	case "DG":
		return "Generate PVV"
	case "CA":
		return "Translate PIN (ZPK→ZPK)"
	case "FA":
		return "Translate PIN Block"
	case "CW":
		return "Generate CVV"
	case "CY":
		return "Verify CVV"
	case "M0":
		return "Encrypt Data Block"
	case "M2":
		return "Decrypt Data Block"
	case "MS":
		return "Generate MAC"
	case "KQ":
		return "Verify ARQC / Generate ARPC"
	case "N0":
		return "Generate Random Data"
	default:
		return "Unknown"
	}
}
