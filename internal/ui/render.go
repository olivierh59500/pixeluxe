package ui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"path/filepath"

	"pixeluxe/internal/pixfont"
)

var (
	ink    = color.RGBA{0, 0, 0, 255}
	paper  = color.RGBA{255, 255, 255, 255}
	face   = color.RGBA{187, 187, 187, 255}
	light  = color.RGBA{238, 238, 238, 255}
	shadow = color.RGBA{85, 85, 85, 255}
	blue   = color.RGBA{0, 85, 170, 255}
)

func fill(dst *image.RGBA, r image.Rectangle, c color.Color) {
	draw.Draw(dst, r, &image.Uniform{C: c}, image.Point{}, draw.Src)
}
func outline(dst *image.RGBA, r image.Rectangle, c color.Color) {
	fill(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), c)
	fill(dst, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), c)
	fill(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), c)
	fill(dst, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), c)
}
func bevel(dst *image.RGBA, r image.Rectangle, pressed bool) {
	fill(dst, r, face)
	a, b := light, shadow
	if pressed {
		a, b = b, a
	}
	fill(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+2), a)
	fill(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+2, r.Max.Y), a)
	fill(dst, image.Rect(r.Min.X, r.Max.Y-2, r.Max.X, r.Max.Y), b)
	fill(dst, image.Rect(r.Max.X-2, r.Min.Y, r.Max.X, r.Max.Y), b)
}
func text(dst *image.RGBA, s string, x, y int, c color.Color)  { pixfont.Draw(dst, s, x, y, 1, c) }
func text2(dst *image.RGBA, s string, x, y int, c color.Color) { pixfont.Draw(dst, s, x, y, 2, c) }
func drawLine(dst *image.RGBA, x0, y0, x1, y1 int, c color.Color) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := sign(x1-x0), sign(y1-y0)
	e := dx + dy
	for {
		dst.Set(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}
func disk(dst *image.RGBA, cx, cy, rx, ry int, c color.Color, solid bool) {
	rx = max(1, rx)
	ry = max(1, ry)
	for y := -ry; y <= ry; y++ {
		for x := -rx; x <= rx; x++ {
			d := float64(x*x)/float64(rx*rx) + float64(y*y)/float64(ry*ry)
			if d <= 1 && (solid || d > 1-3/float64(max(rx, ry))) {
				dst.Set(cx+x, cy+y, c)
			}
		}
	}
}

func (g *Game) Render() *image.RGBA {
	w, h := g.screenSize()
	r := image.Rect(0, 0, w, h)
	if g.frame == nil || g.frame.Bounds() != r {
		g.frame = image.NewRGBA(r)
	}
	if g.Modern {
		return g.renderModern()
	}
	return g.renderClassic()
}

func (g *Game) renderClassic() *image.RGBA {
	dst := g.frame
	fill(dst, dst.Bounds(), face)
	g.normalize()
	r := g.viewport()
	im := g.Canvas.Image
	if g.preview != nil {
		im = g.preview
	}
	o := g.canvasOrigin()
	palette := g.displayPalette(im.Palette)
	colors := make([]color.RGBA, len(palette))
	for i := range palette {
		cc := color.NRGBAModel.Convert(palette[i]).(color.NRGBA)
		colors[i] = color.RGBA{cc.R, cc.G, cc.B, 255}
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		cy := int(math.Floor(float64(y-o.Y) / float64(g.Zoom)))
		row := y * dst.Stride
		for x := r.Min.X; x < r.Max.X; x++ {
			cx := int(math.Floor(float64(x-o.X) / float64(g.Zoom)))
			cc := face
			if image.Pt(cx, cy).In(im.Bounds()) {
				idx := im.ColorIndexAt(cx, cy)
				if int(idx) < len(colors) {
					cc = colors[idx]
				}
				if g.ShowGrid && g.Zoom >= 4 && (cx%g.GridSize == 0 && (x-o.X)%g.Zoom == 0 || cy%g.GridSize == 0 && (y-o.Y)%g.Zoom == 0) {
					cc = color.RGBA{cc.R ^ 85, cc.G ^ 85, cc.B ^ 85, 255}
				}
			} else {
				if (x/8+y/8)%2 == 0 {
					cc = shadow
				}
			}
			i := row + x*4
			dst.Pix[i] = cc.R
			dst.Pix[i+1] = cc.G
			dst.Pix[i+2] = cc.B
			dst.Pix[i+3] = 255
		}
	}
	if g.Tool == BrushSelect && g.dragging {
		a, b := g.start, g.canvasPoint(g.pointer)
		rr := image.Rect(o.X+min(a.X, b.X)*g.Zoom, o.Y+min(a.Y, b.Y)*g.Zoom, o.X+(max(a.X, b.X)+1)*g.Zoom, o.Y+(max(a.Y, b.Y)+1)*g.Zoom)
		g.marquee(rr.Intersect(r))
	}
	if g.MirrorX {
		for y := r.Min.Y + 2; y < r.Max.Y; y += 6 {
			dst.Set(o.X+im.Bounds().Dx()*g.Zoom/2, y, shadow)
		}
	}
	if g.MirrorY {
		for x := r.Min.X + 2; x < r.Max.X; x += 6 {
			dst.Set(x, o.Y+im.Bounds().Dy()*g.Zoom/2, shadow)
		}
	}
	if g.ShowTools {
		g.drawSidebar(dst)
	}
	if g.ShowBar || g.menu >= 0 {
		fill(dst, image.Rect(0, 0, Width, topHeight), paper)
		for i, m := range menus {
			x := menuX(i)
			if i == g.menu {
				fill(dst, image.Rect(x, 0, x+menuWidth(i), topHeight), blue)
				text2(dst, m.Title, x+4, 3, paper)
			} else {
				text2(dst, m.Title, x+4, 3, ink)
			}
		}
		mode := modeNames[g.Mode]
		if g.Canvas.StencilEnabled {
			mode += " S"
		}
		if g.Cycle {
			mode += " C"
		}
		if g.FixedBackground {
			mode += " B"
		}
		text(dst, mode, Width-sidebarWidth-6*len(mode), 6, ink)
		drawLine(dst, 0, topHeight-1, Width-1, topHeight-1, ink)
	}
	fill(dst, image.Rect(0, Height-bottomHeight, Width, Height), paper)
	drawLine(dst, 0, Height-bottomHeight, Width-1, Height-bottomHeight, ink)
	status := g.Status
	if g.statusTicks == 0 {
		status = fmt.Sprintf("%s | %s | %dx%d | %d colors", toolNames[g.Tool], modeNames[g.Mode], im.Bounds().Dx(), im.Bounds().Dy(), len(im.Palette))
		if g.Filename != "" {
			status = filepath.Base(g.Filename) + " | " + status
		}
		if g.Canvas.Dirty() || g.RangeChanged {
			status = "* " + status
		}
	}
	if len(status) > 75 {
		status = status[:75]
	}
	text(dst, status, 4, Height-11, ink)
	if g.ShowCoords {
		p := g.canvasPoint(g.pointer)
		s := fmt.Sprintf("%3d,%3d %dx", p.X, p.Y, g.Zoom)
		text(dst, s, Width-6*len(s)-5, Height-11, ink)
	}
	if g.menu >= 0 {
		g.drawMenu(dst)
	}
	if g.dialog != nil {
		g.drawDialog(dst)
	} else if g.menu < 0 && g.pointer.In(r) {
		g.drawCursor(dst)
	}
	return dst
}

func (g *Game) marquee(r image.Rectangle) {
	for x := r.Min.X; x < r.Max.X; x++ {
		c := ink
		if (x/3+g.Frames/10)%2 == 0 {
			c = paper
		}
		g.frame.Set(x, r.Min.Y, c)
		g.frame.Set(x, r.Max.Y-1, c)
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		c := ink
		if (y/3+g.Frames/10)%2 == 0 {
			c = paper
		}
		g.frame.Set(r.Min.X, y, c)
		g.frame.Set(r.Max.X-1, y, c)
	}
}
func (g *Game) drawCursor(dst *image.RGBA) {
	x, y := g.pointer.X, g.pointer.Y
	for d := -6; d <= 6; d++ {
		if abs(d) < 2 {
			continue
		}
		for _, p := range []image.Point{image.Pt(x+d, y), image.Pt(x, y+d)} {
			if p.In(g.viewport()) {
				i := dst.PixOffset(p.X, p.Y)
				dst.Pix[i] ^= 255
				dst.Pix[i+1] ^= 255
				dst.Pix[i+2] ^= 255
			}
		}
	}
	if g.Brush != nil && (g.Tool == Dots || g.Tool == Freehand) {
		b := g.Brush.Image.Bounds()
		w, h := b.Dx()*g.Zoom, b.Dy()*g.Zoom
		rr := image.Rect(x-w/2, y-h/2, x-w/2+w, y-h/2+h).Intersect(g.viewport())
		g.marquee(rr)
	}
	if g.typing {
		p := g.textStart
		o := g.canvasOrigin()
		x = o.X + (p.X+pixfont.WidthAmiga(g.FontName, g.textBuffer, g.FontScale))*g.Zoom
		y = o.Y + p.Y*g.Zoom
		drawLine(dst, x, y, x, y+pixfont.HeightAmiga(g.FontName, g.FontScale)*g.Zoom, paper)
	}
}

const (
	brushBottom = 74
	toolsBottom = 266
	undoBottom  = 290
)

var toolSlots = []Tool{Dots, Freehand, Line, Curve, Fill, Airbrush, Rectangle, Circle, Ellipse, Polygon, BrushSelect, Text, Grid, Symmetry, Magnify, ZoomTool}

func (g *Game) drawSidebar(dst *image.RGBA) {
	palette := g.displayPalette(g.Canvas.Image.Palette)
	x := Width - sidebarWidth
	fill(dst, image.Rect(x, topHeight, Width, Height-bottomHeight), paper)
	drawLine(dst, x, topHeight, x, Height-bottomHeight, ink)
	for row := 0; row < 2; row++ {
		for i, size := range []int{1, 2, 3, 5} {
			cx, cy := x+i*16+8, topHeight+9+row*18
			selected := g.Brush == nil && g.BrushSize == size && g.BrushShape == row
			if selected {
				fill(dst, image.Rect(cx-7, cy-8, cx+8, cy+9), ink)
			}
			c := ink
			if selected {
				c = paper
			}
			if row == 0 {
				disk(dst, cx, cy, size, size, c, true)
			} else {
				fill(dst, image.Rect(cx-size, cy-size, cx+size+1, cy+size+1), c)
			}
		}
	}
	for i, size := range []int{3, 5} {
		cx, cy := x+16+i*32, brushBottom-9
		selected := g.Brush == nil && g.BrushSize == size && g.BrushShape == 2
		c := ink
		if selected {
			fill(dst, image.Rect(cx-15, cy-8, cx+16, cy+9), ink)
			c = paper
		}
		r := size - 1
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if (dx+dy)&1 == 0 {
					dst.Set(cx+dx, cy+dy, c)
				}
			}
		}
	}
	for i, t := range toolSlots {
		row, col := i/2, i%2
		r := image.Rect(x+col*32, brushBottom+row*24, x+(col+1)*32, brushBottom+(row+1)*24)
		selected := g.Tool == t || (t == Rectangle && g.Tool == FilledRectangle) || (t == Circle && g.Tool == FilledCircle) || (t == Ellipse && g.Tool == FilledEllipse) || (t == Polygon && g.Tool == FilledPolygon) || (t == Grid && g.UseGrid) || (t == Symmetry && (g.MirrorX || g.MirrorY || g.Radial > 1))
		fill(dst, r, paper)
		outline(dst, r, ink)
		c := ink
		if selected {
			fill(dst, r.Inset(1), blue)
			c = paper
		}
		g.drawIcon(dst, t, r.Min.X+4, r.Min.Y+4, c)
	}
	for i, s := range []string{"UNDO", "CLR"} {
		rr := image.Rect(x+i*32, toolsBottom, x+(i+1)*32, undoBottom)
		bevel(dst, rr, false)
		text(dst, s, rr.Min.X+(32-len(s)*6)/2, toolsBottom+9, ink)
	}
	fill(dst, image.Rect(x+1, undoBottom, Width, 350), palette[g.BG])
	disk(dst, x+32, 314, 18, 18, palette[g.FG], true)
	disk(dst, x+32, 314, 19, 19, ink, false)
	for i := 0; i < 32; i++ {
		r := image.Rect(x+i%4*16, 350+i/4*16, x+i%4*16+16, 366+i/4*16)
		if i < len(g.Canvas.Image.Palette) {
			fill(dst, r, palette[i])
			if uint8(i) == g.FG {
				outline(dst, r, ink)
				outline(dst, r.Inset(1), paper)
				outline(dst, r.Inset(2), ink)
			}
			if uint8(i) == g.BG {
				fill(dst, image.Rect(r.Max.X-4, r.Max.Y-4, r.Max.X-1, r.Max.Y-1), paper)
			}
		} else {
			fill(dst, r, face)
			drawLine(dst, r.Min.X, r.Min.Y, r.Max.X-1, r.Max.Y-1, shadow)
		}
	}
	bevel(dst, image.Rect(x, 479, Width, 496), false)
	text(dst, "PALETTE", x+10, 484, ink)
}
func (g *Game) drawIcon(dst *image.RGBA, t Tool, x, y int, c color.Color) {
	switch t {
	case Dots:
		for i, p := range []image.Point{{2, 2}, {8, 4}, {6, 10}, {16, 13}, {18, 5}} {
			_ = i
			fill(dst, image.Rect(x+p.X, y+p.Y, x+p.X+2, y+p.Y+2), c)
		}
	case Freehand:
		pts := []image.Point{{1, 12}, {3, 3}, {7, 1}, {9, 9}, {13, 14}, {17, 9}, {21, 4}}
		for i := 1; i < len(pts); i++ {
			a, b := pts[i-1], pts[i]
			drawLine(dst, x+a.X, y+a.Y, x+b.X, y+b.Y, c)
			drawLine(dst, x+a.X+1, y+a.Y, x+b.X+1, y+b.Y, c)
		}
	case Line:
		drawLine(dst, x+1, y+14, x+22, y+1, c)
		drawLine(dst, x+1, y+15, x+22, y+2, c)
	case Curve:
		for i := 0; i < 22; i++ {
			yy := int(12 * math.Pow(float64(i-10)/12, 2))
			dst.Set(x+i, y+yy, c)
			dst.Set(x+i, y+yy+1, c)
		}
	case Fill:
		drawLine(dst, x+3, y+5, x+10, y, c)
		drawLine(dst, x+10, y, x+17, y+7, c)
		drawLine(dst, x+17, y+7, x+10, y+14, c)
		drawLine(dst, x+10, y+14, x+3, y+5, c)
		drawLine(dst, x+3, y+5, x+17, y+7, c)
		disk(dst, x+20, y+13, 2, 3, c, true)
	case Airbrush:
		fill(dst, image.Rect(x+1, y+7, x+7, y+10), c)
		drawLine(dst, x+7, y+7, x+10, y+4, c)
		drawLine(dst, x+7, y+10, x+10, y+13, c)
		for i := 0; i < 18; i++ {
			xx, yy := 11+(i*7)%12, (i*11)%16
			dst.Set(x+xx, y+yy, c)
		}
	case Rectangle:
		outline(dst, image.Rect(x+2, y+1, x+22, y+16), c)
		for yy := 0; yy < 12; yy++ {
			drawLine(dst, x+4, y+3+yy, x+4+yy, y+3+yy, c)
		}
	case Circle, Ellipse:
		rx, ry := 9, 7
		if t == Circle {
			rx = 7
		}
		disk(dst, x+12, y+8, rx, ry, c, false)
		for yy := 0; yy < 7; yy++ {
			drawLine(dst, x+6, y+8+yy/2, x+12+yy, y+8+yy/2, c)
		}
	case Polygon:
		pts := []image.Point{{1, 14}, {7, 1}, {11, 9}, {20, 3}, {22, 15}, {1, 14}}
		for i := 1; i < len(pts); i++ {
			drawLine(dst, x+pts[i-1].X, y+pts[i-1].Y, x+pts[i].X, y+pts[i].Y, c)
		}
	case BrushSelect:
		for i := 0; i < 6; i += 2 {
			drawLine(dst, x+1+i, y+1, x+2+i, y+1, c)
			drawLine(dst, x+16+i, y+15, x+17+i, y+15, c)
		}
		drawLine(dst, x+1, y+1, x+1, y+5, c)
		drawLine(dst, x+22, y+10, x+22, y+15, c)
		drawLine(dst, x+22, y+1, x+18, y+1, c)
		drawLine(dst, x+1, y+15, x+5, y+15, c)
	case Text:
		text2(dst, "A", x+7, y+1, c)
	case Grid:
		for i := 0; i < 3; i++ {
			outline(dst, image.Rect(x+2+i*7, y+1, x+9+i*7, y+16), c)
			drawLine(dst, x+2, y+1+i*7, x+23, y+1+i*7, c)
		}
	case Symmetry:
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			drawLine(dst, x+12, y+8, x+12+int(math.Cos(a)*9), y+8+int(math.Sin(a)*7), c)
		}
		disk(dst, x+12, y+8, 3, 3, c, false)
	case Magnify:
		disk(dst, x+14, y+6, 6, 6, c, false)
		drawLine(dst, x+9, y+10, x+2, y+16, c)
		drawLine(dst, x+10, y+11, x+3, y+16, c)
	case ZoomTool:
		outline(dst, image.Rect(x+2, y, x+23, y+17), c)
		outline(dst, image.Rect(x+6, y+3, x+20, y+14), c)
		outline(dst, image.Rect(x+10, y+6, x+16, y+11), c)
	}
}
