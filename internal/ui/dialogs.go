package ui

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"pixeluxe/internal/formats"
	"pixeluxe/internal/paint"
)

// Dialog is a modal requester rendered into the same software framebuffer
// as the painting tools and uses no native widgets.
type Dialog struct {
	kind, title, body, err string
	rect                   image.Rectangle
	fields                 []dialogField
	buttons                []dialogButton
	active, selected       int
	accept, onCancel       func()
	entries                []string
	onChoose               func(int)
	list                   image.Rectangle
	scroll                 int
	files                  []dialogFile
	directory              string
	save, brush            bool
	delete, stencil        bool
	lastClick, lastFrame   int
	palettePage, slider    int
	modern                 bool
	sourceWidth            int
}

type dialogField struct {
	label, value string
	rect         image.Rectangle
	cursor       int
	number       bool
	selected     bool
}

type dialogButton struct {
	label  string
	rect   image.Rectangle
	action func()
}

type dialogFile struct {
	name string
	dir  bool
}

func newRequester(kind, title string, w, h int) *Dialog {
	x, y := (Width-w)/2, (Height-h)/2
	return &Dialog{kind: kind, title: title, rect: image.Rect(x, y, x+w, y+h), active: -1, selected: -1, lastClick: -1, slider: -1, sourceWidth: w}
}

func (g *Game) openRequester(d *Dialog) {
	g.cancelGesture()
	g.menu, g.submenu = -1, -1
	if g.Modern && !d.modern {
		old := d.rect
		if d.sourceWidth == 0 {
			d.sourceWidth = old.Dx()
		}
		w, h := g.screenSize()
		d.modern = true
		x, y := (w-old.Dx()*3/2)/2, (h-old.Dy()*3/2)/2
		d.rect = image.Rect(x, y, x+old.Dx()*3/2, y+old.Dy()*3/2)
		for i := range d.fields {
			d.fields[i].rect = d.localRect(d.fields[i].rect.Sub(old.Min))
		}
		for i := range d.buttons {
			d.buttons[i].rect = d.localRect(d.buttons[i].rect.Sub(old.Min))
		}
		if !d.list.Empty() {
			d.list = d.localRect(d.list.Sub(old.Min))
		}
	}
	g.dialog = d
}

func (d *Dialog) localRect(r image.Rectangle) image.Rectangle {
	if d.modern {
		r = image.Rect(r.Min.X*3/2, r.Min.Y*3/2, r.Max.X*3/2, r.Max.Y*3/2)
	}
	return r.Add(d.rect.Min)
}

func (d *Dialog) localPoint(x, y int) image.Point {
	return d.localRect(image.Rect(x, y, x, y)).Min
}

func (d *Dialog) listRowHeight() int {
	if d.modern {
		return 24
	}
	return 16
}

func (d *Dialog) listScrollbarWidth() int {
	if d.modern {
		return 20
	}
	return 16
}

func (d *Dialog) listIndexAt(p image.Point) int {
	if !p.In(d.list) || p.X >= d.list.Max.X-d.listScrollbarWidth() || p.Y < d.list.Min.Y+2 {
		return -1
	}
	row := (p.Y - d.list.Min.Y - 2) / d.listRowHeight()
	index := d.scroll + row
	if row >= dialogListRows(d) || index >= d.itemCount() {
		return -1
	}
	return index
}

func (d *Dialog) addField(label, value string, y int, number bool) {
	r := image.Rect(d.rect.Min.X+146, d.rect.Min.Y+y, d.rect.Max.X-24, d.rect.Min.Y+y+22)
	d.fields = append(d.fields, dialogField{label: label, value: value, rect: r, cursor: len([]rune(value)), number: number})
}

func (d *Dialog) addButton(label string, x, w int, action func()) {
	r := image.Rect(d.rect.Min.X+x, d.rect.Max.Y-34, d.rect.Min.X+x+w, d.rect.Max.Y-12)
	d.buttons = append(d.buttons, dialogButton{label, r, action})
}

func (g *Game) cancelRequester(d *Dialog) {
	if g.dialog != d {
		return
	}
	g.dialog = nil
	if d.onCancel != nil {
		d.onCancel()
	}
}

func (g *Game) requesterButtons(d *Dialog, acceptLabel string) {
	w := d.rect.Dx()
	d.addButton(acceptLabel, w-232, 100, func() {
		if d.accept != nil {
			d.accept()
		}
	})
	d.addButton("Cancel", w-120, 100, func() { g.cancelRequester(d) })
}

func (g *Game) message(title, body string) {
	lines := wrapDialogText(body, 74)
	h := min(420, max(140, 82+len(lines)*13))
	d := newRequester("message", title, 484, h)
	d.body = body
	d.accept = func() { g.dialog = nil }
	d.addButton("OK", d.rect.Dx()/2-50, 100, d.accept)
	g.openRequester(d)
}

func (g *Game) confirm(title, body string, action func()) {
	lines := wrapDialogText(body, 68)
	d := newRequester("confirm", title, 440, max(156, 94+len(lines)*13))
	d.body = body
	d.accept = func() {
		g.dialog = nil
		if action != nil {
			action()
		}
	}
	g.requesterButtons(d, "OK")
	g.openRequester(d)
}

