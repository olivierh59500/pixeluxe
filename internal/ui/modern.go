package ui

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
)

const (
	modernWidth          = 1184
	modernHeight         = 768
	modernHeaderHeight   = 42
	modernMenuHeight     = 30
	modernOptionsHeight  = 40
	modernFooterHeight   = 26
	modernToolsWidth     = 58
	modernInspectorWidth = 272
)

var modernTools = []Tool{Freehand, Dots, Line, Curve, Rectangle, Circle, Ellipse, Polygon, Fill, Airbrush, BrushSelect, Picker, Text, Grid, Symmetry, Magnify, ZoomTool}

func (g *Game) modernViewport() image.Rectangle {
	w, h := g.screenSize()
	left, right := modernToolsWidth, w-modernInspectorWidth
	if !g.ShowTools {
		left, right = 0, w
	}
	top := modernHeaderHeight + modernOptionsHeight + 14
	if g.ShowBar || g.menu >= 0 {
		top += modernMenuHeight
	}
	return image.Rect(left+12, top, right-14, h-modernFooterHeight-10)
}
func (g *Game) modernToolRect(i int) image.Rectangle {
	top := modernHeaderHeight + modernOptionsHeight + modernMenuHeight + 12
	return image.Rect(10, top+i*34, 48, top+i*34+30)
}
func (g *Game) modernControls() map[string]image.Rectangle {
	w, _ := g.screenSize()
	y := modernHeaderHeight + 6
	if g.ShowBar || g.menu >= 0 {
		y += modernMenuHeight
	}
	return map[string]image.Rectangle{
		"new": image.Rect(w-270, 8, w-198, 34), "load": image.Rect(w-190, 8, w-118, 34), "save": image.Rect(w-110, 8, w-16, 34),
		"undo": image.Rect(76, y, 106, y+28), "redo": image.Rect(112, y, 142, y+28), "mode": image.Rect(156, y, 270, y+28),
		"brush-smaller": image.Rect(284, y, 312, y+28), "brush-size": image.Rect(318, y, 396, y+28), "brush-larger": image.Rect(402, y, 430, y+28),
		"fill-settings": image.Rect(446, y, 562, y+28), "grid": image.Rect(578, y, 640, y+28), "symmetry": image.Rect(650, y, 738, y+28), "cycle": image.Rect(748, y, 820, y+28),
		"zoom-out": image.Rect(w-198, y, w-168, y+28), "zoom-reset": image.Rect(w-162, y, w-70, y+28), "zoom-in": image.Rect(w-64, y, w-34, y+28),
	}
}
func (g *Game) modernPresetRect(i int) image.Rectangle {
	w, _ := g.screenSize()
	x := w - modernInspectorWidth + 18
	row, col := i/5, i%5
	return image.Rect(x+col*47, 218+row*42, x+col*47+39, 254+row*42)
}
func (g *Game) modernSwatchRect(i int) image.Rectangle {
	w, _ := g.screenSize()
	x := w - modernInspectorWidth + 18
	return image.Rect(x+i%8*30, 428+i/8*30, x+i%8*30+25, 453+i/8*30)
}
func (g *Game) modernPalettePageRect(next bool) image.Rectangle {
	w, _ := g.screenSize()
	x := w - 77
	if next {
		x = w - 43
	}
	return image.Rect(x, 566, x+26, 589)
}

