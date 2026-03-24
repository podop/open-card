---
name: acs-risk-engine
description: >
  Use this skill whenever a developer on the acs-server project wants to replace the
  DefaultRiskEngine stub with a real risk scoring implementation, or extend the risk
  engine with new signals, rules, or third-party integrations. Trigger this for requests
  like "add real risk scoring", "integrate a fraud engine", "add velocity checks",
  "connect to [vendor] risk API", "add device fingerprint scoring", "implement risk rules",
  "tune the frictionless rate", "add ML risk scoring", "replace the default risk engine",
  "add BIN-level risk config", or "make the risk engine production-ready".
  Also trigger when someone asks to change the OTP/challenge trigger threshold, add
  exemptions (low-value, whitelisting, trusted beneficiary), or implement PSD2 SCA rules.
---

# ACS Risk Engine Skill

This skill guides you through building and integrating a production-quality `RiskEngine`
into the `acs-server`, starting from the `DefaultRiskEngine` stub.

## The Interface You're Implementing

Located at `internal/domain/service/risk_engine.go`:

```go
type RiskEngine interface {
    Evaluate(ctx context.Context, tx *entity.Transaction, browserInfo *BrowserInfo) (*RiskScore, error)
}

type RiskScore struct {
    Score        int    // 0–100; lower = lower risk
    Frictionless bool   // true = skip challenge
    Reason       string // machine-readable label for audit/metrics
}

type BrowserInfo struct {
    IPAddress         string
    UserAgent         string
    Language          string
    ColorDepth        int
    ScreenHeight      int
    ScreenWidth       int
    TimeZoneOffset    int
    JavaEnabled       bool
    JavaScriptEnabled bool
}
```

The `Evaluate` method receives the full `Transaction` aggregate (with merchant, amount,
card BIN, device channel, spec version) plus browser device info. It returns a score and
a frictionless decision. The use case in `authenticate.go` trusts this decision completely.

---

## Architecture: Where Risk Logic Lives

```
domain/service/risk_engine.go    ← interface + BrowserInfo + RiskScore (never change this)
domain/service/risk_rules.go     ← your pure rule implementations (no I/O)
infrastructure/risk/             ← vendor HTTP clients, config loaders, caching
adapter/repository/postgres/     ← risk config repo if rules are DB-driven
```

Keep I/O out of domain logic. Pure Go rule functions are fast, testable, and have zero deps.
Vendor API calls go in `infrastructure/risk/`.

---

## Implementation Approaches

### Approach A: Rule-Based Engine (recommended starting point)

Best for: teams that understand their fraud patterns, want full control, low latency.

Create `internal/domain/service/risk_rules.go`:

```go
package service

import (
    "github.com/your-org/acs-server/internal/domain/entity"
)

// Rule is a single scoring function. It returns a score delta and a reason.
// Positive delta = higher risk. Rules are evaluated in order; first match wins if using
// early-exit, or all rules contribute if using additive scoring.
type Rule func(tx *entity.Transaction, info *BrowserInfo) (scoreDelta int, reason string, matched bool)

// LowValueExemption grants frictionless for transactions under the threshold.
// Adjust threshold per card scheme / regulatory regime (e.g. PSD2 €30 low-value).
func LowValueExemption(thresholdMinorUnits int) Rule {
    return func(tx *entity.Transaction, _ *BrowserInfo) (int, string, bool) {
        if tx.PurchaseAmount == nil {
            return 0, "", false
        }
        // compare numeric minor-unit amount string
        if len(*tx.PurchaseAmount) <= 6 && *tx.PurchaseAmount < padAmount(thresholdMinorUnits) {
            return -50, "low_value_exemption", true
        }
        return 0, "", false
    }
}

// NoDeviceInfo raises risk when the 3DS Method was skipped or returned nothing.
func NoDeviceInfo() Rule {
    return func(_ *entity.Transaction, info *BrowserInfo) (int, string, bool) {
        if info == nil || info.IPAddress == "" {
            return +40, "no_device_info", true
        }
        return 0, "", false
    }
}

// HighRiskMerchantCountry raises risk for specific merchant country codes.
func HighRiskMerchantCountry(countryCodes []string) Rule {
    set := make(map[string]struct{}, len(countryCodes))
    for _, c := range countryCodes {
        set[c] = struct{}{}
    }
    return func(tx *entity.Transaction, _ *BrowserInfo) (int, string, bool) {
        if _, ok := set[tx.MerchantCountry]; ok {
            return +30, "high_risk_merchant_country", true
        }
        return 0, "", false
    }
}
```

