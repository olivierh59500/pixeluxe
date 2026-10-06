package ui

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"pixeluxe/internal/paint"
)

func backgroundTestGame(w, h int) *Game {
	g := New()
	g.Canvas = paint.New(w, h, paint.DefaultPalette()[:4])
	g.FG, g.BG, g.Radial, g.Zoom = 1, 0, 1, 1
	g.CycleLow, g.CycleHigh = 0, 3
	return g
}

func seedBackgroundPicture(g *Game) {
	b := g.Canvas.Image.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			g.Canvas.Pixel(x, y, uint8((x+2*y)%4))
		}
	}
	g.Canvas.MarkSaved()
}

func TestBackgroundFixOwnsPictureAndPalette(t *testing.T) {
	g := backgroundTestGame(12, 8)
	seedBackgroundPicture(g)
	g.action("background-fix")
	if !g.FixedBackground || g.Background == nil || !bytes.Equal(g.Background.Pix, g.Canvas.Image.Pix) {
		t.Fatal("Fix must capture the current picture")
	}
	if g.Canvas.Dirty() || g.Canvas.Undo() {
		t.Fatal("fixing the background must not modify the picture or add history")
	}
	index := g.Background.ColorIndexAt(3, 4)
	r, gg, b, a := g.Background.Palette[1].RGBA()
	g.Canvas.Image.SetColorIndex(3, 4, (index+1)%4)
	g.Canvas.Image.Palette[1] = color.RGBA{17, 34, 51, 255}
	br, bg, bb, ba := g.Background.Palette[1].RGBA()
	if g.Background.ColorIndexAt(3, 4) != index || br != r || bg != gg || bb != b || ba != a {
		t.Fatal("the fixed background must own independent pixels and palette storage")
	}
}

func TestBackgroundEraseRestoresStrokeAndShapeWithUndo(t *testing.T) {
	for _, tool := range []Tool{Freehand, FilledRectangle} {
		t.Run(toolNames[tool], func(t *testing.T) {
			g := backgroundTestGame(12, 8)
			seedBackgroundPicture(g)
			g.action("background-fix")
			g.Canvas.Clear(3)
			g.Canvas.MarkSaved()
			before := paint.CloneImage(g.Canvas.Image)
			g.Tool, g.erase = tool, true
			g.startTool(image.Pt(2, 2))
			g.moveTool(image.Pt(5, 2))
			end := image.Pt(9, 2)
			if tool == FilledRectangle {
				end = image.Pt(9, 5)
			}
			g.finishTool(end)
			maxY := 2
			if tool == FilledRectangle {
				maxY = 5
			}
			for y := 0; y < 8; y++ {
				for x := 0; x < 12; x++ {
					want := uint8(3)
					if x >= 2 && x <= 9 && y >= 2 && y <= maxY {
						want = g.Background.ColorIndexAt(x, y)
					}
					if got := g.Canvas.Image.ColorIndexAt(x, y); got != want {
						t.Fatalf("erase at %d,%d: got index %d, want original background index %d", x, y, got, want)
					}
				}
			}
			if !g.Canvas.Undo() || !bytes.Equal(before.Pix, g.Canvas.Image.Pix) || g.Canvas.Undo() {
				t.Fatal("erasing a gesture must be one complete undo action")
			}
		})
	}
}

func TestBackgroundCustomBrushErasePreservesTransparentMask(t *testing.T) {
	g := backgroundTestGame(12, 8)
	seedBackgroundPicture(g)
	g.action("background-fix")
	g.Canvas.Clear(3)
	brush := image.NewPaletted(image.Rect(0, 0, 3, 1), g.Canvas.Image.Palette)
	brush.Pix = []uint8{1, 0, 1}
	g.Brush = &paint.Brush{Image: brush, Transparent: 0, Mask: []bool{true, false, true}}
	g.Tool, g.erase = Freehand, true
	g.startTool(image.Pt(5, 4))
	g.finishTool(image.Pt(5, 4))
	if g.Canvas.Image.ColorIndexAt(4, 4) != g.Background.ColorIndexAt(4, 4) || g.Canvas.Image.ColorIndexAt(6, 4) != g.Background.ColorIndexAt(6, 4) || g.Canvas.Image.ColorIndexAt(5, 4) != 3 {
		t.Fatal("brush erasing must restore opaque brush locations and preserve its transparent holes")
	}
}

func TestBackgroundClearRestoresAndOffClearsToBackgroundColor(t *testing.T) {
	g := backgroundTestGame(12, 8)
	seedBackgroundPicture(g)
	g.action("background-fix")
	fixed := paint.CloneImage(g.Background)
	g.Canvas.Clear(1)
	before := paint.CloneImage(g.Canvas.Image)
	g.action("clear")
	if !bytes.Equal(g.Canvas.Image.Pix, fixed.Pix) || g.Canvas.Erase {
		t.Fatal("CLR with a fixed background must restore its original picture")
	}
	if !g.Canvas.Undo() || !bytes.Equal(g.Canvas.Image.Pix, before.Pix) {
		t.Fatal("restoring the fixed background must be undoable")
	}
	g.action("background-off")
	if g.FixedBackground || g.Background != nil || g.Canvas.RestoreImage != nil || g.Canvas.Erase {
		t.Fatal("Off must release the fixed background and pending restore state")
	}
	g.BG = 2
	g.action("clear")
	for _, index := range g.Canvas.Image.Pix {
		if index != 2 {
			t.Fatal("CLR after Off must fill with the selected background colour")
		}
	}
}

