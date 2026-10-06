package ui

import (
	"bytes"
	"image"
	"image/color"
	"reflect"
	"testing"

	"pixeluxe/internal/formats"
	"pixeluxe/internal/paint"
)

func TestDesktopLayoutSwitchKeepsDocument(t *testing.T) {
	g := New()
	if !g.Modern {
		t.Fatal("new documents should open in the modern desktop")
	}
	original := paint.CloneImage(g.Canvas.Image)
	modernWidth, modernHeight := g.Layout(0, 0)
	if modernWidth <= Width || modernHeight <= Height {
		t.Fatal("the modern desktop should provide more workspace than the classic interface")
	}
	for _, modern := range []bool{true, false, true} {
		g.Modern = modern
		w, h := g.Layout(0, 0)
		if g.Render().Bounds() != image.Rect(0, 0, w, h) {
			t.Fatal("rendered frame does not match the requested desktop layout")
		}
		if !modern && (w != Width || h != Height) {
			t.Fatal("classic desktop no longer uses the original screen dimensions")
		}
		assertToolImage(t, g.Canvas.Image, original)
		if g.Canvas.Dirty() || g.Canvas.Undo() {
			t.Fatal("changing the desktop must preserve a clean document and its history")
		}
	}
}

func TestModernCanvasCoordinatesMatchRenderedPixels(t *testing.T) {
	g := New()
	g.pointer = image.Pt(-1, -1)
	palette := color.Palette{color.Black, color.RGBA{R: 243, G: 37, B: 19, A: 255}, color.RGBA{R: 17, G: 163, B: 211, A: 255}}
	g.Canvas = paint.New(47, 29, palette)
	for y := 0; y < 29; y++ {
		for x := 0; x < 47; x++ {
			g.Canvas.Image.SetColorIndex(x, y, uint8(1+(x+y)%2))
		}
	}
	g.Canvas.MarkSaved()
	for _, zoom := range []int{1, 2, 4, 16} {
		g.Zoom, g.PanX, g.PanY = zoom, 0, 0
		frame := g.Render()
		o := g.canvasOrigin()
		for _, pixel := range []image.Point{{0, 0}, {13, 7}, {46, 28}} {
			point := image.Pt(o.X+pixel.X*zoom+zoom/2, o.Y+pixel.Y*zoom+zoom/2)
			if !point.In(g.viewport()) {
				continue
			}
			if got := g.canvasPoint(point); got != pixel {
				t.Fatalf("at zoom %d, visible pixel %v maps to %v", zoom, pixel, got)
			}
			want := color.RGBAModel.Convert(g.Canvas.Image.At(pixel.X, pixel.Y))
			if got := frame.RGBAAt(point.X, point.Y); got != want {
				t.Fatalf("at zoom %d, pixel %v renders %v instead of its palette color %v", zoom, pixel, got, want)
			}
		}
		if got := g.canvasPoint(o.Sub(image.Pt(1, 1))); got != image.Pt(-1, -1) {
			t.Fatalf("outside pixels should round toward the preceding cell, got %v", got)
		}
	}
}

func TestModernZoomKeepsClickedPixelWhenPageOverflows(t *testing.T) {
	g := New()
	g.Zoom = 2
	viewport := g.viewport()
	if g.Canvas.Image.Bounds().Dx()*g.Zoom > viewport.Dx() || g.Canvas.Image.Bounds().Dy()*g.Zoom > viewport.Dy() {
		t.Fatal("the default document should fit fully inside the modern workspace")
	}
	o := g.canvasOrigin()
	pixel := image.Pt(140, 100)
	point := image.Pt(o.X+pixel.X*g.Zoom+1, o.Y+pixel.Y*g.Zoom+1)
	for _, zoom := range []int{4, 8, 16, 8, 4} {
		g.zoomAt(zoom, point)
		if got := g.canvasPoint(point); got != pixel {
			t.Fatalf("zooming a centered page to %dx moved clicked pixel %v to %v", zoom, pixel, got)
		}
	}
}

