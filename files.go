package grpcui

import (
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	"github.com/fullstorydev/grpcurl"
)

// AllFilesViaReflection returns a slice that contains the file descriptors
// for all methods exposed by the server on the other end of the given
// connection. This returns an error if the server does not support service
// reflection. (See "google.golang.org/grpc/reflection" for more on service
// reflection.)
func AllFilesViaReflection(ctx context.Context, cc grpc.ClientConnInterface) ([]*desc.FileDescriptor, error) {
	stub := rpb.NewServerReflectionClient(cc)
	cli := grpcreflect.NewClient(ctx, stub)
	cli.ListServices()
	source := grpcurl.DescriptorSourceFromServer(ctx, cli)
	return grpcurl.GetAllFiles(source)
}
