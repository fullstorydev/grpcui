// Command testsvr is a gRPC server for testing grpcui. It has a wide gRPC API
// that exercises every combination of form inputs that the web UI can handle.
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/struct"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

//go:generate protoc --go_out=plugins=grpc:. test.proto

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
	reflection.Register(svr)
	if err := svr.Serve(l); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start gRPC server: %v\n", err)
		os.Exit(1)
	}
}

type testSvr struct{}

func (s testSvr) Ping(context.Context, *empty.Empty) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) Exchange(ctx context.Context, m *TestMessage) (*TestMessage, error) {
	if headers, ok := metadata.FromIncomingContext(ctx); ok {
		hdrs := metadata.MD{}
		tlrs := metadata.MD{}
		for k, v := range headers {
			if strings.HasSuffix("-t", k) {
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
			if strings.HasSuffix("-t", k) {
				tlrs[k] = v
			} else {
				hdrs[k] = v
			}
		}
		_ = stream.SendHeader(hdrs)
		stream.SetTrailer(tlrs)
	}

	var m *TestMessage
	count := 0
	for {
		var err error
		m, err = stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		count++
	}
	if m == nil {
		return status.Error(codes.InvalidArgument, "must provide at least one request message")
	}
	m.NeededNumA = proto.Float32(float32(count))
	return stream.SendAndClose(m)
}

func (s testSvr) DownloadMany(m *TestMessage, stream KitchenSink_DownloadManyServer) error {
	if headers, ok := metadata.FromIncomingContext(stream.Context()); ok {
		hdrs := metadata.MD{}
		tlrs := metadata.MD{}
		for k, v := range headers {
			if strings.HasSuffix("-t", k) {
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
			if strings.HasSuffix("-t", k) {
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

func (s testSvr) SendTimestamp(context.Context, *timestamp.Timestamp) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendDuration(context.Context, *duration.Duration) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendAny(context.Context, *any.Any) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendStruct(context.Context, *structpb.Struct) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendValue(context.Context, *structpb.Value) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendListValue(context.Context, *structpb.ListValue) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendBytes(context.Context, *wrappers.BytesValue) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendString(context.Context, *wrappers.StringValue) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendBool(context.Context, *wrappers.BoolValue) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendDouble(context.Context, *wrappers.DoubleValue) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendFloat(context.Context, *wrappers.FloatValue) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendInt32(context.Context, *wrappers.Int32Value) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendInt64(context.Context, *wrappers.Int64Value) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendUInt32(context.Context, *wrappers.UInt32Value) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendUInt64(context.Context, *wrappers.UInt64Value) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (s testSvr) SendMultipleTimestamp(stream KitchenSink_SendMultipleTimestampServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleDuration(stream KitchenSink_SendMultipleDurationServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleAny(stream KitchenSink_SendMultipleAnyServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleStruct(stream KitchenSink_SendMultipleStructServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleValue(stream KitchenSink_SendMultipleValueServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleListValue(stream KitchenSink_SendMultipleListValueServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleBytes(stream KitchenSink_SendMultipleBytesServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleString(stream KitchenSink_SendMultipleStringServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleBool(stream KitchenSink_SendMultipleBoolServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleDouble(stream KitchenSink_SendMultipleDoubleServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleFloat(stream KitchenSink_SendMultipleFloatServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleInt32(stream KitchenSink_SendMultipleInt32Server) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleInt64(stream KitchenSink_SendMultipleInt64Server) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleUInt32(stream KitchenSink_SendMultipleUInt32Server) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}

func (s testSvr) SendMultipleUInt64(stream KitchenSink_SendMultipleUInt64Server) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&empty.Empty{})
		} else if err != nil {
			return err
		}
	}
}