func (g *Game) fileDialog(save, brush bool) {
	if save && brush && g.Brush == nil {
		g.message("Save brush", "Pick up a custom brush first with the Brush tool (B).")
		return
	}
	title, verb := "Load picture", "Load"
	if save {
		title, verb = "Save picture", "Save"
	}
	if brush {
		title = verb + " brush"
	}
	d := newRequester("file", title, 548, 432)
	d.save, d.brush = save, brush
	directory, err := os.Getwd()
	if err != nil {
		directory = "."
	}
	name := ""
	if g.Filename != "" {
		directory, name = filepath.Dir(g.Filename), filepath.Base(g.Filename)
	}
	if brush {
		name = "brush.iff"
	} else if save && name == "" {
		name = "untitled.iff"
	} else if !save {
		name = ""
	}
	d.fields = []dialogField{
		{label: "Drawer", value: directory, rect: image.Rect(d.rect.Min.X+66, d.rect.Min.Y+47, d.rect.Max.X-62, d.rect.Min.Y+67), cursor: len([]rune(directory))},
		{label: "File", value: name, rect: image.Rect(d.rect.Min.X+66, d.rect.Max.Y-88, d.rect.Max.X-20, d.rect.Max.Y-66), cursor: len([]rune(name))},
	}
	d.list = image.Rect(d.rect.Min.X+16, d.rect.Min.Y+91, d.rect.Max.X-20, d.rect.Max.Y-114)
	d.active = 1
	d.fields[1].selected = true
	d.accept = func() { g.acceptFile(d) }
	g.requesterButtons(d, verb)
	d.buttons = append(d.buttons, dialogButton{"Up", image.Rect(d.rect.Max.X-56, d.rect.Min.Y+47, d.rect.Max.X-20, d.rect.Min.Y+67), func() {
		g.readDirectory(d, filepath.Dir(d.directory))
	}})
	g.openRequester(d)
	g.readDirectory(d, directory)
}

func (g *Game) deleteFileDialog(brush bool) {
	g.fileDialog(false, brush)
	d := g.dialog
	d.delete = true
	d.title = "Delete picture"
	if brush {
		d.title = "Delete brush"
	}
	d.buttons[0].label = "Delete"
}

func (g *Game) stencilFileDialog(save bool) {
	g.fileDialog(save, false)
	d := g.dialog
	d.stencil = true
	d.title = "Load stencil"
	if save {
		d.title = "Save stencil"
	}
	d.fields[1].value = "stencil.iff"
	d.fields[1].cursor = len(d.fields[1].value)
	d.fields[1].selected = true
}

func (g *Game) deleteStencilDialog() {
	g.stencilFileDialog(false)
	g.dialog.delete = true
	g.dialog.title = "Delete stencil"
	g.dialog.buttons[0].label = "Delete"
}

func (g *Game) readDirectory(d *Dialog, path string) bool {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "~"+string(filepath.Separator)) || path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if !filepath.IsAbs(path) && d.directory != "" {
		path = filepath.Join(d.directory, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		d.err = err.Error()
		return false
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		d.err = "Cannot open drawer: " + err.Error()
		return false
	}
	d.directory = abs
	d.fields[0].value, d.fields[0].cursor = abs, len([]rune(abs))
	d.files = nil
	if filepath.Dir(abs) != abs {
		d.files = append(d.files, dialogFile{"..", true})
	}
	for _, entry := range entries {
		if entry.IsDir() || supported(entry.Name()) {
			d.files = append(d.files, dialogFile{entry.Name(), entry.IsDir()})
		}
	}
	sort.SliceStable(d.files, func(i, j int) bool {
		if d.files[i].name == ".." {
			return d.files[j].name != ".."
		}
		if d.files[j].name == ".." {
			return false
		}
		if d.files[i].dir != d.files[j].dir {
			return d.files[i].dir
		}
		return strings.ToLower(d.files[i].name) < strings.ToLower(d.files[j].name)
	})
	d.scroll, d.selected, d.lastClick = 0, -1, -1
	d.err = ""
	return true
}

func (g *Game) activateFile(d *Dialog, index int) {
	if index < 0 || index >= len(d.files) {
		return
	}
	f := d.files[index]
	if f.dir {
		g.readDirectory(d, filepath.Join(d.directory, f.name))
		return
	}
	d.fields[1].value, d.fields[1].cursor = f.name, len([]rune(f.name))
	g.acceptFile(d)
}

func (g *Game) acceptFile(d *Dialog) {
	if filepath.Clean(d.fields[0].value) != d.directory {
		if !g.readDirectory(d, d.fields[0].value) {
			return
		}
	}
	name := strings.TrimSpace(d.fields[1].value)
	if name == "" {
		d.err = "Enter a file name or choose a file from the list."
		d.active = 1
		return
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(d.directory, path)
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		g.readDirectory(d, path)
		d.fields[1].value, d.fields[1].cursor = "", 0
		return
	}
	if d.delete {
		info, err := os.Stat(path)
		if err != nil {
			d.err = "Cannot delete file: " + err.Error()
			return
		}
		if !info.Mode().IsRegular() {
			d.err = "Choose a regular picture, brush or stencil file."
			return
		}
		g.confirm("Delete file?", "Permanently delete "+filepath.Base(path)+"?", func() {
			g.dialog = d
			if err := os.Remove(path); err != nil {
				d.err = "Cannot delete file: " + err.Error()
				return
			}
			g.readDirectory(d, d.directory)
			d.fields[1].value, d.fields[1].cursor = "", 0
			g.notice("Deleted " + filepath.Base(path))
		})
		g.dialog.onCancel = func() { g.dialog = d }
		return
	}
	if d.save {
		if filepath.Ext(path) == "" {
			path += ".iff"
			d.fields[1].value = filepath.Base(path)
			d.fields[1].cursor = len([]rune(d.fields[1].value))
		}
		save := func() {
			g.dialog = d
			var err error
			if d.stencil {
				err = g.SaveStencil(path)
			} else if d.brush {
				err = g.SaveBrush(path)
			} else {
				err = g.Save(path)
			}
			if err != nil {
				d.err = err.Error()
				return
			}
			g.dialog = nil
			if d.stencil {
				g.notice("Saved stencil " + filepath.Base(path))
			} else if d.brush {
				g.notice("Saved brush " + filepath.Base(path))
			}
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			kind := "picture"
			if d.brush {
				kind = "brush"
			} else if d.stencil {
				kind = "stencil"
			}
			g.confirm("Replace file?", filepath.Base(path)+" already exists. Replace it with this "+kind+"?", save)
			g.dialog.onCancel = func() { g.dialog = d }
		} else {
			save()
		}
		return
	}
	im, meta, err := decodePicture(path)
	if err != nil {
		d.err = err.Error()
		return
	}
	if d.stencil {
		if err := g.installStencil(im); err != nil {
			d.err = err.Error()
			return
		}
		g.dialog = nil
		g.notice("Loaded stencil " + filepath.Base(path))
		return
	}
	if d.brush {
		g.Brush = importedBrush(im, g.Canvas.Image.Palette, g.BG)
		g.LastBrush = g.Brush
		g.Tool = Freehand
		g.dialog = nil
		g.notice(fmt.Sprintf("Loaded brush %s  -  %dx%d", filepath.Base(path), im.Rect.Dx(), im.Rect.Dy()))
		return
	}
	g.confirmUnsaved(func() { g.installWithMetadata(im, path, meta); g.dialog = nil })
	if g.dialog != nil && g.dialog != d {
		g.dialog.onCancel = func() { g.dialog = d }
	}
}

