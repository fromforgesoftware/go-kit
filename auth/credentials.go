package auth

import (
	"context"
	"fmt"

	"github.com/fromforgesoftware/go-kit/auth/jwt"
	"github.com/google/uuid"
)

const (
	// SystemAccountID is a fixed UUID for system-internal calls to allow easy identification
	SystemAccountID = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	SystemUsername  = "system"
)

// HmacCredentials implements credentials.PerRPCCredentials
type HmacCredentials struct {
	issuer jwt.Issuer
}

func NewHmacCredentials(issuer jwt.Issuer) *HmacCredentials {
	return &HmacCredentials{
		issuer: issuer,
	}
}

func (c *HmacCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	// Issue a token for the system account
	// In a real system, we might want to rotate keys or have different service accounts.
	// For now, using a static SystemAccountID is sufficient.
	id, _ := uuid.Parse(SystemAccountID)

	// Issue token with short expiry if the issuer supports custom expiry,
	// but the issuer interface just takes ctx.
	token, err := c.issuer.Issue(ctx, id, SystemUsername)
	if err != nil {
		return nil, fmt.Errorf("failed to issue hmac token: %w", err)
	}

	return map[string]string{
		"authorization": "Bearer " + token,
	}, nil
}

func (c *HmacCredentials) RequireTransportSecurity() bool {
	// For internal clsuter traffic without TLS (often the case with service mesh or internal networks),
	// this needs to be false or handled carefully.
	// Since we are using insecure for now in local dev, false is safer to avoid blocks.
	// However, credentials usually require TLS.
	// If we use insecure transport, grpc might complain unless we explicitly allow insecure.
	return false
}
