package ui

import (
	"image"
	"image/color"

	"pixeluxe/internal/paint"
)

// paintShape separates geometry from its paint operation. A one-color mask
// determines the shape's pixels; the selected brush/mode paints contours and
// the configured pattern/gradient paints interiors. Symmetry is applied once.
func (g *Game) paintShape(c *paint.Canvas, a, b image.Point, tool Tool, points []image.Point) {
	if c == nil || c.Image == nil || c.Image.Bounds().Empty() {
		return
	}
	// Keep primitive arithmetic and drag-time work bounded before calculating
	// an inclusive rectangle. Real canvas coordinates fit well within this range.
	validPoint := func(p image.Point) bool {
		return p.X >= -(1<<16) && p.X <= 1<<16 && p.Y >= -(1<<16) && p.Y <= 1<<16
	}
	if !validPoint(a) || !validPoint(b) {
		return
	}
	for _, p := range points {
		if !validPoint(p) {
			return
		}
	}
	g.normalize()
	c.Stencil, c.StencilEnabled = g.Canvas.Stencil, g.Canvas.StencilEnabled
	g.prepareCanvas(c)
	filled := tool == FilledRectangle || tool == FilledEllipse || tool == FilledCircle || tool == FilledPolygon
	if tool == Circle || tool == FilledCircle {
		n := max(abs(b.X-a.X), abs(b.Y-a.Y))
		b = image.Pt(a.X+sign(b.X-a.X)*n, a.Y+sign(b.Y-a.Y)*n)
	}
	bbox := image.Rect(min(a.X, b.X), min(a.Y, b.Y), max(a.X, b.X)+1, max(a.Y, b.Y)+1)
	if tool == Polygon || tool == FilledPolygon {
		if len(points) == 0 {
			return
		}
		lo, hi := points[0], points[0]
		for _, p := range points[1:] {
			lo.X, lo.Y = min(lo.X, p.X), min(lo.Y, p.Y)
			hi.X, hi.Y = max(hi.X, p.X), max(hi.Y, p.Y)
		}
		bbox = image.Rect(lo.X, lo.Y, hi.X+1, hi.Y+1)
	}
	if bbox.Empty() {
		return
	}
	// Brush centers just outside the picture can still paint its edge. Include
	// their halo in the mask, and bound allocations for far-away drag endpoints.
	maskBounds := bbox
	if int64(maskBounds.Dx())*int64(maskBounds.Dy()) > 16<<20 {
		margin := max(0, g.BrushSize-1)
		if g.Brush != nil && g.Brush.Image != nil {
			margin = max(g.Brush.Image.Rect.Dx(), g.Brush.Image.Rect.Dy())
		}
		maskBounds = bbox.Intersect(c.Image.Bounds().Inset(-margin))
		if int64(maskBounds.Dx())*int64(maskBounds.Dy()) > 16<<20 {
			maskBounds = bbox.Intersect(c.Image.Bounds())
		}
	}
	if maskBounds.Empty() {
		return
	}
	palette := color.Palette{color.Black, color.White}
	mask := paint.New(1, 1, palette)
	mask.Image = image.NewPaletted(maskBounds, palette)
	switch tool {
	case Rectangle, FilledRectangle:
		mask.Rectangle(a.X, a.Y, b.X, b.Y, 1, filled)
	case Ellipse, FilledEllipse, Circle, FilledCircle:
		mask.Ellipse(a.X, a.Y, b.X, b.Y, 1, filled)
	case Polygon, FilledPolygon:
		mask.Polygon(points, 1, filled)
	default:
		return
	}
	oldSource := g.effectSource
	if !filled && oldSource == nil && (g.Mode == 3 || g.Mode == 5 || g.Mode == 7) {
		g.effectSource = paint.CloneImage(c.Image)
	}
	defer func() { g.effectSource = oldSource }()
	for y := maskBounds.Min.Y; y < maskBounds.Max.Y; y++ {
		for x := maskBounds.Min.X; x < maskBounds.Max.X; x++ {
			if mask.Image.ColorIndexAt(x, y) == 0 {
				continue
			}
			p := image.Pt(x, y)
			if !filled {
				g.stamp(c, p)
				continue
			}
			if !g.erase && !g.shapePatternOpaque(p, bbox) {
				continue
			}
			col := g.currentColor()
			if !g.erase {
				col = g.fillColor(p, bbox)
			}
			g.symmetry(p, func(q image.Point) { c.Pixel(q.X, q.Y, col) })
		}
	}
}

// Test pattern opacity before symmetry: a transparent source pixel must leave
// each destination alone, even when the mirrored picture has a different color.
func (g *Game) shapePatternOpaque(p image.Point, bbox image.Rectangle) bool {
	if (g.FillStyle != 1 && g.FillStyle != 2) || g.Brush == nil || g.Brush.Image == nil {
		return true
	}
	b := g.Brush.Image.Bounds()
	if b.Empty() {
		return false
	}
	x, y := p.X, p.Y
	if g.FillStyle == 2 {
		x -= bbox.Min.X
		y -= bbox.Min.Y
	}
	x = ((x % b.Dx()) + b.Dx()) % b.Dx()
	y = ((y % b.Dy()) + b.Dy()) % b.Dy()
	if len(g.Brush.Mask) == b.Dx()*b.Dy() {
		return g.Brush.Mask[y*b.Dx()+x]
	}
	return g.Brush.Image.ColorIndexAt(x+b.Min.X, y+b.Min.Y) != g.Brush.Transparent
}