func TestStencilMakeLocksLocationsUntilRemake(t *testing.T) {
	g := backgroundTestGame(10, 6)
	g.Canvas.Pixel(2, 2, 2)
	g.FG = 2
	g.action("stencil-make")
	if !g.Canvas.StencilEnabled || !g.Canvas.Protected(2, 2) || g.Canvas.Protected(7, 2) {
		t.Fatal("Make must capture protected picture locations")
	}
	// A colour replacement may change indices without moving the existing stencil.
	g.Canvas.Image.SetColorIndex(2, 2, 1)
	g.Canvas.Image.SetColorIndex(7, 2, 2)
	g.Canvas.Pixel(2, 2, 3)
	g.Canvas.Pixel(7, 2, 3)
	if g.Canvas.Image.ColorIndexAt(2, 2) != 1 || g.Canvas.Image.ColorIndexAt(7, 2) != 3 {
		t.Fatal("a made stencil must lock coordinates rather than follow later colour indices")
	}
	g.Canvas.Image.SetColorIndex(7, 2, 2)
	g.action("stencil-remake")
	if g.Canvas.Protected(2, 2) || !g.Canvas.Protected(7, 2) {
		t.Fatal("Remake must resample the selected colours from the current picture")
	}
}

func TestStencilLockForegroundReverseAndFree(t *testing.T) {
	g := backgroundTestGame(10, 6)
	g.action("background-fix")
	g.Canvas.Rectangle(2, 2, 4, 3, 1, true)
	g.action("stencil-lockfg")
	if !g.Canvas.Protected(3, 2) || g.Canvas.Protected(7, 2) {
		t.Fatal("Lock FG must protect pixels differing from the fixed background")
	}
	g.Canvas.Pixel(3, 2, 3)
	g.Canvas.Pixel(7, 2, 3)
	if g.Canvas.Image.ColorIndexAt(3, 2) != 1 || g.Canvas.Image.ColorIndexAt(7, 2) != 3 {
		t.Fatal("Lock FG must keep foreground while allowing changes elsewhere")
	}
	g.action("stencil-reverse")
	if g.Canvas.Protected(3, 2) || !g.Canvas.Protected(7, 2) {
		t.Fatal("Reverse must invert the spatial mask")
	}
	g.Canvas.Pixel(3, 2, 2)
	g.Canvas.Pixel(7, 2, 2)
	if g.Canvas.Image.ColorIndexAt(3, 2) != 2 || g.Canvas.Image.ColorIndexAt(7, 2) != 3 {
		t.Fatal("drawing must respect the reversed stencil")
	}
	g.action("stencil-free")
	g.Canvas.Pixel(7, 2, 2)
	if g.Canvas.StencilEnabled || g.Canvas.StencilMask != nil || g.Canvas.Image.ColorIndexAt(7, 2) != 2 {
		t.Fatal("Free must remove all stencil protection")
	}
}

func TestBackgroundClearRespectsLockedForeground(t *testing.T) {
	g := backgroundTestGame(10, 6)
	g.action("background-fix")
	g.Canvas.Pixel(2, 2, 1)
	g.action("stencil-lockfg")
	g.Canvas.Pixel(7, 2, 3)
	g.action("clear")
	if g.Canvas.Image.ColorIndexAt(2, 2) != 1 || g.Canvas.Image.ColorIndexAt(7, 2) != 0 {
		t.Fatal("restoring the fixed background must preserve stencil-locked foreground")
	}
}

func TestCanvasFloodFillSpatialMaskSeparatesIdenticalColors(t *testing.T) {
	c := paint.New(9, 7, paint.DefaultPalette()[:4])
	c.StencilEnabled, c.Stencil[0] = true, true
	c.StencilMask = make([]bool, 9*7)
	for y := 0; y < 7; y++ {
		c.StencilMask[y*9+4] = true
	}
	c.Begin()
	c.FloodFill(2, 3, 2)
	c.Commit()
	for y := 0; y < 7; y++ {
		for x := 0; x < 9; x++ {
			want := uint8(0)
			if x < 4 {
				want = 2
			}
			if got := c.Image.ColorIndexAt(x, y); got != want {
				t.Fatalf("fill must stop at the spatial wall, even when every pixel has the same initial colour: %d,%d = %d", x, y, got)
			}
		}
	}
	if !c.Undo() || c.Dirty() || c.Undo() {
		t.Fatal("a fill separated by a spatial stencil must remain one undo action")
	}
	c.Begin()
	c.FloodFill(4, 3, 2)
	c.Commit()
	if c.Dirty() || c.Undo() {
		t.Fatal("filling a protected seed must leave the picture and history untouched")
	}
}

func TestGradientFillSpatialMaskStopsConnectivity(t *testing.T) {
	g := backgroundTestGame(9, 7)
	g.Canvas.StencilEnabled = true
	g.Canvas.StencilMask = make([]bool, 9*7)
	for y := 0; y < 7; y++ {
		g.Canvas.StencilMask[y*9+4] = true
	}
	g.Tool, g.FillStyle, g.CycleLow, g.CycleHigh = Fill, 3, 1, 3
	g.startTool(image.Pt(2, 3))
	if g.Canvas.Image.ColorIndexAt(2, 0) != 1 || g.Canvas.Image.ColorIndexAt(2, 6) != 3 {
		t.Fatal("a gradient must span its unprotected connected region")
	}
	for y := 0; y < 7; y++ {
		for x := 4; x < 9; x++ {
			if g.Canvas.Image.ColorIndexAt(x, y) != 0 {
				t.Fatal("pattern fill connectivity must not cross a spatial stencil wall")
			}
		}
	}
}
