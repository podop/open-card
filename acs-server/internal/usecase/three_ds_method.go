package usecase

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/repository"
	"go.uber.org/zap"
)

// ThreeDSMethodUseCase handles the 3DS Method device fingerprinting flow.
// Per spec: the ACS must collect device info silently with NO cardholder interaction.
type ThreeDSMethodUseCase struct {
	txRepo      repository.TransactionRepository
	sessionRepo repository.SessionRepository
	log         *zap.Logger
}

// NewThreeDSMethodUseCase wires up the 3DS Method use case.
func NewThreeDSMethodUseCase(
	txRepo repository.TransactionRepository,
	sessionRepo repository.SessionRepository,
	log *zap.Logger,
) *ThreeDSMethodUseCase {
	return &ThreeDSMethodUseCase{
		txRepo:      txRepo,
		sessionRepo: sessionRepo,
		log:         log,
	}
}

// ThreeDSMethodData is the decoded payload from the threeDSMethodData form field.
// The 3DS Server base64url-encodes a JSON object containing these fields.
type ThreeDSMethodData struct {
	ThreeDSServerTransID string `json:"threeDSServerTransID"`
	ThreeDSMethodNotifURL string `json:"threeDSMethodNotificationURL"`
}

// InitiateMethodInput carries the raw base64-encoded form post from the browser.
type InitiateMethodInput struct {
	ThreeDSMethodDataEncoded string // base64url-encoded JSON
	RemoteAddr               string
	UserAgent                string
	AcceptHeader             string
	IPAddress                string
}

// InitiateMethodOutput contains what to render in the hidden iframe.
type InitiateMethodOutput struct {
	SessionID uuid.UUID
	// NotifURL is posted back to by the ACS iframe content when complete
	NotifURL string
}

// Initiate decodes the 3DS Method request and sets up a session.
func (uc *ThreeDSMethodUseCase) Initiate(ctx context.Context, in *InitiateMethodInput) (*InitiateMethodOutput, error) {
	// Decode the base64url payload
	raw, err := base64.RawURLEncoding.DecodeString(in.ThreeDSMethodDataEncoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode threeDSMethodData: %w", err)
	}

	var methodData ThreeDSMethodData
	if err := json.Unmarshal(raw, &methodData); err != nil {
		return nil, fmt.Errorf("unmarshal threeDSMethodData: %w", err)
	}

	serverTransID, err := uuid.Parse(methodData.ThreeDSServerTransID)
	if err != nil {
		return nil, fmt.Errorf("invalid threeDSServerTransID: %w", err)
	}

	// Find the pending transaction (AReq not yet sent; method runs before AReq)
	// In practice the DS sends PRes with card ranges that include the ACS Method URL.
	// The 3DS Server calls the ACS Method URL, not the ACS itself.
	// We create a placeholder session keyed on serverTransID.
	session := &entity.ACSSession{
		ID:          uuid.New(),
		TransactionID: serverTransID, // temporarily using serverTransID as key
		SessionType: "method",
		Status:      entity.SessionStatusPending,
		NotifURL:    methodData.ThreeDSMethodNotifURL,
	}
	session.ExpiresAt = session.CreatedAt.Add(0) // will be set in NewMethodSession
	session = entity.NewMethodSession(serverTransID, methodData.ThreeDSMethodNotifURL)

	// Store device info from HTTP headers
	deviceInfo := map[string]string{
		"ip":           in.IPAddress,
		"userAgent":    in.UserAgent,
		"acceptHeader": in.AcceptHeader,
	}
	deviceBytes, _ := json.Marshal(deviceInfo)
	session.DeviceData = deviceBytes

	if err := uc.sessionRepo.Save(ctx, session); err != nil {
		return nil, fmt.Errorf("save method session: %w", err)
	}

	uc.log.Info("3DS Method initiated",
		zap.String("serverTransID", serverTransID.String()),
		zap.String("sessionID", session.ID.String()),
	)

	return &InitiateMethodOutput{
		SessionID: session.ID,
		NotifURL:  methodData.ThreeDSMethodNotifURL,
	}, nil
}

// Complete is called when the browser posts back to the ACS Method notification URL.
// The ACS must respond with an empty 200 and post to the 3DS Server notification URL.
func (uc *ThreeDSMethodUseCase) Complete(ctx context.Context, sessionID uuid.UUID) (notifURL string, err error) {
	session, err := uc.sessionRepo.FindByID(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("find session: %w", err)
	}
	if session.IsExpired() {
		return "", fmt.Errorf("session expired")
	}

	if err := uc.sessionRepo.UpdateStatus(ctx, sessionID, entity.SessionStatusCompleted); err != nil {
		return "", fmt.Errorf("update session status: %w", err)
	}

	uc.log.Info("3DS Method completed", zap.String("sessionID", sessionID.String()))
	return session.NotifURL, nil
}
