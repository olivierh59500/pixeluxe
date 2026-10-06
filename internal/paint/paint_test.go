package paint

import (
	"image"
	"image/color"
	"testing"
)

func TestHistoryIncludesPaletteDimensionsAndSavedState(t *testing.T) {
	c := New(4, 3, nil)
	if c.Dirty() {
		t.Fatal("a new blank canvas must be clean")
	}
	c.Begin()
	c.Pixel(1, 1, 2)
	c.Commit()
	if !c.Dirty() {
		t.Fatal("drawing must dirty the canvas")
	}
	c.MarkSaved()
	c.Begin()
	c.Image = image.NewPaletted(image.Rect(0, 0, 7, 5), DefaultPalette())
	c.Image.Palette[2] = color.RGBA{1, 2, 3, 255}
	c.Pixel(6, 4, 2)
	c.Commit()
	if !c.Undo() || c.Image.Rect != image.Rect(0, 0, 4, 3) || c.Image.ColorIndexAt(1, 1) != 2 {
		t.Fatal("undo must restore dimensions and pixels")
	}
	if c.Dirty() {
		t.Fatal("undo to the saved state must clear dirty")
	}
	if !c.Redo() || c.Image.Rect != image.Rect(0, 0, 7, 5) || c.Image.ColorIndexAt(6, 4) != 2 {
		t.Fatal("redo must restore dimensions and pixels")
	}
	r, g, b, _ := c.Image.Palette[2].RGBA()
	if r != 257 || g != 514 || b != 771 {
		t.Fatal("redo must restore the changed palette")
	}
	c.Undo()
	c.Begin()
	c.Pixel(0, 0, 3)
	c.Commit()
	if c.Redo() {
		t.Fatal("a new edit must discard the redo branch")
	}
}

func TestCancelAndEmptyActionsDoNotUseHistory(t *testing.T) {
	c := New(2, 2, nil)
	c.Begin()
	c.Begin()
	c.Pixel(0, 0, 2)
	c.Cancel()
	if c.Dirty() || c.Undo() {
		t.Fatal("cancel must restore without an undo entry")
	}
	c.Begin()
	c.Commit()
	if c.Undo() {
		t.Fatal("an empty action must not create an undo entry")
	}
	c.Begin()
	c.Image.Palette[1] = color.Black
	c.Commit()
	if !c.Dirty() || !c.Undo() || c.Dirty() {
		t.Fatal("palette-only changes must be undoable")
	}
}

func TestCloneSubimageHasIndependentPixelsAndPalette(t *testing.T) {
	src := image.NewPaletted(image.Rect(3, 4, 13, 12), DefaultPalette())
	src.SetColorIndex(6, 7, 2)
	sub := src.SubImage(image.Rect(5, 6, 9, 9)).(*image.Paletted)
	cloned := CloneImage(sub)
	if !equalImage(sub, cloned) || cloned.Rect != sub.Rect {
		t.Fatal("clone must preserve a non-zero origin and larger source stride")
	}
	cloned.SetColorIndex(6, 7, 4)
	cloned.Palette[2] = color.White
	if sub.ColorIndexAt(6, 7) != 2 {
		t.Fatal("clone aliases the original pixels")
	}
	r, _, _, _ := sub.Palette[2].RGBA()
	if r != 65535 {
		t.Fatal("clone aliases the original palette")
	}
	_, g, _, _ := sub.Palette[2].RGBA()
	if g != 0 {
		t.Fatal("clone aliases the original palette")
	}
}

func TestShapesClipAndRespectStencil(t *testing.T) {
	c := New(9, 9, nil)
	c.Rectangle(20, 20, -2, -2, 2, true)
	for _, p := range c.Image.Pix {
		if p != 2 {
			t.Fatal("reversed rectangle should cover the canvas after clipping")
		}
	}
	c.StencilEnabled, c.Stencil[2] = true, true
	c.Line(-3, -3, 15, 15, 4)
	c.Clear(5)
	c.FloodFill(0, 0, 6)
	for _, p := range c.Image.Pix {
		if p != 2 {
			t.Fatal("drawing, clearing and filling must preserve stencilled pixels")
		}
	}
	c.StencilEnabled = false
	c.Clear(0)
	c.Line(-3, -3, 12, 12, 2)
	for i := 0; i < 9; i++ {
		if c.Image.ColorIndexAt(i, i) != 2 {
			t.Fatal("line must clip without shifting its diagonal")
		}
	}
	c.Rectangle(7, 7, 1, 1, 3, false)
	if c.Image.ColorIndexAt(1, 4) != 3 || c.Image.ColorIndexAt(4, 1) != 3 || c.Image.ColorIndexAt(4, 3) != 0 {
		t.Fatal("rectangle outline should leave its interior untouched")
	}
}

