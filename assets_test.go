package veyra

import (
	"bytes"
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

func TestAppCSSBundlesStylesWithoutImports(t *testing.T) {
	css := AppCSS()
	if bytes.Contains(css, []byte("@import")) {
		t.Fatal("bundled stylesheet should not contain CSS imports")
	}
	for _, marker := range [][]byte{[]byte(":root"), []byte(".dashboard-page"), []byte(".auth-page")} {
		if !bytes.Contains(css, marker) {
			t.Fatalf("bundled stylesheet is missing %q", marker)
		}
	}
}
