package webform

import _ "embed"

var (
	//go:embed "webform-template.html"
	templateBytes []byte

	//go:embed "webform.js"
	jsBytes []byte

	//go:embed "webform-sample.css"
	cssBytes []byte
)

func Template() []byte {
	return templateBytes
}

func Script() []byte {
	return jsBytes
}

func SampleCSS() []byte {
	return cssBytes
}