func TestFloodFillConnectivityAndLargeRegion(t *testing.T) {
	c := New(640, 480, nil)
	c.Line(320, 0, 320, 479, 1)
	c.FloodFill(0, 0, 2)
	if c.Image.ColorIndexAt(319, 479) != 2 || c.Image.ColorIndexAt(320, 479) != 1 || c.Image.ColorIndexAt(321, 479) != 0 {
		t.Fatal("flood fill must stop at a closed boundary")
	}
	c.FloodFill(0, 0, 2)
	c.FloodFill(-1, 0, 3)
	small := New(2, 2, nil)
	small.Pixel(1, 0, 1)
	small.Pixel(0, 1, 1)
	small.FloodFill(0, 0, 2)
	if small.Image.ColorIndexAt(1, 1) != 0 {
		t.Fatal("diagonally adjacent pixels must not join a four-connected region")
	}
}

func TestEllipseSymmetryAndClippedFill(t *testing.T) {
	for _, size := range []image.Point{{9, 7}, {8, 6}, {2, 8}, {8, 2}, {1, 7}, {7, 1}} {
		for _, filled := range []bool{false, true} {
			c := New(size.X, size.Y, nil)
			c.Ellipse(0, 0, size.X-1, size.Y-1, 2, filled)
			for y := 0; y < size.Y; y++ {
				for x := 0; x < size.X; x++ {
					p := c.Image.ColorIndexAt(x, y)
					if p != c.Image.ColorIndexAt(size.X-1-x, y) || p != c.Image.ColorIndexAt(x, size.Y-1-y) {
						t.Fatalf("ellipse %v, filled=%v is asymmetric at %d,%d", size, filled, x, y)
					}
				}
			}
			if filled && c.Image.ColorIndexAt(size.X/2, size.Y/2) != 2 {
				t.Fatalf("filled ellipse %v must include its centre", size)
			}
		}
	}
	large, clipped := New(20, 20, nil), New(6, 6, nil)
	large.Ellipse(-2, -2, 9, 17, 2, true)
	clipped.Ellipse(-2, -2, 9, 17, 2, true)
	for y := 0; y < 6; y++ {
		for x := 0; x < 6; x++ {
			if large.Image.ColorIndexAt(x, y) != clipped.Image.ColorIndexAt(x, y) {
				t.Fatal("clipping must preserve the ellipse geometry")
			}
		}
	}
}

func TestConcavePolygon(t *testing.T) {
	c := New(10, 10, nil)
	c.Polygon([]image.Point{{1, 1}, {8, 1}, {8, 8}, {5, 8}, {5, 4}, {1, 4}}, 2, true)
	if c.Image.ColorIndexAt(2, 2) != 2 || c.Image.ColorIndexAt(7, 7) != 2 || c.Image.ColorIndexAt(2, 7) != 0 {
		t.Fatal("the even-odd fill must preserve a concave polygon's missing corner")
	}
	if c.Image.ColorIndexAt(1, 3) != 2 || c.Image.ColorIndexAt(8, 8) != 2 {
		t.Fatal("polygon outline must include the boundary")
	}
}

func TestBrushTransformMaskAndStamp(t *testing.T) {
	src := New(3, 2, nil)
	src.Image.Pix = []uint8{0, 2, 3, 4, 5, 6}
	b := Capture(src.Image, src.Image.Rect, 0)
	b.FlipX()
	if b.Image.ColorIndexAt(0, 0) != 3 || b.Mask[2] {
		t.Fatal("horizontal flip must transform pixels and transparency")
	}
	b.FlipY()
	b.Rotate90()
	if b.Image.Rect != image.Rect(0, 0, 2, 3) {
		t.Fatal("rotation must swap dimensions")
	}
	// After both flips and one clockwise turn, the rows are 3,6 / 2,5 / 0,4.
	want := []uint8{3, 6, 2, 5, 0, 4}
	for i, p := range b.Image.Pix {
		if p != want[i] {
			t.Fatalf("rotation pixel %d: got %d, want %d", i, p, want[i])
		}
	}
	b.Resize(4, 6)
	if b.Mask[4*4] || b.Mask[4*4+1] || !b.Mask[4*4+2] {
		t.Fatal("nearest-neighbour resize must preserve the opacity mask")
	}
	dst := New(8, 8, nil)
	dst.Clear(1)
	dst.Stamp(b, 4, 4, 9, true)
	if dst.Image.ColorIndexAt(2, 5) != 1 || dst.Image.ColorIndexAt(4, 5) != 4 || dst.Image.ColorIndexAt(2, 1) != 3 {
		t.Fatal("colour stamp must preserve source indices and transparent destination pixels")
	}
	dst.Stamp(b, 4, 4, 9, false)
	if dst.Image.ColorIndexAt(2, 5) != 1 || dst.Image.ColorIndexAt(4, 5) != 9 {
		t.Fatal("silhouette stamp must use the foreground colour")
	}
	dst.Stamp(b, 0, 0, 9, false)
}

func TestCaptureOutsideCanvasIsTransparent(t *testing.T) {
	c := New(2, 2, nil)
	c.Clear(2)
	b := Capture(c.Image, image.Rect(-1, -1, 2, 2), 0)
	if b.Image.Rect != image.Rect(0, 0, 3, 3) || b.Mask[0] || !b.Mask[4] {
		t.Fatal("out-of-bounds capture must keep the requested dimensions and mask its padding")
	}
	if Capture(c.Image, image.Rectangle{}, 0) != nil {
		t.Fatal("empty captures must return nil")
	}
}
