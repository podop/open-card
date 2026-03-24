package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/repository"
	"github.com/your-org/acs-server/internal/domain/service"
	"go.uber.org/zap"
)

// MFAProviderLookup provides MFA provider data from the DB.
type MFAProviderLookup interface {
	GetActiveByID(ctx context.Context, id int64) (*entity.MFAProvider, error)
}

// ChallengeUseCase orchestrates the OTP challenge interaction.
type ChallengeUseCase struct {
	txRepo      repository.TransactionRepository
	sessionRepo repository.SessionRepository
	otpSvc      *service.OTPService
	pinSvc      *service.PINService
	authValSvc  service.AuthValueService
	auditLog    service.AuditLogger
	mfaLookup   MFAProviderLookup
	log         *zap.Logger
}

// NewChallengeUseCase wires up the challenge use case.
func NewChallengeUseCase(
	txRepo repository.TransactionRepository,
	sessionRepo repository.SessionRepository,
	otpSvc *service.OTPService,
	pinSvc *service.PINService,
	authValSvc service.AuthValueService,
	auditLog service.AuditLogger,
	mfaLookup MFAProviderLookup,
	log *zap.Logger,
) *ChallengeUseCase {
	return &ChallengeUseCase{
		txRepo:      txRepo,
		sessionRepo: sessionRepo,
		otpSvc:      otpSvc,
		pinSvc:      pinSvc,
		authValSvc:  authValSvc,
		auditLog:    auditLog,
		mfaLookup:   mfaLookup,
		log:         log,
	}
}

// ChallengeInfoOutput holds display data for the challenge UI page.
type ChallengeInfoOutput struct {
	ACSTransactionID uuid.UUID
	Network          entity.Network
	MerchantName     string
	PurchaseAmount   *string
	PurchaseCurrency *string
	AttemptsLeft     int
	NotifURL         string // threeDSMethodNotificationURL from the challenge session
	// OTP is returned ONLY in test/dev mode; never in production
	OTPForTesting string
}

// GetChallengeInfo loads transaction and session data for rendering the challenge page.
// It also checks for session timeout and auto-transitions to U if expired.
func (uc *ChallengeUseCase) GetChallengeInfo(ctx context.Context, acsTransID uuid.UUID) (*ChallengeInfoOutput, error) {
	// Check timeout first — if expired, transition to U before rendering.
	if timedOut, err := uc.checkSessionTimeout(ctx, acsTransID); err != nil {
		return nil, fmt.Errorf("timeout check: %w", err)
	} else if timedOut {
		return nil, fmt.Errorf("challenge session expired")
	}

	tx, err := uc.txRepo.FindByACSTransactionID(ctx, acsTransID)
	if err != nil {
		return nil, fmt.Errorf("find transaction: %w", err)
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction not found: %s", acsTransID)
	}
	if !tx.IsChallengeable() {
		return nil, fmt.Errorf("transaction not in challenge state: %s", tx.TransStatus)
	}

	notifURL := ""
	if session, sErr := uc.sessionRepo.FindByTransactionID(ctx, acsTransID, "challenge"); sErr == nil && session != nil {
		notifURL = session.NotifURL
	}

	const maxAttempts = 3
	return &ChallengeInfoOutput{
		ACSTransactionID: tx.ACSTransactionID,
		Network:          tx.Network,
		MerchantName:     tx.MerchantName,
		PurchaseAmount:   tx.PurchaseAmount,
		PurchaseCurrency: tx.PurchaseCurrency,
		AttemptsLeft:     maxAttempts - tx.ChallengeAttempts,
		NotifURL:         notifURL,
	}, nil
}

// TransactionDisplayInfo holds display data for non-challengeable transactions (e.g. fallback).
type TransactionDisplayInfo struct {
	ACSTransactionID    uuid.UUID
	ServerTransactionID uuid.UUID
	SpecVersion         entity.SpecVersion
	Network             entity.Network
	MerchantName        string
	PurchaseAmount      *string
	PurchaseCurrency    *string
	NotifURL            string
}

