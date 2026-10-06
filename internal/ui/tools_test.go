package ui

import (
	"bytes"
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"pixeluxe/internal/formats"
	"pixeluxe/internal/paint"
	"pixeluxe/internal/pixfont"
)

func toolGame(w, h, colors int) *Game {
	g := New()
	g.Canvas = paint.New(w, h, paint.DefaultPalette()[:colors])
	g.FG, g.BG = 1, 0
	g.Zoom, g.Radial = 1, 1
	g.CycleLow, g.CycleHigh = 0, colors-1
	return g
}

func assertToolIndicesValid(t *testing.T, g *Game) {
	t.Helper()
	for i, v := range g.Canvas.Image.Pix {
		if int(v) >= len(g.Canvas.Image.Palette) {
			t.Fatalf("pixel %d has index %d outside the %d-color palette", i, v, len(g.Canvas.Image.Palette))
		}
	}
}

func assertToolImage(t *testing.T, got, want *image.Paletted) {
	t.Helper()
	if got.Rect != want.Rect || !bytes.Equal(got.Pix, want.Pix) || len(got.Palette) != len(want.Palette) {
		t.Fatal("picture dimensions, pixels or palette size changed")
	}
	for i, c := range got.Palette {
		gr, gg, gb, ga := c.RGBA()
		wr, wg, wb, wa := want.Palette[i].RGBA()
		if gr != wr || gg != wg || gb != wb || ga != wa {
			t.Fatalf("palette color %d changed", i)
		}
	}
}

func TestToolFreehandFinalSampleAndSingleUndo(t *testing.T) {
	g := toolGame(20, 12, 4)
	g.Tool = Freehand
	g.startTool(image.Pt(2, 4))
	g.moveTool(image.Pt(5, 4))
	g.moveTool(image.Pt(8, 4))
	// The button can be released at a new pointer position between frames.
	g.finishTool(image.Pt(11, 4))
	for x := 2; x <= 11; x++ {
		if g.Canvas.Image.ColorIndexAt(x, 4) != g.FG {
			t.Fatalf("freehand lost final release sample at (%d,4)", x)
		}
	}
	finished := paint.CloneImage(g.Canvas.Image)
	if !g.Canvas.Dirty() || !g.Canvas.Undo() || g.Canvas.Dirty() {
		t.Fatal("one complete freehand gesture must be one undo action")
	}
	if g.Canvas.Undo() {
		t.Fatal("intermediate freehand samples created extra undo actions")
	}
	if !g.Canvas.Redo() {
		t.Fatal("freehand gesture cannot be redone")
	}
	assertToolImage(t, g.Canvas.Image, finished)
}

func TestToolShapesUseFinalReleasePoint(t *testing.T) {
	for _, tool := range []Tool{Line, Rectangle, FilledRectangle} {
		t.Run(toolNames[tool], func(t *testing.T) {
			g := toolGame(20, 14, 4)
			g.Tool = tool
			g.startTool(image.Pt(2, 2))
			g.moveTool(image.Pt(5, 5))
			if g.Canvas.Dirty() {
				t.Fatal("shape preview changed the document before release")
			}
			g.finishTool(image.Pt(12, 9))
			if g.Canvas.Image.ColorIndexAt(12, 9) != g.FG {
				t.Fatal("shape used the old preview instead of the release endpoint")
			}
			if !g.Canvas.Undo() || g.Canvas.Dirty() || g.Canvas.Undo() {
				t.Fatal("shape gesture must create exactly one undo action")
			}
		})
	}
}

func TestToolFilledShapeUsesConfiguredGradient(t *testing.T) {
	g := toolGame(24, 16, 4)
	g.Tool, g.FillStyle, g.CycleLow, g.CycleHigh = FilledRectangle, 3, 1, 3
	g.startTool(image.Pt(4, 4))
	g.moveTool(image.Pt(14, 10))
	g.finishTool(image.Pt(14, 10))
	if g.Canvas.Image.ColorIndexAt(9, 4) != 1 || g.Canvas.Image.ColorIndexAt(9, 10) != 3 {
		t.Fatal("filled shape ignored the configured fill gradient")
	}
}

func TestToolShapeOutlineUsesSelectedBrushWidth(t *testing.T) {
	g := toolGame(24, 16, 4)
	g.Tool, g.BrushSize, g.BrushShape = Rectangle, 2, 1
	g.startTool(image.Pt(4, 4))
	g.moveTool(image.Pt(14, 10))
	g.finishTool(image.Pt(14, 10))
	if g.Canvas.Image.ColorIndexAt(3, 7) != g.FG || g.Canvas.Image.ColorIndexAt(15, 7) != g.FG {
		t.Fatal("shape outline ignored the selected three-pixel square brush")
	}
}

