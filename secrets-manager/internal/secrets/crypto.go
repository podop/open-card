package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"time"
)

type CertificateOptions struct {
	CommonName   string
	Organization string
	Country      string
	ValidityDays int
	SANs         []string
	KeyBits      int
}

func generateRSAKey(bits int) (pubPEM, privPEM, fingerprint string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return "", "", "", fmt.Errorf("generate RSA key: %w", err)
	}

	privBytes := x509.MarshalPKCS1PrivateKey(key)
	privBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}
	privPEM = string(pem.EncodeToMemory(privBlock))

	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", "", fmt.Errorf("marshal public key: %w", err)
	}
	pubBlock := &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}
	pubPEM = string(pem.EncodeToMemory(pubBlock))

	hash := sha256.Sum256(pubBytes)
	fingerprint = hex.EncodeToString(hash[:])

	return pubPEM, privPEM, fingerprint, nil
}

func generateAESKey(bits int) (keyHex, fingerprint string, err error) {
	keyBytes := make([]byte, bits/8)
	if _, err := io.ReadFull(rand.Reader, keyBytes); err != nil {
		return "", "", fmt.Errorf("generate AES key: %w", err)
	}
	keyHex = hex.EncodeToString(keyBytes)
	hash := sha256.Sum256(keyBytes)
	fingerprint = hex.EncodeToString(hash[:])
	return keyHex, fingerprint, nil
}

func generate3DESKey() (keyHex, fingerprint string, err error) {
	keyBytes := make([]byte, des.BlockSize*3) // 24 bytes
	if _, err := io.ReadFull(rand.Reader, keyBytes); err != nil {
		return "", "", fmt.Errorf("generate 3DES key: %w", err)
	}
	keyHex = hex.EncodeToString(keyBytes)
	hash := sha256.Sum256(keyBytes)
	fingerprint = hex.EncodeToString(hash[:])
	return keyHex, fingerprint, nil
}

func generateSelfSignedCert(opts CertificateOptions) (certPEM, privPEM, fingerprint string, err error) {
	bits := opts.KeyBits
	if bits == 0 {
		bits = 2048
	}

	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return "", "", "", fmt.Errorf("generate key for cert: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", "", fmt.Errorf("generate serial: %w", err)
	}

	notBefore := time.Now()
	validityDays := opts.ValidityDays
	if validityDays <= 0 {
		validityDays = 365
	}
	notAfter := notBefore.Add(time.Duration(validityDays) * 24 * time.Hour)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   opts.CommonName,
			Organization: []string{opts.Organization},
			Country:      []string{opts.Country},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	for _, san := range opts.SANs {
		if ip := net.ParseIP(san); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, san)
		}
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return "", "", "", fmt.Errorf("create certificate: %w", err)
	}

	certBlock := &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}
	certPEM = string(pem.EncodeToMemory(certBlock))

	privBytes := x509.MarshalPKCS1PrivateKey(key)
	privBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}
	privPEM = string(pem.EncodeToMemory(privBlock))

	hash := sha256.Sum256(certBytes)
	fingerprint = hex.EncodeToString(hash[:])

	return certPEM, privPEM, fingerprint, nil
}

func generateCSR(opts CertificateOptions) (csrPEM, privPEM string, err error) {
	bits := opts.KeyBits
	if bits == 0 {
		bits = 2048
	}

	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return "", "", fmt.Errorf("generate key for CSR: %w", err)
	}

	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   opts.CommonName,
			Organization: []string{opts.Organization},
			Country:      []string{opts.Country},
		},
	}

	for _, san := range opts.SANs {
		if ip := net.ParseIP(san); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, san)
		}
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		return "", "", fmt.Errorf("create CSR: %w", err)
	}

	csrBlock := &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes}
	csrPEM = string(pem.EncodeToMemory(csrBlock))

	privBytes := x509.MarshalPKCS1PrivateKey(key)
	privBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}
	privPEM = string(pem.EncodeToMemory(privBlock))

	return csrPEM, privPEM, nil
}

func encryptPrivateKey(plaintext string, masterKeyHex string) (string, error) {
	masterKey, err := hex.DecodeString(masterKeyHex)
	if err != nil {
		return "", fmt.Errorf("decode master key: %w", err)
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

func decryptPrivateKey(ciphertextHex string, masterKeyHex string) (string, error) {
	masterKey, err := hex.DecodeString(masterKeyHex)
	if err != nil {
		return "", fmt.Errorf("decode master key: %w", err)
	}

	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}
