package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var distFS embed.FS

// GetFS returns the filesystem pointing to embedded static assets.
func GetFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
