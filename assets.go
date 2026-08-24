package veyra

import (
	"embed"
	"io/fs"
	"strings"
)

// RuntimeAssets contains the templates and static files required by the web UI.
//
//go:embed internal/http/templates/*.html web/static
var RuntimeAssets embed.FS

var stylesheetPaths = []string{
	"web/static/css/base/foundation.css",
	"web/static/css/base/shell.css",
	"web/static/css/components/data.css",
	"web/static/css/components/badges.css",
	"web/static/css/components/forms.css",
	"web/static/css/components/actions.css",
	"web/static/css/pages/settings.css",
	"web/static/css/pages/dashboard.css",
	"web/static/css/pages/auth.css",
	"web/static/css/pages/admin.css",
	"web/static/css/theme/veyra.css",
}

var appCSS = buildAppCSS()

func TemplateFS() fs.FS {
	templates, err := fs.Sub(RuntimeAssets, "internal/http/templates")
	if err != nil {
		panic(err)
	}
	return templates
}

func StaticFS() fs.FS {
	static, err := fs.Sub(RuntimeAssets, "web/static")
	if err != nil {
		panic(err)
	}
	return static
}

// AppCSS returns the immutable component bundle as one browser-ready response.
// The source files stay separate so they remain easy to navigate and edit.
func AppCSS() string {
	return appCSS
}

func buildAppCSS() string {
	var combined strings.Builder
	for _, path := range stylesheetPaths {
		content, err := RuntimeAssets.ReadFile(path)
		if err != nil {
			panic(err)
		}
		combined.Write(content)
		combined.WriteByte('\n')
	}
	return combined.String()
}
