package standalone

import (
	"encoding/json"
	"google.golang.org/protobuf/encoding/protojson"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

func TestRequest_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		request ExampleRequest
	}{
		{
			name: "Full request, complex data",
			request: ExampleRequest{
				TimeoutSeconds: 1.0,
				Metadata: []ExampleMetadataPair{
					{
						"key",
						"value",
					},
				},
				Data: map[string]interface{}{
					"field": "fieldValue",
				},
			},
		},
		{
			name: "Full request, string data",
			request: ExampleRequest{
				TimeoutSeconds: 0.002,
				Metadata: []ExampleMetadataPair{
					{
						"key",
						"value",
					},
				},
				Data: "a string",
			},
		},
		{
			name: "No timeout",
			request: ExampleRequest{
				Metadata: []ExampleMetadataPair{
					{
						"key",
						"value",
					},
				},
				Data: map[string]interface{}{
					"field": "fieldValue",
				},
			},
		},
		{
			name: "no data",
			request: ExampleRequest{
				TimeoutSeconds: 0.001,
				Metadata: []ExampleMetadataPair{
					{
						"key",
						"value",
					},
				},
			},
		},
		{
			name: "only timeout",
			request: ExampleRequest{
				TimeoutSeconds: 86400,
			},
		},
		{
			name: "only data",
			request: ExampleRequest{
				Data: "some data",
			},
		},
		{
			name: "only metadata",
			request: ExampleRequest{
				Metadata: []ExampleMetadataPair{
					{
						"a",
						"b",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			marshal, err := json.Marshal(test.request)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			t.Logf("marshaled: %q", string(marshal))

			var unmarshaled ExampleRequest
			err = json.Unmarshal(marshal, &unmarshaled)
			if err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			if test.request.TimeoutSeconds != unmarshaled.TimeoutSeconds {
				t.Fatalf("round trip failure: want %v, got %v", test.request.TimeoutSeconds, unmarshaled.TimeoutSeconds)
			}
			if !reflect.DeepEqual(test.request.Metadata, unmarshaled.Metadata) {
				t.Fatalf("round trip failure: want %#v, got %#v", test.request.Metadata, unmarshaled.Metadata)
			}
			if !reflect.DeepEqual(test.request.Data, unmarshaled.Data) {
				t.Fatalf("round trip failure: want %#v, got %#v", test.request.Data, unmarshaled.Data)
			}
		})
	}
}

type testRequest struct {
	TimeoutSeconds float32               `json:"timeout_seconds"`
	Metadata       []ExampleMetadataPair `json:"metadata"`
	Data           json.RawMessage       `json:"data"`
}

func TestRequest_MarshalJSON_ProtoData(t *testing.T) {
	tests := []struct {
		name    string
		request ExampleRequest
		want    *descriptor.DescriptorProto
	}{
		{
			name: "FieldDescriptorProto data",
			request: ExampleRequest{
				Data: &descriptor.DescriptorProto{
					Name: proto.String("a name"),
					Field: []*descriptor.FieldDescriptorProto{
						{
							Name:   proto.String("another name"),
							Number: proto.Int32(1337),
						},
					},
				},
			},
			want: &descriptor.DescriptorProto{
				Name: proto.String("a name"),
				Field: []*descriptor.FieldDescriptorProto{
					{
						Name:   proto.String("another name"),
						Number: proto.Int32(1337),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			marshal, err := json.Marshal(test.request)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			t.Logf("marshaled: %q", string(marshal))

			var raw testRequest
			err = json.Unmarshal(marshal, &raw)
			if err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			b, _ := raw.Data.MarshalJSON()
			var got descriptor.DescriptorProto
			err = protojson.Unmarshal(b, &got)
			if err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			if !proto.Equal(test.want, &got) {
				t.Fatal("Decoded version does not match")
			}
		})
	}
}
