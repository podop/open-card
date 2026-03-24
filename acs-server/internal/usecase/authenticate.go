package usecase

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/repository"
	"github.com/your-org/acs-server/internal/domain/service"
	"go.uber.org/zap"
)

// NetworkPolicy holds per-network flow-control settings for the authenticate use case.
type NetworkPolicy struct {
	// AllowFrictionless controls whether frictionless authentication is permitted.
	// When false, every transaction on this network is always challenged regardless
	// of the risk engine result.
	AllowFrictionless bool
}

// NetworkPolicyStore provides thread-safe access to network policies that can be
// mutated at runtime (e.g. via the admin API). Implementations must be safe for
// concurrent use.
type NetworkPolicyStore interface {
	Get(network entity.Network) (NetworkPolicy, bool)
	Set(network entity.Network, policy NetworkPolicy)
}

// InMemoryPolicyStore is a thread-safe in-memory implementation of NetworkPolicyStore.
type InMemoryPolicyStore struct {
	mu       sync.RWMutex
	policies map[entity.Network]NetworkPolicy
}

// NewInMemoryPolicyStore creates a new store pre-populated with the given policies.
func NewInMemoryPolicyStore(initial map[entity.Network]NetworkPolicy) *InMemoryPolicyStore {
	cp := make(map[entity.Network]NetworkPolicy, len(initial))
	for k, v := range initial {
		cp[k] = v
	}
	return &InMemoryPolicyStore{policies: cp}
}

func (s *InMemoryPolicyStore) Get(network entity.Network) (NetworkPolicy, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.policies[network]
	return p, ok
}

func (s *InMemoryPolicyStore) Set(network entity.Network, policy NetworkPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[network] = policy
}

// AuthenticateInput contains the decoded fields from an AReq message.
type AuthenticateInput struct {
	// Protocol
	MessageVersion  string
	DeviceChannel   entity.DeviceChannel
	MessageCategory entity.MessageCategory

	// Transaction identifiers from DS/3DS Server
	DSTransactionID     uuid.UUID
	ServerTransactionID uuid.UUID
	SDKTransactionID    *uuid.UUID // nil for browser

	// Card
	AcctNumber string

	// Merchant
	MerchantName    string
	MerchantCountry string
	MerchantID      string

	// Purchase
	PurchaseAmount   *string
	PurchaseCurrency *string
	PurchaseExponent *int

	// threeDSRequestorChallengeInd: "04" = challenge mandate
	ThreeDSRequestorChallengeInd string
	// ThreeDSRequestorDecReqInd: "Y" = decoupled auth requested (OOB flow)
	ThreeDSRequestorDecReqInd string

	// Browser / device info (from 3DS Method, DeviceChannel=02)
	BrowserInfo *service.BrowserInfo
	// SDKDeviceInfo carries decrypted app device data (DeviceChannel=01).
	SDKDeviceInfo *service.SDKDeviceInfo
	// SDKEphemPubKey is the SDK ephemeral public key (JWK) for challenge encryption (DeviceChannel=01).
	SDKEphemPubKey        *string
	ThreeDSMethodNotifURL string
}

// AuthenticateOutput maps to the ARes message fields.
type AuthenticateOutput struct {
	ACSTransactionID uuid.UUID
	TransStatus      entity.TransStatus
	// TransStatusReason is required in ARes when TransStatus is N, U, or R (spec Table A.9).
	TransStatusReason *string
	AuthValue        *string
	ECI              *string
	Network          entity.Network
	// Challenge fields (populated when TransStatus == "C")
	ACSChallengeURL *string
	ACSURL          *string // 3DS Method URL for next browser call
}

// FlowSettingsResolver looks up per-BIN flow settings (from network rules + exclusions).
type FlowSettingsResolver interface {
	ResolveFlowSettings(ctx context.Context, network entity.Network, pan string) (*entity.FlowSettings, error)
}

// NetworkLookup provides network metadata from the DB.
type NetworkLookup interface {
	GetByName(ctx context.Context, name string) (*entity.NetworkEntry, error)
}

