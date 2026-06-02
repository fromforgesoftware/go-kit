package postgres

import (
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
	"gorm.io/gorm"
)

type Timestamps struct {
	ECreatedAt time.Time      `gorm:"column:created_at;type:timestamp;autoCreateTime:true"`
	EUpdatedAt time.Time      `gorm:"column:updated_at;type:timestamp;autoUpdateTime:true"`
	EDeletedAt gorm.DeletedAt `gorm:"column:deleted_at;type:timestamp"`
}

func TimestampFromTimes(createdAt, updatedAt time.Time, deletedAt *time.Time) Timestamps {
	var delAt gorm.DeletedAt
	if deletedAt != nil {
		delAt.Time = *deletedAt
		delAt.Valid = true
	}
	return Timestamps{
		ECreatedAt: createdAt,
		EUpdatedAt: updatedAt,
		EDeletedAt: delAt,
	}
}

func (t *Timestamps) CreatedAt() time.Time {
	return t.ECreatedAt
}

func (t *Timestamps) UpdatedAt() time.Time {
	return t.EUpdatedAt
}

func (t *Timestamps) DeletedAt() *time.Time {
	if !t.EDeletedAt.Valid {
		return nil
	}
	return &t.EDeletedAt.Time
}

type Model struct {
	EID string `gorm:"column:id;type:uuid;default:uuid_generate_v4();primaryKey"`
	Timestamps
}

func (d *Model) ID() string {
	return d.EID
}

func (d *Model) LID() string {
	return ""
}

func ModelFromResource(r resource.Resource) Model {
	return Model{
		EID:        r.ID(),
		Timestamps: TimestampFromTimes(r.CreatedAt(), r.UpdatedAt(), r.DeletedAt()),
	}
}