func TestModernRenderingPreservesIndexedDocumentAndRedo(t *testing.T) {
	g := New()
	palette := make(color.Palette, 256)
	for i := range palette {
		palette[i] = color.RGBA{R: uint8(i), G: uint8(255 - i), B: uint8(i * 17), A: 255}
	}
	g.Canvas = paint.New(32, 16, palette)
	for i := range g.Canvas.Image.Pix {
		g.Canvas.Image.Pix[i] = uint8(i)
	}
	g.Canvas.MarkSaved()
	g.FG, g.BG = 250, 252
	g.Cycle, g.MultiCycle = true, true
	g.CycleLow, g.CycleHigh, g.CycleSpeed = 2, 5, 2
	g.pointer = image.Pt(-1, -1)
	g.Canvas.Stencil[3], g.Canvas.StencilEnabled = true, true
	g.Canvas.StencilMask = make([]bool, len(g.Canvas.Image.Pix))
	g.Canvas.StencilMask[4] = true
	g.Background = paint.CloneImage(g.Canvas.Image)
	g.FixedBackground = true
	g.FileMetadata = &formats.Metadata{ColorRanges: []formats.ColorRange{{Rate: 8192, Flags: 1, Low: 2, High: 5}}}
	g.edit(func() { g.Canvas.Pixel(1, 1, 7) })
	g.edit(func() { g.Canvas.Pixel(2, 1, 8) })
	completed := paint.CloneImage(g.Canvas.Image)
	if !g.Canvas.Undo() {
		t.Fatal("setup failed to create a redo action")
	}
	before := paint.CloneImage(g.Canvas.Image)
	background := paint.CloneImage(g.Background)
	mask := append([]bool(nil), g.Canvas.StencilMask...)
	metadata := append([]formats.ColorRange(nil), g.FileMetadata.ColorRanges...)
	for _, tick := range []int{2, 5, 11} {
		g.cycleTick = tick
		g.Render()
		assertToolImage(t, g.Canvas.Image, before)
		assertToolImage(t, g.Background, background)
		if !reflect.DeepEqual(g.Canvas.StencilMask, mask) || !reflect.DeepEqual(g.FileMetadata.ColorRanges, metadata) || !g.FixedBackground || !g.Canvas.StencilEnabled {
			t.Fatal("rendering changed stencil, background or color range settings")
		}
	}
	if !g.Canvas.Redo() {
		t.Fatal("rendering consumed the existing redo action")
	}
	assertToolImage(t, g.Canvas.Image, completed)
}

func modernControlCenter(r image.Rectangle) image.Point {
	return r.Min.Add(r.Size().Div(2))
}

func TestModernControlsStayOutsidePaintingViewport(t *testing.T) {
	g := New()
	for _, showBar := range []bool{true, false} {
		g.ShowBar = showBar
		viewport := g.viewport()
		var controls []image.Rectangle
		for _, r := range g.modernControls() {
			controls = append(controls, r)
		}
		for i := range modernTools {
			controls = append(controls, g.modernToolRect(i))
		}
		for i := 0; i < 10; i++ {
			controls = append(controls, g.modernPresetRect(i))
		}
		for i := 0; i < 32; i++ {
			controls = append(controls, g.modernSwatchRect(i))
		}
		controls = append(controls, g.modernPalettePageRect(false), g.modernPalettePageRect(true))
		for i, r := range controls {
			if r.Overlaps(viewport) {
				t.Fatalf("control %v overlaps the painting viewport when menu bar visibility is %v", r, showBar)
			}
			for _, other := range controls[i+1:] {
				if r.Overlaps(other) {
					t.Fatalf("controls %v and %v have ambiguous hit regions", r, other)
				}
			}
		}
		if g.clickModern(modernControlCenter(viewport), false) {
			t.Fatal("an ordinary canvas click was consumed by desktop controls")
		}
	}
}

func TestModernToolRailPreservesToolsAndFilledVariants(t *testing.T) {
	g := New()
	wanted := []Tool{Freehand, Dots, Line, Curve, Rectangle, Circle, Ellipse, Polygon, Fill, Airbrush, BrushSelect, Picker, Text, Magnify, ZoomTool}
	for _, tool := range wanted {
		index := -1
		for i, candidate := range modernTools {
			if candidate == tool {
				index = i
				break
			}
		}
		if index < 0 {
			t.Fatalf("drawing tool %s is missing from the modern rail", toolNames[tool])
		}
		if !g.clickModern(modernControlCenter(g.modernToolRect(index)), false) || g.Tool != tool {
			t.Fatalf("rail click did not select %s", toolNames[tool])
		}
		filled := map[Tool]Tool{Rectangle: FilledRectangle, Circle: FilledCircle, Ellipse: FilledEllipse, Polygon: FilledPolygon}
		if want, ok := filled[tool]; ok {
			if !g.clickModern(modernControlCenter(g.modernToolRect(index)), true) || g.Tool != want {
				t.Fatalf("right-click on %s did not select its filled variant", toolNames[tool])
			}
		}
	}
	if g.Canvas.Dirty() || g.Canvas.Undo() {
		t.Fatal("selecting drawing tools must not edit the document")
	}
}

