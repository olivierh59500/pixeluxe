package ui

import (
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"pixeluxe/internal/formats"
	"pixeluxe/internal/paint"
	"pixeluxe/internal/pixfont"
)

type MenuItem struct {
	Label, Key, Action string
	Children           []MenuItem
}
type Menu struct {
	Title string
	Items []MenuItem
}

func item(label, key, action string) MenuItem {
	return MenuItem{Label: label, Key: key, Action: action}
}
func branch(label string, children ...MenuItem) MenuItem {
	return MenuItem{Label: label, Children: children}
}

var menus = []Menu{
	{"Picture", []MenuItem{
		item("Load...", "Ctrl+O", "load"), item("Save...", "Ctrl+S", "save"), item("Save As...", "", "saveas"), item("Delete...", "", "delete"), item("New...", "Ctrl+N", "new"), item("Print to PDF...", "", "print"),
		branch("Color Control", item("Palette...", "p", "palette"), item("Use Brush Palette", "", "brush-palette"), item("Restore Palette", "", "restore-palette"), item("Default Palette", "", "default-palette"), item("Color Ranges...", "", "range"), item("Cycle", "Tab", "cycle"), item("Bg -> Fg", "", "bg-fg"), item("Bg <-> Fg", "", "swap-colors"), item("Remap", "", "remap")),
		branch("Spare", item("Swap", "j", "spare-swap"), item("Copy to Spare", "", "spare-copy"), item("Merge in front", "", "spare-front"), item("Merge in back", "", "spare-back"), item("Delete this Page", "", "clear")),
		item("Page Size...", "", "page-size"), item("Show Page", "S", "show-page"), item("Screen Format...", "", "screen-format"), item("Examples...", "", "examples"), item("About Pixeluxe...", "", "about"), item("Quit", "Ctrl+Q", "quit"),
	}},
	{"Brush", []MenuItem{
		item("Load...", "", "brush-load"), item("Save...", "", "brush-save"), item("Delete...", "", "brush-delete"), item("Original brushes...", "", "example-brushes"), item("Restore", "B", "brush-restore"),
		branch("Size", item("Stretch...", "Z", "brush-size"), item("Halve", "h", "brush-half"), item("Double", "H", "brush-double"), item("Double Horiz", "", "brush-doublex"), item("Double Vert", "", "brush-doubley")),
		branch("Flip", item("Horiz", "x", "brush-flipx"), item("Vert", "y", "brush-flipy")),
		branch("Rotate", item("90 Degrees", "z", "brush-rotate"), item("Any Angle...", "", "brush-angle"), item("Shear...", "", "brush-shear")),
		branch("Change Color", item("Bg -> Fg", "", "brush-bg-fg"), item("Bg <-> Fg", "", "brush-swap-colors"), item("Remap", "", "brush-remap")),
		branch("Bend", item("Horiz...", "", "brush-bendx"), item("Vert...", "", "brush-bendy")),
		branch("Handle", item("Center", "", "handle-center"), item("Corner", "", "handle-corner")),
	}},
	{"Mode", []MenuItem{item("Matte", "F1", "mode-0"), item("Color", "F2", "mode-1"), item("Replc", "F3", "mode-2"), item("Smear", "F4", "mode-3"), item("Shade", "F5", "mode-4"), item("Blend", "F6", "mode-5"), item("Cycle", "F7", "mode-6"), item("Smooth", "F8", "mode-7")}},
	{"Effects", []MenuItem{
		branch("Stencil", item("Make...", "", "stencil-settings"), item("Remake", "", "stencil-remake"), item("Lock FG", "", "stencil-lockfg"), item("Make from foreground", "", "stencil-make"), item("Enable / Disable", "", "stencil-toggle"), item("Reverse", "", "stencil-reverse"), item("Free", "", "stencil-free"), item("Load...", "", "stencil-load"), item("Save...", "", "stencil-save"), item("Delete...", "", "stencil-delete")),
		branch("Background", item("Fix", "", "background-fix"), item("Off", "", "background-off")),
		item("Perspective...", "", "perspective"), item("Fill Settings...", "F", "fill-settings"),
	}},
	{"Font", []MenuItem{item("Topaz 8", "", "font-"), item("Size...", "", "font-size"), branch("Style", item("Plain", "", "font-plain"), item("Bold", "", "font-bold"), item("Italic", "", "font-italic"), item("Underline", "", "font-underline"))}},
	{"Prefs", []MenuItem{item("Coords", "", "coords"), item("Fast FB", "", "fast-feedback"), item("ExclBrush", "", "exclude-brush"), item("MultiCycle", "", "multicycle"), item("Grid...", "G", "grid-settings"), item("Show Grid", "", "show-grid"), item("Symmetry...", "/", "symmetry-settings"), item("Full Screen", "F11", "fullscreen"), item("Hide / Show Toolbox", "F10", "show-tools"), item("Keyboard Help...", "", "help")}},
}

