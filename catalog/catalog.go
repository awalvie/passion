// Package catalog is the catalog the app ships. It holds no code: the files
// are embedded so that the binary carries them.
package catalog

import "embed"

//go:embed movements
var Files embed.FS
