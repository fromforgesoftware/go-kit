package auth

import (
	"context"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
)

// Active-organization claims carried by the access token (set by the identity
// service at issuance). Consumers read these to scope tenant data.
const (
	ClaimOrgID   = "org_id"
	ClaimOrgRole = "org_role"
)

// OrgIDFromCtx returns the active organization id from the request token, and
// false when no token or no active org is present.
func OrgIDFromCtx(ctx context.Context) (string, bool) {
	return stringClaimFromCtx(ctx, ClaimOrgID)
}

// OrgRoleFromCtx returns the caller's effective top role on the active org.
func OrgRoleFromCtx(ctx context.Context) (string, bool) {
	return stringClaimFromCtx(ctx, ClaimOrgRole)
}

// MustOrgID returns the active organization id or an Unauthorized error when
// the request carries no active org — the fail-closed default for
// tenant-scoped repositories.
func MustOrgID(ctx context.Context) (string, error) {
	org, ok := OrgIDFromCtx(ctx)
	if !ok {
		return "", apierrors.Unauthorized("no active organization in context")
	}
	return org, nil
}

func stringClaimFromCtx(ctx context.Context, key string) (string, bool) {
	tok := TokenFromCtx(ctx)
	if tok == nil {
		return "", false
	}
	v, ok := tok.Claims().Get(key).(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}