func InstallFontMenu() {
	base := []MenuItem{item("Topaz 8", "", "font-"), item("Size...", "", "font-size"), branch("Style", item("Plain", "", "font-plain"), item("Bold", "", "font-bold"), item("Italic", "", "font-italic"), item("Underline", "", "font-underline"))}
	for _, name := range pixfont.AmigaNames() {
		base = append(base, item(name, "", "font-"+name))
	}
	menus[4].Items = base
}
func menuX(i int) int {
	x := 0
	for j := 0; j < i; j++ {
		x += menuWidth(j)
	}
	return x
}
func menuWidth(i int) int { return len(menus[i].Title)*12 + 8 }

const menuPanelWidth = 220
const menuRowHeight = 22

func (g *Game) menuRect() image.Rectangle {
	x := min(menuX(g.menu), Width-menuPanelWidth)
	return image.Rect(x, topHeight, x+menuPanelWidth, topHeight+len(menus[g.menu].Items)*menuRowHeight+4)
}
func (g *Game) subRect() image.Rectangle {
	r := g.menuRect()
	items := menus[g.menu].Items[g.submenu].Children
	x := r.Max.X
	if x+menuPanelWidth > Width {
		x = r.Min.X - menuPanelWidth
	}
	y := min(topHeight+g.submenu*menuRowHeight, Height-bottomHeight-len(items)*menuRowHeight-4)
	return image.Rect(x, y, x+menuPanelWidth, y+len(items)*menuRowHeight+4)
}

