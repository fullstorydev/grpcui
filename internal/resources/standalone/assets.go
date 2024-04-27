package standalone

import (
	"embed"
	"log"
)

const IndexTemplateName = "index-template.html"

//go:embed *.html *.css *.png *.js
var res embed.FS

func IndexTemplate() []byte {
	return MustAsset(IndexTemplateName)
}

func AssetNames() []string {
	list, err := res.ReadDir(".")
	if err != nil {
		log.Panic(err)
	}

	files := make([]string, len(list))
	for i, item := range list {
		files[i] = item.Name()
	}

	return files
}

func MustAsset(name string) []byte {
	data, err := res.ReadFile(name)
	if err != nil {
		log.Panic(err)
	}

	return data
}