func (g *Game) renderModern() *image.RGBA {
	dst := g.frame
	w, h := g.screenSize()
	fill(dst, dst.Bounds(), modernBG)
	g.normalize()
	r := g.viewport()
	im := g.Canvas.Image
	if g.preview != nil {
		im = g.preview
	}
	o := g.canvasOrigin()
	// Workspace dots are outside the picture only; the document stays exact.
	for y := r.Min.Y + 12; y < r.Max.Y; y += 24 {
		for x := r.Min.X + 12; x < r.Max.X; x += 24 {
			dst.Set(x, y, modernBorder)
		}
	}
	page := image.Rect(o.X, o.Y, o.X+im.Rect.Dx()*g.Zoom, o.Y+im.Rect.Dy()*g.Zoom)
	visible := page.Intersect(r)
	if !visible.Empty() {
		modernFill(dst, visible.Add(image.Pt(6, 7)).Intersect(r), color.RGBA{12, 13, 18, 255}, 4)
	}
	palette := g.displayPalette(im.Palette)
	colors := make([]color.NRGBA, len(palette))
	for i, c := range palette {
		colors[i] = color.NRGBAModel.Convert(c).(color.NRGBA)
	}
	for y := visible.Min.Y; y < visible.Max.Y; y++ {
		cy := (y - o.Y) / g.Zoom
		row := dst.PixOffset(visible.Min.X, y)
		for x := visible.Min.X; x < visible.Max.X; x++ {
			cx := (x - o.X) / g.Zoom
			idx := im.ColorIndexAt(cx, cy)
			cc := colors[min(int(idx), len(colors)-1)]
			if cc.A == 0 {
				if (cx/8+cy/8)%2 == 0 {
					cc = color.NRGBA{55, 58, 70, 255}
				} else {
					cc = color.NRGBA{43, 46, 56, 255}
				}
			}
			if g.ShowGrid && g.Zoom >= 4 && (cx%g.GridSize == 0 && (x-o.X)%g.Zoom == 0 || cy%g.GridSize == 0 && (y-o.Y)%g.Zoom == 0) {
				cc = color.NRGBA{cc.R ^ 64, cc.G ^ 64, cc.B ^ 64, 255}
			}
			dst.Pix[row] = cc.R
			dst.Pix[row+1] = cc.G
			dst.Pix[row+2] = cc.B
			dst.Pix[row+3] = 255
			row += 4
		}
	}
	if !visible.Empty() {
		outline(dst, visible.Inset(-1).Intersect(r), modernBorder)
	}
	if g.Tool == BrushSelect && g.dragging {
		p := g.canvasPoint(g.pointer)
		g.marquee(image.Rect(o.X+min(g.start.X, p.X)*g.Zoom, o.Y+min(g.start.Y, p.Y)*g.Zoom, o.X+(max(g.start.X, p.X)+1)*g.Zoom, o.Y+(max(g.start.Y, p.Y)+1)*g.Zoom).Intersect(r))
	}
	if g.MirrorX {
		for y := r.Min.Y; y < r.Max.Y; y += 8 {
			dst.Set(o.X+im.Rect.Dx()*g.Zoom/2, y, modernAccent)
		}
	}
	if g.MirrorY {
		for x := r.Min.X; x < r.Max.X; x += 8 {
			dst.Set(x, o.Y+im.Rect.Dy()*g.Zoom/2, modernAccent)
		}
	}
	g.drawModernHeader(dst)
	g.drawModernOptions(dst)
	if g.ShowTools {
		g.drawModernTools(dst)
		g.drawModernInspector(dst)
	}
	fill(dst, image.Rect(0, h-modernFooterHeight, w, h), modernPanel)
	drawLine(dst, 0, h-modernFooterHeight, w-1, h-modernFooterHeight, modernBorder)
	status := g.Status
	if g.statusTicks == 0 {
		status = toolNames[g.Tool] + " · " + modeNames[g.Mode] + " · Left: foreground · Right: background"
	}
	modernTextAt(dst, modernFit(status, 12, w-350), 18, h-20, 12, modernMuted)
	if g.ShowCoords {
		p := g.canvasPoint(g.pointer)
		coords := fmt.Sprintf("%d, %d   ·   %d%%", p.X, p.Y, g.Zoom*100)
		modernTextAt(dst, coords, w-modernTextWidth(coords, 12)-18, h-20, 12, modernMuted)
	}
	if g.menu >= 0 {
		g.drawMenu(dst)
	}
	if g.dialog != nil {
		g.drawDialog(dst)
	} else if g.menu < 0 && g.pointer.In(r) {
		g.drawCursor(dst)
	} else if g.menu < 0 && g.ShowTools {
		g.drawModernTooltip(dst)
	}
	return dst
}