// AuthenticateUseCase handles the AReq → ARes flow.
type AuthenticateUseCase struct {
	txRepo        repository.TransactionRepository
	rangeRepo     repository.CardRangeRepository
	sessionRepo   repository.SessionRepository
	riskEngine    service.RiskEngine
	otpSvc        *service.OTPService
	authValSvc    service.AuthValueService
	auditLog      service.AuditLogger
	acsBaseURL    string
	policyStore   NetworkPolicyStore
	flowResolver  FlowSettingsResolver
	networkLookup NetworkLookup
	otpResolver   service.OTPChannelResolver
	oobResolver   service.OOBChannelResolver
	mfaLookup     MFAProviderLookup
	log           *zap.Logger
}

// NewAuthenticateUseCase wires up the use case.
func NewAuthenticateUseCase(
	txRepo repository.TransactionRepository,
	rangeRepo repository.CardRangeRepository,
	sessionRepo repository.SessionRepository,
	riskEngine service.RiskEngine,
	otpSvc *service.OTPService,
	authValSvc service.AuthValueService,
	auditLog service.AuditLogger,
	acsBaseURL string,
	policyStore NetworkPolicyStore,
	flowResolver FlowSettingsResolver,
	networkLookup NetworkLookup,
	otpResolver service.OTPChannelResolver,
	oobResolver service.OOBChannelResolver,
	mfaLookup MFAProviderLookup,
	log *zap.Logger,
) *AuthenticateUseCase {
	return &AuthenticateUseCase{
		txRepo:        txRepo,
		rangeRepo:     rangeRepo,
		sessionRepo:   sessionRepo,
		riskEngine:    riskEngine,
		otpSvc:        otpSvc,
		authValSvc:    authValSvc,
		auditLog:      auditLog,
		acsBaseURL:    acsBaseURL,
		policyStore:   policyStore,
		flowResolver:  flowResolver,
		networkLookup: networkLookup,
		otpResolver:   otpResolver,
		oobResolver:   oobResolver,
		mfaLookup:     mfaLookup,
		log:           log,
	}
}

