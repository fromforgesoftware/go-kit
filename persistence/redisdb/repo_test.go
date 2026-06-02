//go:build integration
// +build integration

package redisdb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/go-kit/persistence/redisdb"
	"github.com/fromforgesoftware/go-kit/persistence/redisdb/redistest"
)

func TestNewRepo(t *testing.T) {
	db := redistest.GetDB(t)

	t.Run("new repo with redis client return repo", func(t *testing.T) {
		repo, err := redisdb.NewRepo(db.Client)
		assert.NoError(t, err)
		assert.NotNil(t, repo)
	})

	t.Run("new repo with nil redis client return error", func(t *testing.T) {
		repo, err := redisdb.NewRepo(nil)
		assert.EqualError(t, err, redisdb.ErrRedisMissingRedisConn.Error())
		assert.Nil(t, repo)
	})
}
