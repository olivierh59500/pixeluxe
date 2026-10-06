// Package assets bundles original pictures and bitmap fonts extracted from the
// user's reference disk. Pixeluxe does not execute the Amiga binary.
package assets

import "embed"

//go:embed pictures/*.iff brushes/*.iff fonts.json
var Files embed.FS
