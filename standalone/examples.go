package standalone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

// Example model of an example gRPC request
type Example struct {
	Name    string         `json:"name"`
	Service string         `json:"service"`
	Method  string         `json:"method"`
	Request ExampleRequest `json:"request"`
}

// ExampleMetadataPair (name, value) pair
type ExampleMetadataPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ExampleRequest gRPC request
type ExampleRequest struct {
	Timeout  time.Duration
	Metadata []ExampleMetadataPair
	Data     interface{}
}

// intermediateRequest intermedia type using for marshaling/unmarshaling
type intermediateRequest struct {
	TimeoutSeconds float64               `json:"timeout_seconds"`
	Metadata       []ExampleMetadataPair `json:"metadata"`
	Data           json.RawMessage       `json:"data"`
}

func (request ExampleRequest) MarshalJSON() ([]byte, error) {
	marshalData, err := marshalData(request.Data)
	if err != nil {
		return nil, err
	}

	jsonRequest := intermediateRequest{
		TimeoutSeconds: request.Timeout.Seconds(),
		Metadata:       request.Metadata,
		Data:           marshalData,
	}

	return json.Marshal(jsonRequest)
}

func marshalData(data interface{}) ([]byte, error) {
	var marshalData []byte
	var err error

	if protoMsg, ok := data.(proto.Message); ok {
		marshalData, err = toJSON(protoMsg)
		if err != nil {
			return nil, err
		}
	} else {
		marshalData, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}

	return marshalData, nil
}

func toJSON(msg proto.Message) ([]byte, error) {
	jsm := jsonpb.Marshaler{}
	var b bytes.Buffer
	if err := jsm.Marshal(&b, msg); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (request *ExampleRequest) UnmarshalJSON(b []byte) (returnedErr error) {
	var intermediate intermediateRequest
	if err := json.Unmarshal(b, &intermediate); err != nil {
		return fmt.Errorf("failed to unmarshal input as object: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			returnedErr = fmt.Errorf("failed to unmarshal input: %v", r)
		}
	}()

	request.Timeout = time.Duration(intermediate.TimeoutSeconds*1000) * time.Millisecond
	request.Metadata = intermediate.Metadata
	request.Data = intermediate.Data

	return nil
}
