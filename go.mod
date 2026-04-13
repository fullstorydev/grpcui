module github.com/fullstorydev/grpcui

go 1.25.0

require (
	github.com/fullstorydev/grpcurl v1.9.3
	github.com/golang/protobuf v1.5.4
	github.com/jhump/protoreflect v1.18.0
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4
	golang.org/x/net v0.53.0
	golang.org/x/term v0.42.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240604185151-ef581f913117
	google.golang.org/grpc v1.66.2
	google.golang.org/protobuf v1.36.11
)

require (
	cel.dev/expr v0.15.0 // indirect
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20240423153145-555b57ec207b // indirect
	github.com/envoyproxy/go-control-plane v0.12.1-0.20240621013728-1eb8caab5155 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.4 // indirect
	github.com/jhump/protoreflect/v2 v2.0.0-beta.1 // indirect
	github.com/petermattis/goid v0.0.0-20260113132338-7c7de50cc741 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	golang.org/x/oauth2 v0.27.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240604185151-ef581f913117 // indirect
)

retract (
	v1.5.1 // Contains retractions only.
	v1.5.0 // Published accidentally.
)