// GetTransactionDisplayInfo loads a transaction for display purposes without
// requiring it to be in a challengeable state. This is used for the fallback
// page where TransStatus=U.
func (uc *ChallengeUseCase) GetTransactionDisplayInfo(ctx context.Context, acsTransID uuid.UUID) (*TransactionDisplayInfo, error) {
	tx, err := uc.txRepo.FindByACSTransactionID(ctx, acsTransID)
	if err != nil {
		return nil, fmt.Errorf("find transaction: %w", err)
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction not found: %s", acsTransID)
	}

	notifURL := ""
	if session, sErr := uc.sessionRepo.FindByTransactionID(ctx, acsTransID, "challenge"); sErr == nil && session != nil {
		notifURL = session.NotifURL
	}

	return &TransactionDisplayInfo{
		ACSTransactionID:    tx.ACSTransactionID,
		ServerTransactionID: tx.ServerTransactionID,
		SpecVersion:         tx.SpecVersion,
		Network:             tx.Network,
		MerchantName:        tx.MerchantName,
		PurchaseAmount:      tx.PurchaseAmount,
		PurchaseCurrency:    tx.PurchaseCurrency,
		NotifURL:            notifURL,
	}, nil
}

// SubmitChallengeInput contains the cardholder's OTP submission.
type SubmitChallengeInput struct {
	ACSTransactionID uuid.UUID
	SubmittedOTP     string
}

// SubmitChallengeOutput contains the result of OTP verification.
type SubmitChallengeOutput struct {
	Success      bool
	TransStatus  entity.TransStatus
	Network      entity.Network
	AuthValue    *string
	ECI          *string
	AttemptsLeft int
	// ServerTransactionID is always populated; needed for threeDSServerTransID in CRes.
	ServerTransactionID uuid.UUID
	// SpecVersion is always populated; needed for messageVersion in CRes.
	SpecVersion entity.SpecVersion
	// NotifURL is the threeDSMethodNotificationURL; populated on final outcome for CRes delivery.
	NotifURL string
	// RReqPayload is the Result Request to send to the DS (populated on final outcome)
	RReqPayload *RReqPayload
	// NeedSecondFactor is true when OTP succeeded but a PIN challenge is next.
	NeedSecondFactor bool
}

// RReqPayload holds the data fields for the RReq message sent to the DS.
type RReqPayload struct {
	ACSTransactionID    uuid.UUID
	DSTransactionID     uuid.UUID
	ServerTransactionID uuid.UUID
	TransStatus         entity.TransStatus
	AuthValue           *string
	ECI                 *string
	MessageVersion      entity.SpecVersion
	Network             entity.Network // used for per-network client cert selection
}

