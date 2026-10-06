// Package ui implements Pixeluxe's Amiga-style desktop, using a software
// framebuffer so all controls and picture pixels have integer edges.
package ui

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"math"
	"math/rand/v2"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"pixeluxe/internal/formats"
	"pixeluxe/internal/paint"
	"pixeluxe/internal/pixfont"
)

const Width, Height = 640, 512
const topHeight, sidebarWidth, bottomHeight = 20, 64, 16

type Tool int

const (
	Dots Tool = iota
	Freehand
	Line
	Curve
	Fill
	Airbrush
	Rectangle
	FilledRectangle
	Ellipse
	FilledEllipse
	Polygon
	FilledPolygon
	BrushSelect
	Text
	Grid
	Symmetry
	Magnify
	Picker
	Circle
	FilledCircle
	ZoomTool
)

var toolNames = []string{"Dots", "Freehand", "Line", "Curve", "Fill", "Airbrush", "Rectangle", "Filled rectangle", "Ellipse", "Filled ellipse", "Polygon", "Filled polygon", "Pick up brush", "Text", "Grid", "Symmetry", "Magnify", "Pick color", "Circle", "Filled circle", "Zoom"}
var modeNames = []string{"Matte", "Color", "Replc", "Smear", "Shade", "Blend", "Cycle", "Smooth"}

type Game struct {
	Canvas                                                     *paint.Canvas
	Spare                                                      *image.Paletted
	FG, BG                                                     uint8
	Tool                                                       Tool
	Mode                                                       int
	Brush                                                      *paint.Brush
	LastBrush                                                  *paint.Brush
	BrushSize, BrushShape                                      int
	Zoom, PanX, PanY                                           int
	ShowTools, ShowCoords, ShowGrid, UseGrid, MirrorX, MirrorY bool
	GridSize, Radial, FontScale                                int
	Cycle                                                      bool
	CycleLow, CycleHigh, CycleSpeed, cycleTick, cycleOffset    int
	Filename, Status                                           string
	FontName                                                   string
	FontStyle                                                  int
	FillStyle                                                  int
	Handles                                                    int
	OriginalPalette                                            color.Palette
	FileMetadata                                               *formats.Metadata
	RangeChanged                                               bool
	MultiCycle                                                 bool
	Background                                                 *image.Paletted
	FixedBackground                                            bool
	FastFeedback                                               bool
	ExcludeBrush                                               bool
	ShowBar                                                    bool
	LastCommand                                                string
	finishing                                                  bool
	effectSource                                               *image.Paletted
	statusTicks                                                int
	BrushFS                                                    fs.FS
	DemoFS                                                     fs.FS
	frame                                                      *image.RGBA
	display                                                    *ebiten.Image
	menu                                                       int
	submenu                                                    int
	menuHover                                                  int
	menuSticky                                                 bool
	dialog                                                     *Dialog
	dragging, panning, erase, shift                            bool
	start, last, pointer, panStart, panOrigin                  image.Point
	base                                                       *image.Paletted
	preview                                                    *image.Paletted
	poly                                                       []image.Point
	curveEnd                                                   *image.Point
	textStart                                                  image.Point
	textBuffer                                                 string
	typing                                                     bool
	Quit                                                       bool
	Frames                                                     int
	SmokeFrames                                                int
	OnSmoke                                                    func(*image.RGBA) error
}

func New() *Game {
	g := &Game{Canvas: paint.New(320, 256, paint.DefaultPalette()), FG: 1, Tool: Freehand, BrushSize: 1, Zoom: 2, ShowTools: true, ShowCoords: true, ShowBar: true, FastFeedback: true, MultiCycle: true, GridSize: 8, Radial: 1, FontScale: 1, CycleLow: 16, CycleHigh: 31, CycleSpeed: 8, menu: -1, submenu: -1, menuHover: -1, frame: image.NewRGBA(image.Rect(0, 0, Width, Height))}
	g.Canvas.MarkSaved()
	g.notice("Pixeluxe  -  Deluxe Paint II  |  Prefs > Keyboard Help")
	return g
}

