package redisdb

import (
	"errors"
	"fmt"
)

// ConnectionErr defines a database connection error.
type ConnectionErr struct {
	wrappedErr error
}

// Error implements the error interface with additional context.
func (err ConnectionErr) Error() string {
	return fmt.Sprintf("redis connection error: %s", err.wrappedErr.Error())
}

// Unwrap returns the child error (the specific error reason of the connection error).
func (err ConnectionErr) Unwrap() error {
	return err.wrappedErr
}

func newErrConn(wrappedErr error) error {
	return ConnectionErr{
		wrappedErr: wrappedErr,
	}
}

func newPingErr() error {
	return newErrConn(errors.New("no PONG received"))
}

func newNotifyKeySpaceEventsErr() error {
	return newErrConn(errors.New("notify-keyspace-events not configured correctly"))
}
