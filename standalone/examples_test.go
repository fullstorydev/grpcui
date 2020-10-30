package standalone

import (
	"encoding/json"
	"github.com/google/go-cmp/cmp"
	"testing"
	"time"
)

var request = Request{
	Timeout:  1,
	Metadata: nil,
	Data:     nil,
}

func TestRequest_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		request Request
	}{
		{
			name: "Full request, map",
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			marshal, err := json.Marshal(test.request)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}

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
			name:  "Empty object",
			input: "{}",
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
