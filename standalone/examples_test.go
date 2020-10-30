package standalone

import (
	"encoding/json"
	testmodels "github.com/fullstorydev/grpcui/testing/testdata"
	"github.com/google/go-cmp/cmp"
	"testing"
	"time"
)

func TestRequest_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		request Request
	}{
		{
			name: "Full request, complex data",
			request: Request{
				Timeout: 100 * time.Second,
				Metadata: []MetadataPair{
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
			request: Request{
				Timeout: 100 * time.Second,
				Metadata: []MetadataPair{
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
			request: Request{
				Metadata: []MetadataPair{
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
			request: Request{
				Timeout: 100 * time.Second,
				Metadata: []MetadataPair{
					{
						"key",
						"value",
					},
				},
			},
		},
		{
			name: "only timeout",
			request: Request{
				Timeout: 100 * time.Second,
			},
		},
		{
			name: "only data",
			request: Request{
				Data: "some data",
			},
		},
		{
			name: "only metadata",
			request: Request{
				Metadata: []MetadataPair{
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

			var unmarshaled Request
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
	request := Request{
		Data: testmodels.TestMessage{
			AInt32:  107,
			AString: "string",
		},
	}

	marshal, err := json.Marshal(request)
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
}

func TestRequest_UnmarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Invalid timeout",
			input: "{\"timeout\": 1}",
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
			var unmarshaled Request
			err := json.Unmarshal([]byte(test.input), &unmarshaled)
			if err == nil {
				t.Error("unmarshal should fail")
			}
		})
	}
}