func importedBrush(src *image.Paletted, _ color.Palette, background uint8) *paint.Brush {
	transparent := background
	if int(transparent) >= len(src.Palette) {
		transparent = 0
	}
	hasAlpha := false
	for i, c := range src.Palette {
		_, _, _, a := c.RGBA()
		if a == 0 {
			transparent, hasAlpha = uint8(i), true
			break
		}
	}
	b := paint.Capture(src, src.Rect, transparent)
	for y := 0; y < b.Image.Rect.Dy(); y++ {
		for x := 0; x < b.Image.Rect.Dx(); x++ {
			i := y*b.Image.Rect.Dx() + x
			index := b.Image.ColorIndexAt(x, y)
			if hasAlpha {
				_, _, _, a := src.Palette[index].RGBA()
				b.Mask[i] = a != 0
			}
		}
	}
	return b
}

func brushExportImage(b *paint.Brush) *image.Paletted {
	if b == nil || b.Image == nil {
		return nil
	}
	im := paint.CloneImage(b.Image)
	var used [256]bool
	hasTransparent := false
	opaqueAt := func(x, y int) bool {
		i := (y-im.Rect.Min.Y)*im.Rect.Dx() + x - im.Rect.Min.X
		if len(b.Mask) == im.Rect.Dx()*im.Rect.Dy() {
			return b.Mask[i]
		}
		return im.ColorIndexAt(x, y) != b.Transparent
	}
	for y := im.Rect.Min.Y; y < im.Rect.Max.Y; y++ {
		for x := im.Rect.Min.X; x < im.Rect.Max.X; x++ {
			if opaqueAt(x, y) {
				used[im.ColorIndexAt(x, y)] = true
			} else {
				hasTransparent = true
			}
		}
	}
	if !hasTransparent {
		return im
	}
	transparent := -1
	if int(b.Transparent) < len(im.Palette) && !used[b.Transparent] {
		transparent = int(b.Transparent)
	} else {
		for i := range im.Palette {
			if !used[i] {
				transparent = i
				break
			}
		}
	}
	if transparent < 0 {
		if len(im.Palette) == 256 {
			return nil // 256 opaque colours plus transparency cannot fit an indexed image.
		}
		transparent = len(im.Palette)
		im.Palette = append(im.Palette, color.RGBA{})
	} else {
		im.Palette[transparent] = color.RGBA{}
	}
	for y := im.Rect.Min.Y; y < im.Rect.Max.Y; y++ {
		for x := im.Rect.Min.X; x < im.Rect.Max.X; x++ {
			if !opaqueAt(x, y) {
				im.SetColorIndex(x, y, uint8(transparent))
			}
		}
	}
	return im
}

func (g *Game) paletteDialog() {
	g.cancelGesture()
	if int(g.FG) >= len(g.Canvas.Image.Palette) {
		g.FG = 0
	}
	d := newRequester("palette", "Color palette", 458, 336)
	d.palettePage = int(g.FG) / 32
	oldFG := g.FG
	g.Canvas.Begin()
	d.accept = func() { g.Canvas.Commit(); g.dialog = nil; g.notice("Palette updated") }
	d.onCancel = func() { g.Canvas.Cancel(); g.FG = oldFG }
	g.requesterButtons(d, "OK")
	if len(g.Canvas.Image.Palette) > 32 {
		d.buttons = append(d.buttons,
			dialogButton{"<", image.Rect(d.rect.Min.X+18, d.rect.Min.Y+154, d.rect.Min.X+42, d.rect.Min.Y+174), func() { d.palettePage = max(0, d.palettePage-1) }},
			dialogButton{">", image.Rect(d.rect.Max.X-42, d.rect.Min.Y+154, d.rect.Max.X-18, d.rect.Min.Y+174), func() { d.palettePage = min((len(g.Canvas.Image.Palette)-1)/32, d.palettePage+1) }},
		)
	}
	g.openRequester(d)
}

func (g *Game) stencilDialog() {
	d := newRequester("stencil", "Color stencil", 458, 264)
	d.palettePage = min(int(g.FG)/32, (len(g.Canvas.Image.Palette)-1)/32)
	oldStencil, oldEnabled := g.Canvas.Stencil, g.Canvas.StencilEnabled
	oldMask := append([]bool(nil), g.Canvas.StencilMask...)
	d.accept = func() { g.makeStencilFromColors(); g.dialog = nil; g.notice("Stencil updated") }
	d.onCancel = func() {
		g.Canvas.Stencil, g.Canvas.StencilEnabled = oldStencil, oldEnabled
		g.Canvas.StencilMask = oldMask
	}
	g.requesterButtons(d, "OK")
	if len(g.Canvas.Image.Palette) > 32 {
		d.buttons = append(d.buttons,
			dialogButton{"<", image.Rect(d.rect.Min.X+18, d.rect.Min.Y+151, d.rect.Min.X+42, d.rect.Min.Y+171), func() { d.palettePage = max(0, d.palettePage-1) }},
			dialogButton{">", image.Rect(d.rect.Max.X-42, d.rect.Min.Y+151, d.rect.Max.X-18, d.rect.Min.Y+171), func() { d.palettePage = min((len(g.Canvas.Image.Palette)-1)/32, d.palettePage+1) }},
		)
	}
	g.openRequester(d)
}