func (g *Game) install(im *image.Paletted, path string) {
	im = normalizePicture(im)
	g.Canvas = paint.New(im.Bounds().Dx(), im.Bounds().Dy(), im.Palette)
	g.Canvas.Image = paint.CloneImage(im)
	g.Canvas.MarkSaved()
	g.OriginalPalette = append(color.Palette{}, im.Palette...)
	g.Filename = path
	g.PanX = 0
	g.PanY = 0
	g.FG = uint8(min(1, len(im.Palette)-1))
	g.BG = 0
	g.Cycle = false
	g.CycleLow = min(16, len(im.Palette)-1)
	g.CycleHigh = len(im.Palette) - 1
	g.poly = nil
	g.typing = false
	g.preview = nil
	g.Spare = nil
	g.Background = nil
	g.FixedBackground = false
	g.FileMetadata = nil
	g.RangeChanged = false
	g.notice(fmt.Sprintf("%s  -  %dx%d  -  %d colors", filepath.Base(path), im.Bounds().Dx(), im.Bounds().Dy(), len(im.Palette)))
}

func (g *Game) notice(s string) { g.Status = s; g.statusTicks = 240 }
func (g *Game) viewport() image.Rectangle {
	w := Width
	if g.ShowTools {
		w -= sidebarWidth
	}
	y := 0
	if g.ShowBar || g.menu >= 0 {
		y = topHeight
	}
	return image.Rect(0, y, w, Height-bottomHeight)
}
func (g *Game) canvasOrigin() image.Point {
	r := g.viewport()
	b := g.Canvas.Image.Bounds()
	return image.Pt(r.Min.X+max(0, (r.Dx()-b.Dx()*g.Zoom)/2)-g.PanX*g.Zoom, r.Min.Y+max(0, (r.Dy()-b.Dy()*g.Zoom)/2)-g.PanY*g.Zoom)
}
func (g *Game) canvasPoint(p image.Point) image.Point {
	o := g.canvasOrigin()
	return image.Pt(int(math.Floor(float64(p.X-o.X)/float64(g.Zoom))), int(math.Floor(float64(p.Y-o.Y)/float64(g.Zoom))))
}
func (g *Game) snap(p image.Point) image.Point {
	if g.UseGrid {
		p.X = int(math.Round(float64(p.X)/float64(g.GridSize))) * g.GridSize
		p.Y = int(math.Round(float64(p.Y)/float64(g.GridSize))) * g.GridSize
	}
	return p
}
func (g *Game) clampPan() {
	r := g.viewport()
	b := g.Canvas.Image.Bounds()
	g.PanX = max(0, min(g.PanX, b.Dx()-r.Dx()/g.Zoom))
	g.PanY = max(0, min(g.PanY, b.Dy()-r.Dy()/g.Zoom))
}
func (g *Game) zoomAt(z int, p image.Point) {
	old := g.canvasPoint(p)
	g.Zoom = max(1, min(32, z))
	r := g.viewport()
	g.PanX = old.X - (p.X-r.Min.X)/g.Zoom
	g.PanY = old.Y - (p.Y-r.Min.Y)/g.Zoom
	g.clampPan()
}

func (g *Game) Layout(_, _ int) (int, int) { return Width, Height }
func (g *Game) Draw(screen *ebiten.Image) {
	g.Render()
	if g.display == nil {
		g.display = ebiten.NewImage(Width, Height)
	}
	g.display.WritePixels(g.frame.Pix)
	screen.DrawImage(g.display, nil)
}

