package ui

import (
	"image"
	"image/color"
	"math"
	"testing"

	"pixeluxe/internal/paint"
)

func TestShapeOutlineUsesBrushFootprintAndMask(t *testing.T) {
	g := toolGame(24, 18, 4)
	brush := image.NewPaletted(image.Rect(0, 0, 3, 1), g.Canvas.Image.Palette)
	brush.Pix = []uint8{2, 0, 1}
	g.Brush = &paint.Brush{Image: brush, Mask: []bool{true, false, true}}
	g.paintShape(g.Canvas, image.Pt(4, 4), image.Pt(14, 12), Rectangle, nil)
	for _, tc := range []struct {
		x, y  int
		color uint8
	}{{3, 8, 2}, {4, 8, 0}, {5, 8, 1}, {13, 8, 2}, {14, 8, 0}, {15, 8, 1}, {9, 8, 0}} {
		if got := g.Canvas.Image.ColorIndexAt(tc.x, tc.y); got != tc.color {
			t.Fatalf("custom contour pixel (%d,%d): got %d want %d", tc.x, tc.y, got, tc.color)
		}
	}
}

func TestShapeGradientUsesGeometricBounds(t *testing.T) {
	for _, tool := range []Tool{FilledRectangle, FilledEllipse, FilledCircle, FilledPolygon} {
		t.Run(toolNames[tool], func(t *testing.T) {
			g := toolGame(24, 20, 4)
			g.FillStyle, g.CycleLow, g.CycleHigh = 3, 1, 3
			bottom := 10
			if tool == FilledCircle {
				bottom = 14
			}
			var points []image.Point
			if tool == FilledPolygon {
				bottom = 12
				points = []image.Point{{4, 4}, {14, 4}, {9, 12}}
			}
			g.paintShape(g.Canvas, image.Pt(4, 4), image.Pt(14, 10), tool, points)
			if g.Canvas.Image.ColorIndexAt(9, 4) != 1 || g.Canvas.Image.ColorIndexAt(9, bottom) != 3 {
				t.Fatalf("gradient does not reach both geometric endpoints: top=%d bottom=%d", g.Canvas.Image.ColorIndexAt(9, 4), g.Canvas.Image.ColorIndexAt(9, bottom))
			}
		})
	}
}

func TestShapePatternTransparencySurvivesMirror(t *testing.T) {
	g := toolGame(24, 12, 4)
	g.Canvas.Clear(2)
	for y := 0; y < 12; y++ {
		for x := 16; x < 24; x++ {
			g.Canvas.Pixel(x, y, 3)
		}
	}
	brush := image.NewPaletted(image.Rect(0, 0, 2, 1), g.Canvas.Image.Palette)
	brush.Pix = []uint8{1, 1}
	g.Brush = &paint.Brush{Image: brush, Mask: []bool{true, false}}
	g.FillStyle, g.MirrorX = 1, true
	g.paintShape(g.Canvas, image.Pt(2, 2), image.Pt(5, 4), FilledRectangle, nil)
	for _, tc := range []struct {
		x, y  int
		color uint8
	}{{2, 3, 1}, {3, 3, 2}, {21, 3, 1}, {20, 3, 3}} {
		if got := g.Canvas.Image.ColorIndexAt(tc.x, tc.y); got != tc.color {
			t.Fatalf("pattern mirror changed opacity at (%d,%d): got %d want %d", tc.x, tc.y, got, tc.color)
		}
	}
}

func TestShapeWrappedPatternAnchorsAtBoundingBox(t *testing.T) {
	for _, style := range []int{1, 2} {
		g := toolGame(16, 12, 4)
		g.Canvas.Clear(2)
		brush := image.NewPaletted(image.Rect(0, 0, 2, 1), g.Canvas.Image.Palette)
		brush.Pix = []uint8{1, 1}
		g.Brush = &paint.Brush{Image: brush, Mask: []bool{true, false}}
		g.FillStyle = style
		g.paintShape(g.Canvas, image.Pt(3, 3), image.Pt(8, 7), FilledRectangle, nil)
		wantLeft, wantNext := uint8(2), uint8(1)
		if style == 2 {
			wantLeft, wantNext = 1, 2
		}
		if g.Canvas.Image.ColorIndexAt(3, 4) != wantLeft || g.Canvas.Image.ColorIndexAt(4, 4) != wantNext {
			t.Fatalf("pattern style %d used the wrong anchor", style)
		}
	}
}