func (g *Game) settingsDialog(kind string) {
	if kind == "stencil" {
		g.stencilDialog()
		return
	}
	if kind == "fill-settings" {
		g.chooseDialog("Fill type", []string{"Solid foreground", "Brush pattern", "Wrap brush", "Color range gradient", "Dithered color range"}, func(i int) {
			g.FillStyle = i
			g.notice("Fill type updated")
		})
		return
	}
	d := newRequester(kind, "Settings", 432, 246)
	b := g.Canvas.Image.Bounds()
	switch kind {
	case "new", "page-size", "screen-format":
		d.title, d.body = "New picture", "Amiga presets: 320 x 256, 640 x 256, 640 x 512"
		if kind == "page-size" {
			d.title, d.body = "Page size", "Resize the page; keep picture pixels at the upper left."
		} else if kind == "screen-format" {
			d.title, d.body = "Screen format", "Set dimensions and colors; keep the current picture."
		}
		d.addField("Width", strconv.Itoa(b.Dx()), 76, true)
		d.addField("Height", strconv.Itoa(b.Dy()), 110, true)
		d.addField("Colors", strconv.Itoa(len(g.Canvas.Image.Palette)), 144, true)
	case "grid-settings":
		d.title, d.body = "Grid", "Grid spacing in picture pixels; G toggles snapping."
		d.addField("Spacing", strconv.Itoa(g.GridSize), 88, true)
	case "symmetry-settings":
		d.title, d.body = "Symmetry", "Mirror: 0 = off, 1 = on. Radial copies: 1 - 32."
		d.addField("Mirror X", boolNumber(g.MirrorX), 76, true)
		d.addField("Mirror Y", boolNumber(g.MirrorY), 110, true)
		d.addField("Radial", strconv.Itoa(g.Radial), 144, true)
	case "range":
		d.title, d.body = "Color range", "Palette indices and ticks per cycling step (60Hz)."
		d.addField("First color", strconv.Itoa(g.CycleLow), 76, true)
		d.addField("Last color", strconv.Itoa(g.CycleHigh), 110, true)
		d.addField("Speed", strconv.Itoa(g.CycleSpeed), 144, true)
	case "brush-size":
		if g.Brush == nil {
			g.message("Brush size", "Pick up a custom brush first with the Brush tool (B).")
			return
		}
		d.title, d.body = "Brush size", "Resize the custom brush with nearest-neighbor sampling."
		d.addField("Width", strconv.Itoa(g.Brush.Image.Rect.Dx()), 88, true)
		d.addField("Height", strconv.Itoa(g.Brush.Image.Rect.Dy()), 122, true)
	case "font-size":
		d.title, d.body = "Text size", "Bitmap font scale: 1, 2, 3 or 4."
		d.addField("Scale", strconv.Itoa(g.FontScale), 88, true)
	case "brush-angle", "brush-shear", "brush-bendx", "brush-bendy", "perspective":
		if g.Brush == nil {
			g.message("Brush transform", "Pick up a custom brush first with the Brush tool (B).")
			return
		}
		switch kind {
		case "brush-angle":
			d.title, d.body = "Rotate brush", "Clockwise rotation in degrees (-360 to 360)."
			d.addField("Angle", "45", 88, true)
		case "brush-shear":
			d.title, d.body = "Shear brush", "Horizontal shear as a percentage (-200 to 200)."
			d.addField("Shear %", "25", 88, true)
		case "brush-bendx":
			d.title, d.body = "Bend brush horizontally", "Horizontal bend in picture pixels."
			d.addField("Bend pixels", "8", 88, true)
		case "brush-bendy":
			d.title, d.body = "Bend brush vertically", "Vertical bend in picture pixels."
			d.addField("Bend pixels", "8", 88, true)
		case "perspective":
			d.title, d.body = "Brush perspective", "Rotate the brush plane around X, Y and Z (degrees)."
			d.addField("X angle", "0", 76, true)
			d.addField("Y angle", "30", 110, true)
			d.addField("Z angle", "0", 144, true)
		}
	default:
		g.message("Settings", "This setting is unavailable.")
		return
	}
	d.active = 0
	d.fields[0].selected = true
	d.accept = func() { g.acceptSettings(d) }
	g.requesterButtons(d, "OK")
	g.openRequester(d)
}

