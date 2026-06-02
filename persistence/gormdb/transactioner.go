package gormdb

import (
	"context"

	"gorm.io/gorm"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/persistence"
)

type txKey struct{}

// injectTx injects transaction in context. If transaction already exists in context, it won't do nothing, returning the context as it is.
// This implementation allows us to compose transactions with nested .Exec() calls, having a single flattened transaction with all of the parts.
func injectTx(ctx context.Context, tx *gorm.DB) context.Context {
	if gormTxExists(ctx) {
		return ctx
	}
	return context.WithValue(ctx, txKey{}, tx)
}

func gormTxExists(ctx context.Context) bool {
	return extractTx(ctx) != nil
}

// extractTx extracts transaction from context
func extractTx(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return nil
}

type transactioner struct {
	db  *DBClient
	log logger.Logger
}

func NewTransactioner(db *DBClient, log logger.Logger) *transactioner {
	return &transactioner{
		db:  db,
		log: log,
	}
}

func (t *transactioner) Exec(ctx context.Context, fn persistence.TxFunc) error {
	if gormTxExists(ctx) {
		return fn(ctx)
	}
	return t.db.Transaction(func(tx *gorm.DB) error {
		return fn(injectTx(ctx, tx))
	})
}
