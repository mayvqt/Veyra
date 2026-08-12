package veyra

import (
	"embed"
	"io/fs"
)

// RuntimeAssets contains the templates and static files required by the web UI.
//
//go:embed internal/http/templates/*.html web/static
var RuntimeAssets embed.FS

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