// SubmitChallenge verifies the OTP and finalises the transaction.
// Checks for session timeout before processing.
func (uc *ChallengeUseCase) SubmitChallenge(ctx context.Context, in *SubmitChallengeInput) (*SubmitChallengeOutput, error) {
	// Check timeout first.
	if timedOut, err := uc.checkSessionTimeout(ctx, in.ACSTransactionID); err != nil {
		return nil, fmt.Errorf("timeout check: %w", err)
	} else if timedOut {
		return nil, fmt.Errorf("challenge session expired")
	}

	tx, err := uc.txRepo.FindByACSTransactionID(ctx, in.ACSTransactionID)
	if err != nil {
		return nil, fmt.Errorf("find transaction: %w", err)
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction not found")
	}
	if !tx.IsChallengeable() {
		return nil, fmt.Errorf("transaction not in challenge state")
	}

	// Load session to retrieve notification URL for CRes delivery.
	notifURL := ""
	if session, sErr := uc.sessionRepo.FindByTransactionID(ctx, in.ACSTransactionID, "challenge"); sErr == nil && session != nil {
		notifURL = session.NotifURL
	}

	// Increment attempt counter
	tx.ChallengeAttempts++
	const maxAttempts = 3
	attemptsLeft := maxAttempts - tx.ChallengeAttempts

	// Verify OTP
	if tx.OTPExpiry == nil {
		return nil, fmt.Errorf("no OTP set for transaction")
	}
	valid := uc.otpSvc.Verify(in.SubmittedOTP, tx.OTPHash, *tx.OTPExpiry)

	if valid {
		// Check if second factor is required
		if tx.SecondFactorRequired && tx.PINHash == "" {
			// First factor passed — transition to PIN phase
			pinPlain, pinHash, pinExpiry, pinErr := uc.pinSvc.Generate()
			if pinErr != nil {
				return nil, fmt.Errorf("pin generate: %w", pinErr)
			}
			tx.PINHash = pinHash
			tx.PINExpiry = &pinExpiry
			uc.log.Debug("PIN generated for second factor (dev only)",
				zap.String("pin", pinPlain),
				zap.String("acs_trans_id", tx.ACSTransactionID.String()),
			)

			if err := uc.txRepo.Update(ctx, tx); err != nil {
				return nil, fmt.Errorf("update transaction: %w", err)
			}

			_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventChallengeSuccess,
				map[string]any{"phase": "first_factor", "pin": pinPlain, "next": "second_factor"}))

			return &SubmitChallengeOutput{
				Success:             true,
				TransStatus:         entity.TransStatusChallenge,
				Network:             tx.Network,
				AttemptsLeft:        3, // fresh PIN attempts
				ServerTransactionID: tx.ServerTransactionID,
				SpecVersion:         tx.SpecVersion,
				NeedSecondFactor:    true,
			}, nil
		}

		// Authentication succeeded (no second factor or already done)
		authValue, eci, err := uc.authValSvc.Generate(ctx, tx)
		if err != nil {
			return nil, fmt.Errorf("auth value generation: %w", err)
		}
		tx.Complete(entity.TransStatusAuthenticated, authValue, eci)

		if err := uc.txRepo.Update(ctx, tx); err != nil {
			return nil, fmt.Errorf("update transaction: %w", err)
		}

		uc.log.Info("challenge succeeded",
			zap.String("acsTransID", tx.ACSTransactionID.String()),
			zap.String("network", string(tx.Network)),
		)
		_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventChallengeSuccess, nil))

		return &SubmitChallengeOutput{
			Success:             true,
			TransStatus:         tx.TransStatus,
			Network:             tx.Network,
			AuthValue:           tx.AuthValue,
			ECI:                 tx.ECI,
			ServerTransactionID: tx.ServerTransactionID,
			SpecVersion:         tx.SpecVersion,
			NotifURL:            notifURL,
			RReqPayload:         uc.buildRReq(tx),
		}, nil
	}

	// OTP invalid
	if uc.otpSvc.MaxAttemptsExceeded(tx.ChallengeAttempts) {
		tx.Complete(entity.TransStatusNotAuthenticated, "", tx.Network.DefaultECIs().NotAuthenticated)
		if err := uc.txRepo.Update(ctx, tx); err != nil {
			return nil, fmt.Errorf("update transaction: %w", err)
		}
		uc.log.Warn("challenge failed: max attempts exceeded",
			zap.String("acsTransID", tx.ACSTransactionID.String()),
		)
		_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventChallengeFail,
			map[string]any{"reason": "max_attempts_exceeded"},
		))
		return &SubmitChallengeOutput{
			Success:             false,
			TransStatus:         tx.TransStatus,
			Network:             tx.Network,
			AttemptsLeft:        0,
			ServerTransactionID: tx.ServerTransactionID,
			SpecVersion:         tx.SpecVersion,
			NotifURL:            notifURL,
			RReqPayload:         uc.buildRReq(tx),
		}, nil
	}

	// Still has attempts remaining — rotate OTP
	newOtpPlain, newHash, newExpiry, err := uc.otpSvc.Generate()
	if err != nil {
		return nil, fmt.Errorf("rotate otp: %w", err)
	}
	tx.OTPHash = newHash
	tx.OTPExpiry = &newExpiry
	if err := uc.txRepo.Update(ctx, tx); err != nil {
		return nil, fmt.Errorf("update transaction: %w", err)
	}

	uc.log.Warn("challenge OTP incorrect",
		zap.Int("attemptsLeft", attemptsLeft),
		zap.String("acsTransID", tx.ACSTransactionID.String()),
	)
	_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventChallengeFail,
		map[string]any{"reason": "otp_incorrect", "attempts_left": attemptsLeft, "new_otp": newOtpPlain},
	))

	return &SubmitChallengeOutput{
		Success:             false,
		TransStatus:         entity.TransStatusChallenge,
		Network:             tx.Network,
		AttemptsLeft:        attemptsLeft,
		ServerTransactionID: tx.ServerTransactionID,
		SpecVersion:         tx.SpecVersion,
	}, nil
}

