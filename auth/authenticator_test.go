package auth_test

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/auth"
	"github.com/fromforgesoftware/go-kit/firebase/firebasetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

type mockTokenClaims struct {
	id     string
	claims map[string]interface{}
}

func (m *mockTokenClaims) ID() string                     { return m.id }
func (m *mockTokenClaims) Email() string                  { return "" }
func (m *mockTokenClaims) EmailVerified() bool            { return true }
func (m *mockTokenClaims) Claims() map[string]interface{} { return m.claims }

// Tests

func TestTokenParsing(t *testing.T) {
	// Create a dummy JWT
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"user123","exp":1600000000,"custom":"value"}`))
	tokenStr := header + "." + claims + "."

	t.Run("NewToken parses JWT", func(t *testing.T) {
		token, err := auth.NewToken(tokenStr, auth.TokenTypeFirebase, nil)
		require.NoError(t, err)
		assert.Equal(t, "user123", token.Claims().Subject())
		assert.Equal(t, "value", token.Claims().Get("custom"))
	})
}

func TestAuthenticator(t *testing.T) {
	// Check GRPC Authenticator flow
	extractor := auth.NewGrpcTokenExtractor()
	injector := auth.NewTokenContextInjector()

	mockClient := firebasetest.NewClient(t)
	mockAuth := firebasetest.NewAuthAPI(t)
	mockClient.On("Auth").Return(mockAuth)

	authenticator := auth.NewGrpcAuthenticator(auth.GrpcAuthenticatorParams{
		TokenExtractor:  extractor,
		ContextInjector: injector,
		FirebaseClient:  mockClient,
	})

	ctx := context.Background()
	// Construct valid token structure
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"original","exp":0}`))
	tokenStr := header + "." + claims + "."

	md := metadata.Pairs(auth.AuthorizationHeader, "Bearer "+tokenStr)

	t.Run("Authenticate success", func(t *testing.T) {
		expectedClaims := &mockTokenClaims{
			id: "verified-user",
			claims: map[string]interface{}{
				"exp": float64(time.Now().Add(time.Hour).Unix()),
				"foo": "bar",
			},
		}

		mockAuth.On("VerifyIDToken", mock.Anything, tokenStr).Return(expectedClaims, nil)

		newCtx, err := authenticator.Authenticate(ctx, md)
		require.NoError(t, err)

		// Check context has token
		token := auth.TokenFromCtx(newCtx)
		require.NotNil(t, token)
		assert.Equal(t, "verified-user", token.Claims().Subject())
		assert.Equal(t, "bar", token.Claims().Get("foo"))
	})
}
