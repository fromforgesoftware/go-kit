// Package jwt provides JWT-based authentication primitives — signing
// helpers, HMAC + RSA verifiers, claims parsing — used by the kit/auth
// HTTP + gRPC interceptors.
package jwt

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Issuer issues JWT tokens
type Issuer interface {
	Issue(ctx context.Context, accountID uuid.UUID, username string) (string, error)
}

// Validator validates JWT tokens
type Validator interface {
	Validate(ctx context.Context, token string) (*Claims, error)
}

// IssuerValidator combines issuer and validator
type IssuerValidator interface {
	Issuer
	Validator
}

// Claims represents JWT claims
type Claims struct {
	AccountID  uuid.UUID
	Username   string
	ExpiryTime time.Time
}

func (c *Claims) Subject() string {
	return c.AccountID.String()
}

func (c *Claims) Expiry() time.Time {
	return c.ExpiryTime
}

func (c *Claims) Get(key string) any {
	switch key {
	case "sub":
		return c.AccountID.String()
	case "username":
		return c.Username
	case "exp":
		return c.ExpiryTime.Unix()
	default:
		return nil
	}
}
