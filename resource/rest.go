package resource

import (
	"reflect"
	"time"

	"github.com/fromforgesoftware/go-kit/instance"
)

type RestDTO struct {
	RID         string        `jsonapi:"primary"`
	RLID        string        `jsonapi:"client-id,omitempty"`
	RType       Type          `jsonapi:"type"`
	RTimestamps *TimestampDTO `jsonapi:"attr,timestamps,omitempty"`
}

func ToRestDTO(r Resource) RestDTO {
	if r == nil {
		return RestDTO{}
	}

	return RestDTO{
		RID:         r.ID(),
		RLID:        r.LID(),
		RType:       r.Type(),
		RTimestamps: TimestampToDTO(r),
	}
}

func (dto *RestDTO) ID() string {
	return dto.RID
}

func (dto *RestDTO) LID() string {
	return dto.RLID
}

func (dto *RestDTO) Type() Type {
	return dto.RType
}

func (dto *RestDTO) CreatedAt() time.Time {
	if dto.RTimestamps == nil {
		return time.Time{}
	}
	return dto.RTimestamps.RCreatedAt
}

func (dto *RestDTO) UpdatedAt() time.Time {
	if dto.RTimestamps == nil {
		return time.Time{}
	}
	return dto.RTimestamps.RUpdatedAt
}

func (dto *RestDTO) DeletedAt() *time.Time {
	if dto.RTimestamps == nil || dto.RTimestamps.RDeletedAt == nil || dto.RTimestamps.RDeletedAt.IsZero() {
		return nil
	}

	return dto.RTimestamps.RDeletedAt
}

type TimestampDTO struct {
	RCreatedAt time.Time  `jsonapi:"attr,createdAt"`
	RUpdatedAt time.Time  `jsonapi:"attr,updatedAt"`
	RDeletedAt *time.Time `jsonapi:"attr,deletedAt,omitempty"`
}

func TimestampToDTO(t Timestamps) *TimestampDTO {
	if t == nil {
		return &TimestampDTO{}
	}

	return &TimestampDTO{
		RCreatedAt: t.CreatedAt(),
		RUpdatedAt: t.UpdatedAt(),
		RDeletedAt: t.DeletedAt(),
	}
}

type identifierDTO struct {
	RID   string `jsonapi:"primary"`
	RLID  string `jsonapi:"client-id,omitempty"`
	RType Type   `jsonapi:"type"`
}

func (dto *identifierDTO) ID() string {
	return dto.RID
}

func (dto *identifierDTO) LID() string {
	return dto.RLID
}

func (dto *identifierDTO) Type() Type {
	return dto.RType
}

func RestIdentifierToDTO(r Identifier) *identifierDTO {
	if r == nil {
		return nil
	}

	return &identifierDTO{
		RID:   r.ID(),
		RType: r.Type(),
	}
}

type (
	fullOrOnlyIdentifierDTOConfig struct {
		// TODO: when we're able to completely remove this as json api creates/updates
		// can be updated to automatically don't return any includeds, we'll remove this option/config
		//nolint:misspell //we want includeds as in jsonapi form, not includes
		isCreateOrUpdateOpt bool
	}
	FullOrOnlyIdentifierDTOConfigOpt func(c *fullOrOnlyIdentifierDTOConfig)
)

func WithCreateOp(c *fullOrOnlyIdentifierDTOConfig) {
	c.isCreateOrUpdateOpt = true
}

func WithUpdateOp(c *fullOrOnlyIdentifierDTOConfig) {
	c.isCreateOrUpdateOpt = true
}

func FullOrOnlyIdentifierDTO[I, O Resource](
	identifier Identifier, mapFunc func(I) O,
	opts ...FullOrOnlyIdentifierDTOConfigOpt,
) O {
	c := new(fullOrOnlyIdentifierDTOConfig)
	for _, opt := range opts {
		opt(c)
	}

	if identifier == nil {
		var zero O
		return zero
	}

	val, ok := identifier.(I)
	if !ok || c.isCreateOrUpdateOpt {
		res := instance.New[O]()

		resValues := reflect.ValueOf(res)
		if resValues.Kind() == reflect.Pointer {
			resValues = resValues.Elem()
		}
		restDTO := resValues.FieldByName("RestDTO")
		if !restDTO.IsValid() {
			panic("dto does not wrap RestDTO struct")
		}
		restDTO.FieldByName("RID").SetString(identifier.ID())
		restDTO.FieldByName("RType").SetString(identifier.Type().String())

		return res
	}

	return mapFunc(val)
}
