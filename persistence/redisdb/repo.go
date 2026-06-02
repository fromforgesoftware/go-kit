package redisdb

import (
	"errors"
)

var ErrRedisMissingRedisConn = errors.New("redis connection is nil")

type Repo struct {
	DB *Client
}

func NewRepo(db *Client) (*Repo, error) {
	if db == nil {
		return nil, ErrRedisMissingRedisConn
	}
	return &Repo{
		DB: db,
	}, nil
}
