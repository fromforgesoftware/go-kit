package grpc_test

import (
	"testing"

	grpctestpb "github.com/fromforgesoftware/go-kit/proto/tb/grpctest/v1"
	transportgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestAnyToProto(t *testing.T) {
	want := &grpctestpb.Test{
		Name: "test",
	}

	anyTest, err := anypb.New(want)
	assert.NoError(t, err)

	got, err := transportgrpc.AnyToProto[*grpctestpb.Test](anyTest)
	assert.NoError(t, err)
	assert.Equal(t, want.Name, got.Name)
}
