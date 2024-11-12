// Command testsvr is a gRPC server for testing grpcui. It has a wide gRPC API
// that exercises every combination of form inputs that the web UI can handle.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/jhump/protoreflect/desc/sourceinfo"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

//go:generate protoc --go_out=. --go-grpc_out=. --gosrcinfo_out=. test.proto

func main() {
	port := flag.Int("port", 0, "Port on which to listen")
	flag.Parse()

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Invalid argument(s): %v\n", flag.Args())
		fmt.Fprintf(os.Stderr, "This program does not take any arguments\n")
		os.Exit(1)
	}

	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create network listener: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Listening on %v\n", l.Addr().(*net.TCPAddr).String())

	svr := grpc.NewServer()
	RegisterKitchenSinkServer(svr, &testSvr{})
	refSvc := reflection.NewServerV1(reflection.ServerOptions{
		Services:           svr,
		DescriptorResolver: sourceinfo.GlobalFiles,
		ExtensionResolver:  sourceinfo.GlobalFiles,
	})
	reflectionpb.RegisterServerReflectionServer(svr, refSvc)
	if err := svr.Serve(l); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start gRPC server: %v\n", err)
		os.Exit(1)
	}
}

type testSvr struct {
	UnimplementedKitchenSinkServer
}

func (s testSvr) Ping(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) Exchange(ctx context.Context, m *TestMessage) (*TestMessage, error) {
	if headers, ok := metadata.FromIncomingContext(ctx); ok {
		hdrs := metadata.MD{}
		tlrs := metadata.MD{}
		for k, v := range headers {
			if strings.HasSuffix(k, "-t") {
				tlrs[k] = v
			} else {
				hdrs[k] = v
			}
		}
		_ = grpc.SendHeader(ctx, hdrs)
		_ = grpc.SetTrailer(ctx, tlrs)
	}
	return m, nil
}

func (s testSvr) UploadMany(stream KitchenSink_UploadManyServer) error {
	if headers, ok := metadata.FromIncomingContext(stream.Context()); ok {
		hdrs := metadata.MD{}
		tlrs := metadata.MD{}
		for k, v := range headers {
			if strings.HasSuffix(k, "-t") {
				tlrs[k] = v
			} else {
				hdrs[k] = v
			}
		}
		_ = stream.SendHeader(hdrs)
		stream.SetTrailer(tlrs)
	}

	var lastReq *TestMessage
	count := 0
	for {
		var err error
		m, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		lastReq = m
		count++
	}
	if lastReq == nil {
		return status.Error(codes.InvalidArgument, "must provide at least one request message")
	}
	lastReq.NeededNumA = proto.Float32(float32(count))
	return stream.SendAndClose(lastReq)
}

func (s testSvr) DownloadMany(m *TestMessage, stream KitchenSink_DownloadManyServer) error {
	if headers, ok := metadata.FromIncomingContext(stream.Context()); ok {
		hdrs := metadata.MD{}
		tlrs := metadata.MD{}
		for k, v := range headers {
			if strings.HasSuffix(k, "-t") {
				tlrs[k] = v
			} else {
				hdrs[k] = v
			}
		}
		_ = stream.SendHeader(hdrs)
		stream.SetTrailer(tlrs)
	}

	numResponses := m.Numbers.GetNeededNum_1()
	for i := int32(0); i < numResponses; i++ {
		err := stream.Send(m)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s testSvr) DoManyThings(stream KitchenSink_DoManyThingsServer) error {
	if headers, ok := metadata.FromIncomingContext(stream.Context()); ok {
		hdrs := metadata.MD{}
		tlrs := metadata.MD{}
		for k, v := range headers {
			if strings.HasSuffix(k, "-t") {
				tlrs[k] = v
			} else {
				hdrs[k] = v
			}
		}
		_ = stream.SendHeader(hdrs)
		stream.SetTrailer(tlrs)
	}

	for {
		m, err := stream.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		err = stream.Send(m)
		if err != nil {
			return err
		}
	}
}

func (s testSvr) Fail(req *FailRequest, stream KitchenSink_FailServer) error {
	msg := &TestMessage{
		Person: &Person{
			Id:   proto.Uint64(123),
			Name: proto.String("123"),
		},
		State:      State_COMPLETE.Enum(),
		NeededNumA: proto.Float32(1.23),
		NeededNumB: proto.Float64(12.3),
	}
	for i := int32(0); i < req.GetNumResponses(); i++ {
		msg.OpaqueId = []byte{byte(i + 1)}
		if err := stream.Send(msg); err != nil {
			return err
		}
	}
	statProto := spb.Status{
		Code:    int32(req.GetCode()),
		Message: req.GetMessage(),
		Details: req.GetDetails(),
	}
	return status.FromProto(&statProto).Err()
}

func (s testSvr) SendTimestamp(context.Context, *timestamppb.Timestamp) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendDuration(context.Context, *durationpb.Duration) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendAny(context.Context, *anypb.Any) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendStruct(context.Context, *structpb.Struct) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendValue(context.Context, *structpb.Value) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendListValue(context.Context, *structpb.ListValue) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendBytes(context.Context, *wrapperspb.BytesValue) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendString(context.Context, *wrapperspb.StringValue) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendBool(context.Context, *wrapperspb.BoolValue) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendDouble(context.Context, *wrapperspb.DoubleValue) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendFloat(context.Context, *wrapperspb.FloatValue) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendInt32(context.Context, *wrapperspb.Int32Value) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendInt64(context.Context, *wrapperspb.Int64Value) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendUInt32(context.Context, *wrapperspb.UInt32Value) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendUInt64(context.Context, *wrapperspb.UInt64Value) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s testSvr) SendMultipleTimestamp(stream KitchenSink_SendMultipleTimestampServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleDuration(stream KitchenSink_SendMultipleDurationServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleAny(stream KitchenSink_SendMultipleAnyServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleStruct(stream KitchenSink_SendMultipleStructServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleValue(stream KitchenSink_SendMultipleValueServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleListValue(stream KitchenSink_SendMultipleListValueServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleBytes(stream KitchenSink_SendMultipleBytesServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleString(stream KitchenSink_SendMultipleStringServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleBool(stream KitchenSink_SendMultipleBoolServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleDouble(stream KitchenSink_SendMultipleDoubleServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleFloat(stream KitchenSink_SendMultipleFloatServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleInt32(stream KitchenSink_SendMultipleInt32Server) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleInt64(stream KitchenSink_SendMultipleInt64Server) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleUInt32(stream KitchenSink_SendMultipleUInt32Server) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleUInt64(stream KitchenSink_SendMultipleUInt64Server) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&emptypb.Empty{})
		} else if err != nil {
			return err
		}
	}
}