func TestToolCancelRestoresGestureWithoutHistory(t *testing.T) {
	tools := []Tool{Dots, Freehand, Airbrush, Line, Rectangle, FilledRectangle, Ellipse, FilledEllipse, Circle, FilledCircle, Curve, Polygon, FilledPolygon, BrushSelect, Text}
	for _, tool := range tools {
		t.Run(toolNames[tool], func(t *testing.T) {
			g := toolGame(32, 24, 4)
			g.Tool = tool
			original := paint.CloneImage(g.Canvas.Image)
			g.startTool(image.Pt(8, 8))
			switch tool {
			case Text:
				g.textBuffer = "Aé"
				g.textPreview()
			case Polygon, FilledPolygon:
				g.startTool(image.Pt(17, 8))
				g.shapePreview(image.Pt(16, 17))
			case Curve:
				g.moveTool(image.Pt(17, 8))
				g.finishTool(image.Pt(17, 8))
				g.shapePreview(image.Pt(12, 18))
			default:
				g.moveTool(image.Pt(17, 12))
			}
			g.cancelGesture()
			assertToolImage(t, g.Canvas.Image, original)
			if g.Canvas.Dirty() || g.Canvas.Undo() || g.dragging || g.typing || g.preview != nil || g.base != nil || len(g.poly) != 0 || g.curveEnd != nil {
				t.Fatal("Cancel left document changes, history or active gesture state")
			}
			if g.Render().Bounds() != image.Rect(0, 0, Width, Height) {
				t.Fatal("cancelled gesture cannot render a desktop frame")
			}
		})
	}
}

func TestToolCurveAndPolygonTransactions(t *testing.T) {
	t.Run("curve", func(t *testing.T) {
		g := toolGame(24, 16, 4)
		g.Tool = Curve
		g.startTool(image.Pt(2, 10))
		g.moveTool(image.Pt(18, 10))
		g.finishTool(image.Pt(18, 10))
		if g.curveEnd == nil || g.Canvas.Dirty() {
			t.Fatal("the curve chord must await the bend without committing")
		}
		g.shapePreview(image.Pt(10, 0))
		g.startTool(image.Pt(10, 0))
		if g.curveEnd != nil || g.preview != nil || g.Canvas.Image.ColorIndexAt(10, 5) != g.FG {
			t.Fatal("curve did not commit its quadratic bend")
		}
		if !g.Canvas.Undo() || g.Canvas.Dirty() || g.Canvas.Undo() {
			t.Fatal("all stages of a curve must form one history action")
		}
	})
	t.Run("filled polygon", func(t *testing.T) {
		g := toolGame(24, 16, 4)
		g.Tool = FilledPolygon
		for _, p := range []image.Point{{2, 2}, {14, 2}, {8, 12}} {
			g.startTool(p)
		}
		g.shapePreview(image.Pt(2, 2))
		if g.Canvas.Dirty() {
			t.Fatal("polygon vertices committed before closure")
		}
		g.startTool(image.Pt(2, 2))
		if len(g.poly) != 0 || g.preview != nil || g.Canvas.Image.ColorIndexAt(8, 5) != g.FG {
			t.Fatal("clicking the first vertex did not fill and close the polygon")
		}
		if !g.Canvas.Undo() || g.Canvas.Dirty() || g.Canvas.Undo() {
			t.Fatal("polygon vertices must form one history action")
		}
	})
}

func TestToolTextCommitUsesCurrentBuffer(t *testing.T) {
	g := toolGame(32, 16, 4)
	g.Tool = Text
	g.startTool(image.Pt(2, 2))
	g.textBuffer = "A"
	g.textPreview()
	if g.Canvas.Dirty() {
		t.Fatal("text preview changed the document")
	}
	// AppendInputChars and Return may arrive in the same Update.
	g.textBuffer = "AB"
	g.commitText()
	want := paint.New(32, 16, g.Canvas.Image.Palette)
	pixfont.WalkAmiga(g.FontName, "AB", 2, 2, g.FontScale, g.FontStyle, func(x, y int) { want.Pixel(x, y, g.FG) })
	if !bytes.Equal(g.Canvas.Image.Pix, want.Image.Pix) {
		t.Fatal("Return committed a stale text preview and lost the newest characters")
	}
	if g.typing || g.textBuffer != "" || g.preview != nil || !g.Canvas.Undo() || g.Canvas.Dirty() || g.Canvas.Undo() {
		t.Fatal("text placement must close its gesture and create one undo action")
	}
}