func TestModernPresetsReplaceCustomBrushWithoutChangingPicture(t *testing.T) {
	g := New()
	original := paint.CloneImage(g.Canvas.Image)
	expected := [][2]int{{0, 1}, {0, 2}, {0, 3}, {0, 5}, {1, 1}, {1, 2}, {1, 3}, {1, 5}, {2, 3}, {2, 5}}
	for i, preset := range expected {
		g.Brush = paint.Capture(g.Canvas.Image, image.Rect(0, 0, 3, 3), g.BG)
		if !g.clickModern(modernControlCenter(g.modernPresetRect(i)), false) {
			t.Fatalf("brush preset %d is unreachable", i)
		}
		if g.Brush != nil || g.BrushShape != preset[0] || g.BrushSize != preset[1] {
			t.Fatalf("brush preset %d did not replace the custom brush with shape %d and size %d", i, preset[0], preset[1])
		}
		assertToolImage(t, g.Canvas.Image, original)
	}
	if g.Canvas.Dirty() || g.Canvas.Undo() {
		t.Fatal("selecting a built-in brush must leave the picture and its history unchanged")
	}
}

func TestModernPalettePagesReachEveryForegroundAndBackgroundColor(t *testing.T) {
	g := New()
	palette := make(color.Palette, 256)
	for i := range palette {
		palette[i] = color.RGBA{R: uint8(i), G: uint8(255 - i), A: 255}
	}
	g.Canvas = paint.New(8, 8, palette)
	for page := 0; page < 8; page++ {
		for cell := 0; cell < 32; cell++ {
			point := modernControlCenter(g.modernSwatchRect(cell))
			want := uint8(page*32 + cell)
			if !g.clickModern(point, false) || g.FG != want {
				t.Fatalf("palette color %d cannot be selected as foreground", want)
			}
			if !g.clickModern(point, true) || g.BG != want {
				t.Fatalf("palette color %d cannot be selected as background", want)
			}
		}
		g.clickModern(modernControlCenter(g.modernPalettePageRect(true)), false)
	}
	if g.PalettePage != 7 {
		t.Fatal("palette Next advanced beyond the last populated page")
	}
	for i := 0; i < 8; i++ {
		g.clickModern(modernControlCenter(g.modernPalettePageRect(false)), false)
	}
	if g.PalettePage != 0 {
		t.Fatal("palette Previous advanced before the first page")
	}
	if g.Canvas.Dirty() || g.Canvas.Undo() {
		t.Fatal("choosing palette colors or pages must not edit the document")
	}
	g.PalettePage = 7
	g.install(paint.New(8, 8, palette[:37]).Image, "short.png")
	g.Render()
	if g.PalettePage > 1 {
		t.Fatal("opening a smaller palette left the inspector on an empty page")
	}
	g.PalettePage, g.FG, g.BG = 1, 35, 36
	g.clickModern(modernControlCenter(g.modernSwatchRect(5)), false)
	g.clickModern(modernControlCenter(g.modernSwatchRect(5)), true)
	if g.FG != 35 || g.BG != 36 {
		t.Fatal("an empty palette cell changed the active foreground or background")
	}
}

