package webform

//go:generate go-bindata -o=bindata.go -pkg=webform webform-template.html webform-sample.css webform.js
//go:generate gofmt -w -s bindata.go