func (uc *ChallengeUseCase) buildRReq(tx *entity.Transaction) *RReqPayload {
	return &RReqPayload{
		ACSTransactionID:    tx.ACSTransactionID,
		DSTransactionID:     tx.DSTransactionID,
		ServerTransactionID: tx.ServerTransactionID,
		TransStatus:         tx.TransStatus,
		AuthValue:           tx.AuthValue,
		ECI:                 tx.ECI,
		MessageVersion:      tx.SpecVersion,
		Network:             tx.Network,
	}
}

// ────────────────────────────────────────────────────────────────
// Cancel Challenge (per EMV 3DS spec §5.4.2 — cardholder cancellation)
// ────────────────────────────────────────────────────────────────

// CancelChallengeOutput holds the result of a challenge cancellation.
type CancelChallengeOutput struct {
	TransStatus         entity.TransStatus
	ServerTransactionID uuid.UUID
	SpecVersion         entity.SpecVersion
	NotifURL            string
	RReqPayload         *RReqPayload
}

// CancelChallenge cancels an in-progress challenge.
// Per spec: transStatus = N, challengeCancel = 01 (cardholder selected Cancel).
func (uc *ChallengeUseCase) CancelChallenge(ctx context.Context, acsTransID uuid.UUID) (*CancelChallengeOutput, error) {
	tx, err := uc.txRepo.FindByACSTransactionID(ctx, acsTransID)
	if err != nil {
		return nil, fmt.Errorf("find transaction: %w", err)
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction not found")
	}
	if !tx.IsChallengeable() {
		return nil, fmt.Errorf("transaction not in challenge state")
	}

	notifURL := ""
	if session, sErr := uc.sessionRepo.FindByTransactionID(ctx, acsTransID, "challenge"); sErr == nil && session != nil {
		notifURL = session.NotifURL
	}

	tx.Complete(entity.TransStatusNotAuthenticated, "", tx.Network.DefaultECIs().NotAuthenticated)
	if err := uc.txRepo.Update(ctx, tx); err != nil {
		return nil, fmt.Errorf("update transaction: %w", err)
	}

	uc.log.Info("challenge cancelled by cardholder",
		zap.String("acsTransID", tx.ACSTransactionID.String()),
	)
	_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventChallengeCancel,
		map[string]any{"reason": "cardholder_cancel", "challengeCancel": "01"},
	))

	return &CancelChallengeOutput{
		TransStatus:         tx.TransStatus,
		ServerTransactionID: tx.ServerTransactionID,
		SpecVersion:         tx.SpecVersion,
		NotifURL:            notifURL,
		RReqPayload:         uc.buildRReq(tx),
	}, nil
}

// ────────────────────────────────────────────────────────────────
// Challenge Timeout (per EMV 3DS spec §5.4.3 — ACS timeout)
// ────────────────────────────────────────────────────────────────

// TimeoutChallengeOutput holds the result of a timed-out challenge.
type TimeoutChallengeOutput struct {
	TransStatus         entity.TransStatus
	ServerTransactionID uuid.UUID
	SpecVersion         entity.SpecVersion
	NotifURL            string
	RReqPayload         *RReqPayload
}