func TestToolBrushSelectionIncludesTransparentOutside(t *testing.T) {
	g := toolGame(8, 8, 4)
	g.Canvas.Clear(2)
	g.Canvas.MarkSaved()
	original := paint.CloneImage(g.Canvas.Image)
	g.Tool = BrushSelect
	g.startTool(image.Pt(6, 6))
	g.moveTool(image.Pt(10, 10))
	g.finishTool(image.Pt(10, 10))
	if g.Brush == nil || g.Brush.Image.Rect != image.Rect(0, 0, 5, 5) || len(g.Brush.Mask) != 25 || g.LastBrush != g.Brush || g.Tool != Freehand {
		t.Fatal("brush selection did not produce its inclusive rectangle")
	}
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			if got, want := g.Brush.Mask[y*5+x], x < 2 && y < 2; got != want {
				t.Fatalf("brush mask at (%d,%d): got %v want %v", x, y, got, want)
			}
		}
	}
	assertToolImage(t, g.Canvas.Image, original)
	if g.Canvas.Dirty() || g.Canvas.Undo() {
		t.Fatal("picking up a brush must not edit the source picture")
	}
}

func TestToolBrushSelectionRespectsDimensionLimit(t *testing.T) {
	g := toolGame(8, 8, 4)
	g.Tool = BrushSelect
	g.startTool(image.Pt(0, 0))
	// A long, one-row capture safely exercises the bound without a large allocation.
	g.finishTool(image.Pt(4096, 0))
	if g.Brush != nil && g.Brush.Image.Rect.Dx() > 4096 {
		t.Fatal("selection bypassed the 4096-pixel brush dimension limit")
	}
}

func TestToolCustomBrushColorModesAndErase(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mode  int
		erase bool
		want  []uint8
	}{{"Matte", 0, false, []uint8{2, 3, 1}}, {"Color", 1, false, []uint8{1, 3, 1}}, {"Replace", 2, false, []uint8{2, 0, 1}}, {"Erase", 0, true, []uint8{0, 3, 0}}} {
		t.Run(tc.name, func(t *testing.T) {
			g := toolGame(12, 8, 4)
			g.Canvas.Clear(3)
			brush := image.NewPaletted(image.Rect(0, 0, 3, 1), g.Canvas.Image.Palette)
			brush.Pix = []uint8{2, 0, 1}
			g.Brush = &paint.Brush{Image: brush, Transparent: 0, Mask: []bool{true, false, true}}
			g.Mode, g.erase = tc.mode, tc.erase
			g.startTool(image.Pt(4, 4))
			g.finishTool(image.Pt(4, 4))
			for i, want := range tc.want {
				if got := g.Canvas.Image.ColorIndexAt(3+i, 4); got != want {
					t.Fatalf("custom brush pixel %d: got %d want %d", i, got, want)
				}
			}
		})
	}
}

func TestToolEffectModesHonorCustomBrushMask(t *testing.T) {
	for _, mode := range []int{3, 4, 5, 7} {
		t.Run(modeNames[mode], func(t *testing.T) {
			g := toolGame(12, 8, 3)
			g.Canvas.Image.Palette = color.Palette{color.Black, color.RGBA{128, 128, 128, 255}, color.White}
			g.Canvas.Pixel(4, 4, 2)
			g.FG, g.Mode, g.last = 0, mode, image.Pt(3, 4)
			brush := image.NewPaletted(image.Rect(0, 0, 3, 1), g.Canvas.Image.Palette)
			g.Brush = &paint.Brush{Image: brush, Mask: []bool{true, false, true}}
			g.stamp(g.Canvas, image.Pt(4, 4))
			if g.Canvas.Image.ColorIndexAt(4, 4) != 2 {
				t.Fatal("effect ignored the custom brush's transparent center and edited that pixel")
			}
		})
	}
}

func TestToolCycleModeChangesCustomBrushBetweenStamps(t *testing.T) {
	g := toolGame(16, 8, 4)
	g.Mode, g.CycleLow, g.CycleHigh = 6, 1, 3
	brush := image.NewPaletted(image.Rect(0, 0, 1, 1), g.Canvas.Image.Palette)
	brush.Pix[0] = 1
	g.Brush = &paint.Brush{Image: brush, Mask: []bool{true}}
	g.cycleTick = 0
	g.stamp(g.Canvas, image.Pt(3, 4))
	g.cycleTick = 1
	g.stamp(g.Canvas, image.Pt(8, 4))
	if g.Canvas.Image.ColorIndexAt(3, 4) == g.Canvas.Image.ColorIndexAt(8, 4) {
		t.Fatal("custom brush Cycle mode behaved exactly like Matte instead of advancing colors")
	}
}

