package webform

//go:generate go-bindata -out=webform-sample-css.go -pkg=webform -func=WebFormSampleCSS webform-sample.css
//go:generate go-bindata -out=webform-template.go -pkg=webform -func=WebFormTemplate webform-template.html
//go:generate go-bindata -out=webform-js.go -pkg=webform -func=WebFormScript webform.js
