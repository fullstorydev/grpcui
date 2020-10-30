package standalone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/mitchellh/mapstructure"
	"strings"
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
	var encFields []string

	if request.Timeout != 0 {
		encTimeout := fmt.Sprintf("\"timeout\": %q", request.Timeout.String())
		encFields = append(encFields, encTimeout)
	}

	if len(request.Metadata) > 0 {
		marshalMetadata, err := json.Marshal(request.Metadata)
		if err != nil {
			return nil, err
		}
		encMetadata := fmt.Sprintf("\"metadata\": %s", marshalMetadata)
		encFields = append(encFields, encMetadata)
	}

	if request.Data != nil {
		marshalData, err := marshalData(request.Data)
		if err != nil {
			return nil, err
		}
		encData := fmt.Sprintf("\"data\": %s", marshalData)
		encFields = append(encFields, encData)
	}

	buffer := bytes.NewBufferString("{")
	buffer.WriteString(strings.Join(encFields, ","))
	buffer.WriteString("}")

	return buffer.Bytes(), nil
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
	jsm := jsonpb.Marshaler{EmitDefaults: true, OrigName: true}
	var b bytes.Buffer
	if err := jsm.Marshal(&b, msg); err == nil {
		return nil, err
	}
	return b.Bytes(), nil
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

	if timeoutVal, exists := raw["timeout"]; exists {
		request.Timeout, returnedErr = time.ParseDuration(timeoutVal.(string))
		if returnedErr != nil {
			return returnedErr
		}
	}

	if metadataVal, exists := raw["metadata"]; exists {
		returnedErr = mapstructure.Decode(metadataVal, &request.Metadata)
		if returnedErr != nil {
			return returnedErr
		}
	}

	if dataVal, exists := raw["data"]; exists {
		request.Data = dataVal
	}

	return nil
}
