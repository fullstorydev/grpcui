package assets_test

import (
	"testing"

	"github.com/zeroflucs-given/grpcui/internal/resources/standalone"
	"github.com/zeroflucs-given/grpcui/internal/resources/webform"
)

func TestAssets(t *testing.T) {
	var assetFuncs = []struct {
		f    func() []byte
		name string
	}{
		{standalone.IndexTemplate, "IndexTemplate"},
		{webform.Template, "Template"},
		{webform.Script, "Script"},
		{webform.SampleCSS, "SampleCSS"},
	}

	for _, a := range assetFuncs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s() did not find corresponding asset file", a.name)
				}
			}()
			b := a.f()
			if len(b) == 0 {
				t.Errorf("%s() returned empty content", a.name)
			}
		}()
	}
}
