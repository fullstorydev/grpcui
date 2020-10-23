package standalone

import (
	"encoding/json"
	"io/ioutil"
)

// Examples list of gRPC request examples
type Examples []struct {
	Name    string `json:"name"`
	Request struct {
		Timeout  string `json:"timeout"`
		Metadata []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"metadata"`
		Data map[string]interface{} `json:"data"`
	} `json:"request"`
	Service string `json:"service"`
	Method  string `json:"method"`
}

// ParseExamplesFile parses the given examples file
func ParseExamplesFile(path string) (*Examples, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ParseExamples(content)
}

// ParseExamplesFile parses the given examples blob
func ParseExamples(content []byte) (*Examples, error) {
	var examples Examples

	err := json.Unmarshal(content, &examples)
	if err != nil {
		return nil, err
	}

	return &examples, nil
}
