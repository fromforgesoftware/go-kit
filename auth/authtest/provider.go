package authtest

import (
	"context"

	"github.com/fromforgesoftware/go-kit/auth/provider"
)

// mockProvider implements provider.Provider for testing
type mockProvider struct {
	uid   string
	email string
}

// NewMockProvider creates a new mock authentication provider for testing
func NewMockProvider(uid, email string) *mockProvider {
	return &mockProvider{
		uid:   uid,
		email: email,
	}
}

func (m *mockProvider) ValidateToken(ctx context.Context, token string) (*provider.UserInfo, error) {
	return &provider.UserInfo{
		ProviderUID: m.uid,
		Email:       m.email,
	}, nil
}

func (m *mockProvider) Name() string {
	return "mock"
}
