package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-org/acs-server/internal/adapter/http/dto"
	"github.com/your-org/acs-server/internal/adapter/http/handler"
	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/repository"
	"github.com/your-org/acs-server/internal/domain/service"
	"github.com/your-org/acs-server/internal/usecase"
	"go.uber.org/zap"
)

// ── Mocks (mirror usecase_test mocks) ────────────────────────────

type stubTxRepo struct{ tx *entity.Transaction }

func (s *stubTxRepo) Save(_ context.Context, tx *entity.Transaction) error {
	s.tx = tx; return nil
}
func (s *stubTxRepo) Update(_ context.Context, tx *entity.Transaction) error {
	s.tx = tx; return nil
}
func (s *stubTxRepo) FindByACSTransactionID(_ context.Context, _ uuid.UUID) (*entity.Transaction, error) {
	return s.tx, nil
}
func (s *stubTxRepo) FindByServerTransactionID(_ context.Context, _ uuid.UUID) (*entity.Transaction, error) {
	return s.tx, nil
}
func (s *stubTxRepo) List(_ context.Context, _ repository.TransactionFilter) ([]*entity.Transaction, int64, error) {
	return nil, 0, nil
}

type enrolledRangeRepo struct{}

func (e *enrolledRangeRepo) FindByPAN(_ context.Context, _ string) (*entity.CardRange, error) {
	return &entity.CardRange{
		StartRange: "400000", EndRange: "499999",
		MinProtocol: entity.SpecVersion220, MaxProtocol: entity.SpecVersion231,
		Active: true,
	}, nil
}
func (e *enrolledRangeRepo) Upsert(_ context.Context, _ *entity.CardRange) error      { return nil }
func (e *enrolledRangeRepo) DeleteByRange(_ context.Context, _, _ string) error        { return nil }
func (e *enrolledRangeRepo) ListAll(_ context.Context) ([]*entity.CardRange, error)    { return nil, nil }

type stubSessionRepo struct{}

func (s *stubSessionRepo) Save(_ context.Context, _ *entity.ACSSession) error { return nil }
func (s *stubSessionRepo) FindByID(_ context.Context, _ uuid.UUID) (*entity.ACSSession, error) {
	return nil, nil
}
func (s *stubSessionRepo) FindByTransactionID(_ context.Context, _ uuid.UUID, _ string) (*entity.ACSSession, error) {
	return nil, nil
}
func (s *stubSessionRepo) UpdateStatus(_ context.Context, _ uuid.UUID, _ entity.SessionStatus) error {
	return nil
}

type stubAuthValSvc struct{}

func (s *stubAuthValSvc) Generate(_ context.Context, _ *entity.Transaction) (string, string, error) {
	return "STUBCAVV==", "05", nil
}

// ── Test ──────────────────────────────────────────────────────────

func TestAuthHandler_HandleAReq_Frictionless(t *testing.T) {
	gin.SetMode(gin.TestMode)

	log, _ := zap.NewDevelopment()
	authUC := usecase.NewAuthenticateUseCase(
		&stubTxRepo{},
		&enrolledRangeRepo{},
		&stubSessionRepo{},
		service.NewDefaultRiskEngine(log),
		service.NewOTPService(),
		&stubAuthValSvc{},
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
	h := handler.NewAuthHandler(authUC, "3DS_LOA_TEST", nil, nil, nil, log)

	r := gin.New()
	r.POST("/v1/auth", h.HandleAReq)

	amount := "001000"
	currency := "840"
	exponent := 2
	accept := "text/html,application/xhtml+xml"
	ua := "Mozilla/5.0"
	lang := "en-US"
	depth := "24"
	height := "900"
	width := "1440"
	tz := -300
	javaEnabled := false
	body := dto.AReqDTO{
		MessageType:          "AReq",
		MessageVersion:       "2.2.0",
		MessageCategory:      "01",
		DeviceChannel:        "02",
		ThreeDSServerTransID: uuid.New().String(),
		DSTransID:            uuid.New().String(),
		AcctNumber:           "4111111111111111",
		MerchantName:         "Test Store",
		MerchantCountry:      "840",
		PurchaseAmount:       &amount,
		PurchaseCurrency:     &currency,
		PurchaseExponent:     &exponent,
		BrowserAcceptHeader:  &accept,
		BrowserUserAgent:     &ua,
		BrowserLanguage:      &lang,
		BrowserColorDepth:    &depth,
		BrowserScreenHeight:  &height,
		BrowserScreenWidth:   &width,
		BrowserTZ:            &tz,
		BrowserJavaEnabled:   &javaEnabled,
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/auth", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp dto.AResDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.MessageType != "ARes" {
		t.Errorf("expected ARes, got %s", resp.MessageType)
	}
	if resp.TransStatus != "Y" {
		t.Errorf("expected Y (frictionless), got %s", resp.TransStatus)
	}
	if resp.ACSReferenceNumber != "3DS_LOA_TEST" {
		t.Errorf("expected ref number in ARes")
	}
}

func TestAuthHandler_HandleAReq_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log, _ := zap.NewDevelopment()
	authUC := usecase.NewAuthenticateUseCase(
		&stubTxRepo{},
		&enrolledRangeRepo{},
		&stubSessionRepo{},
		service.NewDefaultRiskEngine(log),
		service.NewOTPService(),
		&stubAuthValSvc{},
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
	h := handler.NewAuthHandler(authUC, "REF", nil, nil, nil, log)

	r := gin.New()
	r.POST("/v1/auth", h.HandleAReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/auth", bytes.NewBufferString(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