func (g *Game) drawModernHeader(dst *image.RGBA) {
	w, _ := g.screenSize()
	fill(dst, image.Rect(0, 0, w, modernHeaderHeight), modernPanel)
	drawLine(dst, 0, modernHeaderHeight-1, w-1, modernHeaderHeight-1, modernBorder)
	modernFill(dst, image.Rect(16, 9, 40, 33), modernAccentSoft, 5)
	fill(dst, image.Rect(22, 15, 27, 26), modernAccent)
	fill(dst, image.Rect(28, 15, 34, 20), modernAccent)
	fill(dst, image.Rect(28, 22, 33, 27), modernWarm)
	modernTextAt(dst, "Pixeluxe", 50, 9, 20, modernText)
	modernTextAt(dst, "PIXEL ART STUDIO", 145, 16, 10, modernMuted)
	title := "Untitled"
	if g.Filename != "" {
		title = filepath.Base(g.Filename)
	}
	if g.Canvas.Dirty() || g.RangeChanged {
		title += "  •"
	}
	modernTextAt(dst, modernFit(title, 13, w-660), 328, 13, 13, modernText)
	for _, entry := range []struct{ key, label string }{{"new", "New"}, {"load", "Open"}, {"save", "Save"}} {
		rr := g.modernControls()[entry.key]
		modernButton(dst, rr, entry.label, entry.key == "save", g.pointer.In(rr))
	}
	if g.ShowBar || g.menu >= 0 {
		fill(dst, image.Rect(0, modernHeaderHeight, w, modernHeaderHeight+modernMenuHeight), modernPanel)
		for i, m := range menus {
			rr := g.menuTitleRect(i)
			if i == g.menu {
				modernFill(dst, rr.Inset(2), modernRaised, 5)
			}
			modernTextAt(dst, m.Title, rr.Min.X+12, rr.Min.Y+7, 13, modernMuted)
		}
		drawLine(dst, 0, modernHeaderHeight+modernMenuHeight-1, w-1, modernHeaderHeight+modernMenuHeight-1, modernBorder)
	}
}
func (g *Game) drawModernOptions(dst *image.RGBA) {
	w, _ := g.screenSize()
	y := modernHeaderHeight
	if g.ShowBar || g.menu >= 0 {
		y += modernMenuHeight
	}
	fill(dst, image.Rect(0, y, w, y+modernOptionsHeight), modernPanel)
	drawLine(dst, 0, y+modernOptionsHeight-1, w-1, y+modernOptionsHeight-1, modernBorder)
	controls := g.modernControls()
	fillNames := []string{"Solid fill", "Brush pattern", "Wrapped brush", "Gradient", "Dithered fill"}
	labels := map[string]string{"undo": "", "redo": "", "mode": modeNames[g.Mode], "brush-smaller": "−", "brush-size": fmt.Sprintf("%d px", g.BrushSize*2-1), "brush-larger": "+", "fill-settings": fillNames[min(g.FillStyle, len(fillNames)-1)], "grid": "Grid", "symmetry": "Symmetry", "cycle": "Cycle", "zoom-out": "−", "zoom-reset": fmt.Sprintf("%d%%", g.Zoom*100), "zoom-in": "+"}
	if g.Brush != nil {
		labels["brush-size"] = "Custom"
	}
	for key, label := range labels {
		r := controls[key]
		selected := key == "grid" && g.UseGrid || key == "symmetry" && (g.MirrorX || g.MirrorY || g.Radial > 1) || key == "cycle" && g.Cycle
		modernButton(dst, r, label, selected, g.pointer.In(r))
		if key == "mode" || key == "fill-settings" {
			cx, cy := r.Max.X-12, r.Min.Y+r.Dy()/2
			drawLine(dst, cx-3, cy-1, cx, cy+2, modernMuted)
			drawLine(dst, cx, cy+2, cx+3, cy-1, modernMuted)
		}
		if key == "undo" || key == "redo" {
			direction := -1
			if key == "redo" {
				direction = 1
			}
			cx, cy := r.Min.X+r.Dx()/2, r.Min.Y+r.Dy()/2
			points := []image.Point{{cx - direction*7, cy + 6}, {cx - direction*7, cy - 2}, {cx - direction*3, cy - 5}, {cx + direction*7, cy - 5}}
			for i := 1; i < len(points); i++ {
				a, b := points[i-1], points[i]
				drawLine(dst, a.X, a.Y, b.X, b.Y, modernMuted)
			}
			tip := points[len(points)-1]
			drawLine(dst, tip.X, tip.Y, tip.X-direction*4, tip.Y-4, modernMuted)
			drawLine(dst, tip.X, tip.Y, tip.X-direction*4, tip.Y+4, modernMuted)
		}
	}
}
func (g *Game) drawModernTools(dst *image.RGBA) {
	_, h := g.screenSize()
	fill(dst, image.Rect(0, modernHeaderHeight+modernMenuHeight, modernToolsWidth, h-modernFooterHeight), modernPanel)
	drawLine(dst, modernToolsWidth-1, modernHeaderHeight+modernMenuHeight, modernToolsWidth-1, h-modernFooterHeight, modernBorder)
	for i, t := range modernTools {
		r := g.modernToolRect(i)
		selected := g.Tool == t || t == Rectangle && g.Tool == FilledRectangle || t == Ellipse && g.Tool == FilledEllipse || t == Circle && g.Tool == FilledCircle || t == Polygon && g.Tool == FilledPolygon || t == Grid && g.UseGrid || t == Symmetry && (g.MirrorX || g.MirrorY || g.Radial > 1)
		fg := modernMuted
		if selected {
			modernFill(dst, r, modernAccentSoft, 7)
			fg = modernAccent
		} else if g.pointer.In(r) {
			modernFill(dst, r, modernRaised, 7)
			fg = modernText
		}
		modernIcon(dst, t, r, fg)
	}
}
func (g *Game) drawModernInspector(dst *image.RGBA) {
	w, h := g.screenSize()
	x := w - modernInspectorWidth
	fill(dst, image.Rect(x, modernHeaderHeight+modernMenuHeight+modernOptionsHeight, w, h-modernFooterHeight), modernPanel)
	drawLine(dst, x, 112, x, h-modernFooterHeight, modernBorder)
	modernTextAt(dst, "DOCUMENT", x+18, 130, 11, modernMuted)
	modernTextAt(dst, fmt.Sprintf("%d × %d", g.Canvas.Image.Rect.Dx(), g.Canvas.Image.Rect.Dy()), x+18, 151, 20, modernText)
	s := fmt.Sprintf("%d indexed colors  ·  %s", len(g.Canvas.Image.Palette), modeNames[g.Mode])
	modernTextAt(dst, s, x+18, 180, 12, modernMuted)
	modernTextAt(dst, "BRUSH PRESETS", x+18, 199, 11, modernMuted)
	for i := 0; i < 10; i++ {
		r := g.modernPresetRect(i)
		shape, size := presetAt(i)
		selected := g.Brush == nil && g.BrushShape == shape && g.BrushSize == size
		fg := modernMuted
		if selected {
			modernFill(dst, r, modernAccentSoft, 5)
			fg = modernAccent
		} else if g.pointer.In(r) {
			modernFill(dst, r, modernRaised, 5)
			fg = modernText
		}
		cx, cy := r.Min.X+r.Dx()/2, r.Min.Y+r.Dy()/2
		radius := max(1, size-1)
		switch shape {
		case 0:
			disk(dst, cx, cy, radius, radius, fg, true)
		case 1:
			fill(dst, image.Rect(cx-radius, cy-radius, cx+radius+1, cy+radius+1), fg)
		case 2:
			for yy := -radius; yy <= radius; yy++ {
				for xx := -radius; xx <= radius; xx++ {
					if (xx+yy)&1 == 0 {
						dst.Set(cx+xx, cy+yy, fg)
					}
				}
			}
		}
	}
	palette := g.displayPalette(g.Canvas.Image.Palette)
	modernTextAt(dst, "COLORS", x+18, 317, 11, modernMuted)
	for i, idx := range []uint8{g.FG, g.BG} {
		rr := image.Rect(x+18+i*124, 340, x+132+i*124, 388)
		modernFill(dst, rr, modernRaised, 7)
		modernFill(dst, image.Rect(rr.Min.X+8, rr.Min.Y+8, rr.Min.X+40, rr.Max.Y-8), palette[idx], 5)
		label := "Foreground"
		if i == 1 {
			label = "Background"
		}
		modernTextAt(dst, label, rr.Min.X+48, rr.Min.Y+9, 10, modernMuted)
		modernTextAt(dst, fmt.Sprintf("%02d", idx), rr.Min.X+48, rr.Min.Y+23, 14, modernText)
	}
	modernTextAt(dst, "PALETTE", x+18, 406, 11, modernMuted)
	for i := 0; i < 32; i++ {
		idx := g.PalettePage*32 + i
		r := g.modernSwatchRect(i)
		if idx >= len(palette) {
			modernFill(dst, r, modernBG, 4)
			continue
		}
		if uint8(idx) == g.FG {
			modernFill(dst, r.Inset(-3), modernAccent, 5)
		}
		modernFill(dst, r, palette[idx], 3)
		if uint8(idx) == g.BG {
			disk(dst, r.Max.X-5, r.Max.Y-5, 2, 2, modernText, true)
		}
	}
	modernTextAt(dst, fmt.Sprintf("%d / %d", g.PalettePage+1, (len(palette)+31)/32), x+18, 570, 11, modernMuted)
	for _, next := range []bool{false, true} {
		r := g.modernPalettePageRect(next)
		s := "‹"
		if next {
			s = "›"
		}
		modernButton(dst, r, s, false, g.pointer.In(r))
	}
	edit := image.Rect(x+18, 604, w-18, 635)
	modernButton(dst, edit, "Edit palette", false, g.pointer.In(edit))
	swap := image.Rect(x+18, 646, w-18, 677)
	modernButton(dst, swap, "Swap foreground / background", false, g.pointer.In(swap))
	info := "Grid off · Symmetry off"
	if g.UseGrid {
		info = fmt.Sprintf("Grid %d px", g.GridSize)
	}
	if g.MirrorX || g.MirrorY || g.Radial > 1 {
		info += " · Symmetry on"
	}
	modernTextAt(dst, modernFit(info, 11, modernInspectorWidth-36), x+18, 699, 11, modernMuted)
}
func presetAt(i int) (shape, size int) {
	switch {
	case i < 4:
		return 0, []int{1, 2, 3, 5}[i]
	case i < 8:
		return 1, []int{1, 2, 3, 5}[i-4]
	case i == 8:
		return 2, 3
	default:
		return 2, 5
	}
}

