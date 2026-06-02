// Package postgres is the Postgres/GORM implementation of
// outbox.Repository. Producer services wire this; the dispatcher
// package consumes the interface, not the concrete type.
//
// Schema: each producer service ships a migration creating its local
// outbox table (one per service so write contention stays bounded).
// The canonical shape is documented in outbox/postgres/schema.sql —
// copy it verbatim and adjust schema/table names. The table name is
// passed to New() so a service can scope its outbox to its own schema
// (e.g. workspace.outbox, accounting.outbox).
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fromforgesoftware/go-kit/outbox"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
)

// Repository is the gorm-backed implementation. Construct one per
// outbox table.
type Repository struct {
	db    *gormdb.DBClient
	table string
}

// New returns a Repository scoped to a fully-qualified table name like
// "workspace.outbox" or "accounting.outbox". The table is expected to
// match the shape in schema.sql.
func New(db *gormdb.DBClient, table string) *Repository {
	return &Repository{db: db, table: table}
}

// row is the GORM struct mapping the outbox table. Status uses the
// strings "pending" / "done" since Postgres ENUMs would force every
// service to share an ALTER TYPE migration.
type row struct {
	ID        string          `gorm:"column:id;type:uuid;default:uuid_generate_v4();primaryKey"`
	Kind      string          `gorm:"column:kind"`
	Payload   json.RawMessage `gorm:"column:payload;type:jsonb"`
	CreatedAt time.Time       `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time       `gorm:"column:updated_at;autoUpdateTime"`
	Attempts  int             `gorm:"column:attempts;default:0"`
	RetryAt   time.Time       `gorm:"column:retry_at"`
	LastError string          `gorm:"column:last_error"`
	Status    string          `gorm:"column:status;default:pending"` // pending | done
}

// Enqueue inserts the drafts using whatever *gorm.DB sits on the ctx
// — kit's Transactioner stores the txn handle there. Inside a tx, the
// outbox INSERT commits atomically with the producer's writes.
func (r *Repository) Enqueue(ctx context.Context, drafts ...outbox.MessageDraft) error {
	if len(drafts) == 0 {
		return nil
	}
	rows := make([]row, len(drafts))
	for i, d := range drafts {
		rows[i] = row{
			Kind:    d.Kind,
			Payload: d.Payload,
			RetryAt: time.Now(),
			Status:  "pending",
		}
	}
	if err := r.db.WithContext(ctx).Table(r.table).Create(&rows).Error; err != nil {
		return fmt.Errorf("outbox: enqueue: %w", err)
	}
	return nil
}

// Claim grabs up to `batch` pending rows whose retry_at has elapsed.
// FOR UPDATE SKIP LOCKED ensures concurrent drainer replicas see
// disjoint batches. The claim is one transaction (SELECT + UPDATE);
// the caller then processes each message and calls MarkDone /
// MarkFailed separately. The attempts++ inside the claim doubles as
// a "claimed" marker — a crashed drainer's rows sit until retry_at
// elapses, then naturally back off via the Dispatcher's backoff math.
func (r *Repository) Claim(ctx context.Context, batch int) ([]outbox.Message, error) {
	if batch <= 0 {
		batch = 50
	}
	var rows []row
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Table(r.table).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND retry_at <= ?", "pending", time.Now()).
			Order("retry_at ASC, id ASC").
			Limit(batch).
			Find(&rows).Error
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]string, len(rows))
		for i, rw := range rows {
			ids[i] = rw.ID
		}
		return tx.Table(r.table).
			Where("id IN ?", ids).
			UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error
	})
	if err != nil {
		return nil, fmt.Errorf("outbox: claim: %w", err)
	}

	out := make([]outbox.Message, len(rows))
	for i, rw := range rows {
		out[i] = outbox.Message{
			ID:        rw.ID,
			Kind:      rw.Kind,
			Payload:   rw.Payload,
			CreatedAt: rw.CreatedAt,
			Attempts:  rw.Attempts + 1, // we just bumped it
			LastError: rw.LastError,
		}
	}
	return out, nil
}

// MarkDone deletes the row. Keeping done rows around inflates the
// table and slows Claim's WHERE; for audit-style traceability use
// the audit service, not the outbox itself.
func (r *Repository) MarkDone(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Table(r.table).Where("id = ?", id).Delete(nil).Error; err != nil {
		return fmt.Errorf("outbox: mark done: %w", err)
	}
	return nil
}

// MarkFailed updates retry_at + last_error. The Dispatcher computes
// retryAt (exponential backoff) and decides when to dead-letter; this
// repo just persists what it's told.
func (r *Repository) MarkFailed(ctx context.Context, id string, handlerErr error, retryAt time.Time) error {
	msg := ""
	if handlerErr != nil {
		msg = handlerErr.Error()
	}
	if err := r.db.WithContext(ctx).Table(r.table).
		Where("id = ?", id).
		Updates(map[string]any{
			"retry_at":   retryAt,
			"last_error": msg,
		}).Error; err != nil {
		return fmt.Errorf("outbox: mark failed: %w", err)
	}
	return nil
}
