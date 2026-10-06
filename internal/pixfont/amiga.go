package pixfont

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Style bits are intentionally independent of Amiga's FSF_* bit numbering.
const (
	AmigaBold      = 1
	AmigaItalic    = 2
	AmigaUnderline = 4
)

type amigaGlyph struct {
	Code    int      `json:"code"`
	Width   int      `json:"width"`
	Advance int      `json:"advance"`
	Kern    int      `json:"kern"`
	Rows    []string `json:"rows"`
}

type amigaFontJSON struct {
	Name      string       `json:"name"`
	Size      int          `json:"size"`
	Height    int          `json:"height"`
	XSize     int          `json:"xsize"`
	Baseline  int          `json:"baseline"`
	FirstChar int          `json:"first_char"`
	LastChar  int          `json:"last_char"`
	Glyphs    []amigaGlyph `json:"glyphs"`
}

type amigaFont struct {
	height, xsize, baseline int
	glyphs                  map[rune]amigaGlyph
}

var amigaRegistry = struct {
	sync.RWMutex
	fonts map[string]*amigaFont
	names []string
}{fonts: make(map[string]*amigaFont)}

// LoadAmiga replaces the registry with bitmap fonts extracted from Amiga
// TextFont structures. Invalid input leaves the previous registry intact.
// Rendering does not depend on the original font files or an OS font service.
func LoadAmiga(data []byte) error {
	if len(data) > 8*1024*1024 {
		return fmt.Errorf("Amiga font data exceeds 8 MiB")
	}
	var source []amigaFontJSON
	if err := json.Unmarshal(data, &source); err != nil {
		return fmt.Errorf("decode Amiga fonts: %w", err)
	}
	if len(source) == 0 || len(source) > 256 {
		return fmt.Errorf("need 1 to 256 Amiga fonts")
	}
	fonts := make(map[string]*amigaFont, len(source))
	names := make([]string, 0, len(source))
	for _, raw := range source {
		name := strings.ToLower(strings.TrimSpace(raw.Name))
		name = strings.TrimSuffix(name, ".font")
		if name == "" || len(name) > 64 || strings.ContainsAny(name, "/\\\x00\r\n") {
			return fmt.Errorf("invalid Amiga font name %q", raw.Name)
		}
		key := name + "/" + strconv.Itoa(raw.Size)
		if raw.Size < 1 || raw.Size > 128 || raw.Height < 1 || raw.Height > 128 || raw.XSize < 1 || raw.XSize > 256 || raw.Baseline < 0 || raw.Baseline >= raw.Height {
			return fmt.Errorf("invalid metrics for Amiga font %q", key)
		}
		if raw.FirstChar < 0 || raw.FirstChar > 255 || raw.LastChar < raw.FirstChar || raw.LastChar > 255 || len(raw.Glyphs) == 0 || len(raw.Glyphs) > 256 {
			return fmt.Errorf("invalid Latin-1 character range for %q", key)
		}
		if _, exists := fonts[key]; exists {
			return fmt.Errorf("duplicate Amiga font %q", key)
		}
		font := &amigaFont{height: raw.Height, xsize: raw.XSize, baseline: raw.Baseline, glyphs: make(map[rune]amigaGlyph, len(raw.Glyphs))}
		for _, g := range raw.Glyphs {
			if g.Code < raw.FirstChar || g.Code > raw.LastChar || g.Width < 0 || g.Width > 256 || g.Advance < -256 || g.Advance > 256 || g.Kern < -256 || g.Kern > 256 || len(g.Rows) != raw.Height {
				return fmt.Errorf("invalid glyph %d in %q", g.Code, key)
			}
			if _, exists := font.glyphs[rune(g.Code)]; exists {
				return fmt.Errorf("duplicate glyph %d in %q", g.Code, key)
			}
			for y, row := range g.Rows {
				if len(row) != g.Width {
					return fmt.Errorf("invalid glyph %d row %d width in %q", g.Code, y, key)
				}
				for _, bit := range row {
					if bit != '0' && bit != '1' {
						return fmt.Errorf("invalid bitmap bit for glyph %d in %q", g.Code, key)
					}
				}
			}
			font.glyphs[rune(g.Code)] = g
		}
		fonts[key] = font
		names = append(names, key)
	}
	// Families sort alphabetically and sizes numerically within each family.
	sort.Slice(names, func(i, j int) bool {
		a, as, _ := strings.Cut(names[i], "/")
		b, bs, _ := strings.Cut(names[j], "/")
		if a != b {
			return a < b
		}
		an, _ := strconv.Atoi(as)
		bn, _ := strconv.Atoi(bs)
		return an < bn
	})
	amigaRegistry.Lock()
	amigaRegistry.fonts, amigaRegistry.names = fonts, names
	amigaRegistry.Unlock()
	return nil
}

// AmigaNames returns a fresh list of available "family/size" keys.
func AmigaNames() []string {
	amigaRegistry.RLock()
	defer amigaRegistry.RUnlock()
	return append([]string(nil), amigaRegistry.names...)
}