func (g *Game) drawModernTooltip(dst *image.RGBA) {
	for i, t := range modernTools {
		rr := g.modernToolRect(i)
		if g.pointer.In(rr) {
			label := toolNames[t]
			if t == Rectangle || t == Circle || t == Ellipse || t == Polygon {
				label += "  ·  Right-click: filled"
			}
			r := image.Rect(rr.Max.X+8, rr.Min.Y, rr.Max.X+24+modernTextWidth(label, 12), rr.Min.Y+28)
			modernFill(dst, r, modernRaised, 5)
			modernTextAt(dst, label, r.Min.X+8, r.Min.Y+6, 12, modernText)
			return
		}
	}
}

// clickModern routes chrome interactions before painting can begin.
func (g *Game) clickModern(p image.Point, right bool) bool {
	for key, r := range g.modernControls() {
		if !p.In(r) {
			continue
		}
		switch key {
		case "new", "load", "save", "undo", "redo", "fill-settings":
			g.action(key)
		case "mode":
			g.chooseDialog("Paint mode", modeNames, func(i int) { g.Mode = i; g.notice(modeNames[i] + " mode") })
		case "brush-smaller":
			g.Brush = nil
			g.BrushSize = max(1, g.BrushSize-1)
		case "brush-larger":
			g.Brush = nil
			g.BrushSize = min(16, g.BrushSize+1)
		case "brush-size":
			if g.Brush != nil {
				g.action("brush-size")
			} else {
				g.notice("Select a brush preset in the inspector")
			}
		case "grid":
			if right {
				g.action("grid-settings")
			} else {
				g.UseGrid = !g.UseGrid
			}
		case "symmetry":
			if right {
				g.action("symmetry-settings")
			} else {
				g.MirrorX = !g.MirrorX
			}
		case "cycle":
			if right {
				g.action("range")
			} else {
				g.action("cycle")
			}
		case "zoom-in":
			g.zoomAt(g.Zoom*2, g.viewport().Min.Add(g.viewport().Size().Div(2)))
		case "zoom-out":
			g.zoomAt(max(1, g.Zoom/2), g.viewport().Min.Add(g.viewport().Size().Div(2)))
		case "zoom-reset":
			g.zoomAt(2, g.viewport().Min.Add(g.viewport().Size().Div(2)))
		}
		return true
	}
	if !g.ShowTools {
		return false
	}
	for i, t := range modernTools {
		if !p.In(g.modernToolRect(i)) {
			continue
		}
		if t == Grid {
			if right {
				g.action("grid-settings")
			} else {
				g.UseGrid = !g.UseGrid
			}
			return true
		}
		if t == Symmetry {
			if right {
				g.action("symmetry-settings")
			} else {
				g.MirrorX = !g.MirrorX
			}
			return true
		}
		if right {
			switch t {
			case Rectangle:
				t = FilledRectangle
			case Circle:
				t = FilledCircle
			case Ellipse:
				t = FilledEllipse
			case Polygon:
				t = FilledPolygon
			case Fill:
				g.action("fill-settings")
				return true
			case Text:
				g.action("font-size")
				return true
			}
		}
		g.selectTool(t)
		return true
	}
	for i := 0; i < 10; i++ {
		if p.In(g.modernPresetRect(i)) {
			g.cancelGesture()
			g.Brush = nil
			g.BrushShape, g.BrushSize = presetAt(i)
			g.notice(fmt.Sprintf("Brush diameter %d px", g.BrushSize*2-1))
			return true
		}
	}
	for i := 0; i < 32; i++ {
		if p.In(g.modernSwatchRect(i)) {
			idx := g.PalettePage*32 + i
			if idx < len(g.Canvas.Image.Palette) {
				if right {
					g.BG = uint8(idx)
				} else {
					g.FG = uint8(idx)
				}
			}
			return true
		}
	}
	for _, next := range []bool{false, true} {
		if p.In(g.modernPalettePageRect(next)) {
			step := -1
			if next {
				step = 1
			}
			g.PalettePage = max(0, min((len(g.Canvas.Image.Palette)-1)/32, g.PalettePage+step))
			return true
		}
	}
	w, _ := g.screenSize()
	x := w - modernInspectorWidth
	if p.In(image.Rect(x+18, 604, w-18, 635)) {
		g.action("palette")
		return true
	}
	if p.In(image.Rect(x+18, 646, w-18, 677)) {
		g.FG, g.BG = g.BG, g.FG
		return true
	}
	return !p.In(g.viewport())
}
