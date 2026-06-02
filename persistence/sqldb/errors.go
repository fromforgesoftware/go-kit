package sqldb

import (
	"fmt"

	"github.com/fromforgesoftware/go-kit/errors"
)

func newErrConnEmptyDSN() error {
	return errors.New(errors.CodeConfigurationError, errors.WithMessage("connection DSN cannot be empty"))
}

func newErrConnInvalidDriver(driver DriverType) error {
	return errors.New(errors.CodeConfigurationError, errors.WithMessage(fmt.Sprintf("invalid database driver: %s", driver)))
}

func newErrConn(err error) error {
	return errors.Wrap(err, errors.CodeDatabaseError, errors.WithMessage("failed to connect to database"))
}

func newErrEmptyDSNField(field connField) error {
	return errors.New(errors.CodeConfigurationError, errors.WithMessage(fmt.Sprintf("DSN field '%s' cannot be empty", field)))
}

func NewErrEmptyDBConnection() error {
	return errors.New(errors.CodeConfigurationError, errors.WithMessage("empty sql.DB connection handle"))
}