func boolNumber(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func (g *Game) acceptSettings(d *Dialog) {
	n := make([]int, len(d.fields))
	for i, field := range d.fields {
		value, err := strconv.Atoi(strings.TrimSpace(field.value))
		if err != nil {
			d.err = "Enter a whole number for " + field.label + "."
			d.active = i
			return
		}
		n[i] = value
	}
	valid := func(value, low, high int, label string) bool {
		if value < low || value > high {
			d.err = fmt.Sprintf("%s must be between %d and %d.", label, low, high)
			return false
		}
		return true
	}
	switch d.kind {
	case "new", "page-size", "screen-format":
		if !valid(n[0], 1, formats.MaxDimension, "Width") || !valid(n[1], 1, formats.MaxDimension, "Height") || !valid(n[2], 1, 256, "Colors") {
			return
		}
		if int64(n[0])*int64(n[1]) > formats.MaxPixels {
			d.err = "Picture is too large (maximum 16 million pixels)."
			return
		}
		if d.kind == "new" && (n[2] < 2 || n[2]&(n[2]-1) != 0) {
			d.err = "Use 2, 4, 8, 16, 32, 64, 128 or 256 colors."
			return
		}
		if d.kind != "new" {
			g.resizePicture(n[0], n[1], n[2])
			g.dialog = nil
			g.notice(fmt.Sprintf("%s  -  %dx%d  -  %d colors", d.title, n[0], n[1], n[2]))
			return
		}
		g.confirmUnsaved(func() {
			p := make(color.Palette, n[2])
			defaults := paint.DefaultPalette()
			for i := range p {
				if i < len(g.Canvas.Image.Palette) {
					p[i] = g.Canvas.Image.Palette[i]
				} else if i < len(defaults) {
					p[i] = defaults[i]
				} else {
					v := uint8((i - 32) * 255 / max(1, n[2]-33))
					p[i] = color.RGBA{v, v, v, 255}
				}
			}
			g.Canvas.Begin()
			g.Canvas.Image = image.NewPaletted(image.Rect(0, 0, n[0], n[1]), p)
			g.Canvas.Commit()
			g.Canvas.MarkSaved()
			g.Filename = ""
			g.FileMetadata = nil
			g.RangeChanged, g.MultiCycle = false, false
			g.OriginalPalette = append(color.Palette(nil), p...)
			g.FG, g.BG = uint8(min(int(g.FG), n[2]-1)), uint8(min(int(g.BG), n[2]-1))
			g.Cycle, g.CycleLow, g.CycleHigh = false, min(16, n[2]-1), n[2]-1
			g.PanX, g.PanY = 0, 0
			g.Spare = nil
			g.Background, g.FixedBackground = nil, false
			g.Canvas.RestoreImage, g.Canvas.Erase = nil, false
			g.Canvas.Stencil, g.Canvas.StencilMask, g.Canvas.StencilEnabled = [256]bool{}, nil, false
			g.dialog = nil
			g.notice(fmt.Sprintf("New picture  -  %dx%d  -  %d colors", n[0], n[1], n[2]))
		})
		if g.dialog != nil && g.dialog != d {
			g.dialog.onCancel = func() { g.dialog = d }
		}
		return
	case "grid-settings":
		if !valid(n[0], 1, 1024, "Spacing") {
			return
		}
		g.GridSize = n[0]
	case "symmetry-settings":
		if !valid(n[0], 0, 1, "Mirror X") || !valid(n[1], 0, 1, "Mirror Y") || !valid(n[2], 1, 32, "Radial copies") {
			return
		}
		g.MirrorX, g.MirrorY, g.Radial = n[0] == 1, n[1] == 1, n[2]
	case "range":
		if !valid(n[0], 0, len(g.Canvas.Image.Palette)-1, "First color") || !valid(n[1], n[0], len(g.Canvas.Image.Palette)-1, "Last color") || !valid(n[2], 1, 600, "Speed") {
			return
		}
		g.CycleLow, g.CycleHigh, g.CycleSpeed, g.cycleOffset = n[0], n[1], n[2], 0
		g.RangeChanged, g.MultiCycle = true, false
	case "brush-size":
		if !valid(n[0], 1, formats.MaxDimension, "Width") || !valid(n[1], 1, formats.MaxDimension, "Height") || int64(n[0])*int64(n[1]) > formats.MaxPixels {
			if d.err == "" {
				d.err = "Brush is too large (maximum 16 million pixels)."
			}
			return
		}
		g.Brush.Resize(n[0], n[1])
		g.LastBrush = g.Brush
	case "font-size":
		if !valid(n[0], 1, 4, "Scale") {
			return
		}
		g.FontScale = n[0]
	case "brush-angle":
		if !valid(n[0], -360, 360, "Angle") {
			return
		}
		g.Brush.Rotate(float64(n[0]))
		g.LastBrush = g.Brush
	case "brush-shear":
		if !valid(n[0], -200, 200, "Shear") {
			return
		}
		g.Brush.Shear(float64(n[0]) / 100)
		g.LastBrush = g.Brush
	case "brush-bendx", "brush-bendy":
		if !valid(n[0], -8192, 8192, "Bend") {
			return
		}
		g.Brush.Bend(float64(n[0]), d.kind == "brush-bendx")
		g.LastBrush = g.Brush
	case "perspective":
		for i, axis := range []string{"X angle", "Y angle", "Z angle"} {
			if !valid(n[i], -180, 180, axis) {
				return
			}
		}
		if err := g.Brush.Perspective(float64(n[0]), float64(n[1]), float64(n[2])); err != nil {
			d.err = err.Error()
			return
		}
		g.LastBrush = g.Brush
	}
	g.dialog = nil
	g.notice(d.title + " updated")
}

func (g *Game) chooseDialog(title string, entries []string, onChoose func(int)) {
	d := newRequester("choose", title, 486, min(414, max(180, 112+len(entries)*18)))
	d.entries, d.onChoose = append([]string(nil), entries...), onChoose
	d.list = image.Rect(d.rect.Min.X+18, d.rect.Min.Y+48, d.rect.Max.X-18, d.rect.Max.Y-54)
	if len(entries) > 0 {
		d.selected = 0
	}
	d.accept = func() {
		if d.selected < 0 || d.selected >= len(d.entries) {
			return
		}
		g.dialog = nil
		if d.onChoose != nil {
			d.onChoose(d.selected)
		}
	}
	g.requesterButtons(d, "Choose")
	g.openRequester(d)
}

func dialogListRows(d *Dialog) int { return max(1, (d.list.Dy()-4)/d.listRowHeight()) }

func (d *Dialog) itemCount() int {
	if d.kind == "file" {
		return len(d.files)
	}
	return len(d.entries)
}

func (d *Dialog) keepSelectionVisible() {
	rows := dialogListRows(d)
	if d.selected < d.scroll {
		d.scroll = max(0, d.selected)
	}
	if d.selected >= d.scroll+rows {
		d.scroll = d.selected - rows + 1
	}
	d.scroll = max(0, min(d.scroll, max(0, d.itemCount()-rows)))
}

func repeatedKey(k ebiten.Key) bool {
	n := inpututil.KeyPressDuration(k)
	return n == 1 || (n > 24 && n%3 == 0)
}

func (g *Game) updateDialog() {
	d := g.dialog
	if d == nil {
		return
	}
	if pressed(ebiten.KeyEscape) {
		g.cancelRequester(d)
		return
	}
	if pressed(ebiten.KeyTab) && len(d.fields) > 0 {
		step := 1
		if ebiten.IsKeyPressed(ebiten.KeyShift) {
			step = -1
		}
		if !d.list.Empty() {
			focus := (d.active + 1 + step + len(d.fields) + 1) % (len(d.fields) + 1)
			d.active = focus - 1
		} else {
			d.active = (d.active + step + len(d.fields)) % len(d.fields)
		}
		if d.active >= 0 {
			d.fields[d.active].selected = true
		}
	}
	if d.kind == "palette" || d.kind == "stencil" {
		g.updatePaletteRequester(d)
	}
	if !d.list.Empty() {
		_, wheel := ebiten.Wheel()
		if wheel != 0 && g.pointer.In(d.list) {
			d.scroll = max(0, min(d.scroll-int(wheel)*3, max(0, d.itemCount()-dialogListRows(d))))
		}
		if d.active < 0 {
			step := 0
			if repeatedKey(ebiten.KeyArrowUp) {
				step--
			}
			if repeatedKey(ebiten.KeyArrowDown) {
				step++
			}
			if pressed(ebiten.KeyPageUp) {
				step -= dialogListRows(d)
			}
			if pressed(ebiten.KeyPageDown) {
				step += dialogListRows(d)
			}
			if step != 0 && d.itemCount() > 0 {
				d.selected = max(0, min(d.selected+step, d.itemCount()-1))
				d.keepSelectionVisible()
				if d.kind == "file" && !d.files[d.selected].dir {
					f := &d.fields[1]
					f.value, f.cursor = d.files[d.selected].name, len([]rune(d.files[d.selected].name))
				}
			}
		}
	}
	if d.active >= 0 && d.active < len(d.fields) {
		updateDialogField(&d.fields[d.active])
	}
	if pressed(ebiten.KeyEnter) || pressed(ebiten.KeyNumpadEnter) {
		if d.kind == "file" && d.active == 0 {
			if g.readDirectory(d, d.fields[0].value) {
				d.active = 1
				d.fields[1].selected = true
			}
		} else if d.kind == "file" && d.active < 0 && d.selected >= 0 {
			g.activateFile(d, d.selected)
		} else if d.accept != nil {
			d.accept()
		}
		return
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	for _, button := range d.buttons {
		if g.pointer.In(button.rect) {
			button.action()
			return
		}
	}
	for i := range d.fields {
		f := &d.fields[i]
		if g.pointer.In(f.rect) {
			d.active = i
			f.selected = false
			f.cursor = d.fieldCursorAt(f, g.pointer.X)
			return
		}
	}
	if !d.list.Empty() && g.pointer.In(d.list) {
		d.active = -1
		if g.pointer.X >= d.list.Max.X-d.listScrollbarWidth() {
			if g.pointer.Y < d.list.Min.Y+18 {
				d.scroll = max(0, d.scroll-1)
			} else if g.pointer.Y >= d.list.Max.Y-18 {
				d.scroll = min(max(0, d.itemCount()-dialogListRows(d)), d.scroll+1)
			} else {
				fraction := float64(g.pointer.Y-d.list.Min.Y-18) / float64(max(1, d.list.Dy()-36))
				d.scroll = int(fraction * float64(max(0, d.itemCount()-dialogListRows(d))))
			}
			return
		}
		index := d.listIndexAt(g.pointer)
		if index < 0 || index >= d.itemCount() {
			return
		}
		d.selected = index
		if d.kind == "file" && !d.files[index].dir {
			d.fields[1].value, d.fields[1].cursor = d.files[index].name, len([]rune(d.files[index].name))
		}
		if d.lastClick == index && g.Frames-d.lastFrame < 25 {
			d.lastClick = -1
			if d.kind == "file" {
				g.activateFile(d, index)
			} else if d.accept != nil {
				d.accept()
			}
			return
		}
		d.lastClick, d.lastFrame = index, g.Frames
	}
}

func (d *Dialog) fieldWindow(f *dialogField) (chars []rune, start, end int) {
	chars = []rune(f.value)
	f.cursor = max(0, min(f.cursor, len(chars)))
	if !d.modern {
		visible := max(1, (f.rect.Dx()-10)/6)
		start = max(0, f.cursor-visible+1)
		return chars, start, min(len(chars), start+visible)
	}
	width := max(1, f.rect.Dx()-20)
	start = f.cursor
	for start > 0 && modernTextWidth(string(chars[start-1:f.cursor]), 13) <= width-10 {
		start--
	}
	end = f.cursor
	for end < len(chars) && modernTextWidth(string(chars[start:end+1]), 13) <= width {
		end++
	}
	return chars, start, end
}

func (d *Dialog) fieldCursorAt(f *dialogField, x int) int {
	chars, start, end := d.fieldWindow(f)
	if !d.modern {
		return min(len(chars), start+max(0, (x-f.rect.Min.X-5)/6))
	}
	x -= f.rect.Min.X + 10
	lastWidth := 0
	for i := start; i < end; i++ {
		width := modernTextWidth(string(chars[start:i+1]), 13)
		if x < (lastWidth+width)/2 {
			return i
		}
		lastWidth = width
	}
	return end
}

func updateDialogField(f *dialogField) {
	runes := []rune(f.value)
	f.cursor = max(0, min(f.cursor, len(runes)))
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)
	if ctrl && pressed(ebiten.KeyA) {
		f.selected = true
		return
	}
	for _, r := range ebiten.AppendInputChars(nil) {
		if unicode.IsControl(r) || len(runes) >= 2048 {
			continue
		}
		if f.number && !unicode.IsDigit(r) && !(r == '-' && (f.cursor == 0 || f.selected) && (f.selected || !strings.ContainsRune(string(runes), '-'))) {
			continue
		}
		if f.selected {
			runes, f.cursor, f.selected = nil, 0, false
		}
		runes = append(runes, 0)
		copy(runes[f.cursor+1:], runes[f.cursor:])
		runes[f.cursor] = r
		f.cursor++
	}
	if repeatedKey(ebiten.KeyBackspace) {
		if f.selected {
			runes, f.cursor, f.selected = nil, 0, false
		} else if f.cursor > 0 {
			runes = append(runes[:f.cursor-1], runes[f.cursor:]...)
			f.cursor--
		}
	}
	if repeatedKey(ebiten.KeyDelete) {
		if f.selected {
			runes, f.cursor, f.selected = nil, 0, false
		} else if f.cursor < len(runes) {
			runes = append(runes[:f.cursor], runes[f.cursor+1:]...)
		}
	}
	if repeatedKey(ebiten.KeyArrowLeft) {
		f.cursor, f.selected = max(0, f.cursor-1), false
	}
	if repeatedKey(ebiten.KeyArrowRight) {
		f.cursor, f.selected = min(len(runes), f.cursor+1), false
	}
	if pressed(ebiten.KeyHome) {
		f.cursor, f.selected = 0, false
	}
	if pressed(ebiten.KeyEnd) {
		f.cursor, f.selected = len(runes), false
	}
	f.value = string(runes)
}

func paletteSwatch(d *Dialog, index int) image.Rectangle {
	x := 37 + index%8*48
	y := 46 + index/8*25
	return d.localRect(image.Rect(x, y, x+45, y+22))
}

func paletteSlider(d *Dialog, channel int) image.Rectangle {
	w := d.sourceWidth
	if w == 0 {
		w = d.rect.Dx()
	}
	return d.localRect(image.Rect(84, 184+channel*30, w-70, 202+channel*30))
}

func stencilCheckbox(d *Dialog) image.Rectangle {
	return d.localRect(image.Rect(37, 184, 62, 206))
}

func (g *Game) updatePaletteRequester(d *Dialog) {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.palettePointerDown(d, g.pointer)
	}
	if d.kind == "stencil" {
		return
	}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		d.slider = -1
	}
	if d.slider >= 0 {
		g.applyPaletteSlider(d, g.pointer)
	}
}