// TimeoutChallenge transitions a challenge to U (unavailable) due to expiry.
// Called when the session TTL has elapsed without successful OTP verification.
// Accepts both C (challenge) and D (decoupled/OOB) states.
func (uc *ChallengeUseCase) TimeoutChallenge(ctx context.Context, acsTransID uuid.UUID) (*TimeoutChallengeOutput, error) {
	tx, err := uc.txRepo.FindByACSTransactionID(ctx, acsTransID)
	if err != nil {
		return nil, fmt.Errorf("find transaction: %w", err)
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction not found")
	}
	if tx.TransStatus != entity.TransStatusChallenge && tx.TransStatus != entity.TransStatusDecoupled {
		return nil, fmt.Errorf("transaction not in challenge/decoupled state: %s", tx.TransStatus)
	}

	notifURL := ""
	if session, sErr := uc.sessionRepo.FindByTransactionID(ctx, acsTransID, "challenge"); sErr == nil && session != nil {
		notifURL = session.NotifURL
	}

	// Per spec: transStatus = U when ACS cannot perform authentication (timeout).
	tx.Complete(entity.TransStatusUnavailable, "", "")
	if err := uc.txRepo.Update(ctx, tx); err != nil {
		return nil, fmt.Errorf("update transaction: %w", err)
	}

	uc.log.Warn("challenge timed out",
		zap.String("acsTransID", tx.ACSTransactionID.String()),
	)
	_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventChallengeTimeout,
		map[string]any{"reason": "session_expired"},
	))

	return &TimeoutChallengeOutput{
		TransStatus:         tx.TransStatus,
		ServerTransactionID: tx.ServerTransactionID,
		SpecVersion:         tx.SpecVersion,
		NotifURL:            notifURL,
		RReqPayload:         uc.buildRReq(tx),
	}, nil
}

// checkSessionTimeout checks if the challenge session has expired and, if so,
// transitions the transaction to U. Returns true if the session was timed out.
func (uc *ChallengeUseCase) checkSessionTimeout(ctx context.Context, acsTransID uuid.UUID) (timedOut bool, err error) {
	session, err := uc.sessionRepo.FindByTransactionID(ctx, acsTransID, "challenge")
	if err != nil || session == nil {
		return false, nil
	}
	if !session.IsExpired() {
		return false, nil
	}
	// Session expired — auto-timeout the challenge.
	_, err = uc.TimeoutChallenge(ctx, acsTransID)
	if err != nil {
		return false, err
	}
	_ = uc.sessionRepo.UpdateStatus(ctx, session.ID, entity.SessionStatusExpired)
	return true, nil
}

// ────────────────────────────────────────────────────────────────
// OOB (Out-of-Band) Challenge Support
// Per EMV 3DS spec §5.5 — Decoupled Authentication
// ────────────────────────────────────────────────────────────────

// OOBChallengeOutput holds the result of initiating an OOB challenge.
type OOBChallengeOutput struct {
	ACSTransactionID    uuid.UUID
	ServerTransactionID uuid.UUID
	SpecVersion         entity.SpecVersion
	// OOBAppURL is the deep-link or push URL the cardholder should open.
	OOBAppURL string
	// OOBContinueLabel is optional UI text ("Open your banking app to approve").
	OOBContinueLabel string
}

// InitiateOOBChallenge starts an OOB/decoupled authentication flow.
// The ACS sends a push notification (simulated here) and the cardholder
// authenticates on a separate channel (e.g. banking app).
func (uc *ChallengeUseCase) InitiateOOBChallenge(ctx context.Context, acsTransID uuid.UUID) (*OOBChallengeOutput, error) {
	tx, err := uc.txRepo.FindByACSTransactionID(ctx, acsTransID)
	if err != nil {
		return nil, fmt.Errorf("find transaction: %w", err)
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction not found")
	}
	if !tx.IsChallengeable() {
		return nil, fmt.Errorf("transaction not in challenge state")
	}

	// Transition to decoupled state.
	tx.TransStatus = entity.TransStatusDecoupled
	tx.UpdatedAt = time.Now().UTC()
	if err := uc.txRepo.Update(ctx, tx); err != nil {
		return nil, fmt.Errorf("update transaction: %w", err)
	}

	uc.log.Info("OOB challenge initiated",
		zap.String("acsTransID", tx.ACSTransactionID.String()),
	)
	_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventDecoupledStart, nil))

	return &OOBChallengeOutput{
		ACSTransactionID:    tx.ACSTransactionID,
		ServerTransactionID: tx.ServerTransactionID,
		SpecVersion:         tx.SpecVersion,
		OOBAppURL:           fmt.Sprintf("bankapp://3ds/approve?txn=%s", tx.ACSTransactionID),
		OOBContinueLabel:    "Open your banking app to approve this transaction",
	}, nil
}

