package grpc

import (
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	kitanypb "github.com/fromforgesoftware/go-kit/pb/anypb"
)

// ErrInvalidProtoType is deprecated: use kit/pb/anypb.ErrInvalidProtoType.
var ErrInvalidProtoType = kitanypb.ErrInvalidProtoType

// AnyToProto is deprecated: use kit/pb/anypb.AnyToProto. Kept as a thin
// shim so existing callers continue to compile while migrating.
func AnyToProto[P protoreflect.ProtoMessage](a *anypb.Any) (P, error) {
	return kitanypb.AnyToProto[P](a)
}
