package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/repository"
	"github.com/your-org/acs-server/internal/domain/service"
	"github.com/your-org/acs-server/internal/usecase"
	"go.uber.org/zap"
)

// ── Mocks ────────────────────────────────────────────────────────

type mockTxRepo struct {
	saved   *entity.Transaction
	updated *entity.Transaction
}

func (m *mockTxRepo) Save(_ context.Context, tx *entity.Transaction) error {
	m.saved = tx
	return nil
}
func (m *mockTxRepo) Update(_ context.Context, tx *entity.Transaction) error {
	m.updated = tx
	return nil
}
func (m *mockTxRepo) FindByACSTransactionID(_ context.Context, _ uuid.UUID) (*entity.Transaction, error) {
	return m.saved, nil
}
func (m *mockTxRepo) FindByServerTransactionID(_ context.Context, _ uuid.UUID) (*entity.Transaction, error) {
	return m.saved, nil
}
func (m *mockTxRepo) List(_ context.Context, _ repository.TransactionFilter) ([]*entity.Transaction, int64, error) {
	return nil, 0, nil
}

type mockRangeRepo struct {
	cardRange *entity.CardRange
}

func (m *mockRangeRepo) FindByPAN(_ context.Context, _ string) (*entity.CardRange, error) {
	return m.cardRange, nil
}
func (m *mockRangeRepo) Upsert(_ context.Context, _ *entity.CardRange) error  { return nil }
func (m *mockRangeRepo) DeleteByRange(_ context.Context, _, _ string) error   { return nil }
func (m *mockRangeRepo) ListAll(_ context.Context) ([]*entity.CardRange, error) {
	return nil, nil
}

type mockSessionRepo struct{}

func (m *mockSessionRepo) Save(_ context.Context, _ *entity.ACSSession) error { return nil }
func (m *mockSessionRepo) FindByID(_ context.Context, _ uuid.UUID) (*entity.ACSSession, error) {
	return nil, nil
}
func (m *mockSessionRepo) FindByTransactionID(_ context.Context, _ uuid.UUID, _ string) (*entity.ACSSession, error) {
	return nil, nil
}
func (m *mockSessionRepo) UpdateStatus(_ context.Context, _ uuid.UUID, _ entity.SessionStatus) error {
	return nil
}

type mockAuthValSvc struct{}

func (m *mockAuthValSvc) Generate(_ context.Context, _ *entity.Transaction) (string, string, error) {
	return "MOCKCAVV==", "05", nil
}

// ── Tests ─────────────────────────────────────────────────────────

func newTestUC(cardRange *entity.CardRange) *usecase.AuthenticateUseCase {
	log, _ := zap.NewDevelopment()
	return usecase.NewAuthenticateUseCase(
		&mockTxRepo{},
		&mockRangeRepo{cardRange: cardRange},
		&mockSessionRepo{},
		service.NewDefaultRiskEngine(log),
		service.NewOTPService(),
		&mockAuthValSvc{},
		&service.NoopAuditLogger{},
		"https://acs.test",
		usecase.NewInMemoryPolicyStore(nil),
		nil,
		nil,
		nil,
		nil,
		nil,
		log,
	)
}

func TestAuthenticate_CardNotEnrolled(t *testing.T) {
	uc := newTestUC(nil) // nil = not enrolled

	out, err := uc.Execute(context.Background(), &usecase.AuthenticateInput{
		MessageVersion:      "2.2.0",
		DeviceChannel:       entity.DeviceChannelBrowser,
		MessageCategory:     entity.MessageCategoryPayment,
		DSTransactionID:     uuid.New(),
		ServerTransactionID: uuid.New(),
		AcctNumber:          "5000001234567890",
		MerchantName:        "ACME Store",
		MerchantCountry:     "840",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TransStatus != entity.TransStatusUnavailable {
		t.Errorf("expected U status for unenrolled card, got %s", out.TransStatus)
	}
}

func TestAuthenticate_FrictionlessFlow(t *testing.T) {
	enrolled := &entity.CardRange{
		StartRange:  "400000",
		EndRange:    "499999",
		MinProtocol: entity.SpecVersion220,
		MaxProtocol: entity.SpecVersion231,
		Active:      true,
	}
	uc := newTestUC(enrolled)

	amount := "001000" // low-value → frictionless by default risk engine
	out, err := uc.Execute(context.Background(), &usecase.AuthenticateInput{
		MessageVersion:      "2.2.0",
		DeviceChannel:       entity.DeviceChannelBrowser,
		MessageCategory:     entity.MessageCategoryPayment,
		DSTransactionID:     uuid.New(),
		ServerTransactionID: uuid.New(),
		AcctNumber:          "4111111111111111",
		MerchantName:        "Test Merchant",
		MerchantCountry:     "826",
		PurchaseAmount:      &amount,
		BrowserInfo: &service.BrowserInfo{
			IPAddress: "1.2.3.4",
			UserAgent: "Mozilla/5.0",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TransStatus != entity.TransStatusAuthenticated {
		t.Errorf("expected Y (frictionless), got %s", out.TransStatus)
	}
	if out.AuthValue == nil {
		t.Error("expected auth value in frictionless response")
	}
}

func TestAuthenticate_ChallengeFlow(t *testing.T) {
	enrolled := &entity.CardRange{
		StartRange:  "400000",
		EndRange:    "499999",
		MinProtocol: entity.SpecVersion220,
		MaxProtocol: entity.SpecVersion231,
		Active:      true,
	}
	uc := newTestUC(enrolled)

	// High amount + no device info → challenge
	amount := "999999"
	out, err := uc.Execute(context.Background(), &usecase.AuthenticateInput{
		MessageVersion:      "2.2.0",
		DeviceChannel:       entity.DeviceChannelBrowser,
		MessageCategory:     entity.MessageCategoryPayment,
		DSTransactionID:     uuid.New(),
		ServerTransactionID: uuid.New(),
		AcctNumber:          "4111111111111111",
		MerchantName:        "Big Ticket Merchant",
		MerchantCountry:     "826",
		PurchaseAmount:      &amount,
		BrowserInfo:         nil, // no device info → risk engine triggers challenge
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TransStatus != entity.TransStatusChallenge {
		t.Errorf("expected C (challenge), got %s", out.TransStatus)
	}
	if out.ACSChallengeURL == nil {
		t.Error("expected ACS challenge URL in response")
	}
}

func TestAuthenticate_UnsupportedVersion(t *testing.T) {
	uc := newTestUC(nil)
	_, err := uc.Execute(context.Background(), &usecase.AuthenticateInput{
		MessageVersion:      "1.0.0",
		DeviceChannel:       entity.DeviceChannelBrowser,
		MessageCategory:     entity.MessageCategoryPayment,
		DSTransactionID:     uuid.New(),
		ServerTransactionID: uuid.New(),
		AcctNumber:          "4111111111111111",
		MerchantName:        "Test",
		MerchantCountry:     "840",
	})
	if err == nil {
		t.Error("expected error for unsupported version")
	}
}
