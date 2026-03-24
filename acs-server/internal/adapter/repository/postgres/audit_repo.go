package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/your-org/acs-server/internal/domain/service"
)

// AuditRepo implements service.AuditLogger using PostgreSQL.
type AuditRepo struct {
	db *sqlx.DB
}

// NewAuditRepo creates a new AuditRepo.
func NewAuditRepo(db *sqlx.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

// Log inserts an AuditEvent into the audit_log table.
func (r *AuditRepo) Log(ctx context.Context, evt *service.AuditEvent) error {
	detail, err := json.Marshal(evt.Detail)
	if err != nil {
		return fmt.Errorf("marshal audit detail: %w", err)
	}

	const q = `
		INSERT INTO audit_log (
			acs_transaction_id, ds_transaction_id,
			event_type, trans_status,
			card_bin, merchant_country,
			device_channel, spec_version,
			detail, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`

	_, err = r.db.ExecContext(ctx, q,
		evt.ACSTransactionID,
		evt.DSTransactionID,
		string(evt.EventType),
		string(evt.TransStatus),
		evt.CardBIN,
		evt.MerchantCountry,
		string(evt.DeviceChannel),
		string(evt.SpecVersion),
		detail,
		evt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("AuditRepo.Log: %w", err)
	}
	return nil
}
