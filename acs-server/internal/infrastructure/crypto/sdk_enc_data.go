package crypto

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"

	"github.com/go-jose/go-jose/v3"
)

// SDKEncryptedPayload is the JSON content inside the sdkEncData JWE.
// The EMV 3DS SDK encrypts this with the ACS public key (RSA-OAEP / A128CBC-HS256).
// Fields follow EMV 3DS Specification Table A.3.
type SDKEncryptedPayload struct {
	// DeviceChannel is always "01" for app-based transactions.
	DeviceChannel string `json:"deviceChannel"`
	// SDKType: "01" = default SDK, "02" = split-SDK.
	SDKType string `json:"sdkType"`
	// DeviceBindingStatus: "01"=bound, "02"=not bound, "03"=not available, "04"=not eligible.
	DeviceBindingStatus string `json:"deviceBindingStatus"`
	// ClientDeviceInfo is a base64url-encoded blob of device characteristics.
	ClientDeviceInfo string `json:"clientDeviceInfo"`
	// SDKVersion is the SDK version string (e.g. "1.0.0").
	SDKVersion string `json:"sdkVersion"`
	// AppVersion is the 3DS Requestor app version.
	AppVersion string `json:"appVersion"`
	// SDKReferenceNumber is the EMVCo-assigned SDK Reference Number.
	SDKReferenceNumber string `json:"sdkReferenceNumber"`
}

// SDKEncDataDecryptor decrypts the sdkEncData JWE field from an app-based AReq.
// Supported algorithms: RSA-OAEP + A128CBC-HS256 (spec default) and RSA-OAEP-256 + A256GCM.
type SDKEncDataDecryptor struct {
	privateKey *rsa.PrivateKey
}

// NewSDKEncDataDecryptorFromPEM parses a PEM-encoded RSA private key and returns a decryptor.
// Accepts both PKCS#8 ("BEGIN PRIVATE KEY") and PKCS#1 ("BEGIN RSA PRIVATE KEY") formats.
func NewSDKEncDataDecryptorFromPEM(pemBytes []byte) (*SDKEncDataDecryptor, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("sdk enc data decryptor: no PEM block found")
	}

	var rsaKey *rsa.PrivateKey
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("sdk enc data decryptor: parse PKCS1: %w", err)
		}
		rsaKey = key
	default: // "PRIVATE KEY" (PKCS#8)
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("sdk enc data decryptor: parse PKCS8: %w", err)
		}
		var ok bool
		rsaKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("sdk enc data decryptor: expected RSA key, got %T", key)
		}
	}

	return &SDKEncDataDecryptor{privateKey: rsaKey}, nil
}

// Decrypt decrypts a JWE compact serialisation and unmarshals the JSON payload.
func (d *SDKEncDataDecryptor) Decrypt(sdkEncData string) (*SDKEncryptedPayload, error) {
	jwe, err := jose.ParseEncrypted(sdkEncData)
	if err != nil {
		return nil, fmt.Errorf("sdk enc data: parse JWE: %w", err)
	}

	plaintext, err := jwe.Decrypt(d.privateKey)
	if err != nil {
		return nil, fmt.Errorf("sdk enc data: decrypt JWE: %w", err)
	}

	var payload SDKEncryptedPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, fmt.Errorf("sdk enc data: unmarshal payload: %w", err)
	}
	return &payload, nil
}