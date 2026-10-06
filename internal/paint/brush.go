package paint

import "image"

// Brush is an indexed image with an optional per-pixel opacity mask. If Mask is
// absent, Transparent identifies the transparent palette index.
type Brush struct {
	Image       *image.Paletted
	Transparent uint8
	Mask        []bool
}

// Capture copies a rectangular part of an image into a zero-origin brush.
// Areas outside the source are transparent, as is the selected background index.
func Capture(img *image.Paletted, r image.Rectangle, transparent uint8) *Brush {
	if img == nil || r.Empty() {
		return nil
	}
	b := &Brush{
		Image:       image.NewPaletted(image.Rect(0, 0, r.Dx(), r.Dy()), clonePalette(img.Palette)),
		Transparent: transparent,
		Mask:        make([]bool, r.Dx()*r.Dy()),
	}
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			index := transparent
			p := image.Pt(r.Min.X+x, r.Min.Y+y)
			if p.In(img.Rect) {
				index = img.ColorIndexAt(p.X, p.Y)
				b.Mask[y*r.Dx()+x] = index != transparent
			}
			b.Image.SetColorIndex(x, y, index)
		}
	}
	return b
}

func (b *Brush) opaque(x, y int) bool {
	if b == nil || b.Image == nil || !image.Pt(x, y).In(b.Image.Rect) {
		return false
	}
	if len(b.Mask) == b.Image.Rect.Dx()*b.Image.Rect.Dy() {
		return b.Mask[(y-b.Image.Rect.Min.Y)*b.Image.Rect.Dx()+x-b.Image.Rect.Min.X]
	}
	return b.Image.ColorIndexAt(x, y) != b.Transparent
}

func (b *Brush) transform(w, h int, source func(x, y int) image.Point) {
	if b == nil || b.Image == nil || w < 1 || h < 1 {
		return
	}
	dst := image.NewPaletted(image.Rect(0, 0, w, h), clonePalette(b.Image.Palette))
	mask := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := source(x, y).Add(b.Image.Rect.Min)
			dst.SetColorIndex(x, y, b.Image.ColorIndexAt(p.X, p.Y))
			mask[y*w+x] = b.opaque(p.X, p.Y)
		}
	}
	b.Image, b.Mask = dst, mask
}

func (b *Brush) FlipX() {
	if b == nil || b.Image == nil {
		return
	}
	w, h := b.Image.Rect.Dx(), b.Image.Rect.Dy()
	b.transform(w, h, func(x, y int) image.Point { return image.Pt(w-1-x, y) })
}

func (b *Brush) FlipY() {
	if b == nil || b.Image == nil {
		return
	}
	w, h := b.Image.Rect.Dx(), b.Image.Rect.Dy()
	b.transform(w, h, func(x, y int) image.Point { return image.Pt(x, h-1-y) })
}

// Rotate90 rotates the brush clockwise, preserving its opacity mask.
func (b *Brush) Rotate90() {
	if b == nil || b.Image == nil {
		return
	}
	w, h := b.Image.Rect.Dx(), b.Image.Rect.Dy()
	b.transform(h, w, func(x, y int) image.Point { return image.Pt(y, h-1-x) })
}

// Resize uses nearest-neighbour scaling so palette indices remain unchanged.
func (b *Brush) Resize(w, h int) {
	if b == nil || b.Image == nil || w < 1 || h < 1 {
		return
	}
	sw, sh := b.Image.Rect.Dx(), b.Image.Rect.Dy()
	if sw == 0 || sh == 0 {
		return
	}
	b.transform(w, h, func(x, y int) image.Point { return image.Pt(x*sw/w, y*sh/h) })
}

// Stamp places the centre of a brush at x,y. replace selects the source colours;
// otherwise all opaque brush pixels receive col, like a monochrome silhouette.
func (c *Canvas) Stamp(b *Brush, x, y int, col uint8, replace bool) {
	if c.Image == nil || b == nil || b.Image == nil {
		return
	}
	w, h := b.Image.Rect.Dx(), b.Image.Rect.Dy()
	left, top := x-w/2, y-h/2
	for by := 0; by < h; by++ {
		for bx := 0; bx < w; bx++ {
			source := image.Pt(bx, by).Add(b.Image.Rect.Min)
			if !b.opaque(source.X, source.Y) {
				continue
			}
			index := col
			if replace {
				index = b.Image.ColorIndexAt(source.X, source.Y)
			}
			c.Pixel(left+bx, top+by, index)
		}
	}
}
