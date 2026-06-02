//go:build integration
// +build integration

package firebase_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fromforgesoftware/go-kit/firebase"
	"github.com/fromforgesoftware/go-kit/sops"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: These tests require valid Firebase credentials using SOPS.
// Usage: go test -tags=integration -v ./...

func TestUserManagement(t *testing.T) {
	// Attempt to load credentials from SOPS
	// If it fails (e.g. access denied), we skip the test.
	loader := sops.NewSOPSEnvVarLoader()
	if err := loader.LoadEnvFromFile(t, filepath.Join("testdata", "integ.yaml")); err != nil {
		t.Skipf("Skipping integration test: sops load failed (check KMS creds): %v", err)
	}

	var (
		cli = firebase.NewClient()
		ctx = context.Background()
	)

	uid := uuid.NewString()
	email := "integ-test-" + uid + "@example.com"

	user := firebase.NewUser(
		email,
		"password123",
		"Integration",
		firebase.WithID(uid),
		firebase.WithUserLastName("Test"),
		firebase.WithUserEmailVerified(true),
	)

	t.Cleanup(func() {
		_ = cli.UserManagement().Delete(ctx, uid)
	})

	t.Run("create user", func(t *testing.T) {
		created, err := cli.UserManagement().Create(ctx, user)
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Equal(t, uid, created.ID())
		assert.Equal(t, email, created.Email())
		assert.Equal(t, "Integration", created.FirstName())
		assert.Equal(t, "Test", created.LastName())
		assert.True(t, created.EmailVerified())
	})

	t.Run("get user by id", func(t *testing.T) {
		found, err := cli.UserManagement().Get(ctx, uid)
		require.NoError(t, err)
		assert.Equal(t, uid, found.ID())
	})

	t.Run("get user by email", func(t *testing.T) {
		found, err := cli.UserManagement().GetByEmail(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, uid, found.ID())
		assert.Equal(t, email, found.Email())
	})

	t.Run("update user", func(t *testing.T) {
		updatedNameUser := firebase.NewUser(
			email,
			"",
			"Updated",
			firebase.WithUserLastName("TestUpdated"),
			firebase.WithUserEmailVerified(false),
		)

		updated, err := cli.UserManagement().Update(ctx, uid, updatedNameUser)
		require.NoError(t, err)
		assert.Equal(t, "Updated", updated.FirstName())
		assert.Equal(t, "TestUpdated", updated.LastName())
		assert.False(t, updated.EmailVerified())
	})

	t.Run("delete user", func(t *testing.T) {
		err := cli.UserManagement().Delete(ctx, uid)
		require.NoError(t, err)

		_, err = cli.UserManagement().Get(ctx, uid)
		require.Error(t, err)
		t.Logf("Get deleted user error: %v", err)
		assert.True(t,
			strings.Contains(err.Error(), "NOT_FOUND") ||
				strings.Contains(strings.ToLower(err.Error()), "not found") ||
				strings.Contains(strings.ToLower(err.Error()), "no user record found") ||
				strings.Contains(strings.ToLower(err.Error()), "no user exists"),
		)
	})
}
