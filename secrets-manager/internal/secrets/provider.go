package secrets

import "context"

type GeneratedKey struct {
	PublicKeyPEM   string
	PrivateKeyEnc  string
	CertificatePEM string
	HSMKeyHandle   string
	Fingerprint    string
	KeySizeBits    int
}

type SecurityProviderBackend interface {
	GenerateKeyPair(ctx context.Context, keyType string, bits int) (*GeneratedKey, error)
	GenerateSymmetricKey(ctx context.Context, keyType string, bits int) (*GeneratedKey, error)
	GenerateCertificate(ctx context.Context, opts CertificateOptions) (*GeneratedKey, error)
	GenerateCSR(ctx context.Context, opts CertificateOptions) (csrPEM, privKeyEnc string, err error)
	SupportsKeyType(keyType string) bool
	TestConnection(ctx context.Context) error
}

func GetProviderBackend(provider *SecurityProvider, masterKeyHex string) SecurityProviderBackend {
	switch provider.ProviderType {
	case "hsm":
		return NewHSMProvider(provider)
	case "ca":
		return NewCAProvider(provider, masterKeyHex)
	default:
		return NewInternalProvider(masterKeyHex)
	}
}
