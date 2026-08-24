package veyra

import (
	"html/template"
	"io/fs"
	"strings"
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

func TestTemplatesParseTogether(t *testing.T) {
	if _, err := template.ParseFS(TemplateFS(), "*.html"); err != nil {
		t.Fatal(err)
	}
}

func TestAppCSSBundlesStylesWithoutImports(t *testing.T) {
	css := AppCSS()
	if strings.Contains(css, "@import") {
		t.Fatal("bundled stylesheet should not contain CSS imports")
	}
	for _, marker := range []string{":root", ".dashboard-page", ".auth-page"} {
		if !strings.Contains(css, marker) {
			t.Fatalf("bundled stylesheet is missing %q", marker)
		}
	}
}
