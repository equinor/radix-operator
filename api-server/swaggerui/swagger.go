package swaggerui

import (
	"embed"
	"io/fs"
)

//go:embed html
var swaggerFS embed.FS

// FS returns a FS with SwaggerUI files in root
func FS() fs.FS {
	rootFS, err := fs.Sub(swaggerFS, "html")
	if err != nil {
		panic(err)
	}
	return rootFS
}
