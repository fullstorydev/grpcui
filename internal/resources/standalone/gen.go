package standalone

// The index template is done separate, so it's not in the TOC.
// The go-bindata-all.sh script handles *.css, *.js, and *.png.

//go:generate go-bindata -out=index-template.go -pkg=standalone -func=GetIndexTemplate index-template.html
//go:generate ./go-bindata-all.sh

// GetResources is an exported accessor for resource file contents.
func GetResources() map[string]func() []byte {
	return go_bindata
}
