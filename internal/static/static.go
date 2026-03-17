// Package static holds embedded frontend assets served via embed.FS.
package static

import (
	"embed"
	"io/fs"
)

//go:embed *.html css/* js/*
var content embed.FS

// FS returns the embedded filesystem, rooted at the static directory.
func FS() fs.FS {
	sub, err := fs.Sub(content, ".")
	if err != nil {
		panic("static: failed to create sub filesystem: " + err.Error())
	}
	return sub
}