func (g *Game) Update() error {
	g.Frames++
	if g.statusTicks > 0 {
		g.statusTicks--
	}
	g.cycleTick++
	if g.Cycle && g.CycleHigh > g.CycleLow && g.cycleTick%max(1, g.CycleSpeed) == 0 {
		g.cycleOffset = (g.cycleOffset + 1) % (g.CycleHigh - g.CycleLow + 1)
	}
	x, y := ebiten.CursorPosition()
	g.pointer = image.Pt(x, y)
	g.shift = ebiten.IsKeyPressed(ebiten.KeyShift)
	if g.Quit {
		return ebiten.Termination
	}
	if ebiten.IsWindowBeingClosed() {
		g.requestQuit()
	}
	if g.SmokeFrames > 0 && g.Frames >= g.SmokeFrames {
		if g.OnSmoke != nil {
			if err := g.OnSmoke(g.Render()); err != nil {
				return err
			}
		}
		return ebiten.Termination
	}
	if dropped := ebiten.DroppedFiles(); dropped != nil {
		fs.WalkDir(dropped, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			f, e := dropped.Open(p)
			if e != nil {
				return e
			}
			defer f.Close()
			r := bufio.NewReader(f)
			header, _ := r.Peek(4)
			var im *image.Paletted
			var meta *formats.Metadata
			if string(header) == "FORM" {
				im, meta, e = formats.DecodeILBM(r)
			} else {
				im, e = formats.Decode(r)
			}
			if e != nil {
				g.notice(e.Error())
				return nil
			}
			g.confirmUnsaved(func() {
				g.installWithMetadata(im, "", meta)
				g.notice("Opened " + filepath.Base(p) + "  |  Save As to choose a path")
			})
			return fs.SkipAll
		})
	}
	if g.dialog != nil {
		g.updateDialog()
		return nil
	}
	if g.updateMenu() {
		return nil
	}
	if g.keys() {
		return nil
	}
	left := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	right := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	middle := ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle)
	space := ebiten.IsKeyPressed(ebiten.KeySpace)
	if (middle || (space && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft))) && g.pointer.In(g.viewport()) {
		if !g.panning {
			g.panning = true
			g.panStart = g.pointer
			g.panOrigin = image.Pt(g.PanX, g.PanY)
		}
		g.PanX = g.panOrigin.X - (x-g.panStart.X)/g.Zoom
		g.PanY = g.panOrigin.Y - (y-g.panStart.Y)/g.Zoom
		g.clampPan()
		return nil
	}
	g.panning = false
	_, wy := ebiten.Wheel()
	if wy != 0 && g.pointer.In(g.viewport()) {
		z := g.Zoom
		if wy > 0 {
			z *= 2
		} else {
			z /= 2
		}
		g.zoomAt(z, g.pointer)
	}
	if (left || right) && g.ShowTools && g.pointer.In(image.Rect(Width-sidebarWidth, topHeight, Width, Height-bottomHeight)) {
		g.clickSidebar(g.pointer, right)
		return nil
	}
	p := g.snap(g.canvasPoint(g.pointer))
	if (left || right) && g.pointer.In(g.viewport()) && p.In(g.Canvas.Image.Bounds()) {
		g.erase = right
		if ebiten.IsKeyPressed(ebiten.KeyAlt) || g.Tool == Picker {
			g.pick(p, right)
			return nil
		}
		g.startTool(p)
	}
	if g.dragging {
		down := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
		if g.erase {
			down = ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
		}
		if !down {
			g.finishTool(p)
		} else {
			g.moveTool(p)
		}
	}
	if len(g.poly) > 0 || g.curveEnd != nil {
		g.shapePreview(p)
	}
	return nil
}

func (g *Game) currentColor() uint8 {
	if g.erase {
		return uint8(min(int(g.BG), len(g.Canvas.Image.Palette)-1))
	}
	return uint8(min(int(g.FG), len(g.Canvas.Image.Palette)-1))
}
func (g *Game) pick(p image.Point, bg bool) {
	if bg {
		g.BG = g.Canvas.Image.ColorIndexAt(p.X, p.Y)
	} else {
		g.FG = g.Canvas.Image.ColorIndexAt(p.X, p.Y)
	}
	g.notice(fmt.Sprintf("Color %d", g.FG))
}
func (g *Game) selectTool(t Tool) { g.cancelGesture(); g.Tool = t; g.notice(toolNames[t]) }
func (g *Game) cancelGesture() {
	if g.base != nil || g.dragging || len(g.poly) > 0 || g.typing || g.curveEnd != nil {
		g.Canvas.Cancel()
	}
	g.dragging = false
	g.preview = nil
	g.base = nil
	g.poly = nil
	g.typing = false
	g.textBuffer = ""
	g.curveEnd = nil
}

