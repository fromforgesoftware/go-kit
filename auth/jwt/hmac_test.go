package jwt_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/auth/jwt"
)

const testSecret = "test-secret-key-for-hmac-signing"

func TestNewHMACIssuer(t *testing.T) {
	t.Run("returns issuer for non-empty secret", func(t *testing.T) {
		iss, err := jwt.NewHMACIssuer(testSecret)
		require.NoError(t, err)
		require.NotNil(t, iss)
	})

	t.Run("rejects empty secret", func(t *testing.T) {
		iss, err := jwt.NewHMACIssuer("")
		require.Error(t, err)
		assert.Nil(t, iss)
	})
}

func TestHMACIssue_Validate_RoundTrip(t *testing.T) {
	iss, err := jwt.NewHMACIssuer(testSecret)
	require.NoError(t, err)

	accountID := uuid.New()
	const username = "alice"

	token, err := iss.Issue(context.Background(), accountID, username)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := iss.Validate(context.Background(), token)
	require.NoError(t, err)
	require.NotNil(t, claims)

	assert.Equal(t, accountID, claims.AccountID)
	assert.Equal(t, username, claims.Username)
	assert.Equal(t, accountID.String(), claims.Subject())
	// exp should be ~24h out.
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), claims.Expiry(), time.Minute)
}

func TestHMACClaims_Get(t *testing.T) {
	iss, err := jwt.NewHMACIssuer(testSecret)
	require.NoError(t, err)

	accountID := uuid.New()
	token, err := iss.Issue(context.Background(), accountID, "bob")
	require.NoError(t, err)

	claims, err := iss.Validate(context.Background(), token)
	require.NoError(t, err)

	assert.Equal(t, accountID.String(), claims.Get("sub"))
	assert.Equal(t, "bob", claims.Get("username"))
	assert.Equal(t, claims.ExpiryTime.Unix(), claims.Get("exp"))
	assert.Nil(t, claims.Get("unknown"))
}

func TestHMACValidate_ExpiredToken(t *testing.T) {
	iss, err := jwt.NewHMACIssuer(testSecret)
	require.NoError(t, err)

	// Forge an already-expired token signed with the correct secret.
	accountID := uuid.New()
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, jwtlib.MapClaims{
		"sub":      accountID.String(),
		"iss":      "HMAC",
		"username": "carol",
		"iat":      time.Now().Add(-48 * time.Hour).Unix(),
		"exp":      time.Now().Add(-24 * time.Hour).Unix(),
	})
	signed, err := tok.SignedString([]byte(testSecret))
	require.NoError(t, err)

	claims, err := iss.Validate(context.Background(), signed)
	require.Error(t, err)
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, jwtlib.ErrTokenExpired)
}

func TestHMACValidate_WrongKey(t *testing.T) {
	issuer, err := jwt.NewHMACIssuer(testSecret)
	require.NoError(t, err)

	verifier, err := jwt.NewHMACIssuer("a-completely-different-secret-key")
	require.NoError(t, err)

	token, err := issuer.Issue(context.Background(), uuid.New(), "dave")
	require.NoError(t, err)

	claims, err := verifier.Validate(context.Background(), token)
	require.Error(t, err)
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, jwtlib.ErrSignatureInvalid)
}

func TestHMACValidate_TamperedSignature(t *testing.T) {
	iss, err := jwt.NewHMACIssuer(testSecret)
	require.NoError(t, err)

	token, err := iss.Issue(context.Background(), uuid.New(), "erin")
	require.NoError(t, err)

	// Flip the last character of the signature segment.
	tampered := token[:len(token)-1]
	if token[len(token)-1] == 'a' {
		tampered += "b"
	} else {
		tampered += "a"
	}

	claims, err := iss.Validate(context.Background(), tampered)
	require.Error(t, err)
	assert.Nil(t, claims)
}

func TestHMACValidate_MalformedToken(t *testing.T) {
	iss, err := jwt.NewHMACIssuer(testSecret)
	require.NoError(t, err)

	for _, tc := range []string{"", "not-a-jwt", "a.b", "a.b.c.d"} {
		claims, err := iss.Validate(context.Background(), tc)
		require.Error(t, err, "input %q should fail", tc)
		assert.Nil(t, claims)
	}
}

// TestHMACValidate_AlgConfusion_RSA verifies the HMAC validator rejects a token
// signed with an asymmetric (RSA) algorithm. Without the explicit signing-method
// check in Validate, an attacker could present an RS256 token and have the
// public key interpreted as the HMAC secret — the classic JWT alg-confusion
// attack. This test guards that defense.
func TestHMACValidate_AlgConfusion_RSA(t *testing.T) {
	iss, err := jwt.NewHMACIssuer(testSecret)
	require.NoError(t, err)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, jwtlib.MapClaims{
		"sub":      uuid.New().String(),
		"username": "mallory",
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	signed, err := tok.SignedString(key)
	require.NoError(t, err)

	claims, err := iss.Validate(context.Background(), signed)
	require.Error(t, err)
	assert.Nil(t, claims)
}

// TestHMACValidate_AlgNone verifies the validator rejects "alg: none" tokens,
// which carry no signature and would otherwise let anyone forge claims.
func TestHMACValidate_AlgNone(t *testing.T) {
	iss, err := jwt.NewHMACIssuer(testSecret)
	require.NoError(t, err)

	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodNone, jwtlib.MapClaims{
		"sub":      uuid.New().String(),
		"username": "mallory",
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	signed, err := tok.SignedString(jwtlib.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	claims, err := iss.Validate(context.Background(), signed)
	require.Error(t, err)
	assert.Nil(t, claims)
}

func TestHMACValidate_InvalidAccountID(t *testing.T) {
	iss, err := jwt.NewHMACIssuer(testSecret)
	require.NoError(t, err)

	// Signed with the correct key but "sub" is not a UUID.
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, jwtlib.MapClaims{
		"sub":      "not-a-uuid",
		"username": "frank",
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	signed, err := tok.SignedString([]byte(testSecret))
	require.NoError(t, err)

	claims, err := iss.Validate(context.Background(), signed)
	require.Error(t, err)
	assert.Nil(t, claims)
}

func TestHMACValidate_MissingUsername(t *testing.T) {
	iss, err := jwt.NewHMACIssuer(testSecret)
	require.NoError(t, err)

	accountID := uuid.New()
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, jwtlib.MapClaims{
		"sub": accountID.String(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := tok.SignedString([]byte(testSecret))
	require.NoError(t, err)

	claims, err := iss.Validate(context.Background(), signed)
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, accountID, claims.AccountID)
	assert.Empty(t, claims.Username)
}

func TestNewHMACIssuerFromEnv(t *testing.T) {
	t.Run("uses HMAC_SECRET when set", func(t *testing.T) {
		t.Setenv("HMAC_SECRET", "env-provided-secret")
		iss, err := jwt.NewHMACIssuerFromEnv()
		require.NoError(t, err)
		require.NotNil(t, iss)

		// Round-trip works with the env-provided secret.
		token, err := iss.Issue(context.Background(), uuid.New(), "grace")
		require.NoError(t, err)
		_, err = iss.Validate(context.Background(), token)
		require.NoError(t, err)
	})

	t.Run("falls back to dev default when unset", func(t *testing.T) {
		t.Setenv("HMAC_SECRET", "")
		iss, err := jwt.NewHMACIssuerFromEnv()
		require.NoError(t, err)
		require.NotNil(t, iss)
	})
}
