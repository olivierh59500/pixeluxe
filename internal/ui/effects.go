package ui

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"math"
	"os"
)

// fillArea keeps connectivity in the original indexed picture, then colors
// the resulting region. Pattern and gradient operations share the stencil.
func (g *Game) fillArea(p image.Point) {
	if !p.In(g.Canvas.Image.Bounds()) {
		return
	}
	if g.FillStyle == 0 && !(g.erase && g.FixedBackground) {
		g.Canvas.FloodFill(p.X, p.Y, g.currentColor())
		return
	}
	im := g.Canvas.Image
	target := im.ColorIndexAt(p.X, p.Y)
	if g.Canvas.Protected(p.X, p.Y) {
		return
	}
	b := im.Bounds()
	seen := make([]bool, b.Dx()*b.Dy())
	points := []image.Point{p}
	seen[(p.Y-b.Min.Y)*b.Dx()+p.X-b.Min.X] = true
	lo, hi := p, p
	for i := 0; i < len(points); i++ {
		q := points[i]
		lo.X = min(lo.X, q.X)
		lo.Y = min(lo.Y, q.Y)
		hi.X = max(hi.X, q.X)
		hi.Y = max(hi.Y, q.Y)
		for _, d := range []image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			v := q.Add(d)
			if !v.In(b) {
				continue
			}
			idx := (v.Y-b.Min.Y)*b.Dx() + v.X - b.Min.X
			if !seen[idx] && !g.Canvas.Protected(v.X, v.Y) && im.ColorIndexAt(v.X, v.Y) == target {
				seen[idx] = true
				points = append(points, v)
			}
		}
	}
	for _, q := range points {
		g.Canvas.Pixel(q.X, q.Y, g.fillColor(q, image.Rect(lo.X, lo.Y, hi.X+1, hi.Y+1)))
	}
}
func (g *Game) fillColor(p image.Point, r image.Rectangle) uint8 {
	if (g.FillStyle == 1 || g.FillStyle == 2) && g.Brush != nil {
		b := g.Brush.Image.Bounds()
		x, y := p.X, p.Y
		if g.FillStyle == 2 {
			x -= r.Min.X
			y -= r.Min.Y
		}
		x = ((x % b.Dx()) + b.Dx()) % b.Dx()
		y = ((y % b.Dy()) + b.Dy()) % b.Dy()
		idx := g.Brush.Image.ColorIndexAt(x+b.Min.X, y+b.Min.Y)
		opaque := idx != g.Brush.Transparent
		if len(g.Brush.Mask) == b.Dx()*b.Dy() {
			opaque = g.Brush.Mask[y*b.Dx()+x]
		}
		if !opaque {
			return g.Canvas.Image.ColorIndexAt(p.X, p.Y)
		}
		return nearest(g.Canvas.Image.Palette, g.Brush.Image.At(x+b.Min.X, y+b.Min.Y))
	}
	if g.FillStyle == 3 || g.FillStyle == 4 {
		lo, hi := min(g.CycleLow, len(g.Canvas.Image.Palette)-1), min(g.CycleHigh, len(g.Canvas.Image.Palette)-1)
		hi = max(lo, hi)
		t := float64(p.Y-r.Min.Y) / float64(max(1, r.Dy()-1)) * float64(hi-lo)
		i := int(t)
		if g.FillStyle == 4 {
			matrix := [4][4]float64{{0, 8, 2, 10}, {12, 4, 14, 6}, {3, 11, 1, 9}, {15, 7, 13, 5}}
			if t-float64(i) > (matrix[p.Y&3][p.X&3]+0.5)/16 {
				i++
			}
		}
		return uint8(min(hi, lo+i))
	}
	return g.currentColor()
}

// PrintPDF creates a portable, lossless A4 print page using only the standard
// library. The image is centered and fitted without altering the document.
func (g *Game) PrintPDF(path string) error {
	im := g.Canvas.Image
	b := im.Bounds()
	var raw bytes.Buffer
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, b, _ := im.At(x, y).RGBA()
			raw.Write([]byte{byte(r >> 8), byte(g >> 8), byte(b >> 8)})
		}
	}
	var compressed bytes.Buffer
	z := zlib.NewWriter(&compressed)
	if _, err := z.Write(raw.Bytes()); err != nil {
		return err
	}
	if err := z.Close(); err != nil {
		return err
	}
	scale := math.Min(523/float64(b.Dx()), 770/float64(b.Dy()))
	w, h := float64(b.Dx())*scale, float64(b.Dy())*scale
	content := fmt.Sprintf("q %.3f 0 0 %.3f %.3f %.3f cm /Im0 Do Q\n", w, h, (595-w)/2, (842-h)/2)
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im0 4 0 R >> >> /Contents 5 0 R >>"),
		append(append([]byte(fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n", b.Dx(), b.Dy(), compressed.Len())), compressed.Bytes()...), []byte("\nendstream")...),
		[]byte(fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content)),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(obj)
		out.WriteString("\nendobj\n")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, o := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return os.WriteFile(path, out.Bytes(), 0644)
}