func (g *Game) palettePointerDown(d *Dialog, p image.Point) {
	for i := 0; i < 32; i++ {
		index := d.palettePage*32 + i
		if index < len(g.Canvas.Image.Palette) && p.In(paletteSwatch(d, i)) {
			if d.kind == "stencil" {
				g.Canvas.Stencil[index] = !g.Canvas.Stencil[index]
			} else {
				g.FG = uint8(index)
			}
		}
	}
	if d.kind == "stencil" {
		if p.In(stencilCheckbox(d)) {
			g.Canvas.StencilEnabled = !g.Canvas.StencilEnabled
		}
		return
	}
	for channel := 0; channel < 3; channel++ {
		if p.In(paletteSlider(d, channel)) {
			d.slider = channel
		}
	}
}

func (g *Game) applyPaletteSlider(d *Dialog, p image.Point) {
	r := paletteSlider(d, d.slider)
	value := max(0, min(15, (p.X-r.Min.X)*15/max(1, r.Dx()-1)))
	cr, cg, cb, _ := g.Canvas.Image.Palette[g.FG].RGBA()
	channels := [3]uint8{uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8)}
	channels[d.slider] = uint8(value * 17)
	g.Canvas.Image.Palette[g.FG] = color.RGBA{channels[0], channels[1], channels[2], 255}
}