func TestShapeBlendSnapshotsBeforeOverlappingStamps(t *testing.T) {
	g := toolGame(24, 16, 4)
	g.Canvas.Image.Palette = color.Palette{color.Black, color.RGBA{85, 85, 85, 255}, color.RGBA{170, 170, 170, 255}, color.White}
	g.Mode, g.FG, g.BrushSize, g.BrushShape = 5, 3, 2, 1
	g.paintShape(g.Canvas, image.Pt(4, 4), image.Pt(14, 10), Rectangle, nil)
	painted := 0
	for _, index := range g.Canvas.Image.Pix {
		if index == 0 {
			continue
		}
		painted++
		if index != 1 {
			t.Fatal("overlapping contour brush footprints repeatedly blended the same source pixel")
		}
	}
	if painted == 0 || g.effectSource != nil {
		t.Fatal("shape emitted no pixels or leaked its temporary effect source")
	}
	prior := paint.CloneImage(g.Canvas.Image)
	g.effectSource = prior
	g.paintShape(g.Canvas, image.Pt(5, 5), image.Pt(12, 9), Ellipse, nil)
	if g.effectSource != prior {
		t.Fatal("shape replaced its caller's effect source")
	}
}

func TestShapeEraseRestoresFixedBackgroundAndStencil(t *testing.T) {
	g := toolGame(20, 14, 4)
	g.Canvas.Clear(2)
	background := paint.New(20, 14, g.Canvas.Image.Palette)
	background.Clear(1)
	g.Background, g.FixedBackground, g.erase = background.Image, true, true
	g.Canvas.StencilEnabled = true
	g.Canvas.StencilMask = make([]bool, 20*14)
	g.Canvas.StencilMask[5*20+5] = true
	g.FillStyle, g.CycleLow, g.CycleHigh = 3, 0, 3
	g.paintShape(g.Canvas, image.Pt(4, 4), image.Pt(8, 8), FilledRectangle, nil)
	if g.Canvas.Image.ColorIndexAt(6, 6) != 1 || g.Canvas.Image.ColorIndexAt(5, 5) != 2 || g.Canvas.Image.ColorIndexAt(3, 3) != 2 {
		t.Fatal("shape erase ignored the fixed background, stencil, or geometry boundary")
	}
}

func TestShapePolygonUsesRadialAndMirrorSymmetry(t *testing.T) {
	g := toolGame(24, 24, 4)
	g.Radial, g.MirrorX, g.MirrorY = 4, true, true
	points := []image.Point{{2, 3}, {6, 3}, {4, 7}}
	g.paintShape(g.Canvas, image.Point{}, image.Point{}, FilledPolygon, points)
	if g.Canvas.Image.ColorIndexAt(4, 4) != 1 || g.Canvas.Image.ColorIndexAt(19, 4) != 1 || g.Canvas.Image.ColorIndexAt(4, 19) != 1 || g.Canvas.Image.ColorIndexAt(19, 19) != 1 || g.Canvas.Image.ColorIndexAt(20, 2) != 1 {
		t.Fatal("polygon geometry did not pass through radial and mirror symmetry")
	}
	if g.Canvas.Image.ColorIndexAt(11, 11) != 0 {
		t.Fatal("polygon symmetry unexpectedly filled the center of the canvas")
	}
}

func TestShapeClipsGeometryAndRejectsExtremeCoordinates(t *testing.T) {
	g := toolGame(24, 16, 4)
	g.BrushSize, g.BrushShape = 2, 1
	g.paintShape(g.Canvas, image.Pt(-1, 3), image.Pt(7, 12), Rectangle, nil)
	if g.Canvas.Image.ColorIndexAt(0, 7) != 1 {
		t.Fatal("a contour brush just outside the canvas lost its visible footprint")
	}
	before := paint.CloneImage(g.Canvas.Image)
	g.paintShape(g.Canvas, image.Pt(math.MinInt, math.MinInt), image.Pt(math.MaxInt, math.MaxInt), FilledRectangle, nil)
	assertToolImage(t, g.Canvas.Image, before)
	g.paintShape(g.Canvas, image.Point{}, image.Point{}, FilledPolygon, nil)
	g.paintShape(g.Canvas, image.Point{}, image.Point{}, Dots, nil)
	g.paintShape(nil, image.Point{}, image.Point{}, Rectangle, nil)
}
