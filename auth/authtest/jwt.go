package authtest

import (
	"context"

	"github.com/fromforgesoftware/go-kit/auth/jwt"
	"github.com/google/uuid"
)

// mockJWT implements jwt.IssuerValidator for testing
type mockJWT struct {
	accountID uuid.UUID
	username  string
}

// NewMockJWT creates a new mock JWT issuer/validator for testing
func NewMockJWT(accountID uuid.UUID, username string) *mockJWT {
	return &mockJWT{
		accountID: accountID,
		username:  username,
	}
}

func (m *mockJWT) Issue(ctx context.Context, id uuid.UUID, u string) (string, error) {
	// Return a dummy JWT with iss: HMAC so it's detected correctly
	// Header: {"alg":"none"} -> eyJhbGciOiJub25lIn0
	// Payload: {"iss":"HMAC"} -> eyJpc3MiOiJITUFDIn0
	return "eyJhbGciOiJub25lIn0.eyJpc3MiOiJITUFDIn0.", nil
}

func (m *mockJWT) Validate(ctx context.Context, token string) (*jwt.Claims, error) {
	return &jwt.Claims{AccountID: m.accountID, Username: m.username}, nil
}
