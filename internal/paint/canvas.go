// Package paint implements Pixeluxe's indexed-colour drawing operations.
// It has no window-system dependencies and keeps the original palette indices
// throughout editing, undo and brush transformations.
package paint

import (
	"image"
	"image/color"
)

const historyLimit = 64

// Canvas holds an editable indexed-colour image. A true stencil entry protects
// pixels of that colour when StencilEnabled is set.
type Canvas struct {
	Image          *image.Paletted
	Stencil        [256]bool
	StencilEnabled bool
	// StencilMask optionally locks fixed picture locations, as the original
	// Make/Remake and Lock FG commands do. Indices remain the fallback.
	StencilMask []bool
	// RestoreImage supplies the fixed background for erasing operations.
	RestoreImage *image.Paletted
	Erase        bool
	before       *image.Paletted
	undo         []*image.Paletted
	redo         []*image.Paletted
	saved        *image.Paletted
}

// New creates a blank canvas with its own copy of the supplied palette.
func New(w, h int, p color.Palette) *Canvas {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if len(p) == 0 {
		p = DefaultPalette()
	}
	if len(p) > 256 {
		p = p[:256]
	}
	c := &Canvas{Image: image.NewPaletted(image.Rect(0, 0, w, h), clonePalette(p))}
	c.MarkSaved()
	return c
}

// DefaultPalette returns 32 colours on the Amiga's twelve-bit RGB grid.
func DefaultPalette() color.Palette {
	values := [...]uint16{
		0x000, 0xfff, 0xf00, 0xf80, 0xff0, 0x8f0, 0x0f0, 0x0f8,
		0x0ff, 0x08f, 0x00f, 0x80f, 0xf0f, 0xf08, 0x888, 0x444,
		0x222, 0x666, 0xaaa, 0xccc, 0x600, 0xa40, 0xc80, 0xfc8,
		0x640, 0x864, 0xa86, 0xca8, 0x046, 0x068, 0x48a, 0x8ce,
	}
	p := make(color.Palette, len(values))
	for i, v := range values {
		p[i] = color.RGBA{uint8(v>>8&15) * 17, uint8(v>>4&15) * 17, uint8(v&15) * 17, 255}
	}
	return p
}

func clonePalette(p color.Palette) color.Palette {
	copy := make(color.Palette, len(p))
	for i, c := range p {
		if c == nil {
			copy[i] = color.RGBA{A: 255}
			continue
		}
		r, g, b, a := c.RGBA()
		copy[i] = color.RGBA64{uint16(r), uint16(g), uint16(b), uint16(a)}
	}
	return copy
}

// CloneImage makes a deep copy, including palettes, non-zero origins and the
// visible part of a subimage with a larger stride.
func CloneImage(src *image.Paletted) *image.Paletted {
	if src == nil {
		return nil
	}
	dst := image.NewPaletted(src.Rect, clonePalette(src.Palette))
	for y := src.Rect.Min.Y; y < src.Rect.Max.Y; y++ {
		si, di := src.PixOffset(src.Rect.Min.X, y), dst.PixOffset(dst.Rect.Min.X, y)
		copy(dst.Pix[di:di+dst.Rect.Dx()], src.Pix[si:si+src.Rect.Dx()])
	}
	return dst
}

// Begin starts one undoable action. Repeated calls do not discard its beginning.
func (c *Canvas) Begin() {
	if c.before == nil {
		c.before = CloneImage(c.Image)
	}
}

// Commit records an action only when image dimensions, indices or colours changed.
func (c *Canvas) Commit() {
	if c.before == nil {
		return
	}
	if !equalImage(c.before, c.Image) {
		c.undo = append(c.undo, c.before)
		if len(c.undo) > historyLimit {
			copy(c.undo, c.undo[len(c.undo)-historyLimit:])
			c.undo = c.undo[:historyLimit]
		}
		c.redo = nil
	}
	c.before = nil
}

// Cancel restores the state at Begin without adding an undo entry.
func (c *Canvas) Cancel() {
	if c.before != nil {
		c.Image = c.before
		c.before = nil
	}
}

func (c *Canvas) Undo() bool {
	c.Commit()
	if len(c.undo) == 0 {
		return false
	}
	c.redo = append(c.redo, CloneImage(c.Image))
	c.Image = c.undo[len(c.undo)-1]
	c.undo = c.undo[:len(c.undo)-1]
	return true
}

func (c *Canvas) Redo() bool {
	c.Commit()
	if len(c.redo) == 0 {
		return false
	}
	c.undo = append(c.undo, CloneImage(c.Image))
	c.Image = c.redo[len(c.redo)-1]
	c.redo = c.redo[:len(c.redo)-1]
	return true
}

func (c *Canvas) Dirty() bool { return !equalImage(c.Image, c.saved) }

func (c *Canvas) MarkSaved() { c.saved = CloneImage(c.Image) }

func equalImage(a, b *image.Paletted) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Rect != b.Rect || len(a.Palette) != len(b.Palette) {
		return false
	}
	for i := range a.Palette {
		ar, ag, ab, aa := a.Palette[i].RGBA()
		br, bg, bb, ba := b.Palette[i].RGBA()
		if ar != br || ag != bg || ab != bb || aa != ba {
			return false
		}
	}
	for y := a.Rect.Min.Y; y < a.Rect.Max.Y; y++ {
		ai, bi := a.PixOffset(a.Rect.Min.X, y), b.PixOffset(b.Rect.Min.X, y)
		for x := 0; x < a.Rect.Dx(); x++ {
			if a.Pix[ai+x] != b.Pix[bi+x] {
				return false
			}
		}
	}
	return true
}

// Pixel writes a palette index, clipped to the canvas and stencil.
func (c *Canvas) Pixel(x, y int, col uint8) {
	if c.Image == nil || !image.Pt(x, y).In(c.Image.Rect) || int(col) >= len(c.Image.Palette) {
		return
	}
	i := c.Image.PixOffset(x, y)
	if c.Protected(x, y) {
		return
	}
	if c.Erase && c.RestoreImage != nil && image.Pt(x, y).In(c.RestoreImage.Bounds()) {
		col = c.RestoreImage.ColorIndexAt(x, y)
		if int(col) >= len(c.Image.Palette) {
			col = uint8(c.Image.Palette.Index(c.RestoreImage.At(x, y)))
		}
	}
	c.Image.Pix[i] = col
}

// Protected reports whether a stencil prevents editing a picture location.
func (c *Canvas) Protected(x, y int) bool {
	if c.Image == nil || !image.Pt(x, y).In(c.Image.Bounds()) {
		return true
	}
	if !c.StencilEnabled {
		return false
	}
	b := c.Image.Bounds()
	if len(c.StencilMask) == b.Dx()*b.Dy() {
		return c.StencilMask[(y-b.Min.Y)*b.Dx()+x-b.Min.X]
	}
	return c.Stencil[c.Image.ColorIndexAt(x, y)]
}

func (c *Canvas) Clear(col uint8) {
	if c.Image == nil {
		return
	}
	for y := c.Image.Rect.Min.Y; y < c.Image.Rect.Max.Y; y++ {
		for x := c.Image.Rect.Min.X; x < c.Image.Rect.Max.X; x++ {
			c.Pixel(x, y, col)
		}
	}
}
