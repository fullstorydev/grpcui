package assets_test

import (
	"testing"

	"github.com/fullstorydev/grpcui/internal/resources/standalone"
	"github.com/fullstorydev/grpcui/internal/resources/webform"
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

func TestStandaloneIncludesRequiredAssets(t *testing.T) {
	assets := standalone.AssetNames()
	seen := map[string]bool{}
	for _, name := range assets {
		seen[name] = true
	}

	required := []string{
		"jquery-3.4.1.min.js",
		"jquery-ui-1.12.1.min.css",
		"jquery-ui-1.12.1.min.js",
		"jquery.json-viewer-v1.5.0.js",
		"jquery.json-viewer-v1.5.0.css",
	}
	for _, name := range required {
		if !seen[name] {
			t.Fatalf("standalone assets missing %q", name)
		}
	}
}