func (g *Game) startTool(p image.Point) {
	if !p.In(g.Canvas.Image.Bounds()) {
		return
	}
	g.normalize()
	if g.typing {
		g.commitText()
	}
	if g.Tool == Magnify || g.Tool == ZoomTool {
		z := g.Zoom * 2
		if g.erase {
			z = max(1, g.Zoom/2)
		}
		g.zoomAt(z, g.pointer)
		return
	}
	if g.Tool == Polygon || g.Tool == FilledPolygon {
		if g.erase {
			g.commitPolygon()
			return
		}
		if len(g.poly) == 0 {
			g.Canvas.Begin()
			g.base = paint.CloneImage(g.Canvas.Image)
		}
		if len(g.poly) > 2 && abs(p.X-g.poly[0].X)+abs(p.Y-g.poly[0].Y) <= max(2, 6/g.Zoom) {
			g.commitPolygon()
			return
		}
		g.poly = append(g.poly, p)
		return
	}
	if g.Tool == Curve && g.curveEnd != nil {
		g.drawCurve(g.Canvas, g.start, *g.curveEnd, p)
		g.Canvas.Commit()
		g.base = nil
		g.preview = nil
		g.curveEnd = nil
		return
	}
	g.Canvas.Begin()
	g.prepareCanvas(g.Canvas)
	g.base = paint.CloneImage(g.Canvas.Image)
	g.start = p
	g.last = p
	switch g.Tool {
	case Fill:
		g.symmetry(p, func(q image.Point) { g.fillArea(q) })
		g.Canvas.Commit()
		g.base = nil
	case Text:
		g.typing = true
		g.textStart = p
		g.textBuffer = ""
		g.notice("Type text  |  Enter: place  |  Esc: cancel")
	case Dots, Freehand:
		g.dragging = true
		g.stamp(g.Canvas, p)
	case Airbrush:
		g.dragging = true
		g.spray(p)
	default:
		g.dragging = true
		g.shapePreview(p)
	}
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func (g *Game) constrained(p image.Point) image.Point {
	if !g.shift {
		return p
	}
	dx, dy := p.X-g.start.X, p.Y-g.start.Y
	if g.Tool == Line || g.Tool == Curve {
		if abs(dx) > abs(dy)*2 {
			p.Y = g.start.Y
		} else if abs(dy) > abs(dx)*2 {
			p.X = g.start.X
		} else {
			n := max(abs(dx), abs(dy))
			p.X = g.start.X + sign(dx)*n
			p.Y = g.start.Y + sign(dy)*n
		}
	} else {
		n := max(abs(dx), abs(dy))
		p.X = g.start.X + sign(dx)*n
		p.Y = g.start.Y + sign(dy)*n
	}
	return p
}
func sign(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}
func (g *Game) moveTool(p image.Point) {
	p = g.constrained(p)
	switch g.Tool {
	case Freehand:
		g.stroke(g.Canvas, g.last, p)
	case Dots:
		if p != g.last {
			g.stamp(g.Canvas, p)
		}
	case Airbrush:
		g.spray(p)
	default:
		g.shapePreview(p)
	}
	g.last = p
}
func (g *Game) finishTool(p image.Point) {
	g.finishing = true
	defer func() { g.finishing = false }()
	p = g.constrained(p)
	switch g.Tool {
	case Freehand:
		if p != g.last {
			g.stroke(g.Canvas, g.last, p)
		}
	case Dots:
		if p != g.last {
			g.stamp(g.Canvas, p)
		}
	case Airbrush:
	default:
		g.shapePreview(p)
	}
	g.dragging = false
	if g.Tool == BrushSelect {
		r := image.Rect(min(g.start.X, p.X), min(g.start.Y, p.Y), max(g.start.X, p.X)+1, max(g.start.Y, p.Y)+1)
		if g.UseGrid && g.ExcludeBrush {
			r.Max.X = max(r.Min.X+1, r.Max.X-1)
			r.Max.Y = max(r.Min.Y+1, r.Max.Y-1)
		}
		if r.Empty() || r.Dx() > 4096 || r.Dy() > 4096 || r.Dx()*r.Dy() > 4<<20 {
			g.Canvas.Cancel()
			g.preview = nil
			g.base = nil
			g.notice("Brush selection too large")
			return
		}
		g.Brush = paint.Capture(g.Canvas.Image, r, g.BG)
		g.LastBrush = g.Brush
		g.Canvas.Cancel()
		g.Tool = Freehand
		g.notice(fmt.Sprintf("Custom brush %dx%d", g.Brush.Image.Bounds().Dx(), g.Brush.Image.Bounds().Dy()))
	} else if g.Tool == Curve {
		q := p
		g.curveEnd = &q
		g.notice("Curve: click to set the bend")
		return
	} else {
		if g.preview != nil {
			g.Canvas.Image = paint.CloneImage(g.preview)
		}
		g.Canvas.Commit()
	}
	g.preview = nil
	g.base = nil
}

func (g *Game) shapePreview(p image.Point) {
	if g.base == nil {
		return
	}
	if g.FastFeedback && !g.finishing && (g.Tool == Line || g.Tool == Curve || g.Tool == Rectangle || g.Tool == Circle || g.Tool == Ellipse || g.Tool == Polygon) {
		oldBrush, oldSize, oldMode := g.Brush, g.BrushSize, g.Mode
		g.Brush = nil
		g.BrushSize = 1
		g.Mode = 1
		defer func() { g.Brush, g.BrushSize, g.Mode = oldBrush, oldSize, oldMode }()
	}
	p = g.constrained(p)
	c := paint.New(g.base.Bounds().Dx(), g.base.Bounds().Dy(), g.base.Palette)
	c.Image = paint.CloneImage(g.base)
	c.Stencil = g.Canvas.Stencil
	c.StencilEnabled = g.Canvas.StencilEnabled
	g.prepareCanvas(c)
	if g.curveEnd != nil {
		g.drawCurve(c, g.start, *g.curveEnd, p)
	} else {
		switch g.Tool {
		case Line, Curve:
			g.stroke(c, g.start, p)
		case Rectangle, FilledRectangle, Ellipse, FilledEllipse, Circle, FilledCircle, Polygon, FilledPolygon:
			pts := append(append([]image.Point{}, g.poly...), p)
			g.paintShape(c, g.start, p, g.Tool, pts)
		}
	}
	g.preview = c.Image
}
func (g *Game) commitPolygon() {
	if len(g.poly) > 1 {
		g.paintShape(g.Canvas, image.Point{}, image.Point{}, g.Tool, g.poly)
		g.Canvas.Commit()
	} else {
		g.Canvas.Cancel()
	}
	g.poly = nil
	g.preview = nil
	g.base = nil
}
func (g *Game) drawCurve(c *paint.Canvas, a, b, control image.Point) {
	last := a
	steps := max(16, 2*(abs(a.X-b.X)+abs(a.Y-b.Y)))
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		u := 1 - t
		q := image.Pt(int(math.Round(u*u*float64(a.X)+2*u*t*float64(control.X)+t*t*float64(b.X))), int(math.Round(u*u*float64(a.Y)+2*u*t*float64(control.Y)+t*t*float64(b.Y))))
		g.stroke(c, last, q)
		last = q
	}
}

