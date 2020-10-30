package standalone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"time"
)

// Example model of an example gRPC request
type Example struct {
	Name    string  `json:"name"`
	Service string  `json:"service"`
	Method  string  `json:"method"`
	Request Request `json:"request"`
}

// MetadataPair (name, value) pair
type MetadataPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Request gRPC request
type Request struct {
	Timeout  time.Duration
	Metadata []MetadataPair
	Data     interface{}
}

func (request Request) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString("{")

	buffer.WriteString(fmt.Sprintf("\"timeout\": %q,", request.Timeout.String()))

	marshalMetadata, err := json.Marshal(request.Metadata)
	if err != nil {
		return nil, err
	}
	buffer.WriteString(fmt.Sprintf("\"metadata\": %s,", marshalMetadata))

	marshalData, err := json.Marshal(request.Data)
	if err != nil {
		return nil, err
	}
	buffer.WriteString(fmt.Sprintf("\"data\": %s", marshalData))

	buffer.WriteString("}")

	fmt.Println(buffer.String())

	return buffer.Bytes(), nil
}

func (request *Request) UnmarshalJSON(b []byte) (returnedErr error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal input as object: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			returnedErr = fmt.Errorf("failed to unmarshal input: %v", r)
		}
	}()

	request.Timeout, returnedErr = time.ParseDuration(raw["timeout"].(string))
	if returnedErr != nil {
		return returnedErr
	}

	returnedErr = mapstructure.Decode(raw["metadata"], &request.Metadata)
	if returnedErr != nil {
		return returnedErr
	}

	request.Data = raw["data"]

	return nil
}
