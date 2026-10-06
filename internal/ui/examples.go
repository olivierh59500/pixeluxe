package ui

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"pixeluxe/internal/formats"
)

func (g *Game) brushExamples() {
	if g.BrushFS == nil {
		g.message("Original brushes", "No original brushes are bundled.")
		return
	}
	entries, err := fs.ReadDir(g.BrushFS, ".")
	if err != nil {
		g.message("Original brushes", err.Error())
		return
	}
	names, paths := []string{}, []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".iff") {
			names = append(names, strings.TrimSuffix(entry.Name(), ".iff"))
			paths = append(paths, entry.Name())
		}
	}
	g.chooseDialog("Original Amiga brushes", names, func(i int) {
		f, err := g.BrushFS.Open(paths[i])
		if err != nil {
			g.message("Brush load failed", err.Error())
			return
		}
		defer f.Close()
		im, _, err := formats.DecodeILBM(f)
		if err != nil {
			g.message("Brush load failed", err.Error())
			return
		}
		g.Brush = importedBrush(im, g.Canvas.Image.Palette, g.BG)
		g.LastBrush = g.Brush
		g.Tool = Freehand
		g.notice(fmt.Sprintf("%s brush  -  %dx%d", filepath.Base(paths[i]), im.Rect.Dx(), im.Rect.Dy()))
	})
}
