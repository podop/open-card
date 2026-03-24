package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	otpLength     = 6
	otpTTL        = 5 * time.Minute
	maxAttempts   = 3
)

// OTPService handles generation and verification of one-time passcodes.
type OTPService struct {
	Source            string // "internal" or "hsm"
	SecretsManagerURL string
	HsmEndpointId    int
}

// NewOTPService creates a new OTPService with internal random source.
func NewOTPService() *OTPService {
	return &OTPService{Source: "internal"}
}

// NewOTPServiceWithConfig creates an OTPService with the given configuration.
func NewOTPServiceWithConfig(source, secretsManagerURL string, hsmEndpointId int) *OTPService {
	if source == "" {
		source = "internal"
	}
	return &OTPService{
		Source:            source,
		SecretsManagerURL: secretsManagerURL,
		HsmEndpointId:    hsmEndpointId,
	}
}

// Generate returns a random numeric OTP and its bcrypt hash, plus expiry time.
func (s *OTPService) Generate() (otp, hash string, expiry time.Time, err error) {
	digits := make([]byte, otpLength)

	if s.Source == "hsm" && s.SecretsManagerURL != "" && s.HsmEndpointId > 0 {
		// Try HSM-backed random generation
		hsmDigits, hsmErr := s.generateFromHSM()
		if hsmErr == nil {
			copy(digits, hsmDigits)
		} else {
			// Fall back to crypto/rand
			if err := s.generateInternal(digits); err != nil {
				return "", "", time.Time{}, err
			}
		}
	} else {
		if err := s.generateInternal(digits); err != nil {
			return "", "", time.Time{}, err
		}
	}

	otp = string(digits)

	b, err := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("otp hashing failed: %w", err)
	}
	hash = base64.StdEncoding.EncodeToString(b)
	expiry = time.Now().UTC().Add(otpTTL)
	return otp, hash, expiry, nil
}

func (s *OTPService) generateInternal(digits []byte) error {
	for i := range digits {
		n, e := rand.Int(rand.Reader, big.NewInt(10))
		if e != nil {
			return fmt.Errorf("otp generation failed: %w", e)
		}
		digits[i] = byte('0') + byte(n.Int64())
	}
	return nil
}

func (s *OTPService) generateFromHSM() ([]byte, error) {
	url := fmt.Sprintf("%s/secrets/hsm/v1/admin/generate-random", strings.TrimRight(s.SecretsManagerURL, "/"))
	body := fmt.Sprintf(`{"length":%d,"endpointId":%d}`, otpLength, s.HsmEndpointId)

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hsm random request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hsm random returned status %d", resp.StatusCode)
	}

	var result struct {
		Random string `json:"random"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode hsm response: %w", err)
	}

	randomBytes, err := hex.DecodeString(result.Random)
	if err != nil {
		return nil, fmt.Errorf("decode hex: %w", err)
	}

	// Convert random bytes to decimal digits
	digits := make([]byte, otpLength)
	for i := 0; i < otpLength && i < len(randomBytes); i++ {
		digits[i] = byte('0') + (randomBytes[i] % 10)
	}
	return digits, nil
}

// Verify compares a submitted OTP against the stored hash.
// Returns true only if the code matches and has not expired.
func (s *OTPService) Verify(submitted, storedHash string, expiry time.Time) bool {
	if time.Now().UTC().After(expiry) {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(storedHash)
	if err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword(decoded, []byte(submitted)) == nil
}

// MaxAttemptsExceeded returns true if the challenge attempt count has been exhausted.
func (s *OTPService) MaxAttemptsExceeded(attempts int) bool {
	return attempts >= maxAttempts
}
