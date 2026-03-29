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
	config       HSMConfig
	masterKeyHex string
	log          *zap.Logger
}

func NewHSMProvider(provider *SecurityProvider, masterKeyHex string) *HSMProvider {
	var cfg HSMConfig
	json.Unmarshal(provider.Config, &cfg) //nolint:errcheck
	return &HSMProvider{
		config:       cfg,
		masterKeyHex: masterKeyHex,
		log:          zap.L(),
	}
}

func (p *HSMProvider) GenerateKeyPair(ctx context.Context, keyType string, bits int) (*GeneratedKey, error) {
	p.log.Info("HSM: GenerateKeyPair (emulated)",
		zap.String("model", p.config.Model),
		zap.String("keyType", keyType),
		zap.Int("bits", bits),
	)

	pubPEM, privPEM, fingerprint, err := generateRSAKey(bits)
	if err != nil {
		return nil, fmt.Errorf("HSM emulator: generate RSA key: %w", err)
	}

	privEnc, err := encryptPrivateKey(privPEM, p.masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("HSM emulator: encrypt private key: %w", err)
	}

	handle := fmt.Sprintf("hsm://%s:%d/slot/%d/key/%s-%d", p.config.Address, p.config.Port, p.config.SlotID, keyType, bits)
	return &GeneratedKey{
		PublicKeyPEM:  pubPEM,
		PrivateKeyEnc: privEnc,
		HSMKeyHandle:  handle,
		Fingerprint:   fingerprint,
		KeySizeBits:   bits,
	}, nil
}

func (p *HSMProvider) GenerateSymmetricKey(ctx context.Context, keyType string, bits int) (*GeneratedKey, error) {
	p.log.Info("HSM: GenerateSymmetricKey (emulated)",
		zap.String("model", p.config.Model),
		zap.String("keyType", keyType),
		zap.Int("bits", bits),
	)

	var keyHex, fingerprint string
	var err error

	switch keyType {
	case "3DES":
		keyHex, fingerprint, err = generate3DESKey()
		bits = 192
	default:
		keyHex, fingerprint, err = generateAESKey(bits)
	}
	if err != nil {
		return nil, fmt.Errorf("HSM emulator: generate symmetric key: %w", err)
	}

	enc, err := encryptPrivateKey(keyHex, p.masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("HSM emulator: encrypt symmetric key: %w", err)
	}

	handle := fmt.Sprintf("hsm://%s:%d/slot/%d/sym/%s-%d", p.config.Address, p.config.Port, p.config.SlotID, keyType, bits)
	return &GeneratedKey{
		PrivateKeyEnc: enc,
		HSMKeyHandle:  handle,
		Fingerprint:   fingerprint,
		KeySizeBits:   bits,
	}, nil
}

func (p *HSMProvider) GenerateCertificate(ctx context.Context, opts CertificateOptions) (*GeneratedKey, error) {
	p.log.Info("HSM: GenerateCertificate (emulated)",
		zap.String("model", p.config.Model),
		zap.String("cn", opts.CommonName),
	)

	certPEM, privPEM, fingerprint, err := generateSelfSignedCert(opts)
	if err != nil {
		return nil, fmt.Errorf("HSM emulator: generate certificate: %w", err)
	}

	privEnc, err := encryptPrivateKey(privPEM, p.masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("HSM emulator: encrypt cert key: %w", err)
	}

	bits := opts.KeyBits
	if bits == 0 {
		bits = 2048
	}

	handle := fmt.Sprintf("hsm://%s:%d/slot/%d/cert/%s", p.config.Address, p.config.Port, p.config.SlotID, opts.CommonName)
	return &GeneratedKey{
		CertificatePEM: certPEM,
		PrivateKeyEnc:  privEnc,
		HSMKeyHandle:   handle,
		Fingerprint:    fingerprint,
		KeySizeBits:    bits,
	}, nil
}

func (p *HSMProvider) GenerateCSR(ctx context.Context, opts CertificateOptions) (string, string, error) {
	p.log.Info("HSM: GenerateCSR (emulated)", zap.String("cn", opts.CommonName))

	csrPEM, privPEM, err := generateCSR(opts)
	if err != nil {
		return "", "", fmt.Errorf("HSM emulator: generate CSR: %w", err)
	}

	privEnc, err := encryptPrivateKey(privPEM, p.masterKeyHex)
	if err != nil {
		return "", "", fmt.Errorf("HSM emulator: encrypt CSR key: %w", err)
	}

	return csrPEM, privEnc, nil
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
	p.log.Info("HSM: would send NC (Diagnostics) command",
		zap.String("target", fmt.Sprintf("%s:%d", p.config.Address, p.config.Port)),
		zap.Int("headerLen", p.config.HeaderLength),
	)
	return nil
}