func TestToolPatternFillPreservesConnectivityAndMask(t *testing.T) {
	g := toolGame(10, 8, 4)
	g.Canvas.Clear(2)
	for y := 0; y < 8; y++ {
		g.Canvas.Pixel(5, y, 3)
	}
	brush := image.NewPaletted(image.Rect(0, 0, 2, 1), g.Canvas.Image.Palette)
	brush.Pix = []uint8{1, 1}
	g.Brush = &paint.Brush{Image: brush, Mask: []bool{true, false}}
	g.Tool, g.FillStyle = Fill, 1
	g.startTool(image.Pt(2, 3))
	for y := 0; y < 8; y++ {
		for x := 0; x < 10; x++ {
			want := uint8(2)
			if x == 5 {
				want = 3
			} else if x < 5 && x%2 == 0 {
				want = 1
			}
			if got := g.Canvas.Image.ColorIndexAt(x, y); got != want {
				t.Fatalf("pattern connectivity or brush mask at (%d,%d): got %d want %d", x, y, got, want)
			}
		}
	}
	if !g.Canvas.Undo() || g.Canvas.Undo() {
		t.Fatal("pattern fill must be one undo action")
	}
}

func TestToolPatternFillRadialSymmetryClipsSeeds(t *testing.T) {
	g := toolGame(12, 8, 4)
	g.Tool, g.FillStyle, g.Radial = Fill, 3, 4
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("a valid fill click produced an out-of-bounds symmetric seed: %v", p)
		}
	}()
	g.startTool(image.Pt(0, 0))
	assertToolIndicesValid(t, g)
}

func TestToolStencilProtectsShapesTextAndPattern(t *testing.T) {
	for _, tool := range []Tool{Freehand, FilledRectangle, Text, Fill} {
		t.Run(toolNames[tool], func(t *testing.T) {
			g := toolGame(24, 18, 4)
			g.Canvas.Clear(2)
			g.Canvas.MarkSaved()
			g.Canvas.Stencil[2], g.Canvas.StencilEnabled = true, true
			g.Tool = tool
			if tool == Fill {
				g.FillStyle = 3
			}
			g.startTool(image.Pt(2, 2))
			if tool == Text {
				g.textBuffer = "AB"
				g.textPreview()
				g.commitText()
			} else if tool != Fill {
				g.moveTool(image.Pt(12, 10))
				g.finishTool(image.Pt(12, 10))
			}
			for _, v := range g.Canvas.Image.Pix {
				if v != 2 {
					t.Fatal("tool overwrote a stencil-protected color")
				}
			}
			if g.Canvas.Dirty() || g.Canvas.Undo() {
				t.Fatal("a completely stencil-blocked gesture should not enter history")
			}
		})
	}
}

func TestToolPaletteUndoNormalizesSelectionsBeforeBlend(t *testing.T) {
	g := toolGame(12, 8, 2)
	g.edit(func() { g.Canvas.Image.Palette = paint.DefaultPalette() })
	g.FG, g.BG, g.Mode = 31, 30, 5
	g.action("undo")
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("Blend after palette Undo used a stale foreground index: %v", p)
		}
	}()
	g.startTool(image.Pt(4, 4))
	g.finishTool(image.Pt(4, 4))
	if int(g.FG) >= len(g.Canvas.Image.Palette) || int(g.BG) >= len(g.Canvas.Image.Palette) {
		t.Fatal("Undo did not normalize foreground and background to the restored palette")
	}
	assertToolIndicesValid(t, g)
}

func TestToolRestoredShortPaletteRetainsValidPixels(t *testing.T) {
	g := toolGame(12, 8, 4)
	g.OriginalPalette = append(color.Palette(nil), g.Canvas.Image.Palette...)
	brush := image.NewPaletted(image.Rect(0, 0, 1, 1), paint.DefaultPalette())
	g.Brush = &paint.Brush{Image: brush, Mask: []bool{true}}
	g.action("brush-palette")
	g.edit(func() { g.Canvas.Pixel(4, 4, 31) })
	g.FG, g.BG = 31, 30
	g.action("restore-palette")
	assertToolIndicesValid(t, g)
	if int(g.FG) >= len(g.Canvas.Image.Palette) || int(g.BG) >= len(g.Canvas.Image.Palette) {
		t.Fatal("restoring a shorter palette left invalid active color selections")
	}
	if err := formats.Save(filepath.Join(t.TempDir(), "restored.png"), g.Canvas.Image); err != nil {
		t.Fatalf("restored picture cannot be exported: %v", err)
	}
}

