package veyra

import (
	"io/fs"
	"testing"
)

func TestRuntimeAssetsIncludeTemplatesAndStaticFiles(t *testing.T) {
	for _, test := range []struct {
		assets fs.FS
		path   string
	}{
		{assets: TemplateFS(), path: "dashboard.html"},
		{assets: StaticFS(), path: "dashboard.js"},
		{assets: StaticFS(), path: "css/base/foundation.css"},
	} {
		if _, err := fs.Stat(test.assets, test.path); err != nil {
			t.Fatalf("runtime asset %q missing: %v", test.path, err)
		}
	}
}