func (g *Game) updateMenu() bool {
	p := g.pointer
	left := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	right := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if p.Y < topHeight && (left || right || g.menu >= 0) {
		for i := range menus {
			if p.X >= menuX(i) && p.X < menuX(i)+menuWidth(i) {
				if g.menu < 0 {
					g.cancelGesture()
				}
				g.menu = i
				g.submenu = -1
				g.menuHover = -1
				if left {
					g.menuSticky = true
				}
				if right {
					g.menuSticky = false
				}
				return true
			}
		}
	}
	if g.menu < 0 {
		return false
	}
	if pressed(ebiten.KeyEscape) {
		g.menu = -1
		g.submenu = -1
		return true
	}
	var chosen *MenuItem
	if g.submenu >= 0 && p.In(g.subRect()) {
		r := g.subRect()
		row := (p.Y - r.Min.Y - 2) / menuRowHeight
		items := menus[g.menu].Items[g.submenu].Children
		if row >= 0 && row < len(items) {
			chosen = &items[row]
		}
	}
	if chosen == nil && p.In(g.menuRect()) {
		r := g.menuRect()
		row := (p.Y - r.Min.Y - 2) / menuRowHeight
		items := menus[g.menu].Items
		if row >= 0 && row < len(items) {
			g.menuHover = row
			if len(items[row].Children) > 0 {
				g.submenu = row
			} else {
				g.submenu = -1
				chosen = &items[row]
			}
		}
	}
	release := inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonRight) && !g.menuSticky
	if (left || release) && chosen != nil && chosen.Action != "" {
		action := chosen.Action
		g.menu = -1
		g.submenu = -1
		g.action(action)
		return true
	}
	if (left && !p.In(g.menuRect()) && (g.submenu < 0 || !p.In(g.subRect()))) || release {
		g.menu = -1
		g.submenu = -1
	}
	return true
}
func (g *Game) drawMenu(dst *image.RGBA) {
	r := g.menuRect()
	g.drawMenuPanel(dst, r, menus[g.menu].Items, true)
	if g.submenu >= 0 {
		g.drawMenuPanel(dst, g.subRect(), menus[g.menu].Items[g.submenu].Children, false)
	}
}
func (g *Game) checked(action string) bool {
	switch action {
	case "cycle":
		return g.Cycle
	case "multicycle":
		return g.MultiCycle
	case "fast-feedback":
		return g.FastFeedback
	case "exclude-brush":
		return g.ExcludeBrush
	case "background-fix":
		return g.FixedBackground
	case "coords":
		return g.ShowCoords
	case "show-grid":
		return g.ShowGrid
	case "stencil-toggle":
		return g.Canvas.StencilEnabled
	case "font-bold":
		return g.FontStyle&1 != 0
	case "font-italic":
		return g.FontStyle&2 != 0
	case "font-underline":
		return g.FontStyle&4 != 0
	case "handle-corner":
		return g.Handles == 1
	case "handle-center":
		return g.Handles == 0
	}
	if strings.HasPrefix(action, "mode-") {
		return action == fmt.Sprintf("mode-%d", g.Mode)
	}
	if strings.HasPrefix(action, "font-") {
		return action == "font-"+g.FontName
	}
	return false
}
func (g *Game) drawMenuPanel(dst *image.RGBA, r image.Rectangle, items []MenuItem, root bool) {
	fill(dst, r.Add(image.Pt(3, 3)), shadow)
	fill(dst, r, paper)
	outline(dst, r, ink)
	for i, it := range items {
		rr := image.Rect(r.Min.X+1, r.Min.Y+2+i*menuRowHeight, r.Max.X-1, r.Min.Y+2+(i+1)*menuRowHeight)
		selected := g.pointer.In(rr) || (root && i == g.submenu)
		c := ink
		if selected {
			fill(dst, rr, blue)
			c = paper
		}
		s := it.Label
		if g.checked(it.Action) {
			text(dst, "*", rr.Min.X+4, rr.Min.Y+7, c)
		}
		text(dst, s, rr.Min.X+14, rr.Min.Y+7, c)
		if len(it.Children) > 0 {
			text(dst, ">", rr.Max.X-12, rr.Min.Y+7, c)
		} else if it.Key != "" {
			text(dst, it.Key, rr.Max.X-7-len(it.Key)*6, rr.Min.Y+7, c)
		}
	}
}

