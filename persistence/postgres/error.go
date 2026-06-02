// Package postgres holds Postgres-specific error translation + small
// connection helpers (SQLSTATE → apierrors, advisory-lock primitives)
// shared by the gormdb and sqldb adapters.
package postgres

import (
	"errors"
	"fmt"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const ErrDuplicateKey = "23505"

func ErrorIs(err error, code string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == code {
		return true
	}
	return false
}

// NewErrUnknown wraps a raw db/gorm error into an apierrors.Error.
// gorm.ErrRecordNotFound is translated to apierrors.NotFound so the
// REST/gRPC encoders surface it as 404 instead of 500 — every repo
// First()/Get() that didn't find a row would otherwise leak as a
// generic InternalError. Other errors stay InternalError; callers
// looking for SQLSTATE-specific translation should use ErrorIs first.
func NewErrUnknown(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apierrors.New(apierrors.CodeNotFound,
			apierrors.WithMessage(err.Error()),
			apierrors.WithHTTPStatus(404),
		)
	}
	return apierrors.InternalError(fmt.Sprintf("query failed, please check the database adapter logs, %s", err.Error()))
}