func (g *Game) symmetry(p image.Point, f func(image.Point)) {
	b := g.Canvas.Image.Bounds()
	cx, cy := float64(b.Dx()-1)/2, float64(b.Dy()-1)/2
	seen := map[image.Point]bool{}
	for i := 0; i < max(1, g.Radial); i++ {
		a := 2 * math.Pi * float64(i) / float64(max(1, g.Radial))
		dx, dy := float64(p.X)-cx, float64(p.Y)-cy
		q := image.Pt(int(math.Round(cx+dx*math.Cos(a)-dy*math.Sin(a))), int(math.Round(cy+dx*math.Sin(a)+dy*math.Cos(a))))
		pts := []image.Point{q}
		if g.MirrorX {
			pts = append(pts, image.Pt(b.Dx()-1-q.X, q.Y))
		}
		if g.MirrorY {
			for _, v := range append([]image.Point{}, pts...) {
				pts = append(pts, image.Pt(v.X, b.Dy()-1-v.Y))
			}
		}
		for _, v := range pts {
			if !seen[v] {
				f(v)
				seen[v] = true
			}
		}
	}
}
func (g *Game) stroke(c *paint.Canvas, a, b image.Point) {
	oldSource := g.effectSource
	if g.Mode == 3 || g.Mode == 5 || g.Mode == 7 {
		g.effectSource = paint.CloneImage(c.Image)
	}
	defer func() { g.effectSource = oldSource }()
	dx, dy := abs(b.X-a.X), -abs(b.Y-a.Y)
	sx, sy := sign(b.X-a.X), sign(b.Y-a.Y)
	e := dx + dy
	for {
		g.stamp(c, a)
		g.last = a
		if a == b {
			break
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			a.X += sx
		}
		if e2 <= dx {
			e += dx
			a.Y += sy
		}
	}
}
func (g *Game) stamp(c *paint.Canvas, p image.Point) { g.paintStamp(c, p) }
func mix(a, b color.Color) color.Color {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	return color.RGBA{uint8((ar + br) >> 9), uint8((ag + bg) >> 9), uint8((ab + bb) >> 9), 255}
}
func nearest(p color.Palette, c color.Color) uint8 { return uint8(p.Index(c)) }
func (g *Game) spray(p image.Point) {
	r := max(4, g.BrushSize*3)
	for i := 0; i < max(3, r/2); i++ {
		dx, dy := rand.IntN(2*r+1)-r, rand.IntN(2*r+1)-r
		if dx*dx+dy*dy <= r*r {
			g.symmetry(image.Pt(p.X+dx, p.Y+dy), func(q image.Point) { g.Canvas.Pixel(q.X, q.Y, g.currentColor()) })
		}
	}
}
func (g *Game) textPreview() {
	if g.base == nil {
		return
	}
	g.preview = paint.CloneImage(g.base)
	c := paint.New(g.preview.Bounds().Dx(), g.preview.Bounds().Dy(), g.preview.Palette)
	c.Image = g.preview
	c.Stencil = g.Canvas.Stencil
	c.StencilEnabled = g.Canvas.StencilEnabled
	g.prepareCanvas(c)
	pixfont.WalkAmiga(g.FontName, g.textBuffer, g.textStart.X, g.textStart.Y, g.FontScale, g.FontStyle, func(x, y int) { c.Pixel(x, y, g.FG) })
}
func (g *Game) commitText() {
	g.textPreview()
	if g.preview != nil {
		g.Canvas.Image = paint.CloneImage(g.preview)
	}
	g.Canvas.Commit()
	g.typing = false
	g.preview = nil
	g.base = nil
	g.textBuffer = ""
}

