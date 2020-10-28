package standalone

// Example model of an example gRPC request
type Example struct {
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
