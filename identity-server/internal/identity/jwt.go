package identity

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"
)

// JWTService handles RS256 signing and verification.
type JWTService struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func NewJWTService(privPEM, pubPEM string) (*JWTService, error) {
	block, _ := pem.Decode([]byte(privPEM))
	if block == nil {
		return nil, errors.New("failed to decode RSA private key PEM")
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	pubBlock, _ := pem.Decode([]byte(pubPEM))
	if pubBlock == nil {
		return nil, errors.New("failed to decode RSA public key PEM")
	}
	pubIface, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	pub, ok := pubIface.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}

	return &JWTService{privateKey: priv, publicKey: pub}, nil
}

// NewJWTServiceFromPublicKey creates a verify-only service (no signing).
func NewJWTServiceFromPublicKey(pubPEM string) (*JWTService, error) {
	pubBlock, _ := pem.Decode([]byte(pubPEM))
	if pubBlock == nil {
		return nil, errors.New("failed to decode RSA public key PEM")
	}
	pubIface, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	pub, ok := pubIface.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	return &JWTService{publicKey: pub}, nil
}

// Claims represents the JWT payload.
type Claims struct {
	Sub     string   `json:"sub"`
	Iss     string   `json:"iss"`
	Exp     int64    `json:"exp"`
	Iat     int64    `json:"iat"`
	Jti     string   `json:"jti"`
	UserID  int      `json:"uid"`
	Actions []string `json:"actions"`
}

func (j *JWTService) Sign(claims *Claims) (string, error) {
	if j.privateKey == nil {
		return "", errors.New("no private key available for signing")
	}
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64url(headerJSON)
	claimsB64 := base64url(claimsJSON)
	signingInput := headerB64 + "." + claimsB64

	h := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, j.privateKey, crypto.SHA256, h[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}

	return signingInput + "." + base64url(sig), nil
}

func (j *JWTService) Verify(tokenStr string) (*Claims, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	sigBytes, err := base64urlDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}

	h := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(j.publicKey, crypto.SHA256, h[:], sigBytes); err != nil {
		return nil, errors.New("invalid signature")
	}

	claimsJSON, err := base64urlDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}

	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}

// GenerateRSAKeyPair generates a new RSA 2048 key pair and returns PEM strings.
func GenerateRSAKeyPair() (privPEM, pubPEM string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	privBytes := x509.MarshalPKCS1PrivateKey(key)
	privBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}
	privPEM = string(pem.EncodeToMemory(privBlock))

	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", err
	}
	pubBlock := &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}
	pubPEM = string(pem.EncodeToMemory(pubBlock))

	return privPEM, pubPEM, nil
}

func base64url(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func base64urlDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
