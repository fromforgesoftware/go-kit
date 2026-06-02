//go:build integration
// +build integration

package redisdb_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/go-kit/persistence/redisdb"
	"github.com/fromforgesoftware/go-kit/persistence/redisdb/redistest"
)

type clientTestWant struct {
	res    any
	err    error
	panics bool
}
type clientTest struct {
	name       string
	execCliOps []func(cli *redisdb.Client) (any, error)
	want       []*clientTestWant
}

//nolint:funlen // test
func TestNewClient(t *testing.T) {
	t.Parallel()

	var (
		ctx = context.Background()
		cli = initConfig(t)
	)
	t.Cleanup(func() { cli.Close() })

	tests := []*clientTest{
		{
			name: "SET kv with an expiry and GET k before expiry returns v",
			execCliOps: []func(cli *redisdb.Client) (any, error){
				func(cli *redisdb.Client) (any, error) {
					return cli.Set(ctx, "x", "y", 60*time.Second).Result()
				},
				func(cli *redisdb.Client) (any, error) {
					return cli.Get(ctx, "x").Result()
				},
			},
			want: []*clientTestWant{
				{
					"OK", nil, false,
				},
				{
					"y", nil, false,
				},
			},
		},
		{
			name: "find unexisting key returns empty result with redis.Nil err",
			execCliOps: []func(cli *redisdb.Client) (any, error){
				func(cli *redisdb.Client) (any, error) {
					return cli.Get(ctx, "unexistingkey").Result()
				},
			},
			want: []*clientTestWant{
				{
					"", redis.Nil, false,
				},
			},
		},
		{
			name: "change db index and find key which was set in another db index results in not found",
			execCliOps: []func(cli *redisdb.Client) (any, error){
				func(cli *redisdb.Client) (any, error) {
					return cli.Do(ctx, "SELECT", 1).Result()
				},
				func(cli *redisdb.Client) (any, error) {
					return cli.Get(ctx, "x").Result()
				},
			},
			want: []*clientTestWant{
				{
					"OK", nil, false,
				},
				{
					"", redis.Nil, false,
				},
			},
		},
		{
			name: "In a pipeline execute INCR counter and EXPIRE",
			execCliOps: []func(cli *redisdb.Client) (any, error){
				func(cli *redisdb.Client) (any, error) {
					pipe := cli.Pipeline()
					incr := pipe.Incr(ctx, "pipeline_counter")
					pipe.Expire(ctx, "pipeline_counter", time.Hour)
					_, err := pipe.Exec(ctx)
					return incr.Val(), err
				},
			},
			want: []*clientTestWant{
				{
					int64(1), nil, false,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for i, op := range test.execCliOps {
				if test.want[i].panics {
					assert.True(t, assert.Panics(t, func() { op(cli) }))
					continue
				}

				gotVal, gotErr := op(cli)
				assert.ErrorIs(t, gotErr, test.want[i].err)
				assert.Equal(t, test.want[i].res, gotVal)
			}
		})
	}
}

func initConfig(t *testing.T) *redisdb.Client {
	t.Helper()

	db := redistest.GetDB(t)
	return db.Client
}