func (g *Game) drawDialog(dst *image.RGBA) {
	d := g.dialog
	if d == nil {
		return
	}
	if d.modern {
		g.drawModernDialog(dst)
		return
	}
	// Dithered shadows echo the original requesters while keeping the picture visible.
	for y := 0; y < dst.Rect.Dy(); y++ {
		for x := (y & 1); x < dst.Rect.Dx(); x += 2 {
			p := dst.RGBAAt(x, y)
			p.R, p.G, p.B = p.R/2, p.G/2, p.B/2
			dst.SetRGBA(x, y, p)
		}
	}
	fill(dst, d.rect.Add(image.Pt(5, 5)), ink)
	fill(dst, d.rect, face)
	bevel(dst, d.rect, false)
	fill(dst, image.Rect(d.rect.Min.X+3, d.rect.Min.Y+3, d.rect.Max.X-3, d.rect.Min.Y+31), blue)
	text2(dst, clippedDialogText(d.title, (d.rect.Dx()-24)/12), d.rect.Min.X+12, d.rect.Min.Y+10, paper)
	if d.body != "" {
		maxLines := max(1, (d.rect.Dy()-88)/13)
		if len(d.fields) > 0 {
			maxLines = 2
		}
		for i, line := range wrapDialogText(d.body, (d.rect.Dx()-36)/6) {
			if i >= maxLines {
				break
			}
			text(dst, line, d.rect.Min.X+18, d.rect.Min.Y+47+i*13, ink)
		}
	}
	if d.kind == "file" {
		text(dst, "Double-click to open  |  Wheel: scroll  |  Enter: select", d.rect.Min.X+18, d.rect.Min.Y+76, ink)
		text(dst, "PNG / GIF / IFF-ILBM / LBM / BRUSH", d.rect.Min.X+18, d.rect.Max.Y-104, ink)
	}
	if !d.list.Empty() {
		g.drawRequesterList(dst, d)
	}
	for i := range d.fields {
		f := &d.fields[i]
		text(dst, f.label+":", d.rect.Min.X+18, f.rect.Min.Y+7, ink)
		fill(dst, f.rect, paper)
		bevel(dst, f.rect, true)
		chars := []rune(f.value)
		f.cursor = max(0, min(f.cursor, len(chars)))
		visible := max(1, (f.rect.Dx()-10)/6)
		start := max(0, f.cursor-visible+1)
		end := min(len(chars), start+visible)
		fg := color.Color(ink)
		if d.active == i && f.selected {
			fill(dst, image.Rect(f.rect.Min.X+3, f.rect.Min.Y+3, f.rect.Max.X-3, f.rect.Max.Y-3), blue)
			fg = paper
		}
		text(dst, string(chars[start:end]), f.rect.Min.X+5, f.rect.Min.Y+7, fg)
		if d.active == i && !f.selected && g.Frames%60 < 35 {
			x := f.rect.Min.X + 5 + (f.cursor-start)*6
			fill(dst, image.Rect(x, f.rect.Min.Y+5, x+1, f.rect.Max.Y-4), ink)
		}
	}
	if d.kind == "palette" || d.kind == "stencil" {
		g.drawPaletteRequester(dst, d)
	}
	if d.err != "" {
		text(dst, clippedDialogText(d.err, (d.rect.Dx()-36)/6), d.rect.Min.X+18, d.rect.Max.Y-52, color.RGBA{164, 24, 24, 255})
	}
	for _, button := range d.buttons {
		down := g.pointer.In(button.rect) && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
		fill(dst, button.rect, face)
		bevel(dst, button.rect, down)
		x := button.rect.Min.X + (button.rect.Dx()-len([]rune(button.label))*6)/2
		y := button.rect.Min.Y + (button.rect.Dy()-7)/2
		if down {
			x, y = x+1, y+1
		}
		text(dst, button.label, x, y, ink)
	}
}

