// Package anypb provides helpers for working with protobuf Any messages.
// It lives outside any transport package so REST, gRPC and message-broker
// consumers can all pull from one place without importing transport code.
package anypb

import (
	"errors"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/fromforgesoftware/go-kit/instance"
)

// ErrInvalidProtoType is returned when the Any message does not contain
// the expected proto type.
var ErrInvalidProtoType = errors.New("message is not valid type")

// AnyToProto unmarshals a protobuf Any message into the specified proto type.
//
// The type parameter P must be a pointer to a proto message (e.g. *pb.User).
//
// Example:
//
//	anyMsg := &anypb.Any{...}
//	user, err := anypb.AnyToProto[*pb.User](anyMsg)
func AnyToProto[P protoreflect.ProtoMessage](a *anypb.Any) (P, error) {
	if a == nil {
		var zero P
		return zero, nil
	}

	p := instance.New[P]()

	if !a.MessageIs(p) {
		var zero P
		return zero, ErrInvalidProtoType
	}

	if err := a.UnmarshalTo(p); err != nil {
		var zero P
		return zero, err
	}
	return p, nil
}