func TestModernQuickActionsUseDocumentHistoryAndRequesters(t *testing.T) {
	g := New()
	initial := paint.CloneImage(g.Canvas.Image)
	g.Tool = Freehand
	g.startTool(image.Pt(12, 8))
	g.finishTool(image.Pt(18, 8))
	drawn := paint.CloneImage(g.Canvas.Image)
	controls := g.modernControls()
	if !g.clickModern(modernControlCenter(controls["undo"]), false) {
		t.Fatal("quick Undo is unreachable")
	}
	assertToolImage(t, g.Canvas.Image, initial)
	if !g.clickModern(modernControlCenter(controls["redo"]), false) {
		t.Fatal("quick Redo is unreachable")
	}
	assertToolImage(t, g.Canvas.Image, drawn)
	for _, action := range []string{"new", "load", "save"} {
		t.Run(action, func(t *testing.T) {
			fresh := New()
			if !fresh.clickModern(modernControlCenter(fresh.modernControls()[action]), false) || fresh.dialog == nil {
				t.Fatalf("quick %s did not open its requester", action)
			}
		})
	}
	g.clickModern(modernControlCenter(controls["new"]), false)
	if g.dialog == nil || g.dialog.kind != "new" {
		t.Fatal("quick New did not open the existing document settings requester")
	}
	g.dialog.accept()
	if g.dialog == nil || g.dialog.kind != "confirm" {
		t.Fatal("accepting quick New settings did not protect the unsaved drawing")
	}
	assertToolImage(t, g.Canvas.Image, drawn)
}

func TestModernOptionControlsKeepPaintingSettingsAccessible(t *testing.T) {
	g := New()
	click := func(key string, right bool) {
		t.Helper()
		if !g.clickModern(modernControlCenter(g.modernControls()[key]), right) {
			t.Fatalf("option control %s is unreachable", key)
		}
	}
	for mode := range modeNames {
		click("mode", false)
		if g.dialog == nil || !reflect.DeepEqual(g.dialog.entries, modeNames) {
			t.Fatal("paint mode control did not offer all original modes")
		}
		g.dialog.selected = mode
		g.dialog.accept()
		if g.Mode != mode {
			t.Fatalf("paint mode control did not apply %s", modeNames[mode])
		}
	}
	click("fill-settings", false)
	if g.dialog == nil || len(g.dialog.entries) != 5 {
		t.Fatal("fill options are missing from the modern toolbar")
	}
	g.dialog.selected = 4
	g.dialog.accept()
	if g.FillStyle != 4 {
		t.Fatal("toolbar fill settings did not apply the dithered gradient")
	}
	for _, tc := range []struct {
		key, requester string
		active         func() bool
	}{{"grid", "grid-settings", func() bool { return g.UseGrid }}, {"symmetry", "symmetry-settings", func() bool { return g.MirrorX }}, {"cycle", "range", func() bool { return g.Cycle }}} {
		click(tc.key, false)
		if !tc.active() {
			t.Fatalf("toolbar %s toggle did not activate its feature", tc.key)
		}
		click(tc.key, true)
		if g.dialog == nil || g.dialog.kind != tc.requester {
			t.Fatalf("toolbar %s right-click did not open its settings", tc.key)
		}
		g.cancelRequester(g.dialog)
	}
	g.Brush = paint.Capture(g.Canvas.Image, image.Rect(0, 0, 3, 3), g.BG)
	g.BrushSize = 1
	click("brush-smaller", false)
	if g.Brush != nil || g.BrushSize != 1 {
		t.Fatal("decreasing a brush should restore a built-in brush without going below one pixel")
	}
	g.BrushSize = 16
	click("brush-larger", false)
	if g.BrushSize != 16 {
		t.Fatal("increasing a brush exceeded the existing size limit")
	}
	g.Zoom = 2
	click("zoom-in", false)
	if g.Zoom != 4 {
		t.Fatal("toolbar zoom-in did not double the view scale")
	}
	click("zoom-out", false)
	if g.Zoom != 2 {
		t.Fatal("toolbar zoom-out did not halve the view scale")
	}
	g.Zoom = 8
	click("zoom-reset", false)
	if g.Zoom != 2 {
		t.Fatal("toolbar zoom reset did not restore the default view scale")
	}
	if g.Canvas.Dirty() || g.Canvas.Undo() {
		t.Fatal("changing painting options must not edit the indexed picture")
	}
}

func TestModernCoordinatesPreferenceChangesDisplayedFooter(t *testing.T) {
	g := New()
	g.pointer = image.Pt(-1, -1)
	withCoords := append([]byte(nil), g.Render().Pix...)
	g.action("coords")
	if g.ShowCoords {
		t.Fatal("coordinates preference did not switch off")
	}
	withoutCoords := g.Render()
	if bytes.Equal(withCoords, withoutCoords.Pix) {
		t.Fatal("coordinates preference has no visible effect on the modern desktop")
	}
	g.action("coords")
	if !bytes.Equal(withCoords, g.Render().Pix) {
		t.Fatal("switching coordinates back on did not restore the displayed footer")
	}
}