// Execute processes an AReq and returns an ARes payload.
func (uc *AuthenticateUseCase) Execute(ctx context.Context, in *AuthenticateInput) (*AuthenticateOutput, error) {
	// 1. Parse spec version
	specVer := entity.SpecVersion(in.MessageVersion)
	if specVer != entity.SpecVersion220 && specVer != entity.SpecVersion231 {
		return nil, fmt.Errorf("unsupported protocol version: %s", in.MessageVersion)
	}

	// 2. Check card range enrollment
	cardRange, err := uc.rangeRepo.FindByPAN(ctx, in.AcctNumber)
	if err != nil {
		return nil, fmt.Errorf("card range lookup: %w", err)
	}
	if cardRange == nil || !cardRange.SupportsVersion(specVer) {
		// Card not enrolled → U status (authentication unavailable)
		reason := "13" // Not Authenticated — Returned in ARes (cardholder not enrolled)
		return &AuthenticateOutput{
			ACSTransactionID:  uuid.New(),
			TransStatus:       entity.TransStatusUnavailable,
			TransStatusReason: &reason,
		}, nil
	}

	// 3. Build transaction aggregate
	tx := entity.NewTransaction(specVer, in.DeviceChannel, in.MessageCategory,
		in.DSTransactionID, in.ServerTransactionID)
	tx.SDKTransactionID = in.SDKTransactionID
	tx.AcctNumber = in.AcctNumber // stored encrypted by repo layer
	tx.CardBIN = safePrefix(in.AcctNumber, 6)
	tx.Network = cardRange.Network
	tx.MerchantName = in.MerchantName
	tx.MerchantCountry = in.MerchantCountry
	tx.MerchantID = in.MerchantID
	tx.PurchaseAmount = in.PurchaseAmount
	tx.PurchaseCurrency = in.PurchaseCurrency
	tx.PurchaseExponent = in.PurchaseExponent

	uc.log.Info("transaction network resolved",
		zap.String("network", string(tx.Network)),
		zap.String("acs_trans_id", tx.ACSTransactionID.String()),
	)

	// 3b. Check if network is active
	if uc.networkLookup != nil {
		netEntry, netErr := uc.networkLookup.GetByName(ctx, string(tx.Network))
		if netErr != nil {
			uc.log.Warn("network lookup error", zap.Error(netErr))
		} else if netEntry != nil && !netEntry.Active {
			reason := "13"
			uc.log.Info("network inactive, declining transaction",
				zap.String("network", string(tx.Network)),
				zap.String("acs_trans_id", tx.ACSTransactionID.String()),
			)
			return &AuthenticateOutput{
				ACSTransactionID:  tx.ACSTransactionID,
				TransStatus:       entity.TransStatusNotAuthenticated,
				TransStatusReason: &reason,
			}, nil
		}
	}

	// 4. Resolve BIN-level flow settings (network rule + exclusion override).
	var flow *entity.FlowSettings
	if uc.flowResolver != nil {
		flow, err = uc.flowResolver.ResolveFlowSettings(ctx, tx.Network, in.AcctNumber)
		if err != nil {
			uc.log.Warn("flow settings resolver error, using defaults", zap.Error(err))
		}
	}
	if flow == nil {
		// Fallback: derive from legacy policy store
		pol, ok := uc.policyStore.Get(tx.Network)
		allowFrictionless := true // permissive default when no policy exists
		if ok {
			allowFrictionless = pol.AllowFrictionless
		}
		flow = &entity.FlowSettings{
			AllowFrictionless:       allowFrictionless,
			AllowOTP:                true,
			AllowOOB:                true,
			DefaultChallengeChannel: "otp",
			Source:                  "legacy_policy",
		}
	}

	uc.log.Info("flow settings resolved",
		zap.String("source", flow.Source),
		zap.Bool("allowFrictionless", flow.AllowFrictionless),
		zap.Bool("allowOTP", flow.AllowOTP),
		zap.Bool("allowOOB", flow.AllowOOB),
		zap.String("defaultChannel", flow.DefaultChallengeChannel),
	)

	// Determine decoupled mode: requestor asks for OOB, or BIN rule defaults to OOB.
	decoupled := in.ThreeDSRequestorDecReqInd == "Y"
	if !decoupled && flow.DefaultChallengeChannel == "oob" && flow.AllowOOB {
		decoupled = true
	}
	// If requestor asks OOB but BIN rule disallows it, fall back to OTP.
	if decoupled && !flow.AllowOOB {
		uc.log.Info("OOB requested but disallowed by BIN rule, falling back to OTP")
		decoupled = false
	}
	// If OTP challenge but BIN rule disallows OTP, force OOB if allowed.
	if !decoupled && !flow.AllowOTP && flow.AllowOOB {
		uc.log.Info("OTP disallowed by BIN rule, switching to OOB")
		decoupled = true
	}

	// 5a. Requestor challenge mandate: "04" = challenge requested (mandate).
	if in.ThreeDSRequestorChallengeInd == "04" || in.ThreeDSRequestorChallengeInd == "03" {
		uc.log.Info("requestor challenge mandate", zap.String("challengeInd", in.ThreeDSRequestorChallengeInd))
		resolvedDecoupled, unavailable := uc.resolveChannel(ctx, decoupled, flow, in.AcctNumber, tx.Network)
		if unavailable {
			return uc.handleChannelUnavailable(ctx, tx, in.ThreeDSMethodNotifURL)
		}
		return uc.handleChallenge(ctx, tx, in.ThreeDSMethodNotifURL, resolvedDecoupled, flow)
	}

	// 5b. BIN/Network rule forces challenge (frictionless not allowed).
	if !flow.AllowFrictionless {
		uc.log.Info("frictionless disallowed by BIN/network rule", zap.String("source", flow.Source))
		resolvedDecoupled, unavailable := uc.resolveChannel(ctx, decoupled, flow, in.AcctNumber, tx.Network)
		if unavailable {
			return uc.handleChannelUnavailable(ctx, tx, in.ThreeDSMethodNotifURL)
		}
		return uc.handleChallenge(ctx, tx, in.ThreeDSMethodNotifURL, resolvedDecoupled, flow)
	}

	// 6. Risk evaluation
	risk, err := uc.riskEngine.Evaluate(ctx, tx, in.BrowserInfo, in.SDKDeviceInfo)
	if err != nil {
		uc.log.Warn("risk engine error, defaulting to challenge", zap.Error(err))
		risk = &service.RiskScore{Frictionless: false, Reason: "risk_engine_error"}
	}

	uc.log.Info("risk evaluation",
		zap.Int("score", risk.Score),
		zap.Bool("frictionless", risk.Frictionless),
		zap.String("reason", risk.Reason),
	)

	// 7. Decide authentication path
	if risk.Frictionless {
		return uc.handleFrictionless(ctx, tx)
	}
	resolvedDecoupled, unavailable := uc.resolveChannel(ctx, decoupled, flow, in.AcctNumber, tx.Network)
	if unavailable {
		return uc.handleChannelUnavailable(ctx, tx, in.ThreeDSMethodNotifURL)
	}
	return uc.handleChallenge(ctx, tx, in.ThreeDSMethodNotifURL, resolvedDecoupled, flow)
}