// OOBResultInput carries the result from the OOB channel (e.g. banking app callback).
type OOBResultInput struct {
	ACSTransactionID uuid.UUID
	// Approved is true if the cardholder approved the transaction in the OOB channel.
	Approved bool
}

// OOBResultOutput holds the final outcome after OOB authentication.
type OOBResultOutput struct {
	TransStatus         entity.TransStatus
	ServerTransactionID uuid.UUID
	SpecVersion         entity.SpecVersion
	NotifURL            string
	RReqPayload         *RReqPayload
}

// CompleteOOBChallenge finalises a decoupled/OOB authentication.
// Called when the OOB channel (banking app, push notification) delivers a result.
func (uc *ChallengeUseCase) CompleteOOBChallenge(ctx context.Context, in *OOBResultInput) (*OOBResultOutput, error) {
	tx, err := uc.txRepo.FindByACSTransactionID(ctx, in.ACSTransactionID)
	if err != nil {
		return nil, fmt.Errorf("find transaction: %w", err)
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction not found")
	}
	// Accept both C and D states — OOB may be initiated from either.
	if tx.TransStatus != entity.TransStatusChallenge && tx.TransStatus != entity.TransStatusDecoupled {
		return nil, fmt.Errorf("transaction not in OOB-eligible state: %s", tx.TransStatus)
	}

	notifURL := ""
	if session, sErr := uc.sessionRepo.FindByTransactionID(ctx, in.ACSTransactionID, "challenge"); sErr == nil && session != nil {
		notifURL = session.NotifURL
	}

	if in.Approved {
		authValue, eci, err := uc.authValSvc.Generate(ctx, tx)
		if err != nil {
			return nil, fmt.Errorf("auth value generation: %w", err)
		}
		tx.Complete(entity.TransStatusAuthenticated, authValue, eci)
	} else {
		tx.Complete(entity.TransStatusNotAuthenticated, "", tx.Network.DefaultECIs().NotAuthenticated)
	}

	if err := uc.txRepo.Update(ctx, tx); err != nil {
		return nil, fmt.Errorf("update transaction: %w", err)
	}

	uc.log.Info("OOB challenge completed",
		zap.String("acsTransID", tx.ACSTransactionID.String()),
		zap.String("transStatus", string(tx.TransStatus)),
		zap.Bool("approved", in.Approved),
	)
	_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventDecoupledComplete,
		map[string]any{"approved": in.Approved},
	))

	return &OOBResultOutput{
		TransStatus:         tx.TransStatus,
		ServerTransactionID: tx.ServerTransactionID,
		SpecVersion:         tx.SpecVersion,
		NotifURL:            notifURL,
		RReqPayload:         uc.buildRReq(tx),
	}, nil
}

// FallbackToOTPOutput holds the result of reverting from OOB back to OTP challenge.
type FallbackToOTPOutput struct {
	ACSTransactionID uuid.UUID
	TransStatus      entity.TransStatus
	AttemptsLeft     int
	Network          entity.Network
}

// FallbackOOBToOTP reverts a decoupled (D) transaction back to challenge (C)
// when the OOB push expires. This allows the cardholder to fall back to OTP.
func (uc *ChallengeUseCase) FallbackOOBToOTP(ctx context.Context, acsTransID uuid.UUID) (*FallbackToOTPOutput, error) {
	tx, err := uc.txRepo.FindByACSTransactionID(ctx, acsTransID)
	if err != nil {
		return nil, fmt.Errorf("find transaction: %w", err)
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction not found")
	}
	if tx.TransStatus != entity.TransStatusDecoupled && tx.TransStatus != entity.TransStatusChallenge {
		return nil, fmt.Errorf("transaction not in OOB/challenge state: %s", tx.TransStatus)
	}

	// Revert to challenge state and re-generate OTP.
	tx.TransStatus = entity.TransStatusChallenge
	tx.UpdatedAt = time.Now().UTC()

	otpPlain, otpHash, otpExpiry, err := uc.otpSvc.Generate()
	if err != nil {
		return nil, fmt.Errorf("otp generate: %w", err)
	}
	uc.log.Debug("otp generated for OOB fallback (dev only)",
		zap.String("otp", otpPlain),
		zap.String("acs_trans_id", tx.ACSTransactionID.String()),
	)
	tx.OTPHash = otpHash
	tx.OTPExpiry = &otpExpiry

	if err := uc.txRepo.Update(ctx, tx); err != nil {
		return nil, fmt.Errorf("update transaction: %w", err)
	}

	_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventChallengeStart,
		map[string]any{"otp": otpPlain, "reason": "oob_fallback"}))

	uc.log.Info("OOB fallback to OTP",
		zap.String("acsTransID", tx.ACSTransactionID.String()),
	)

	const maxAttempts = 3
	return &FallbackToOTPOutput{
		ACSTransactionID: tx.ACSTransactionID,
		TransStatus:      entity.TransStatusChallenge,
		AttemptsLeft:     maxAttempts - tx.ChallengeAttempts,
		Network:          tx.Network,
	}, nil
}

