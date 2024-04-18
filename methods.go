package grpcui

import (
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// AllMethodsForServices returns a slice that contains the method descriptors
// for all methods in the given services.
func AllMethodsForServices(descs []*desc.ServiceDescriptor) []*desc.MethodDescriptor {
	seen := map[string]struct{}{}
	var allMethods []*desc.MethodDescriptor
	for _, sd := range descs {
		if _, ok := seen[sd.GetFullyQualifiedName()]; ok {
			// duplicate
			continue
		}
		seen[sd.GetFullyQualifiedName()] = struct{}{}
		allMethods = append(allMethods, sd.GetMethods()...)
	}
	return allMethods
}

// AllMethodsForServer returns a slice that contains the method descriptors for
// all methods exposed by the given gRPC server.
func AllMethodsForServer(svr *grpc.Server) ([]*desc.MethodDescriptor, error) {
	svcs, err := grpcreflect.LoadServiceDescriptors(svr)
	if err != nil {
		return nil, err
	}
	var descs []*desc.ServiceDescriptor
	for _, sd := range svcs {
		descs = append(descs, sd)
	}
	return AllMethodsForServices(descs), nil
}

// AllMethodsViaReflection returns a slice that contains the method descriptors
// for all methods exposed by the server on the other end of the given
// connection. This returns an error if the server does not support service
// reflection. (See "google.golang.org/grpc/reflection" for more on service
// reflection.)
// This automatically skips the reflection service, since it is assumed this is not
// a desired inclusion.
func AllMethodsViaReflection(ctx context.Context, cc grpc.ClientConnInterface) ([]*desc.MethodDescriptor, error) {
	cli := grpcreflect.NewClientAuto(ctx, cc)
	svcNames, err := cli.ListServices()
	if err != nil {
		return nil, err
	}
	var descs []*desc.ServiceDescriptor
	for _, svcName := range svcNames {
		sd, err := cli.ResolveService(svcName)
		if err != nil {
			return nil, err
		}
		fullyQualifiedName := sd.GetFullyQualifiedName()
		if fullyQualifiedName == "grpc.reflection.v1alpha.ServerReflection" || fullyQualifiedName == "grpc.reflection.v1.ServerReflection" {
			continue // skip reflection service
		}
		descs = append(descs, sd)
	}
	return AllMethodsForServices(descs), nil
}

// AllMethodsViaInProcess returns a slice that contains the method descriptors
// for all methods exposed by the given server.
// This automatically skips the reflection service, since it is assumed this is not
// a desired inclusion.
func AllMethodsViaInProcess(svr reflection.GRPCServer) ([]*desc.MethodDescriptor, error) {
	sds, err := grpcreflect.LoadServiceDescriptors(svr)
	if err != nil {
		return nil, err
	}
	var descs []*desc.ServiceDescriptor
	for _, sd := range sds {
		fullyQualifiedName := sd.GetFullyQualifiedName()
		if fullyQualifiedName == "grpc.reflection.v1alpha.ServerReflection" || fullyQualifiedName == "grpc.reflection.v1.ServerReflection" {
			continue // skip reflection service
		}
		descs = append(descs, sd)
	}
	return AllMethodsForServices(descs), nil
}