// handleFrictionless completes authentication without cardholder interaction.
func (uc *AuthenticateUseCase) handleFrictionless(ctx context.Context, tx *entity.Transaction) (*AuthenticateOutput, error) {
	authValue, eci, err := uc.authValSvc.Generate(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("auth value generation: %w", err)
	}
	tx.Complete(entity.TransStatusAuthenticated, authValue, eci)

	if err := uc.txRepo.Save(ctx, tx); err != nil {
		return nil, fmt.Errorf("save transaction: %w", err)
	}

	_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventFrictionless, nil))

	return &AuthenticateOutput{
		ACSTransactionID: tx.ACSTransactionID,
		TransStatus:      tx.TransStatus,
		AuthValue:        tx.AuthValue,
		ECI:              tx.ECI,
		Network:          tx.Network,
	}, nil
}

// handleChallenge persists the transaction, creates a challenge session, and
// returns the ACS challenge URL for the 3DS Server to redirect the cardholder.
func (uc *AuthenticateUseCase) handleChallenge(ctx context.Context, tx *entity.Transaction, notifURL string, decoupled bool, flow ...*entity.FlowSettings) (*AuthenticateOutput, error) {
	if decoupled {
		tx.TransStatus = entity.TransStatusDecoupled
	} else {
		tx.TransStatus = entity.TransStatusChallenge
	}

	// Check if second factor is required based on flow settings.
	if len(flow) > 0 && flow[0] != nil {
		f := flow[0]
		if f.SecondFactorMode != "" && f.SecondFactorMode != "none" && f.MFAProviderID != nil {
			mode := f.SecondFactorMode
			needMFA := mode == "both" ||
				(mode == "otp" && !decoupled) ||
				(mode == "oob" && decoupled)
			if needMFA {
				// Verify the MFA provider is active before enabling second factor.
				providerActive := false
				if uc.mfaLookup != nil {
					if prov, err := uc.mfaLookup.GetActiveByID(ctx, *f.MFAProviderID); err == nil && prov != nil {
						providerActive = true
					}
				}
				if providerActive {
					tx.SecondFactorRequired = true
					uc.log.Info("second factor will be required after first factor",
						zap.String("mode", mode),
						zap.Int64("mfaProviderId", *f.MFAProviderID),
					)
				} else {
					uc.log.Info("second factor skipped — MFA provider not active",
						zap.Int64("mfaProviderId", *f.MFAProviderID),
					)
				}
			}
		}
	}

	auditDetail := map[string]any{}

	if !decoupled {
		// OTP-based challenge — pre-generate OTP and store hash
		otpPlain, otpHash, otpExpiry, err := uc.otpSvc.Generate()
		if err != nil {
			return nil, fmt.Errorf("otp generate: %w", err)
		}
		uc.log.Debug("otp generated (dev only)", zap.String("otp", otpPlain), zap.String("acs_trans_id", tx.ACSTransactionID.String()))
		tx.OTPHash = otpHash
		tx.OTPExpiry = &otpExpiry
		auditDetail["otp"] = otpPlain
	} else {
		uc.log.Info("OOB/decoupled challenge initiated (no OTP)", zap.String("acs_trans_id", tx.ACSTransactionID.String()))
		auditDetail["method"] = "oob"
	}

	if err := uc.txRepo.Save(ctx, tx); err != nil {
		return nil, fmt.Errorf("save transaction: %w", err)
	}

	// Create challenge session
	session := entity.NewChallengeSession(tx.ACSTransactionID)
	session.NotifURL = notifURL
	if err := uc.sessionRepo.Save(ctx, session); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventChallengeStart, auditDetail))

	challengeURL := fmt.Sprintf("%s/v1/challenge/%s", uc.acsBaseURL, tx.ACSTransactionID)
	if decoupled {
		challengeURL += "?flow=oob"
	}
	return &AuthenticateOutput{
		ACSTransactionID: tx.ACSTransactionID,
		TransStatus:      tx.TransStatus,
		Network:          tx.Network,
		ACSChallengeURL:  &challengeURL,
	}, nil
}

