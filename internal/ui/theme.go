package ui

import (
	"image"
	"image/color"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var (
	modernBG          = color.RGBA{20, 21, 27, 255}
	modernPanel       = color.RGBA{29, 31, 39, 255}
	modernRaised      = color.RGBA{39, 42, 52, 255}
	modernBorder      = color.RGBA{56, 59, 72, 255}
	modernText        = color.RGBA{235, 237, 245, 255}
	modernMuted       = color.RGBA{148, 154, 175, 255}
	modernAccent      = color.RGBA{174, 160, 255, 255}
	modernAccentSoft  = color.RGBA{61, 51, 88, 255}
	modernGood        = color.RGBA{129, 210, 178, 255}
	modernWarm        = color.RGBA{242, 177, 124, 255}
	fontLock          sync.Mutex
	uiFonts           = map[int]font.Face{}
	uiRegular, uiBold *opentype.Font
)

// Font data ships with the Go font package; no installed OS fonts or C
// libraries are needed to render the application chrome.
func modernFont(size int) font.Face {
	if uiRegular == nil {
		var err error
		uiRegular, err = opentype.Parse(goregular.TTF)
		if err != nil {
			panic(err)
		}
		uiBold, err = opentype.Parse(gobold.TTF)
		if err != nil {
			panic(err)
		}
	}
	if f := uiFonts[size]; f != nil {
		return f
	}
	data := uiRegular
	if size >= 18 {
		data = uiBold
	}
	f, err := opentype.NewFace(data, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	uiFonts[size] = f
	return f
}

func modernTextAt(dst *image.RGBA, s string, x, y, size int, c color.Color) {
	fontLock.Lock()
	defer fontLock.Unlock()
	f := modernFont(size)
	d := font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: f, Dot: fixed.P(x, y+f.Metrics().Ascent.Ceil())}
	d.DrawString(s)
}
func modernTextWidth(s string, size int) int {
	fontLock.Lock()
	defer fontLock.Unlock()
	return font.MeasureString(modernFont(size), s).Ceil()
}

func modernFit(s string, size, width int) string {
	if modernTextWidth(s, size) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 {
		r = r[:len(r)-1]
		if modernTextWidth(string(r)+"…", size) <= width {
			return string(r) + "…"
		}
	}
	return ""
}

func modernFill(dst *image.RGBA, r image.Rectangle, c color.Color, radius int) {
	if r.Empty() {
		return
	}
	radius = min(radius, min(r.Dx(), r.Dy())/2)
	if radius < 1 {
		fill(dst, r, c)
		return
	}
	fill(dst, image.Rect(r.Min.X+radius, r.Min.Y, r.Max.X-radius, r.Max.Y), c)
	fill(dst, image.Rect(r.Min.X, r.Min.Y+radius, r.Max.X, r.Max.Y-radius), c)
	for y := 0; y < radius; y++ {
		for x := 0; x < radius; x++ {
			dx, dy := radius-x-1, radius-y-1
			if dx*dx+dy*dy <= radius*radius {
				dst.Set(r.Min.X+x, r.Min.Y+y, c)
				dst.Set(r.Max.X-1-x, r.Min.Y+y, c)
				dst.Set(r.Min.X+x, r.Max.Y-1-y, c)
				dst.Set(r.Max.X-1-x, r.Max.Y-1-y, c)
			}
		}
	}
}

func modernButton(dst *image.RGBA, r image.Rectangle, label string, selected, hover bool) {
	bg, fg := modernRaised, modernText
	if hover {
		bg = color.RGBA{51, 54, 66, 255}
	}
	if selected {
		bg, fg = modernAccentSoft, modernAccent
	}
	modernFill(dst, r, bg, 6)
	if selected {
		fill(dst, image.Rect(r.Min.X+8, r.Max.Y-2, r.Max.X-8, r.Max.Y-1), modernAccent)
	}
	s := modernFit(label, 13, r.Dx()-16)
	modernTextAt(dst, s, r.Min.X+(r.Dx()-modernTextWidth(s, 13))/2, r.Min.Y+(r.Dy()-16)/2, 13, fg)
}

func modernIcon(dst *image.RGBA, t Tool, r image.Rectangle, c color.Color) {
	x, y := r.Min.X+(r.Dx()-24)/2, r.Min.Y+(r.Dy()-18)/2
	// Keep geometric tool glyphs tied to the original tool identities.
	g := &Game{}
	if t == Picker {
		drawLine(dst, x+6, y+13, x+18, y+1, c)
		drawLine(dst, x+7, y+15, x+20, y+2, c)
		drawLine(dst, x+14, y+1, x+20, y+7, c)
		drawLine(dst, x+6, y+13, x+7, y+15, c)
		return
	}
	g.drawIcon(dst, t, x, y, c)
}
