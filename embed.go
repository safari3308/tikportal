package tikportal

import (
	"embed"
	"io/fs"
)

//go:embed all:web/dist
var embeddedDist embed.FS

// FrontendFS returns the embedded web/dist filesystem rooted at web/dist.
func FrontendFS() (fs.FS, error) {
	return fs.Sub(embeddedDist, "web/dist")
}
