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

// MutableModel is the embeddable base for mutable resources that are HARD-deleted:
// a UUID primary key + created_at + updated_at, but NO deleted_at. Because it does
// not carry gorm.DeletedAt, queries are not soft-delete scoped and a delete removes
// the row for good. Use Model instead when you want soft-delete.
type MutableModel struct {
	EID        string    `gorm:"column:id;type:uuid;default:uuid_generate_v4();primaryKey"`
	ECreatedAt time.Time `gorm:"column:created_at;type:timestamp;autoCreateTime:true"`
	EUpdatedAt time.Time `gorm:"column:updated_at;type:timestamp;autoUpdateTime:true"`
}

func (m *MutableModel) ID() string  { return m.EID }
func (m *MutableModel) LID() string { return "" }

func (m *MutableModel) CreatedAt() time.Time { return m.ECreatedAt }
func (m *MutableModel) UpdatedAt() time.Time { return m.EUpdatedAt }

func (m *MutableModel) DeletedAt() *time.Time { return nil }

func MutableModelFromResource(r resource.Resource) MutableModel {
	return MutableModel{
		EID:        r.ID(),
		ECreatedAt: r.CreatedAt(),
		EUpdatedAt: r.UpdatedAt(),
	}
}

// ImmutableModel is the embeddable base for append-only / immutable resources
// (published snapshots, ledger or schedule rows): a UUID primary key + created_at
// only. It has NO updated_at and NO deleted_at, so — unlike Model — queries are not
// soft-delete scoped and rows are hard-deleted (typically via ON DELETE CASCADE).
// UpdatedAt aliases CreatedAt so the type still satisfies the resource timestamps
// contract.
type ImmutableModel struct {
	EID        string    `gorm:"column:id;type:uuid;default:uuid_generate_v4();primaryKey"`
	ECreatedAt time.Time `gorm:"column:created_at;type:timestamp;autoCreateTime:true"`
}

func (m *ImmutableModel) ID() string  { return m.EID }
func (m *ImmutableModel) LID() string { return "" }

func (m *ImmutableModel) CreatedAt() time.Time { return m.ECreatedAt }
func (m *ImmutableModel) UpdatedAt() time.Time { return m.ECreatedAt }

func (m *ImmutableModel) DeletedAt() *time.Time { return nil }

func ImmutableModelFromResource(r resource.Resource) ImmutableModel {
	return ImmutableModel{
		EID:        r.ID(),
		ECreatedAt: r.CreatedAt(),
	}
}
