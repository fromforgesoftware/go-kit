package resource

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/fromforgesoftware/go-kit/pb/timepb"
	resourcepb "github.com/fromforgesoftware/go-kit/proto/tb/v1"
	"github.com/fromforgesoftware/go-kit/slicesx"
)

func ToProto(r Resource) *resourcepb.Resource {
	if r == nil {
		return &resourcepb.Resource{}
	}

	return &resourcepb.Resource{
		Id:        r.ID(),
		CreatedAt: timestamppb.New(r.CreatedAt()),
		UpdatedAt: timestamppb.New(r.UpdatedAt()),
		DeletedAt: timepb.TimePointerToTimestamp(r.DeletedAt()),
		Type:      r.Type().String(),
	}
}

func FromProto(r *resourcepb.Resource) Resource {
	if r == nil {
		return nil
	}
	return &resource{
		id:        r.GetId(),
		createdAt: r.GetCreatedAt().AsTime(),
		updatedAt: r.GetUpdatedAt().AsTime(),
		deletedAt: timepb.TimestampToTimePointer(r.GetDeletedAt()),
		kind:      Type(r.GetType()),
	}
}

func IdentifiersToProto(rs []Identifier) []*resourcepb.ResourceIdentifier {
	return slicesx.Map(rs, IdentifierToProto)
}

func IdentifierToProto(r Identifier) *resourcepb.ResourceIdentifier {
	if r == nil {
		return &resourcepb.ResourceIdentifier{}
	}
	return &resourcepb.ResourceIdentifier{
		Id:   r.ID(),
		Type: r.Type().String(),
	}
}

func IdentifierFromProto(r *resourcepb.ResourceIdentifier) Identifier {
	if r == nil {
		return nil
	}
	return NewIdentifier(r.Id, Type(r.Type))
}
