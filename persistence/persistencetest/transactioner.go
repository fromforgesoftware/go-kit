package persistencetest

import (
	context "context"

	"github.com/fromforgesoftware/go-kit/persistence"
)

type transactioner struct{}

func NewTransactioner() *transactioner {
	return &transactioner{}
}

func (ts *transactioner) Exec(ctx context.Context, fn persistence.TxFunc) error {
	err := fn(ctx)
	if err != nil {
		return err
	}
	return nil
}