// ── PIN (Second Factor) Challenge ──────────────────────────────────────────

// SubmitPINInput contains the cardholder's PIN submission.
type SubmitPINInput struct {
	ACSTransactionID uuid.UUID
	SubmittedPIN     string
}

// SubmitPINOutput contains the result of PIN verification.
type SubmitPINOutput struct {
	Success             bool
	TransStatus         entity.TransStatus
	Network             entity.Network
	AuthValue           *string
	ECI                 *string
	AttemptsLeft        int
	ServerTransactionID uuid.UUID
	SpecVersion         entity.SpecVersion
	NotifURL            string
	RReqPayload         *RReqPayload
}

// SubmitPIN verifies the card PIN (second factor) and finalises the transaction.
func (uc *ChallengeUseCase) SubmitPIN(ctx context.Context, in *SubmitPINInput) (*SubmitPINOutput, error) {
	tx, err := uc.txRepo.FindByACSTransactionID(ctx, in.ACSTransactionID)
	if err != nil {
		return nil, fmt.Errorf("find transaction: %w", err)
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction not found")
	}
	if !tx.IsChallengeable() {
		return nil, fmt.Errorf("transaction not in challenge state")
	}
	if tx.PINHash == "" {
		return nil, fmt.Errorf("transaction not in PIN phase")
	}

	notifURL := ""
	if session, sErr := uc.sessionRepo.FindByTransactionID(ctx, in.ACSTransactionID, "challenge"); sErr == nil && session != nil {
		notifURL = session.NotifURL
	}

	tx.PINAttempts++
	const maxPINAttempts = 3
	attemptsLeft := maxPINAttempts - tx.PINAttempts

	valid := uc.pinSvc.Verify(in.SubmittedPIN, tx.PINHash, *tx.PINExpiry)

	if valid {
		authValue, eci, err := uc.authValSvc.Generate(ctx, tx)
		if err != nil {
			return nil, fmt.Errorf("auth value generation: %w", err)
		}
		tx.Complete(entity.TransStatusAuthenticated, authValue, eci)

		if err := uc.txRepo.Update(ctx, tx); err != nil {
			return nil, fmt.Errorf("update transaction: %w", err)
		}

		uc.log.Info("second factor (PIN) succeeded",
			zap.String("acsTransID", tx.ACSTransactionID.String()),
		)
		_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventChallengeSuccess,
			map[string]any{"phase": "second_factor"}))

		return &SubmitPINOutput{
			Success:             true,
			TransStatus:         tx.TransStatus,
			Network:             tx.Network,
			AuthValue:           tx.AuthValue,
			ECI:                 tx.ECI,
			ServerTransactionID: tx.ServerTransactionID,
			SpecVersion:         tx.SpecVersion,
			NotifURL:            notifURL,
			RReqPayload:         uc.buildRReq(tx),
		}, nil
	}

	// PIN invalid
	if tx.PINAttempts >= maxPINAttempts {
		tx.Complete(entity.TransStatusNotAuthenticated, "", tx.Network.DefaultECIs().NotAuthenticated)
		if err := uc.txRepo.Update(ctx, tx); err != nil {
			return nil, fmt.Errorf("update transaction: %w", err)
		}
		uc.log.Warn("second factor (PIN) failed: max attempts",
			zap.String("acsTransID", tx.ACSTransactionID.String()),
		)
		_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventChallengeFail,
			map[string]any{"reason": "pin_max_attempts"}))
		return &SubmitPINOutput{
			Success:             false,
			TransStatus:         tx.TransStatus,
			Network:             tx.Network,
			AttemptsLeft:        0,
			ServerTransactionID: tx.ServerTransactionID,
			SpecVersion:         tx.SpecVersion,
			NotifURL:            notifURL,
			RReqPayload:         uc.buildRReq(tx),
		}, nil
	}

	// Attempts remaining — regenerate PIN
	newPIN, newHash, newExpiry, pinErr := uc.pinSvc.Generate()
	if pinErr != nil {
		return nil, fmt.Errorf("rotate pin: %w", pinErr)
	}
	tx.PINHash = newHash
	tx.PINExpiry = &newExpiry
	if err := uc.txRepo.Update(ctx, tx); err != nil {
		return nil, fmt.Errorf("update transaction: %w", err)
	}

	uc.log.Warn("second factor (PIN) incorrect",
		zap.Int("attemptsLeft", attemptsLeft),
		zap.String("acsTransID", tx.ACSTransactionID.String()),
	)
	_ = uc.auditLog.Log(ctx, service.NewAuditEvent(tx, service.AuditEventChallengeFail,
		map[string]any{"reason": "pin_incorrect", "attempts_left": attemptsLeft, "new_pin": newPIN}))

	return &SubmitPINOutput{
		Success:             false,
		TransStatus:         entity.TransStatusChallenge,
		Network:             tx.Network,
		AttemptsLeft:        attemptsLeft,
		ServerTransactionID: tx.ServerTransactionID,
		SpecVersion:         tx.SpecVersion,
	}, nil
}