func (g *Game) drawRequesterList(dst *image.RGBA, d *Dialog) {
	fill(dst, d.list, paper)
	bevel(dst, d.list, true)
	rows := dialogListRows(d)
	for row := 0; row < rows; row++ {
		index := d.scroll + row
		if index >= d.itemCount() {
			break
		}
		y := d.list.Min.Y + 2 + row*16
		fg := color.Color(ink)
		if d.selected == index {
			fill(dst, image.Rect(d.list.Min.X+2, y, d.list.Max.X-17, y+16), blue)
			fg = paper
		}
		label := ""
		if d.kind == "file" {
			f := d.files[index]
			label = "    " + f.name
			if f.dir {
				label = "[D] " + f.name
			}
		} else {
			label = d.entries[index]
		}
		text(dst, clippedDialogText(label, (d.list.Dx()-26)/6), d.list.Min.X+6, y+4, fg)
	}
	bar := image.Rect(d.list.Max.X-16, d.list.Min.Y+2, d.list.Max.X-2, d.list.Max.Y-2)
	fill(dst, bar, face)
	bevel(dst, bar, false)
	text(dst, "^", bar.Min.X+4, bar.Min.Y+5, ink)
	text(dst, "v", bar.Min.X+4, bar.Max.Y-12, ink)
	track := image.Rect(bar.Min.X+2, bar.Min.Y+18, bar.Max.X-2, bar.Max.Y-18)
	fill(dst, track, shadow)
	if d.itemCount() > 0 {
		thumbHeight := max(12, track.Dy()*min(rows, d.itemCount())/d.itemCount())
		thumbY := track.Min.Y
		if d.itemCount() > rows {
			thumbY += (track.Dy() - thumbHeight) * d.scroll / (d.itemCount() - rows)
		}
		thumb := image.Rect(track.Min.X, thumbY, track.Max.X, thumbY+thumbHeight)
		fill(dst, thumb, face)
		bevel(dst, thumb, false)
	}
}

func (g *Game) drawPaletteRequester(dst *image.RGBA, d *Dialog) {
	p := g.Canvas.Image.Palette
	for i := 0; i < 32; i++ {
		r := paletteSwatch(d, i)
		index := d.palettePage*32 + i
		if index >= len(p) {
			fill(dst, r, shadow)
			continue
		}
		fill(dst, r, p[index])
		selected := index == int(g.FG)
		if d.kind == "stencil" {
			selected = g.Canvas.Stencil[index]
		}
		bevel(dst, r, selected)
		if selected {
			fill(dst, image.Rect(r.Min.X+2, r.Min.Y+2, r.Max.X-2, r.Min.Y+3), paper)
			fill(dst, image.Rect(r.Min.X+2, r.Max.Y-3, r.Max.X-2, r.Max.Y-2), paper)
		}
	}
	if d.kind == "stencil" {
		text(dst, "Click colors to protect their pixels from drawing.", d.rect.Min.X+26, d.rect.Min.Y+158, ink)
		checkbox := stencilCheckbox(d)
		fill(dst, checkbox, paper)
		bevel(dst, checkbox, true)
		if g.Canvas.StencilEnabled {
			text2(dst, "X", checkbox.Min.X+7, checkbox.Min.Y+3, ink)
		}
		text(dst, "Stencil enabled", checkbox.Max.X+12, checkbox.Min.Y+8, ink)
		return
	}
	cr, cg, cb, _ := p[g.FG].RGBA()
	channels := [3]int{int(cr >> 12), int(cg >> 12), int(cb >> 12)}
	text(dst, fmt.Sprintf("Color %d  -  RGB %X%X%X  -  12-bit Amiga", g.FG, channels[0], channels[1], channels[2]), d.rect.Min.X+62, d.rect.Min.Y+158, ink)
	for channel, name := range []string{"Red", "Green", "Blue"} {
		r := paletteSlider(d, channel)
		text(dst, name, d.rect.Min.X+26, r.Min.Y+5, ink)
		for value := 0; value < 16; value++ {
			var c color.RGBA
			c.A = 255
			switch channel {
			case 0:
				c.R = uint8(value * 17)
			case 1:
				c.G = uint8(value * 17)
			case 2:
				c.B = uint8(value * 17)
			}
			fill(dst, image.Rect(r.Min.X+value*r.Dx()/16, r.Min.Y, r.Min.X+(value+1)*r.Dx()/16, r.Max.Y), c)
		}
		bevel(dst, r, true)
		x := r.Min.X + channels[channel]*(r.Dx()-6)/15
		thumb := image.Rect(x, r.Min.Y-2, x+6, r.Max.Y+2)
		fill(dst, thumb, face)
		bevel(dst, thumb, false)
		text(dst, fmt.Sprintf("%2d", channels[channel]), r.Max.X+18, r.Min.Y+5, ink)
	}
}

func clippedDialogText(s string, maxChars int) string {
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	if maxChars < 4 {
		return string(r[:max(0, maxChars)])
	}
	return string(r[:maxChars-3]) + "..."
}

func wrapDialogText(s string, width int) []string {
	width = max(1, width)
	var lines []string
	for _, paragraph := range strings.Split(s, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		line := ""
		for _, word := range strings.Fields(paragraph) {
			if line != "" && len([]rune(line))+1+len([]rune(word)) > width {
				lines = append(lines, line)
				line = ""
			}
			for len([]rune(word)) > width {
				r := []rune(word)
				lines = append(lines, string(r[:width]))
				word = string(r[width:])
			}
			if line != "" {
				line += " "
			}
			line += word
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