func pressed(k ebiten.Key) bool { return inpututil.IsKeyJustPressed(k) }
func (g *Game) keys() bool {
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)
	if g.typing {
		for _, r := range ebiten.AppendInputChars(nil) {
			if r >= 32 {
				g.textBuffer += string(r)
			}
		}
		if pressed(ebiten.KeyBackspace) && len(g.textBuffer) > 0 {
			r := []rune(g.textBuffer)
			g.textBuffer = string(r[:len(r)-1])
		}
		if pressed(ebiten.KeyEnter) {
			g.commitText()
		} else if pressed(ebiten.KeyEscape) {
			g.cancelGesture()
		} else {
			g.textPreview()
		}
		return true
	}
	if pressed(ebiten.KeyEscape) || (pressed(ebiten.KeySpace) && (g.dragging || g.typing || len(g.poly) > 0 || g.curveEnd != nil)) {
		g.cancelGesture()
		return true
	}
	if ctrl {
		switch {
		case pressed(ebiten.KeyZ):
			g.cancelGesture()
			if g.shift {
				g.Canvas.Redo()
			} else {
				g.Canvas.Undo()
			}
		case pressed(ebiten.KeyY):
			g.Canvas.Redo()
		case pressed(ebiten.KeyO):
			g.action("load")
		case pressed(ebiten.KeyS):
			if g.shift || g.Filename == "" {
				g.action("saveas")
			} else {
				g.action("save")
			}
		case pressed(ebiten.KeyN):
			g.action("new")
		case pressed(ebiten.KeyQ):
			g.requestQuit()
		}
		return false
	}
	for i, k := range []ebiten.Key{ebiten.KeyF1, ebiten.KeyF2, ebiten.KeyF3, ebiten.KeyF4, ebiten.KeyF5, ebiten.KeyF6, ebiten.KeyF7, ebiten.KeyF8} {
		if pressed(k) {
			g.Mode = i
			g.notice(modeNames[i] + " mode")
		}
	}
	if pressed(ebiten.KeyF10) {
		g.ShowTools = !g.ShowTools
		g.ShowBar = g.ShowTools
	}
	if pressed(ebiten.KeyF9) {
		g.ShowBar = !g.ShowBar
	}
	if pressed(ebiten.KeyA) && g.LastCommand != "" {
		g.action(g.LastCommand)
	}
	if pressed(ebiten.KeyTab) {
		g.Cycle = !g.Cycle
		g.cycleOffset = 0
	}
	if pressed(ebiten.KeyU) {
		g.cancelGesture()
		if g.shift {
			g.Canvas.Redo()
		} else {
			g.Canvas.Undo()
		}
	}
	if pressed(ebiten.KeyP) {
		g.action("palette")
		return true
	}
	if pressed(ebiten.KeyJ) {
		g.action("spare-swap")
	}
	if pressed(ebiten.KeyX) {
		g.action("brush-flipx")
	}
	if pressed(ebiten.KeyY) {
		g.action("brush-flipy")
	}
	if pressed(ebiten.KeyZ) {
		if g.shift {
			g.action("brush-size")
		} else {
			g.action("brush-rotate")
		}
	}
	if pressed(ebiten.KeyH) {
		if g.shift {
			g.action("brush-double")
		} else {
			g.action("brush-half")
		}
	}
	if pressed(ebiten.KeyB) {
		if g.shift {
			g.action("brush-restore")
		} else {
			g.selectTool(BrushSelect)
		}
	}
	if pressed(ebiten.KeyD) {
		g.selectTool(Freehand)
		if g.shift {
			g.Brush = nil
			g.BrushSize = 1
		}
	}
	if pressed(ebiten.KeyV) {
		g.selectTool(Line)
	}
	if pressed(ebiten.KeyQ) {
		g.selectTool(Curve)
	}
	if pressed(ebiten.KeyF) {
		if g.shift {
			g.action("fill-settings")
		} else {
			g.selectTool(Fill)
		}
	}
	if pressed(ebiten.KeyR) {
		if g.shift {
			g.selectTool(FilledRectangle)
		} else {
			g.selectTool(Rectangle)
		}
	}
	if pressed(ebiten.KeyC) {
		if g.shift {
			g.selectTool(FilledCircle)
		} else {
			g.selectTool(Circle)
		}
	}
	if pressed(ebiten.KeyE) {
		if g.shift {
			g.selectTool(FilledEllipse)
		} else {
			g.selectTool(Ellipse)
		}
	}
	if pressed(ebiten.KeyT) {
		g.selectTool(Text)
	}
	if pressed(ebiten.KeyK) && g.shift {
		g.action("clear")
	}
	if pressed(ebiten.KeyG) {
		if g.shift {
			g.action("grid-settings")
		} else {
			g.UseGrid = !g.UseGrid
		}
	}
	if pressed(ebiten.KeyM) {
		g.selectTool(Magnify)
	}
	if pressed(ebiten.KeyS) {
		if g.shift {
			g.action("show-page")
		} else {
			g.selectTool(Dots)
		}
	}
	if pressed(ebiten.KeySlash) {
		g.MirrorX = !g.MirrorX
	}
	if pressed(ebiten.KeyComma) {
		if g.shift {
			g.zoomAt(max(1, g.Zoom/2), g.pointer)
		} else {
			g.selectTool(Picker)
		}
	}
	if pressed(ebiten.KeyPeriod) {
		if g.shift {
			g.zoomAt(g.Zoom*2, g.pointer)
		} else {
			g.Brush = nil
			g.BrushSize = 1
		}
	}
	if pressed(ebiten.KeyN) {
		g.PanX = max(0, g.Canvas.Image.Bounds().Dx()/2-g.viewport().Dx()/g.Zoom/2)
		g.PanY = max(0, g.Canvas.Image.Bounds().Dy()/2-g.viewport().Dy()/g.Zoom/2)
	}
	if pressed(ebiten.KeyF11) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	if pressed(ebiten.KeyNumpadAdd) {
		g.zoomAt(g.Zoom*2, g.pointer)
	}
	if pressed(ebiten.KeyNumpadSubtract) {
		g.zoomAt(max(1, g.Zoom/2), g.pointer)
	}
	if pressed(ebiten.KeyEqual) {
		g.Brush = nil
		g.BrushSize = min(16, g.BrushSize+1)
	}
	if pressed(ebiten.KeyMinus) {
		g.Brush = nil
		g.BrushSize = max(1, g.BrushSize-1)
	}
	if pressed(ebiten.KeyBracketRight) {
		g.FG = uint8((int(g.FG) + 1) % len(g.Canvas.Image.Palette))
	}
	if pressed(ebiten.KeyBracketLeft) {
		g.FG = uint8((int(g.FG) + len(g.Canvas.Image.Palette) - 1) % len(g.Canvas.Image.Palette))
	}
	if pressed(ebiten.KeyEnter) && len(g.poly) > 0 {
		g.commitPolygon()
	}
	step := 8
	if pressed(ebiten.KeyArrowLeft) {
		g.PanX -= step
	}
	if pressed(ebiten.KeyArrowRight) {
		g.PanX += step
	}
	if pressed(ebiten.KeyArrowUp) {
		g.PanY -= step
	}
	if pressed(ebiten.KeyArrowDown) {
		g.PanY += step
	}
	g.clampPan()
	return g.dialog != nil
}

