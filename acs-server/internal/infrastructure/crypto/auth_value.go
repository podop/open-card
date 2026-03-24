package crypto

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/service"
)

// HMACAuthValueService generates Authentication Values (CAVV/AAV) using HMAC-SHA256.
//
// Per-network master keys are supported: if a key is registered for tx.Network it is
// used; otherwise the service falls back to the global default key.
//
// NOTE: The EMV 3DS spec mandates a network-specific CAVV algorithm (e.g. Visa CAVV,
// Mastercard AAV, Amex AEVV). This implementation uses HMAC-SHA256 as a structurally
// correct placeholder. Replace the core computation with the scheme-mandated algorithm
// before going to production / EMVCo compliance testing.
type HMACAuthValueService struct {
	defaultKey  []byte
	networkKeys map[entity.Network][]byte
}

// NewHMACAuthValueService creates a new HMACAuthValueService.
//
//   - defaultKeyHex    – global fallback key (hex-encoded, ≥ 16 bytes after decode).
//   - networkKeyHexes  – optional per-network overrides keyed by entity.Network value;
//     entries with an empty string are silently skipped.
func NewHMACAuthValueService(defaultKeyHex string, networkKeyHexes map[entity.Network]string) (*HMACAuthValueService, error) {
	dk, err := decodeKey(defaultKeyHex)
	if err != nil {
		return nil, fmt.Errorf("default master key: %w", err)
	}

	nk := make(map[entity.Network][]byte, len(networkKeyHexes))
	for network, hexStr := range networkKeyHexes {
		if hexStr == "" {
			continue
		}
		key, err := decodeKey(hexStr)
		if err != nil {
			return nil, fmt.Errorf("network %q master key: %w", network, err)
		}
		nk[network] = key
	}

	return &HMACAuthValueService{defaultKey: dk, networkKeys: nk}, nil
}

// Generate computes an Authentication Value and the correct ECI for the given transaction.
// The ECI is derived from the card network (e.g. "05" for Visa, "02" for Mastercard).
func (s *HMACAuthValueService) Generate(_ context.Context, tx *entity.Transaction) (authValue, eci string, err error) {
	key := s.keyForNetwork(tx.Network)

	// Input: network || ACSTransID || DSTransID [|| amount]
	input := string(tx.Network) + "|" + tx.ACSTransactionID.String() + "|" + tx.DSTransactionID.String()
	if tx.PurchaseAmount != nil {
		input += "|" + *tx.PurchaseAmount
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(input))
	raw := mac.Sum(nil)

	// Encode as base64 (20 bytes per EMV spec recommendation; we use the first 20 bytes)
	authValue = base64.StdEncoding.EncodeToString(raw[:20])
	eci = tx.Network.DefaultECIs().FullyAuthenticated
	return authValue, eci, nil
}

// keyForNetwork returns the per-network key if registered, otherwise the default key.
func (s *HMACAuthValueService) keyForNetwork(n entity.Network) []byte {
	if k, ok := s.networkKeys[n]; ok {
		return k
	}
	return s.defaultKey
}

func decodeKey(hexStr string) ([]byte, error) {
	key, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("hex decode: %w", err)
	}
	if len(key) < 16 {
		return nil, fmt.Errorf("key too short: need at least 16 bytes, got %d", len(key))
	}
	return key, nil
}

// Ensure HMACAuthValueService satisfies the domain service interface.
var _ service.AuthValueService = (*HMACAuthValueService)(nil)