func TestToolSpareSwapAndStencilMerge(t *testing.T) {
	t.Run("swap clamps short palette", func(t *testing.T) {
		g := toolGame(12, 8, 32)
		g.FG, g.BG, g.Mode = 31, 30, 5
		g.Spare = paint.New(5, 3, paint.DefaultPalette()[:2]).Image
		g.Spare.SetColorIndex(2, 1, 1)
		g.action("spare-swap")
		if g.Canvas.Image.Rect != image.Rect(0, 0, 5, 3) || g.Spare.Rect != image.Rect(0, 0, 12, 8) {
			t.Fatal("spare swap did not exchange page dimensions")
		}
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("Blend after spare swap used a stale palette index: %v", p)
			}
		}()
		g.startTool(image.Pt(1, 1))
		g.finishTool(image.Pt(1, 1))
		assertToolIndicesValid(t, g)
	})
	t.Run("merge honors stencil and undo", func(t *testing.T) {
		g := toolGame(8, 6, 4)
		g.Canvas.Pixel(2, 2, 2)
		g.Canvas.MarkSaved()
		g.Canvas.Stencil[2], g.Canvas.StencilEnabled = true, true
		g.Spare = paint.New(8, 6, g.Canvas.Image.Palette).Image
		for i := range g.Spare.Pix {
			g.Spare.Pix[i] = 1
		}
		original := paint.CloneImage(g.Canvas.Image)
		g.action("spare-back")
		if g.Canvas.Image.ColorIndexAt(2, 2) != 2 || g.Canvas.Image.ColorIndexAt(3, 2) != 1 {
			t.Fatal("spare merge bypassed the stencil or omitted unprotected pixels")
		}
		if !g.Canvas.Undo() {
			t.Fatal("spare merge cannot be undone")
		}
		assertToolImage(t, g.Canvas.Image, original)
	})
}

func TestToolDrawingRoundTripThroughGameFiles(t *testing.T) {
	for _, ext := range []string{".png", ".gif", ".iff"} {
		t.Run(ext, func(t *testing.T) {
			g := toolGame(24, 16, 4)
			g.Tool = FilledRectangle
			g.startTool(image.Pt(2, 2))
			g.moveTool(image.Pt(12, 10))
			g.finishTool(image.Pt(12, 10))
			if !g.Canvas.Dirty() {
				t.Fatal("completed drawing is not marked changed")
			}
			path := filepath.Join(t.TempDir(), "picture"+ext)
			if err := g.Save(path); err != nil {
				t.Fatal(err)
			}
			if g.Canvas.Dirty() || g.Filename != path {
				t.Fatal("successful save did not update the filename and saved state")
			}
			loaded := New()
			if err := loaded.Load(path); err != nil {
				t.Fatal(err)
			}
			assertToolImage(t, loaded.Canvas.Image, g.Canvas.Image)
			if loaded.Canvas.Dirty() || loaded.Canvas.Undo() {
				t.Fatal("loading a picture must start a clean editing history")
			}
			loaded.Render()
		})
	}
}

func TestToolInstallingOffsetImageNormalizesCanvasCoordinates(t *testing.T) {
	// GIF frames and indexed subimages can have a nonzero rectangle origin.
	src := image.NewPaletted(image.Rect(7, 11, 13, 15), paint.DefaultPalette()[:4])
	src.SetColorIndex(7, 11, 2)
	src.SetColorIndex(12, 14, 3)
	g := New()
	g.install(src, "offset.gif")
	if g.Canvas.Image.Rect != image.Rect(0, 0, 6, 4) || g.Canvas.Image.ColorIndexAt(0, 0) != 2 || g.Canvas.Image.ColorIndexAt(5, 3) != 3 {
		t.Fatal("install did not normalize an offset image into editable canvas coordinates")
	}
	g.Tool = Dots
	g.startTool(image.Pt(1, 1))
	g.finishTool(image.Pt(1, 1))
	if g.Canvas.Image.ColorIndexAt(1, 1) != g.FG {
		t.Fatal("normalized picture cannot be painted at its visible canvas coordinates")
	}
	g.Render()
}
