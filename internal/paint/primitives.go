package paint

import (
	"image"
	"math"
	"sort"
)

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func ordered(a, b int) (int, int) {
	if a > b {
		return b, a
	}
	return a, b
}

// Line uses integer Bresenham rasterisation, including both endpoints.
func (c *Canvas) Line(x0, y0, x1, y1 int, col uint8) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		c.Pixel(x0, y0, col)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func (c *Canvas) Rectangle(x0, y0, x1, y1 int, col uint8, filled bool) {
	x0, x1 = ordered(x0, x1)
	y0, y1 = ordered(y0, y1)
	if !filled {
		c.Line(x0, y0, x1, y0, col)
		c.Line(x1, y0, x1, y1, col)
		c.Line(x1, y1, x0, y1, col)
		c.Line(x0, y1, x0, y0, col)
		return
	}
	if c.Image == nil {
		return
	}
	r := image.Rect(x0, y0, x1+1, y1+1).Intersect(c.Image.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c.Pixel(x, y, col)
		}
	}
}

// Ellipse rasterises the ellipse inside the inclusive bounding rectangle.
func (c *Canvas) Ellipse(x0, y0, x1, y1 int, col uint8, filled bool) {
	if c.Image == nil {
		return
	}
	x0, x1 = ordered(x0, x1)
	y0, y1 = ordered(y0, y1)
	r := image.Rect(x0, y0, x1+1, y1+1).Intersect(c.Image.Rect)
	if r.Empty() {
		return
	}
	if x0 == x1 || y0 == y1 {
		c.Line(x0, y0, x1, y1, col)
		return
	}
	var left, right []int
	if filled {
		left, right = make([]int, r.Dy()), make([]int, r.Dy())
		for i := range left {
			left[i], right[i] = x1+1, x0-1
		}
	}
	plot := func(x, y int) {
		if !filled {
			c.Pixel(x, y, col)
			return
		}
		if y < r.Min.Y || y >= r.Max.Y {
			return
		}
		i := y - r.Min.Y
		left[i] = min(left[i], x)
		right[i] = max(right[i], x)
	}
	// Incremental midpoint algorithm for all odd/even bounding-box sizes.
	a, b := int64(x1-x0), int64(y1-y0)
	originalHeight := b
	bParity := b & 1
	dx, dy := 4*(1-a)*b*b, 4*(bParity+1)*a*a
	err := dx + dy + bParity*a*a
	y0 += int((b + 1) / 2)
	y1 = y0 - int(bParity)
	a, b = 8*a*a, 8*b*b
	for {
		plot(x1, y0)
		plot(x0, y0)
		plot(x0, y1)
		plot(x1, y1)
		e2 := 2 * err
		if e2 <= dy {
			y0++
			y1--
			dy += a
			err += dy
		}
		if e2 >= dx || 2*err > dy {
			x0++
			x1--
			dx += b
			err += dx
		}
		if x0 > x1 {
			break
		}
	}
	for int64(y0-y1) < originalHeight {
		plot(x0-1, y0)
		plot(x1+1, y0)
		y0++
		plot(x0-1, y1)
		plot(x1+1, y1)
		y1--
	}
	if filled {
		for i := range left {
			for x := max(left[i], r.Min.X); x <= min(right[i], r.Max.X-1); x++ {
				c.Pixel(x, r.Min.Y+i, col)
			}
		}
	}
}

// Polygon fills concave polygons using the even-odd rule and closes its outline.
func (c *Canvas) Polygon(points []image.Point, col uint8, filled bool) {
	if c.Image == nil || len(points) == 0 {
		return
	}
	if filled && len(points) > 2 {
		minY, maxY := points[0].Y, points[0].Y
		for _, p := range points[1:] {
			minY, maxY = min(minY, p.Y), max(maxY, p.Y)
		}
		minY, maxY = max(minY, c.Image.Rect.Min.Y), min(maxY, c.Image.Rect.Max.Y-1)
		intersections := make([]float64, 0, len(points))
		for y := minY; y <= maxY; y++ {
			intersections = intersections[:0]
			scanY := float64(y) + 0.5
			for i, p := range points {
				q := points[(i+1)%len(points)]
				if (float64(p.Y) <= scanY && float64(q.Y) > scanY) || (float64(q.Y) <= scanY && float64(p.Y) > scanY) {
					x := float64(p.X) + (scanY-float64(p.Y))*float64(q.X-p.X)/float64(q.Y-p.Y)
					intersections = append(intersections, x)
				}
			}
			sort.Float64s(intersections)
			for i := 0; i+1 < len(intersections); i += 2 {
				start := max(c.Image.Rect.Min.X, int(math.Ceil(intersections[i]-0.5)))
				end := min(c.Image.Rect.Max.X-1, int(math.Floor(intersections[i+1]-0.5)))
				for x := start; x <= end; x++ {
					c.Pixel(x, y, col)
				}
			}
		}
	}
	for i, p := range points {
		q := points[(i+1)%len(points)]
		c.Line(p.X, p.Y, q.X, q.Y, col)
	}
}

// FloodFill replaces a four-connected region with an iterative scanline fill.
func (c *Canvas) FloodFill(x, y int, col uint8) {
	if c.Image == nil || !image.Pt(x, y).In(c.Image.Rect) || int(col) >= len(c.Image.Palette) {
		return
	}
	target := c.Image.ColorIndexAt(x, y)
	restoring := c.Erase && c.RestoreImage != nil
	if (target == col && !restoring) || c.Protected(x, y) {
		return
	}
	bounds := c.Image.Rect
	var visited []bool
	if restoring {
		visited = make([]bool, bounds.Dx()*bounds.Dy())
	}
	matchPixel := func(px, py int) bool {
		if visited != nil && visited[(py-bounds.Min.Y)*bounds.Dx()+px-bounds.Min.X] {
			return false
		}
		return c.Image.ColorIndexAt(px, py) == target && !c.Protected(px, py)
	}
	stack := []image.Point{image.Pt(x, y)}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !matchPixel(p.X, p.Y) {
			continue
		}
		left, right := p.X, p.X
		for left > bounds.Min.X && matchPixel(left-1, p.Y) {
			left--
		}
		for right < bounds.Max.X-1 && matchPixel(right+1, p.Y) {
			right++
		}
		for xx := left; xx <= right; xx++ {
			if visited != nil {
				visited[(p.Y-bounds.Min.Y)*bounds.Dx()+xx-bounds.Min.X] = true
			}
			c.Pixel(xx, p.Y, col)
		}
		for _, yy := range [...]int{p.Y - 1, p.Y + 1} {
			if yy < bounds.Min.Y || yy >= bounds.Max.Y {
				continue
			}
			inRun := false
			for xx := left; xx <= right; xx++ {
				match := matchPixel(xx, yy)
				if match && !inRun {
					stack = append(stack, image.Pt(xx, yy))
				}
				inRun = match
			}
		}
	}
}
