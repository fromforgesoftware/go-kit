package gormdb

import (
	"context"
)

type deleter[R any] struct {
	db *DBClient
}

func NewDeleter[R any](db *DBClient) *deleter[R] {
	return &deleter[R]{
		db: db,
	}
}

func (d *deleter[R]) Delete(ctx context.Context, id string) error {
	var res R
	return d.db.WithContext(ctx).Delete(&res, "id = ?", id).Error
}

func (d *deleter[R]) Undelete(ctx context.Context, id string) error {
	var res R
	return d.db.WithContext(ctx).Unscoped().Model(&res).
		Where("id = ?", id).Update("deleted_at", nil).Error
}
