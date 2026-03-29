package config

import (
	"encoding/json"
	"os"
	"sync"
)

// Config mirrors the configuration parameters of a production PayShield 10K.
type Config struct {
	mu sync.RWMutex

	// ── Network ──────────────────────────────────────
	HostAddress string `json:"hostAddress"` // bind address
	HostPort    int    `json:"hostPort"`    // host command TCP port (default 1500)
	ConsolePort int    `json:"consolePort"` // console/admin TCP port (default 2222)

	// ── Host Interface ───────────────────────────────
	HeaderLength    int  `json:"headerLength"`    // length-prefix size: 2 or 4 bytes
	MaxConnections  int  `json:"maxConnections"`  // max concurrent host connections
	CommandTimeout  int  `json:"commandTimeout"`  // seconds
	MessageLengthLE bool `json:"messageLengthLE"` // false=big-endian (default), true=little-endian

	// ── HSM Identity ─────────────────────────────────
	FirmwareVersion string `json:"firmwareVersion"` // e.g. "0650-E000"
	SerialNumber    string `json:"serialNumber"`    // e.g. "A000-0000-00F1"
	Model           string `json:"model"`           // "PayShield 10K"
	LMKIdentifier   int    `json:"lmkIdentifier"`   // 0-based LMK set index

	// ── Security ─────────────────────────────────────
	Authorized   bool `json:"authorized"`   // HSM must be authorized to process host commands
	LMKLoaded    bool `json:"lmkLoaded"`    // whether LMK keys have been loaded
	AuditEnabled bool `json:"auditEnabled"` // log all commands

	// ── Enabled Commands ─────────────────────────────
	DisabledCommands map[string]bool `json:"disabledCommands"` // commands that are explicitly disabled
}

func DefaultConfig() *Config {
	return &Config{
		HostAddress:      "0.0.0.0",
		HostPort:         1500,
		ConsolePort:      2222,
		HeaderLength:     2,
		MaxConnections:   64,
		CommandTimeout:   30,
		FirmwareVersion:  "0650-E000",
		SerialNumber:     "EMUL-0000-0001",
		Model:            "PayShield 10K (Emulator)",
		Authorized:       false,
		LMKLoaded:        false,
		AuditEnabled:     true,
		DisabledCommands: make(map[string]bool),
	}
}

func (c *Config) IsAuthorized() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Authorized
}

func (c *Config) SetAuthorized(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Authorized = v
}

func (c *Config) IsLMKLoaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LMKLoaded
}

func (c *Config) SetLMKLoaded(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LMKLoaded = v
}

func (c *Config) IsCommandDisabled(cmd string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.DisabledCommands[cmd]
}

// Save persists config to a JSON file.
func (c *Config) Save(path string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Load reads config from a JSON file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
