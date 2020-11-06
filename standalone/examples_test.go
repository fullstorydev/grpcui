package standalone

import (
	"encoding/json"
	"github.com/fullstorydev/grpcui/testing/testdata"
	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"testing"
	"time"
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

			if diff := cmp.Diff(test.request, unmarshaled); diff != "" {
				t.Fatalf("OSRM gateway request mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRequest_MarshalJSON_ProtoData(t *testing.T) {
	tests := []struct {
		name    string
		request ExampleRequest
	}{
		{
			name: "proto3 data",
			request: ExampleRequest{
				Data: testmodels.TestMessage3{
					AInt32:  107,
					AString: "string",
				},
			},
		},
		{
			name: "proto2 data",
			request: ExampleRequest{
				Data: testmodels.TestMessage2{
					AInt32:  proto.Int32(107),
					AString: proto.String("string"),
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

			var raw interface{}
			err = json.Unmarshal(marshal, &raw)
			if err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			data := raw.(map[string]interface{})["data"].(map[string]interface{})
			if data["aInt32"] != float64(107) {
				t.Fatalf("aInt32 == %v != 107", data["aInt32"])
			}
			if data["aString"] != "string" {
				t.Fatalf("aString == %v != \"string\"", data["aString"])
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
			input: "{\"timeoutSeconds\": \"1s\"}",
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
