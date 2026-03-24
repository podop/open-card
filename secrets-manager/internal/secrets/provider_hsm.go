package secrets

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

type HSMConfig struct {
	// Common
	Model       string `json:"model"`
	LibraryPath string `json:"libraryPath"`
	SlotID      int    `json:"slotId"`
	PIN         string `json:"pin"`
	Address     string `json:"address"`
	Port        int    `json:"port"`

	// Thales-specific
	Preset             string `json:"preset"`             // "payshield-10k", "payshield-9k", "custom"
	Protocol           string `json:"protocol"`           // "TCP", "TLS", "UDP"
	HeaderLength       int    `json:"headerLength"`       // message header length in bytes (typically 4)
	FirmwareVersion    string `json:"firmwareVersion"`    // e.g. "1.4A0000" for PS10k
	HostCommandTimeout int    `json:"hostCommandTimeout"` // seconds
	ConnectionTimeout  int    `json:"connectionTimeout"`  // seconds
	MaxConnections     int    `json:"maxConnections"`     // connection pool size
	LMKIdentifier      int    `json:"lmkIdentifier"`      // LMK set (0-based)
}

type HSMProvider struct {
	config HSMConfig
	log    *zap.Logger
}

func NewHSMProvider(provider *SecurityProvider) *HSMProvider {
	var cfg HSMConfig
	json.Unmarshal(provider.Config, &cfg) //nolint:errcheck
	return &HSMProvider{
		config: cfg,
		log:    zap.L(),
	}
}

func (p *HSMProvider) GenerateKeyPair(ctx context.Context, keyType string, bits int) (*GeneratedKey, error) {
	p.log.Info("HSM: GenerateKeyPair (stub)",
		zap.String("model", p.config.Model),
		zap.String("keyType", keyType),
		zap.Int("bits", bits),
	)
	// Thales host command EI (Generate RSA Key Pair)
	handle := fmt.Sprintf("hsm://%s:%d/slot/%d/key/%s-%d", p.config.Address, p.config.Port, p.config.SlotID, keyType, bits)
	return &GeneratedKey{
		HSMKeyHandle: handle,
		Fingerprint:  fmt.Sprintf("hsm-%s-%d", keyType, bits),
		KeySizeBits:  bits,
	}, nil
}

func (p *HSMProvider) GenerateSymmetricKey(ctx context.Context, keyType string, bits int) (*GeneratedKey, error) {
	p.log.Info("HSM: GenerateSymmetricKey (stub)",
		zap.String("model", p.config.Model),
		zap.String("keyType", keyType),
		zap.Int("bits", bits),
	)
	// Thales host command A0 (Generate Key)
	handle := fmt.Sprintf("hsm://%s:%d/slot/%d/sym/%s-%d", p.config.Address, p.config.Port, p.config.SlotID, keyType, bits)
	return &GeneratedKey{
		HSMKeyHandle: handle,
		Fingerprint:  fmt.Sprintf("hsm-%s-%d", keyType, bits),
		KeySizeBits:  bits,
	}, nil
}

func (p *HSMProvider) GenerateCertificate(ctx context.Context, opts CertificateOptions) (*GeneratedKey, error) {
	p.log.Info("HSM: GenerateCertificate (stub)",
		zap.String("model", p.config.Model),
		zap.String("cn", opts.CommonName),
	)
	bits := opts.KeyBits
	if bits == 0 {
		bits = 2048
	}
	handle := fmt.Sprintf("hsm://%s:%d/slot/%d/cert/%s", p.config.Address, p.config.Port, p.config.SlotID, opts.CommonName)
	return &GeneratedKey{
		HSMKeyHandle: handle,
		Fingerprint:  fmt.Sprintf("hsm-cert-%s", opts.CommonName),
		KeySizeBits:  bits,
	}, nil
}

func (p *HSMProvider) GenerateCSR(ctx context.Context, opts CertificateOptions) (string, string, error) {
	p.log.Info("HSM: GenerateCSR (stub)", zap.String("cn", opts.CommonName))
	return "", "", fmt.Errorf("HSM CSR generation not yet implemented")
}

func (p *HSMProvider) SupportsKeyType(keyType string) bool {
	switch keyType {
	case "RSA-2048", "RSA-3072", "RSA-4096", "AES-128", "AES-256", "3DES":
		return true
	}
	return false
}

func (p *HSMProvider) TestConnection(ctx context.Context) error {
	p.log.Info("HSM: TestConnection (stub)",
		zap.String("model", p.config.Model),
		zap.String("address", p.config.Address),
		zap.Int("port", p.config.Port),
		zap.String("protocol", p.config.Protocol),
	)
	if p.config.Address == "" {
		return fmt.Errorf("HSM address not configured")
	}
	if p.config.Port == 0 {
		return fmt.Errorf("HSM port not configured")
	}
	// Thales host command NC (Perform Diagnostics) — would be sent here
	p.log.Info("HSM: would send NC (Diagnostics) command",
		zap.String("target", fmt.Sprintf("%s:%d", p.config.Address, p.config.Port)),
		zap.Int("headerLen", p.config.HeaderLength),
	)
	return nil
}