// GetPINInfo loads transaction info for rendering the PIN challenge page.
func (uc *ChallengeUseCase) GetPINInfo(ctx context.Context, acsTransID uuid.UUID) (*ChallengeInfoOutput, error) {
	tx, err := uc.txRepo.FindByACSTransactionID(ctx, acsTransID)
	if err != nil {
		return nil, fmt.Errorf("find transaction: %w", err)
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction not found: %s", acsTransID)
	}
	if !tx.IsChallengeable() {
		return nil, fmt.Errorf("transaction not in challenge state: %s", tx.TransStatus)
	}
	if tx.PINHash == "" {
		return nil, fmt.Errorf("transaction not in PIN phase")
	}

	notifURL := ""
	if session, sErr := uc.sessionRepo.FindByTransactionID(ctx, acsTransID, "challenge"); sErr == nil && session != nil {
		notifURL = session.NotifURL
	}

	const maxPINAttempts = 3
	return &ChallengeInfoOutput{
		ACSTransactionID: tx.ACSTransactionID,
		Network:          tx.Network,
		MerchantName:     tx.MerchantName,
		PurchaseAmount:   tx.PurchaseAmount,
		PurchaseCurrency: tx.PurchaseCurrency,
		AttemptsLeft:     maxPINAttempts - tx.PINAttempts,
		NotifURL:         notifURL,
	}, nil
}

// OOBStatusOutput is returned by the polling endpoint.
type OOBStatusOutput struct {
	// TransStatus is the current status: D=pending, Y=approved, N=denied.
	TransStatus entity.TransStatus
	// Completed is true if the OOB flow has concluded.
	Completed bool
}

// GetOOBStatus returns the current status of an OOB/decoupled authentication.
// The browser polls this endpoint while waiting for the cardholder to act.
func (uc *ChallengeUseCase) GetOOBStatus(ctx context.Context, acsTransID uuid.UUID) (*OOBStatusOutput, error) {
	tx, err := uc.txRepo.FindByACSTransactionID(ctx, acsTransID)
	if err != nil {
		return nil, fmt.Errorf("find transaction: %w", err)
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction not found")
	}

	completed := tx.TransStatus != entity.TransStatusChallenge && tx.TransStatus != entity.TransStatusDecoupled
	return &OOBStatusOutput{
		TransStatus: tx.TransStatus,
		Completed:   completed,
	}, nil
}
