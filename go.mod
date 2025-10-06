module github.com/fullstorydev/grpcui

go 1.23.0

toolchain go1.24.1

require (
	github.com/fullstorydev/grpcurl v1.9.3
	github.com/golang/protobuf v1.5.4
	github.com/jhump/protoreflect v1.17.0
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4
	golang.org/x/net v0.43.0
	golang.org/x/term v0.34.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231106174013-bbf56f31fb17
	google.golang.org/grpc v1.61.0
	google.golang.org/protobuf v1.36.10
)

require (
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	github.com/bufbuild/protocompile v0.14.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cncf/udpa/go v0.0.0-20220112060539-c52dc94e7fbe // indirect
	github.com/cncf/xds/go v0.0.0-20231109132714-523115ebc101 // indirect
	github.com/envoyproxy/go-control-plane v0.11.1 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.2 // indirect
	golang.org/x/oauth2 v0.27.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	google.golang.org/genproto v0.0.0-20231106174013-bbf56f31fb17 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20231106174013-bbf56f31fb17 // indirect
)

retract (
	v1.5.1 // Contains retractions only.
	v1.5.0 // Published accidentally.
)