func (g *Game) confirmUnsaved(next func()) {
	g.cancelGesture()
	if g.Canvas.Dirty() || g.RangeChanged {
		g.confirm("Unsaved picture", "Discard changes to the current picture?", next)
	} else {
		next()
	}
}
func (g *Game) requestQuit() {
	if g.dialog != nil {
		return
	}
	g.confirmUnsaved(func() { g.Quit = true })
}
func (g *Game) edit(f func()) {
	g.cancelGesture()
	g.erase = false
	g.Canvas.Erase = false
	g.Canvas.Begin()
	f()
	g.Canvas.Commit()
}
func (g *Game) needBrush() bool {
	if g.Brush != nil {
		return true
	}
	g.message("Custom brush", "Pick up a brush with B, then drag a rectangle,\nor use Brush > Load.")
	return false
}
func (g *Game) action(a string) {
	if a != "help" && a != "about" && a != "quit" {
		g.LastCommand = a
	}
	if strings.HasPrefix(a, "mode-") {
		n, _ := strconv.Atoi(strings.TrimPrefix(a, "mode-"))
		g.Mode = n
		g.notice(modeNames[n] + " mode")
		return
	}
	if strings.HasPrefix(a, "font-") && a != "font-size" {
		switch a {
		case "font-plain":
			g.FontStyle = 0
		case "font-bold":
			g.FontStyle ^= 1
		case "font-italic":
			g.FontStyle ^= 2
		case "font-underline":
			g.FontStyle ^= 4
		default:
			g.FontName = strings.TrimPrefix(a, "font-")
		}
		return
	}
	if strings.HasPrefix(a, "brush-") && a != "brush-load" && a != "brush-restore" && a != "brush-delete" && !g.needBrush() {
		return
	}
	switch a {
	case "load":
		g.confirmUnsaved(func() { g.fileDialog(false, false) })
	case "save":
		g.cancelGesture()
		if g.Filename == "" {
			g.fileDialog(true, false)
		} else if err := g.Save(g.Filename); err != nil {
			g.message("Save failed", err.Error())
		}
	case "saveas":
		g.cancelGesture()
		g.fileDialog(true, false)
	case "delete":
		g.deleteFileDialog(false)
	case "brush-delete":
		g.deleteFileDialog(true)
	case "stencil-load":
		g.stencilFileDialog(false)
	case "stencil-save":
		g.stencilFileDialog(true)
	case "stencil-delete":
		g.deleteStencilDialog()
	case "brush-load":
		g.fileDialog(false, true)
	case "brush-save":
		g.fileDialog(true, true)
	case "brush-restore":
		if g.LastBrush != nil {
			g.Brush = g.LastBrush
			g.selectTool(Freehand)
		} else {
			g.message("Brush", "No previous custom brush.")
		}
	case "undo":
		g.cancelGesture()
		g.Canvas.Undo()
		g.clampPan()
	case "redo":
		g.cancelGesture()
		g.Canvas.Redo()
		g.clampPan()
	case "clear":
		g.edit(func() {
			g.Canvas.Erase = g.FixedBackground
			g.Canvas.RestoreImage = g.Background
			g.Canvas.Clear(g.BG)
			g.Canvas.Erase = false
		})
		g.notice("Page cleared  |  U: undo")
	case "new", "page-size", "screen-format", "grid-settings", "symmetry-settings", "range", "font-size":
		g.cancelGesture()
		g.settingsDialog(a)
	case "brush-size", "brush-angle", "brush-shear", "brush-bendx", "brush-bendy", "perspective":
		if g.needBrush() {
			g.settingsDialog(a)
		}
	case "fill-settings":
		g.chooseDialog("Fill settings", []string{"Solid", "Brush pattern", "Wrapped brush", "Gradient", "Dithered gradient"}, func(i int) { g.FillStyle = i })
	case "palette":
		g.cancelGesture()
		g.paletteDialog()
	case "default-palette":
		g.edit(func() {
			p := paint.DefaultPalette()
			n := len(g.Canvas.Image.Palette)
			g.Canvas.Image.Palette = append(color.Palette{}, p[:min(n, len(p))]...)
			for len(g.Canvas.Image.Palette) < n {
				g.Canvas.Image.Palette = append(g.Canvas.Image.Palette, ink)
			}
		})
	case "restore-palette":
		if len(g.OriginalPalette) > 0 {
			g.edit(func() {
				old := append(color.Palette{}, g.Canvas.Image.Palette...)
				g.Canvas.Image.Palette = append(color.Palette{}, g.OriginalPalette...)
				remap(g.Canvas.Image, old, g.Canvas.Image.Palette)
				g.normalize()
			})
		} else {
			g.action("default-palette")
		}
	case "brush-palette":
		g.edit(func() { g.Canvas.Image.Palette = append(color.Palette{}, g.Brush.Image.Palette...); g.safeIndices() })
	case "cycle":
		g.Cycle = !g.Cycle
		g.cycleOffset = 0
	case "bg-fg":
		g.edit(func() { replaceIndex(g.Canvas.Image, g.BG, g.FG, false) })
	case "swap-colors":
		g.edit(func() { replaceIndex(g.Canvas.Image, g.BG, g.FG, true) })
	case "remap":
		old := paint.CloneImage(g.Canvas.Image)
		g.edit(func() { remap(g.Canvas.Image, old.Palette, g.Canvas.Image.Palette) })
	case "spare-copy":
		g.Spare = paint.CloneImage(g.Canvas.Image)
		g.notice("Picture copied to spare")
	case "spare-swap":
		g.cancelGesture()
		if g.Spare == nil {
			g.Spare = image.NewPaletted(g.Canvas.Image.Bounds(), append(color.Palette{}, g.Canvas.Image.Palette...))
		}
		old := paint.CloneImage(g.Canvas.Image)
		g.Canvas.Begin()
		g.Canvas.Image = paint.CloneImage(g.Spare)
		g.Spare = old
		g.Canvas.Commit()
		g.clampPan()
		g.notice("Spare page swapped")
	case "spare-front", "spare-back":
		if g.Spare == nil {
			g.message("Spare", "Copy a picture to the spare page first.")
			return
		}
		g.edit(func() {
			r := g.Canvas.Image.Bounds().Intersect(g.Spare.Bounds())
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					fg, bg := g.Canvas.Image.ColorIndexAt(x, y), g.Spare.ColorIndexAt(x, y)
					if a == "spare-back" && fg == g.BG && bg != g.BG || a == "spare-front" && bg != g.BG {
						g.Canvas.Pixel(x, y, nearest(g.Canvas.Image.Palette, g.Spare.At(x, y)))
					}
				}
			}
		})
	case "brush-flipx":
		g.Brush.FlipX()
	case "brush-flipy":
		g.Brush.FlipY()
	case "brush-rotate":
		g.Brush.Rotate90()
	case "brush-half":
		b := g.Brush.Image.Bounds()
		g.Brush.Resize(max(1, b.Dx()/2), max(1, b.Dy()/2))
	case "brush-double", "brush-doublex", "brush-doubley":
		b := g.Brush.Image.Bounds()
		w, h := b.Dx(), b.Dy()
		if a != "brush-doubley" {
			w *= 2
		}
		if a != "brush-doublex" {
			h *= 2
		}
		if w <= 4096 && h <= 4096 && w*h <= 4<<20 {
			g.Brush.Resize(w, h)
		} else {
			g.message("Brush", "Maximum brush size is 4096 x 4096 (4M pixels).")
		}
	case "brush-bg-fg":
		replaceIndex(g.Brush.Image, g.BG, g.FG, false)
	case "brush-swap-colors":
		replaceIndex(g.Brush.Image, g.BG, g.FG, true)
	case "brush-remap":
		remap(g.Brush.Image, g.Brush.Image.Palette, g.Canvas.Image.Palette)
		g.Brush.Image.Palette = append(color.Palette{}, g.Canvas.Image.Palette...)
	case "handle-center":
		g.Handles = 0
	case "handle-corner":
		g.Handles = 1
	case "show-page":
		g.ShowTools = !g.ShowTools
		g.Zoom = 1
		g.PanX = 0
		g.PanY = 0
	case "show-tools":
		g.ShowTools = !g.ShowTools
	case "coords":
		g.ShowCoords = !g.ShowCoords
	case "show-grid":
		g.ShowGrid = !g.ShowGrid
	case "fullscreen":
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	case "stencil-make":
		g.Canvas.Stencil[g.FG] = true
		g.makeStencilFromColors()
		g.Canvas.StencilEnabled = true
		g.notice(fmt.Sprintf("Color %d protected by stencil", g.FG))
	case "stencil-toggle":
		g.Canvas.StencilEnabled = !g.Canvas.StencilEnabled
	case "stencil-reverse":
		if g.Canvas.StencilMask == nil {
			g.makeStencilFromColors()
		}
		for i := range g.Canvas.StencilMask {
			g.Canvas.StencilMask[i] = !g.Canvas.StencilMask[i]
		}
		for i := range g.Canvas.Image.Palette {
			g.Canvas.Stencil[i] = !g.Canvas.Stencil[i]
		}
		g.Canvas.StencilEnabled = true
	case "stencil-free":
		g.Canvas.Stencil = [256]bool{}
		g.Canvas.StencilMask = nil
		g.Canvas.StencilEnabled = false
	case "stencil-settings":
		g.settingsDialog("stencil")
	case "stencil-remake":
		g.makeStencilFromColors()
	case "stencil-lockfg":
		g.lockForeground()
	case "background-fix":
		g.cancelGesture()
		g.Background = paint.CloneImage(g.Canvas.Image)
		g.FixedBackground = true
		g.notice("Background fixed  |  Erase and CLR restore it")
	case "background-off":
		g.Background = nil
		g.FixedBackground = false
		g.Canvas.RestoreImage = nil
		g.Canvas.Erase = false
	case "multicycle":
		g.MultiCycle = !g.MultiCycle
	case "fast-feedback":
		g.FastFeedback = !g.FastFeedback
	case "exclude-brush":
		g.ExcludeBrush = !g.ExcludeBrush
	case "example-brushes":
		g.brushExamples()
	case "examples":
		g.examples()
	case "print":
		g.confirm("Print picture", "Write an A4 PDF as pixeluxe-print.pdf?", func() {
			if err := g.PrintPDF("pixeluxe-print.pdf"); err != nil {
				g.message("Print failed", err.Error())
			} else {
				g.notice("Created pixeluxe-print.pdf")
			}
		})
	case "about":
		g.message("Pixeluxe", "PIXELUXE\nA Deluxe Paint II Amiga remake in pure Go.\n\nIndexed colors, original IFF pictures and brushes,\nPAL-style toolbox and original bitmap fonts.\nReference disk: Deluxe Paint II 2.0P, 1987.\n\nF10: toolbox   Right button: background color\nPrefs > Keyboard Help: all shortcuts")
	case "help":
		g.message("Keyboard help", "s / d  Dots / continuous     v / q  Line / curve\nf  Fill    r/R  Rectangle    c/C  Circle\ne/E  Ellipse    b/B  Pick/restore brush    t  Text\nu / Shift-U  Undo/redo       K  Clear page\np  Palette    j  Spare page  /  Mirror symmetry\nx/y/z  Flip X/Y/rotate90     h/H  Half/double brush\nF1..F8  Brush modes          Tab  Color cycling\n- / =  Brush size           < / >  Zoom\n, or Alt-click  Pick color  .  One pixel brush\ng  Grid    G  Grid settings  F10  Hide toolbox\nArrows / Space-drag / middle-drag  Pan\nEsc / Space  Cancel    Enter  Finish polygon/text\nCtrl/Cmd N/O/S  New/Open/Save    F11 Full screen")
	case "quit":
		g.requestQuit()
	}
}
func replaceIndex(im *image.Paletted, from, to uint8, swap bool) {
	for y := im.Rect.Min.Y; y < im.Rect.Max.Y; y++ {
		for x := im.Rect.Min.X; x < im.Rect.Max.X; x++ {
			v := im.ColorIndexAt(x, y)
			if v == from {
				im.SetColorIndex(x, y, to)
			} else if swap && v == to {
				im.SetColorIndex(x, y, from)
			}
		}
	}
}
func remap(im *image.Paletted, old, new color.Palette) {
	var lut [256]uint8
	for i, c := range old {
		lut[i] = nearest(new, c)
	}
	for y := im.Rect.Min.Y; y < im.Rect.Max.Y; y++ {
		for x := im.Rect.Min.X; x < im.Rect.Max.X; x++ {
			im.SetColorIndex(x, y, lut[im.ColorIndexAt(x, y)])
		}
	}
}
func (g *Game) safeIndices() {
	n := len(g.Canvas.Image.Palette)
	for i, v := range g.Canvas.Image.Pix {
		if int(v) >= n {
			g.Canvas.Image.Pix[i] = 0
		}
	}
}
func (g *Game) examples() {
	if g.DemoFS == nil {
		g.message("Examples", "No bundled examples are available.")
		return
	}
	names := []string{}
	paths := []string{}
	fs.WalkDir(g.DemoFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".iff") {
			names = append(names, strings.TrimSuffix(filepath.Base(p), ".iff"))
			paths = append(paths, p)
		}
		return nil
	})
	g.chooseDialog("Original Amiga pictures", names, func(i int) {
		g.confirmUnsaved(func() {
			if err := g.LoadExample(paths[i]); err != nil {
				g.message("Load failed", err.Error())
			}
		})
	})
}
func (g *Game) LoadExample(path string) error {
	f, err := g.DemoFS.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	im, meta, err := formats.DecodeILBM(f)
	if err != nil {
		return err
	}
	g.installWithMetadata(im, "", meta)
	g.notice(filepath.Base(path) + "  -  original Amiga IFF")
	if len(meta.ColorRanges) > 0 {
		for _, r := range meta.ColorRanges {
			if r.Active() {
				g.CycleLow = int(r.Low)
				g.CycleHigh = int(r.High)
				g.CycleSpeed = max(1, int(mathRound(60/r.StepsPerSecond())))
				break
			}
		}
	}
	return nil
}
func mathRound(v float64) float64 { return float64(int(v + 0.5)) }
