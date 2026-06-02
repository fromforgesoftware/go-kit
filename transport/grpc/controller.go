package grpc

type Controller interface {
	SD() ServiceDesc
}