func (g *Game) clickSidebar(p image.Point, right bool) {
	x, y := p.X-(Width-sidebarWidth), p.Y
	if x < 0 || x >= sidebarWidth || y < topHeight {
		return
	}
	if y < brushBottom {
		g.Brush = nil
		g.BrushShape = (y - topHeight) / 18
		if g.BrushShape == 2 {
			g.BrushSize = 3
			if x >= 32 {
				g.BrushSize = 5
			}
		} else {
			g.BrushSize = []int{1, 2, 3, 5}[x/16]
		}
		g.notice(fmt.Sprintf("Brush size %d", g.BrushSize))
		return
	}
	if y < toolsBottom {
		row := (y - brushBottom) / 24
		col := x / 32
		t := toolSlots[row*2+col]
		if t == Grid {
			if right {
				g.action("grid-settings")
			} else {
				g.UseGrid = !g.UseGrid
			}
			return
		}
		if t == Symmetry {
			if right {
				g.action("symmetry-settings")
			} else {
				g.MirrorX = !g.MirrorX
			}
			return
		}
		if right {
			switch t {
			case Rectangle:
				t = FilledRectangle
			case Ellipse:
				t = FilledEllipse
			case Circle:
				t = FilledCircle
			case Polygon:
				t = FilledPolygon
			case Fill:
				g.action("fill-settings")
				return
			case Text:
				g.action("font-size")
				return
			}
		}
		g.selectTool(t)
		return
	}
	if y < undoBottom {
		if x < 32 {
			if right {
				g.action("redo")
			} else {
				g.action("undo")
			}
		} else {
			g.action("clear")
		}
		return
	}
	if y < 350 {
		g.FG, g.BG = g.BG, g.FG
		return
	}
	if y >= 350 && y < 478 {
		idx := (y-350)/16*4 + x/16
		if idx < len(g.Canvas.Image.Palette) {
			if right {
				g.BG = uint8(idx)
			} else {
				g.FG = uint8(idx)
			}
		}
		return
	}
	if y >= 478 {
		g.action("palette")
	}
}

func supported(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".gif", ".jpg", ".jpeg", ".iff", ".ilbm", ".lbm", ".brush", ".pic":
		return true
	}
	return false
}
