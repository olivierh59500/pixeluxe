package ui

import (
	"image"
	"image/color"

	"pixeluxe/internal/paint"
)

func (g *Game) normalize() {
	n := len(g.Canvas.Image.Palette)
	g.FG = uint8(min(int(g.FG), n-1))
	g.BG = uint8(min(int(g.BG), n-1))
	g.CycleLow = max(0, min(g.CycleLow, n-1))
	g.CycleHigh = max(g.CycleLow, min(g.CycleHigh, n-1))
	g.CycleSpeed = max(1, g.CycleSpeed)
	if g.Background != nil && g.Background.Bounds() != g.Canvas.Image.Bounds() {
		g.Background = nil
		g.FixedBackground = false
	}
}
func (g *Game) prepareCanvas(c *paint.Canvas) {
	c.Erase = g.erase && g.FixedBackground
	c.RestoreImage = g.Background
	c.StencilMask = g.Canvas.StencilMask
}

func (g *Game) paintStamp(c *paint.Canvas, p image.Point) {
	g.normalize()
	g.prepareCanvas(c)
	source := c.Image
	if g.effectSource != nil {
		source = g.effectSource
	} else if g.Mode == 3 || g.Mode == 5 || g.Mode == 7 {
		source = paint.CloneImage(c.Image)
	}
	var lut [256]uint8
	if g.Brush != nil {
		for i, col := range g.Brush.Image.Palette {
			lut[i] = nearest(c.Image.Palette, col)
		}
	}
	g.symmetry(p, func(q image.Point) {
		r := max(0, g.BrushSize-1)
		x0, y0, x1, y1 := q.X-r, q.Y-r, q.X+r+1, q.Y+r+1
		var br image.Rectangle
		if g.Brush != nil {
			br = g.Brush.Image.Bounds()
			x0, y0 = q.X-br.Dx()/2, q.Y-br.Dy()/2
			if g.Handles == 1 {
				x0, y0 = q.X, q.Y
			}
			x1, y1 = x0+br.Dx(), y0+br.Dy()
		}
		clip := image.Rect(x0, y0, x1, y1).Intersect(c.Image.Bounds())
		for y := clip.Min.Y; y < clip.Max.Y; y++ {
			for x := clip.Min.X; x < clip.Max.X; x++ {
				col := g.currentColor()
				opaque := true
				var brushColor color.Color = c.Image.Palette[g.FG]
				if g.Brush != nil {
					bx, by := x-x0, y-y0
					idx := g.Brush.Image.ColorIndexAt(bx+br.Min.X, by+br.Min.Y)
					opaque = idx != g.Brush.Transparent
					if len(g.Brush.Mask) == br.Dx()*br.Dy() {
						opaque = g.Brush.Mask[by*br.Dx()+bx]
					}
					brushColor = g.Brush.Image.Palette[idx]
					if (g.Mode == 0 || g.Mode == 2) && !g.erase {
						col = lut[idx]
					}
				}
				if g.Brush == nil && g.BrushShape == 0 && (x-q.X)*(x-q.X)+(y-q.Y)*(y-q.Y) > r*r {
					continue
				}
				if g.Brush == nil && g.BrushShape == 2 && (x+y)&1 != 0 {
					continue
				}
				if !opaque && (g.Mode != 2 || g.erase) {
					continue
				}
				if !g.erase {
					switch g.Mode {
					case 3:
						sx, sy := x-(p.X-g.last.X), y-(p.Y-g.last.Y)
						if image.Pt(sx, sy).In(source.Bounds()) {
							col = source.ColorIndexAt(sx, sy)
						} else {
							continue
						}
					case 4:
						i := int(source.ColorIndexAt(x, y))
						if i >= g.CycleLow && i <= g.CycleHigh {
							col = uint8(min(g.CycleHigh, i+1))
						} else {
							col = uint8(min(len(source.Palette)-1, i+1))
						}
					case 5:
						col = nearest(c.Image.Palette, mix(source.At(x, y), brushColor))
					case 6:
						col = uint8(g.CycleLow + g.cycleTick%(g.CycleHigh-g.CycleLow+1))
					case 7:
						var rr, gg, bb, n uint32
						for yy := y - 1; yy <= y+1; yy++ {
							for xx := x - 1; xx <= x+1; xx++ {
								if image.Pt(xx, yy).In(source.Bounds()) {
									r, g, b, _ := source.At(xx, yy).RGBA()
									rr += r
									gg += g
									bb += b
									n++
								}
							}
						}
						if n > 0 {
							col = nearest(c.Image.Palette, color.RGBA{uint8(rr / n >> 8), uint8(gg / n >> 8), uint8(bb / n >> 8), 255})
						}
					}
				}
				c.Pixel(x, y, col)
			}
		}
	})
}

func (g *Game) makeStencilFromColors() {
	b := g.Canvas.Image.Bounds()
	g.Canvas.StencilMask = make([]bool, b.Dx()*b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			g.Canvas.StencilMask[(y-b.Min.Y)*b.Dx()+x-b.Min.X] = g.Canvas.Stencil[g.Canvas.Image.ColorIndexAt(x, y)]
		}
	}
}
func (g *Game) lockForeground() {
	if !g.FixedBackground || g.Background == nil {
		g.message("Lock foreground", "Fix a background first, then draw over it.")
		return
	}
	b := g.Canvas.Image.Bounds()
	g.Canvas.StencilMask = make([]bool, b.Dx()*b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			g.Canvas.StencilMask[(y-b.Min.Y)*b.Dx()+x-b.Min.X] = g.Canvas.Image.ColorIndexAt(x, y) != g.Background.ColorIndexAt(x, y)
		}
	}
	g.Canvas.StencilEnabled = true
}