Then build the engine that composes rules:

```go
type RuleBasedEngine struct {
    rules     []Rule
    threshold int // score >= threshold → challenge
    log       *zap.Logger
}

func NewRuleBasedEngine(rules []Rule, threshold int, log *zap.Logger) *RuleBasedEngine {
    return &RuleBasedEngine{rules: rules, threshold: threshold, log: log}
}

func (e *RuleBasedEngine) Evaluate(ctx context.Context, tx *entity.Transaction, info *BrowserInfo) (*RiskScore, error) {
    total := 0
    reason := "default"
    for _, rule := range e.rules {
        delta, r, matched := rule(tx, info)
        if matched {
            total += delta
            reason = r
            if total >= e.threshold {
                break // early exit: already high risk
            }
        }
    }
    // clamp to 0–100
    if total < 0 { total = 0 }
    if total > 100 { total = 100 }

    frictionless := total < e.threshold
    e.log.Debug("risk evaluated", zap.Int("score", total), zap.String("reason", reason), zap.Bool("frictionless", frictionless))
    return &RiskScore{Score: total, Frictionless: frictionless, Reason: reason}, nil
}
```

Wire in `main.go`:
```go
rules := []service.Rule{
    service.LowValueExemption(3000),        // < €30.00
    service.NoDeviceInfo(),
    service.HighRiskMerchantCountry([]string{"706", "408"}),
}
riskEngine := service.NewRuleBasedEngine(rules, 40, log)
```

### Approach B: DB-Driven BIN-Level Config

Best for: different risk thresholds per card program or issuer.

Add a `risk_config` table (use the `acs-add-repository` skill to scaffold it properly):

```sql
CREATE TABLE risk_config (
    id              BIGSERIAL PRIMARY KEY,
    card_bin_prefix VARCHAR(8)   NOT NULL UNIQUE,
    low_value_limit INTEGER      NOT NULL DEFAULT 3000, -- minor units
    frictionless_rate NUMERIC(4,3) NOT NULL DEFAULT 0.80, -- target 80% frictionless
    challenge_always BOOLEAN     NOT NULL DEFAULT FALSE,
    active          BOOLEAN      NOT NULL DEFAULT TRUE
);
```

Create `domain/repository/interfaces.go` addition:
```go
type RiskConfigRepository interface {
    FindByBIN(ctx context.Context, bin string) (*entity.RiskConfig, error)
}
```

Then inject this repo into a `ConfigurableRiskEngine`:
```go
type ConfigurableRiskEngine struct {
    configRepo repository.RiskConfigRepository
    fallback    RiskEngine // DefaultRiskEngine for unknown BINs
    log         *zap.Logger
}

func (e *ConfigurableRiskEngine) Evaluate(ctx context.Context, tx *entity.Transaction, info *BrowserInfo) (*RiskScore, error) {
    cfg, err := e.configRepo.FindByBIN(ctx, tx.CardBIN)
    if err != nil || cfg == nil {
        return e.fallback.Evaluate(ctx, tx, info) // safe fallback
    }
    if cfg.ChallengeAlways {
        return &RiskScore{Score: 100, Frictionless: false, Reason: "bin_challenge_always"}, nil
    }
    // apply cfg.LowValueLimit, cfg.FrictionlessRate, etc.
}
```

### Approach C: Third-Party Vendor API

Best for: teams using a vendor fraud platform (e.g. Stripe Radar, Featurespace, Emailage).

Create `internal/infrastructure/risk/vendor_client.go`:

```go
package risk

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type VendorClient struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

func NewVendorClient(baseURL, apiKey string) *VendorClient {
    return &VendorClient{
        baseURL: baseURL,
        apiKey:  apiKey,
        httpClient: &http.Client{Timeout: 500 * time.Millisecond}, // keep latency low
    }
}

type VendorRequest struct {
    TransactionID string  `json:"transaction_id"`
    Amount        *string `json:"amount,omitempty"`
    Currency      *string `json:"currency,omitempty"`
    MerchantID    string  `json:"merchant_id"`
    IPAddress     string  `json:"ip_address"`
    CardBIN       string  `json:"card_bin"`
}

type VendorResponse struct {
    Score      int    `json:"score"`
    Decision   string `json:"decision"` // "allow" | "challenge" | "block"
    ReasonCode string `json:"reason_code"`
}

func (c *VendorClient) Score(ctx context.Context, req VendorRequest) (*VendorResponse, error) {
    // implementation: POST to vendor API, unmarshal response
    // Always set a context deadline — vendor API latency directly adds to 3DS latency
}
```

Then wrap it as a `RiskEngine` in `infrastructure/risk/vendor_engine.go`:

```go
type VendorRiskEngine struct {
    client   *VendorClient
    fallback service.RiskEngine // used if vendor call fails or times out
    log      *zap.Logger
}

func (e *VendorRiskEngine) Evaluate(ctx context.Context, tx *entity.Transaction, info *service.BrowserInfo) (*service.RiskScore, error) {
    req := VendorRequest{ ... }

    // Use a tight deadline — don't let vendor latency blow your 3DS SLA
    vendorCtx, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
    defer cancel()

    resp, err := e.client.Score(vendorCtx, req)
    if err != nil {
        // Graceful degradation: log and fall back to local rules
        e.log.Warn("vendor risk API failed, using fallback", zap.Error(err))
        return e.fallback.Evaluate(ctx, tx, info)
    }

    frictionless := resp.Decision == "allow"
    return &service.RiskScore{
        Score:        resp.Score,
        Frictionless: frictionless,
        Reason:       resp.ReasonCode,
    }, nil
}
```

**Always implement graceful degradation** — if the vendor is down, fall back to your
rule-based engine. Never let a vendor outage cause all transactions to fail.

---

## PSD2 / SCA Exemptions

If operating under PSD2 (European issuers), the following exemption flags map to
frictionless outcomes with specific reason codes. Add these as rules or as a pre-check:

| Exemption | Condition | Reason Code |
|---|---|---|
| Low value | Amount < €30 AND cumulative count/total not exceeded | `low_value_exemption` |
| Trusted beneficiary | Merchant in cardholder's whitelist (from `acsWhitelistStatus=Y`) | `trusted_beneficiary` |
| Recurring (low-risk) | MIT with same amount/merchant as prior authenticated TX | `recurring_transaction` |
| Transaction Risk Analysis (TRA) | Issuer fraud rate below scheme thresholds | `tra_exemption` |

These are frictionless decisions — set `Frictionless: true` and include the reason code.
The reason code flows into the `ECI` and eventually the authorisation request.

---

## Testing Your Risk Engine

Every rule function should have a unit test — they're pure functions with no I/O:

```go
func TestLowValueExemption(t *testing.T) {
    rule := service.LowValueExemption(3000)
    amount := "002500" // €25.00 < threshold
    tx := &entity.Transaction{PurchaseAmount: &amount}

    delta, reason, matched := rule(tx, nil)
    if !matched { t.Error("should match low-value tx") }
    if delta >= 0 { t.Error("should reduce risk score") }
    if reason != "low_value_exemption" { t.Errorf("wrong reason: %s", reason) }
}
```

For the vendor engine, use `httptest.NewServer` to mock the vendor API:

```go
func TestVendorRiskEngine_Fallback(t *testing.T) {
    // simulate vendor timeout
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(600 * time.Millisecond) // exceeds 400ms deadline
    }))
    defer srv.Close()
    // assert fallback engine is called instead
}
```

---

## Checklist

- [ ] New engine implements `service.RiskEngine` interface
- [ ] `Evaluate` never returns an error for "normal" low/high risk — errors are infrastructure failures only
- [ ] Vendor client has a context deadline ≤ 500ms
- [ ] Graceful degradation: vendor failure falls back to rule engine
- [ ] Every rule function has a unit test
- [ ] `Reason` string in `RiskScore` is machine-readable (lowercase_underscore) for audit log
- [ ] Wired in `main.go` replacing `service.NewDefaultRiskEngine`
- [ ] Config (thresholds, vendor URL/key) sourced from `config.yaml` / env vars, not hardcoded