func getAmiga(name string) *amigaFont {
	amigaRegistry.RLock()
	font := amigaRegistry.fonts[strings.ToLower(strings.TrimSpace(name))]
	amigaRegistry.RUnlock()
	return font
}

func (font *amigaFont) glyph(r rune) amigaGlyph {
	if g, ok := font.glyphs[r]; ok {
		return g
	}
	if g, ok := font.glyphs['?']; ok {
		return g
	}
	// The extracted registry does not contain the extra TextFont default glyph.
	// A font without '?' still advances unsupported characters consistently.
	return amigaGlyph{Advance: font.xsize}
}

// WidthAmiga measures the longest line's pen advance, including both CharKern
// and CharSpace. It preserves Unicode Latin-1 characters such as é and ç.
// Unknown or empty font names use the existing compact font.
func WidthAmiga(name, text string, scale int) int {
	if scale <= 0 {
		return 0
	}
	font := getAmiga(name)
	if font == nil {
		return Width(text, scale)
	}
	width, pen := 0, 0
	for _, r := range text {
		if r == '\n' {
			width = max(width, pen)
			pen = 0
			continue
		}
		g := font.glyph(r)
		pen += g.Kern + g.Advance
	}
	return max(width, pen) * scale
}

// HeightAmiga returns one unstyled font cell's height. Multiline rendering
// advances by two additional pixels per line, like the existing Walk renderer.
func HeightAmiga(name string, scale int) int {
	if scale <= 0 {
		return 0
	}
	if font := getAmiga(name); font != nil {
		return font.height * scale
	}
	return 7 * scale
}

// WalkAmiga emits foreground pixels at integer scale. x/y locate the top of
// the font cell, rather than its baseline. Styles smear bold pixels one column
// right, shear italics around the baseline, and underline beneath it. Styled
// pixels may extend beyond the ordinary advance; WidthAmiga measures the pen.
func WalkAmiga(name, text string, x, y, scale, style int, set func(int, int)) {
	if set == nil || scale <= 0 {
		return
	}
	font := getAmiga(name)
	if font == nil {
		walkFallback(text, x, y, scale, style, set)
		return
	}
	pen, lineY := 0, y
	underline := min(font.baseline+2, font.height-1)
	var underlineInk map[int]bool
	if style&AmigaUnderline != 0 {
		underlineInk = make(map[int]bool)
	}
	finishLine := func() {
		if underlineInk == nil {
			return
		}
		for col := 0; col < pen; col++ {
			// Leave one-pixel gaps around descenders, as the Amiga text renderer
			// does, while retaining pixels belonging to the glyph itself.
			if underlineInk[col] || (!underlineInk[col-1] && !underlineInk[col+1]) {
				emitAmigaPixel(x+col*scale, lineY+underline*scale, scale, set)
			}
		}
		clear(underlineInk)
	}
	for _, r := range text {
		if r == '\n' {
			finishLine()
			pen = 0
			lineY += (font.height + 2) * scale
			continue
		}
		g := font.glyph(r)
		pen += g.Kern
		for gy, row := range g.Rows {
			shift := 0
			if style&AmigaItalic != 0 {
				shift = italicOffset(font.baseline, gy)
			}
			for gx := 0; gx < len(row); gx++ {
				if row[gx] != '1' {
					continue
				}
				col := pen + gx + shift
				emitAmigaPixel(x+col*scale, lineY+gy*scale, scale, set)
				if underlineInk != nil && gy == underline {
					underlineInk[col] = true
				}
				if style&AmigaBold != 0 {
					emitAmigaPixel(x+(col+1)*scale, lineY+gy*scale, scale, set)
					if underlineInk != nil && gy == underline {
						underlineInk[col+1] = true
					}
				}
			}
		}
		pen += g.Advance
	}
	finishLine()
}

func italicOffset(baseline, row int) int {
	d := baseline - row
	if d < 0 {
		return (d - 1) / 2
	}
	return d / 2
}

func emitAmigaPixel(x, y, scale int, set func(int, int)) {
	for sy := 0; sy < scale; sy++ {
		for sx := 0; sx < scale; sx++ {
			set(x+sx, y+sy)
		}
	}
}

func walkFallback(text string, x, y, scale, style int, set func(int, int)) {
	if style == 0 {
		Walk(text, x, y, scale, set)
		return
	}
	Walk(text, x, y, scale, func(px, py int) {
		if style&AmigaItalic != 0 {
			px += italicOffset(6, ((py-y)/scale)%9) * scale
		}
		set(px, py)
		if style&AmigaBold != 0 {
			set(px+scale, py)
		}
	})
	if style&AmigaUnderline != 0 {
		for line, s := range strings.Split(text, "\n") {
			for col := 0; col < Width(s, 1); col++ {
				emitAmigaPixel(x+col*scale, y+(line*9+6)*scale, scale, set)
			}
		}
	}
}
