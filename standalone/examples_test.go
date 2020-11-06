package standalone

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/stretchr/testify/assert"
)

func TestRequest_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		request ExampleRequest
	}{
		{
			name: "Full request, complex data",
			request: ExampleRequest{
				Timeout: 1 * time.Second,
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
				Timeout: 2 * time.Millisecond,
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
				Timeout: 1 * time.Millisecond,
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
				Timeout: 24 * time.Hour,
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

			assert.Equal(t, test.request.Timeout, unmarshaled.Timeout)
			assert.Equal(t, test.request.Metadata, unmarshaled.Metadata)

			b, _ := unmarshaled.Data.(json.RawMessage).MarshalJSON()
			var dataUnmarshaled interface{}
			err = json.Unmarshal(b, &dataUnmarshaled)
			if err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			assert.Equal(t, test.request.Data, dataUnmarshaled)
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
			err = jsonpb.Unmarshal(bytes.NewReader(b), &got)
			if err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			if !proto.Equal(test.want, &got) {
				t.Fatal("Decoded version does not match")
			}
		})
	}
}

func TestRequest_UnmarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Invalid timeout",
			input: "{\"timeout_seconds\": \"1s\"}",
		},
		{
			name:  "Top level not an object",
			input: "\"boom\"",
		},
		{
			name:  "Invalid metadata",
			input: "{\"metadata\": \"string\"}",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var unmarshaled ExampleRequest
			err := json.Unmarshal([]byte(test.input), &unmarshaled)
			if err == nil {
				t.Error("unmarshal should fail")
			}
		})
	}
}
