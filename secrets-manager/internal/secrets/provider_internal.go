package secrets

import (
	"context"
	"fmt"
)

type InternalProvider struct {
	masterKeyHex string
}

func NewInternalProvider(masterKeyHex string) *InternalProvider {
	return &InternalProvider{masterKeyHex: masterKeyHex}
}

func (p *InternalProvider) GenerateKeyPair(ctx context.Context, keyType string, bits int) (*GeneratedKey, error) {
	pubPEM, privPEM, fingerprint, err := generateRSAKey(bits)
	if err != nil {
		return nil, err
	}

	privEnc, err := encryptPrivateKey(privPEM, p.masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("encrypt private key: %w", err)
	}

	return &GeneratedKey{
		PublicKeyPEM:  pubPEM,
		PrivateKeyEnc: privEnc,
		Fingerprint:   fingerprint,
		KeySizeBits:   bits,
	}, nil
}

func (p *InternalProvider) GenerateSymmetricKey(ctx context.Context, keyType string, bits int) (*GeneratedKey, error) {
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
		return nil, err
	}

	enc, err := encryptPrivateKey(keyHex, p.masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("encrypt symmetric key: %w", err)
	}

	return &GeneratedKey{
		PrivateKeyEnc: enc,
		Fingerprint:   fingerprint,
		KeySizeBits:   bits,
	}, nil
}

func (p *InternalProvider) GenerateCertificate(ctx context.Context, opts CertificateOptions) (*GeneratedKey, error) {
	certPEM, privPEM, fingerprint, err := generateSelfSignedCert(opts)
	if err != nil {
		return nil, err
	}

	privEnc, err := encryptPrivateKey(privPEM, p.masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("encrypt cert private key: %w", err)
	}

	bits := opts.KeyBits
	if bits == 0 {
		bits = 2048
	}

	return &GeneratedKey{
		CertificatePEM: certPEM,
		PrivateKeyEnc:  privEnc,
		Fingerprint:    fingerprint,
		KeySizeBits:    bits,
	}, nil
}

func (p *InternalProvider) GenerateCSR(ctx context.Context, opts CertificateOptions) (string, string, error) {
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

func (p *InternalProvider) SupportsKeyType(keyType string) bool {
	switch keyType {
	case "RSA-1024", "RSA-2048", "RSA-3072", "RSA-4096",
		"AES-128", "AES-192", "AES-256", "3DES", "X509":
		return true
	}
	return false
}

func (p *InternalProvider) TestConnection(ctx context.Context) error {
	return nil // internal provider is always available
}
