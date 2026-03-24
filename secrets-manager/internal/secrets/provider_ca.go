package secrets

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

type CAConfig struct {
	Vendor string `json:"vendor"`
	APIURL string `json:"apiUrl"`
	APIKey string `json:"apiKey"`
}

type CAProvider struct {
	config       CAConfig
	masterKeyHex string
	log          *zap.Logger
}

func NewCAProvider(provider *SecurityProvider, masterKeyHex string) *CAProvider {
	var cfg CAConfig
	json.Unmarshal(provider.Config, &cfg) //nolint:errcheck
	return &CAProvider{
		config:       cfg,
		masterKeyHex: masterKeyHex,
		log:          zap.L(),
	}
}

func (p *CAProvider) GenerateKeyPair(ctx context.Context, keyType string, bits int) (*GeneratedKey, error) {
	return nil, fmt.Errorf("CA provider does not support raw key pair generation; use GenerateCSR instead")
}

func (p *CAProvider) GenerateSymmetricKey(ctx context.Context, keyType string, bits int) (*GeneratedKey, error) {
	return nil, fmt.Errorf("CA provider does not support symmetric key generation")
}

func (p *CAProvider) GenerateCertificate(ctx context.Context, opts CertificateOptions) (*GeneratedKey, error) {
	p.log.Info("CA: GenerateCertificate — generating CSR for manual CA submission",
		zap.String("cn", opts.CommonName), zap.String("vendor", p.config.Vendor))

	csrPEM, privPEM, err := generateCSR(opts)
	if err != nil {
		return nil, fmt.Errorf("generate CSR: %w", err)
	}

	privEnc, err := encryptPrivateKey(privPEM, p.masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("encrypt private key: %w", err)
	}

	bits := opts.KeyBits
	if bits == 0 {
		bits = 2048
	}

	return &GeneratedKey{
		CertificatePEM: csrPEM, // CSR stored as certificate PEM until CA signs it
		PrivateKeyEnc:  privEnc,
		Fingerprint:    fmt.Sprintf("ca-csr-%s", opts.CommonName),
		KeySizeBits:    bits,
	}, nil
}

func (p *CAProvider) GenerateCSR(ctx context.Context, opts CertificateOptions) (string, string, error) {
	csrPEM, privPEM, err := generateCSR(opts)
	if err != nil {
		return "", "", err
	}

	privEnc, err := encryptPrivateKey(privPEM, p.masterKeyHex)
	if err != nil {
		return "", "", fmt.Errorf("encrypt CSR private key: %w", err)
	}

	return csrPEM, privEnc, nil
}

func (p *CAProvider) SupportsKeyType(keyType string) bool {
	return keyType == "X509"
}

func (p *CAProvider) TestConnection(ctx context.Context) error {
	p.log.Info("CA: TestConnection (stub)", zap.String("apiUrl", p.config.APIURL))
	if p.config.APIURL == "" {
		return fmt.Errorf("CA API URL not configured")
	}
	return nil
}
