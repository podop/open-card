package hsm

// Capability domains for HSM routing
const (
	CapKeys   = "keys"
	CapPIN    = "pin"
	CapEMV    = "emv"
	CapCards  = "cards"
	CapP2PE   = "p2pe"
	CapRKL    = "rkl"
	CapTokens = "tokens"
	CapAdmin  = "admin"
)

// AllCapabilities is the full set of capability domains.
var AllCapabilities = []string{CapKeys, CapPIN, CapEMV, CapCards, CapP2PE, CapRKL, CapTokens, CapAdmin}

// ModelCapabilities maps HSM model identifiers to their supported capability sets.
var ModelCapabilities = map[string][]string{
	"payshield-10k": {CapKeys, CapPIN, CapEMV, CapCards, CapP2PE, CapRKL, CapTokens, CapAdmin},
	"payshield-9k":  {CapKeys, CapPIN, CapEMV, CapCards, CapAdmin},
	"custom":        {CapKeys, CapPIN, CapEMV, CapCards, CapAdmin},
}

// ModelDescriptions provides display info for each model.
var ModelDescriptions = map[string]ModelInfo{
	"payshield-10k": {
		Name:        "Thales payShield 10000",
		Vendor:      "Thales",
		Description: "Next-generation payment HSM. AES-256, RSA up to 4096-bit, 3DES. EFTSec Ethernet, 4-byte header. Up to 10,000 RSA 2048 sign/sec. Dual-LMK, FIPS 140-2 L3 / PCI HSM v3.",
		MaxRSATPS:   10000,
		HeaderLen:   4,
		Capabilities: ModelCapabilities["payshield-10k"],
	},
	"payshield-9k": {
		Name:        "Thales payShield 9000",
		Vendor:      "Thales",
		Description: "Legacy payment HSM. 3DES, AES-256, RSA up to 4096-bit. 2-byte header. Up to 1,500 RSA 2048 sign/sec. Single LMK, FIPS 140-2 L3 / PCI HSM v2.",
		MaxRSATPS:   1500,
		HeaderLen:   2,
		Capabilities: ModelCapabilities["payshield-9k"],
	},
}

type ModelInfo struct {
	Name         string   `json:"name"`
	Vendor       string   `json:"vendor"`
	Description  string   `json:"description"`
	MaxRSATPS    int      `json:"maxRsaTps"`
	HeaderLen    int      `json:"headerLen"`
	Capabilities []string `json:"capabilities"`
}

// GetDefaultCapabilities returns the default capability set for a model.
func GetDefaultCapabilities(model string) []string {
	if caps, ok := ModelCapabilities[model]; ok {
		return caps
	}
	return ModelCapabilities["custom"]
}

// HasCapability checks if a set of capabilities includes the given capability.
func HasCapability(capabilities []string, cap string) bool {
	for _, c := range capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// APIOperationCapability maps an API operation path to the required capability.
func APIOperationCapability(operation string) string {
	switch {
	case len(operation) >= 5 && operation[:5] == "/keys":
		return CapKeys
	case len(operation) >= 4 && operation[:4] == "/pin":
		return CapPIN
	case len(operation) >= 4 && operation[:4] == "/emv":
		return CapEMV
	case len(operation) >= 6 && operation[:6] == "/cards":
		return CapCards
	case len(operation) >= 5 && operation[:5] == "/p2pe":
		return CapP2PE
	case len(operation) >= 4 && operation[:4] == "/rkl":
		return CapRKL
	case len(operation) >= 7 && operation[:7] == "/tokens":
		return CapTokens
	default:
		return CapAdmin
	}
}