// resolveChannel checks whether the preferred challenge channel is reachable
// and falls back to the alternative if not. Returns the resolved decoupled
// flag and whether both channels are unavailable.
func (uc *AuthenticateUseCase) resolveChannel(ctx context.Context, preferOOB bool, flow *entity.FlowSettings, pan string, network entity.Network) (decoupled bool, unavailable bool) {
	if preferOOB {
		// Prefer OOB → fallback OTP
		if uc.oobResolver != nil {
			avail, err := uc.oobResolver.CheckOOBAvailable(ctx, pan, network)
			if err == nil && !avail.Available {
				uc.log.Info("OOB channel unavailable, trying OTP fallback", zap.String("reason", avail.Reason))
				// Try OTP fallback
				if uc.otpResolver != nil {
					otpAvail, otpErr := uc.otpResolver.CheckOTPAvailable(ctx, pan, network)
					if otpErr == nil && !otpAvail.Available {
						uc.log.Warn("both OOB and OTP channels unavailable")
						return false, true
					}
				}
				return false, false // fallback to OTP
			}
		}
		return true, false // OOB available (or resolver nil = assume available)
	}

	// Prefer OTP → fallback OOB
	if uc.otpResolver != nil {
		avail, err := uc.otpResolver.CheckOTPAvailable(ctx, pan, network)
		if err == nil && !avail.Available {
			uc.log.Info("OTP channel unavailable, trying OOB fallback", zap.String("reason", avail.Reason))
			// Try OOB fallback (only if flow allows OOB)
			if flow.AllowOOB && uc.oobResolver != nil {
				oobAvail, oobErr := uc.oobResolver.CheckOOBAvailable(ctx, pan, network)
				if oobErr == nil && !oobAvail.Available {
					uc.log.Warn("both OTP and OOB channels unavailable")
					return false, true
				}
				return true, false // fallback to OOB
			}
			// OOB not allowed or no resolver — check if OOB is at least an option
			if !flow.AllowOOB {
				uc.log.Warn("OTP unavailable and OOB disallowed by flow settings")
				return false, true
			}
			return true, false // fallback to OOB (no resolver = assume available)
		}
	}
	return false, false // OTP available (or resolver nil = assume available)
}

// handleChannelUnavailable saves the transaction with TransStatus=U and returns
// a challenge URL with ?flow=fallback so the handler renders the fallback page.
func (uc *AuthenticateUseCase) handleChannelUnavailable(ctx context.Context, tx *entity.Transaction, notifURL string) (*AuthenticateOutput, error) {
	tx.TransStatus = entity.TransStatusUnavailable
	now := func() *time.Time { t := time.Now().UTC(); return &t }()
	tx.CompletedAt = now
	tx.UpdatedAt = *now

	if err := uc.txRepo.Save(ctx, tx); err != nil {
		return nil, fmt.Errorf("save transaction: %w", err)
	}

	// Create session so fallback page can retrieve notifURL
	session := entity.NewChallengeSession(tx.ACSTransactionID)
	session.NotifURL = notifURL
	if err := uc.sessionRepo.Save(ctx, session); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventUnavailable,
		map[string]any{"reason": "channel_unavailable"}))

	challengeURL := fmt.Sprintf("%s/v1/challenge/%s?flow=fallback", uc.acsBaseURL, tx.ACSTransactionID)
	return &AuthenticateOutput{
		ACSTransactionID: tx.ACSTransactionID,
		TransStatus:      entity.TransStatusUnavailable,
		Network:          tx.Network,
		ACSChallengeURL:  &challengeURL,
	}, nil
}

func safePrefix(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}