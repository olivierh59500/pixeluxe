package paint

import (
	"errors"
	"image"
	"math"
)

const (
	maxTransformDimension = 8192
	maxTransformPixels    = 16_000_000
)

var (
	ErrInvalidBrushTransform  = errors.New("invalid brush transform")
	ErrBrushTransformTooLarge = errors.New("brush transform exceeds 8192 pixels per side or 16 million pixels")
)

type brushPoint struct{ x, y float64 }

func finiteTransform(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// transformSize validates the source before geometry or allocation. The Pix
// check also covers subimages, whose stride can exceed their visible width.
func (b *Brush) transformSize() (int, int, error) {
	if b == nil || b.Image == nil {
		return 0, 0, ErrInvalidBrushTransform
	}
	w, h := b.Image.Rect.Dx(), b.Image.Rect.Dy()
	if w < 1 || h < 1 {
		return 0, 0, ErrInvalidBrushTransform
	}
	if w > maxTransformDimension || h > maxTransformDimension || w > maxTransformPixels/h {
		return 0, 0, ErrBrushTransformTooLarge
	}
	if b.Image.Stride < w || len(b.Image.Pix) < w || (h > 1 && b.Image.Stride > (len(b.Image.Pix)-w)/(h-1)) {
		return 0, 0, ErrInvalidBrushTransform
	}
	return w, h, nil
}

func brushBounds(points [4]brushPoint) (brushPoint, brushPoint) {
	lo, hi := points[0], points[0]
	for _, p := range points[1:] {
		lo.x, lo.y = math.Min(lo.x, p.x), math.Min(lo.y, p.y)
		hi.x, hi.y = math.Max(hi.x, p.x), math.Max(hi.y, p.y)
	}
	return lo, hi
}

// resampleBrush takes pixel-edge bounds and maps destination pixel centres
// back to source pixel-edge coordinates. A nearest neighbour is therefore
// floor(source), and uncovered destination pixels remain transparent.
func (b *Brush) resampleBrush(lo, hi brushPoint, inverse func(x, y float64) (float64, float64)) error {
	sw, sh, err := b.transformSize()
	if err != nil {
		return err
	}
	spanX, spanY := hi.x-lo.x, hi.y-lo.y
	if !finiteTransform(lo.x) || !finiteTransform(lo.y) || !finiteTransform(hi.x) || !finiteTransform(hi.y) || spanX <= 0 || spanY <= 0 {
		return ErrInvalidBrushTransform
	}
	// Remove only floating point noise at integer bounds (not an actual pixel).
	wf, hf := math.Ceil(spanX-1e-9), math.Ceil(spanY-1e-9)
	if wf < 1 || hf < 1 {
		return ErrInvalidBrushTransform
	}
	if wf > maxTransformDimension || hf > maxTransformDimension || wf*hf > maxTransformPixels {
		return ErrBrushTransformTooLarge
	}
	w, h := int(wf), int(hf)
	dst := image.NewPaletted(image.Rect(0, 0, w, h), clonePalette(b.Image.Palette))
	mask := make([]bool, w*h)
	for i := range dst.Pix {
		dst.Pix[i] = b.Transparent
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx, sy := inverse(lo.x+float64(x)+0.5, lo.y+float64(y)+0.5)
			if !finiteTransform(sx) || !finiteTransform(sy) || sx < 0 || sy < 0 || sx >= float64(sw) || sy >= float64(sh) {
				continue
			}
			px, py := int(math.Floor(sx))+b.Image.Rect.Min.X, int(math.Floor(sy))+b.Image.Rect.Min.Y
			dst.Pix[y*w+x] = b.Image.ColorIndexAt(px, py)
			mask[y*w+x] = b.opaque(px, py)
		}
	}
	// Commit only after the complete transform succeeds.
	b.Image, b.Mask = dst, mask
	return nil
}

func transformRadians(degrees float64) (float64, bool) {
	if !finiteTransform(degrees) {
		return 0, false
	}
	return math.Mod(degrees, 360) * math.Pi / 180, true
}

// Rotate rotates clockwise around the centre of the brush. Invalid parameters
// or transformations exceeding the allocation limits leave the brush intact.
func (b *Brush) Rotate(angleDegrees float64) {
	w, h, err := b.transformSize()
	angle, ok := transformRadians(angleDegrees)
	if err != nil || !ok {
		return
	}
	s, c := math.Sincos(angle)
	cx, cy := float64(w)/2, float64(h)/2
	points := [4]brushPoint{{-cx, -cy}, {cx, -cy}, {cx, cy}, {-cx, cy}}
	for i, p := range points {
		points[i] = brushPoint{c*p.x - s*p.y, s*p.x + c*p.y}
	}
	lo, hi := brushBounds(points)
	_ = b.resampleBrush(lo, hi, func(x, y float64) (float64, float64) {
		return c*x + s*y + cx, -s*x + c*y + cy
	})
}

// Shear displaces x by amount*y relative to the brush centre. amount is a
// dimensionless slope: 1 moves the bottom edge one brush-height to the right
// relative to the top edge.
func (b *Brush) Shear(amount float64) {
	w, h, err := b.transformSize()
	if err != nil || !finiteTransform(amount) {
		return
	}
	cx, cy := float64(w)/2, float64(h)/2
	points := [4]brushPoint{{-cx, -cy}, {cx, -cy}, {cx, cy}, {-cx, cy}}
	for i, p := range points {
		points[i].x = p.x + amount*p.y
	}
	lo, hi := brushBounds(points)
	_ = b.resampleBrush(lo, hi, func(x, y float64) (float64, float64) {
		return x - amount*y + cx, y + cy
	})
}

// Bend displaces pixels by amount*sin(pi*position/extent). The endpoints stay
// fixed and the centre moves by amount pixels. horizontal bends x as a
// function of y; false bends y as a function of x.
func (b *Brush) Bend(amount float64, horizontal bool) {
	w, h, err := b.transformSize()
	if err != nil || !finiteTransform(amount) {
		return
	}
	lo, hi := brushPoint{}, brushPoint{float64(w), float64(h)}
	if horizontal {
		lo.x, hi.x = math.Min(0, amount), float64(w)+math.Max(0, amount)
	} else {
		lo.y, hi.y = math.Min(0, amount), float64(h)+math.Max(0, amount)
	}
	_ = b.resampleBrush(lo, hi, func(x, y float64) (float64, float64) {
		if horizontal {
			return x - amount*math.Sin(math.Pi*y/float64(h)), y
		}
		return x, y - amount*math.Sin(math.Pi*x/float64(w))
	})
}

// Perspective rotates the brush plane around its centre about x, then y,
// then z. Angles are degrees; positive z is clockwise on screen. A camera
// distance of twice the largest side keeps all four corners in front of the
// camera. The projection is inverted analytically to preserve palette indices
// and masks, without holes between forward-projected pixels.
func (b *Brush) Perspective(rx, ry, rz float64) error {
	w, h, err := b.transformSize()
	if err != nil {
		return err
	}
	xAngle, xOK := transformRadians(rx)
	yAngle, yOK := transformRadians(ry)
	zAngle, zOK := transformRadians(rz)
	if !xOK || !yOK || !zOK {
		return ErrInvalidBrushTransform
	}
	sx, cx := math.Sincos(xAngle)
	sy, cy := math.Sincos(yAngle)
	sz, cz := math.Sincos(zAngle)
	// The first two columns of Rz * Ry * Rx map a plane point (x,y,0).
	u := [3]float64{cz * cy, sz * cy, -sy}
	v := [3]float64{cz*sy*sx - sz*cx, sz*sy*sx + cz*cx, cy * sx}
	// An edge-on plane has no invertible projected area.
	if math.Abs(cx*cy) < 1e-6 {
		return ErrInvalidBrushTransform
	}
	distance := 2 * float64(max(w, h))
	halfW, halfH := float64(w)/2, float64(h)/2
	points := [4]brushPoint{{-halfW, -halfH}, {halfW, -halfH}, {halfW, halfH}, {-halfW, halfH}}
	for i, p := range points {
		depth := distance + u[2]*p.x + v[2]*p.y
		if !finiteTransform(depth) || depth <= 0 {
			return ErrInvalidBrushTransform
		}
		points[i] = brushPoint{
			distance * (u[0]*p.x + v[0]*p.y) / depth,
			distance * (u[1]*p.x + v[1]*p.y) / depth,
		}
	}
	lo, hi := brushBounds(points)
	return b.resampleBrush(lo, hi, func(x, y float64) (float64, float64) {
		a, bb := distance*u[0]-x*u[2], distance*v[0]-x*v[2]
		c, d := distance*u[1]-y*u[2], distance*v[1]-y*v[2]
		det := a*d - bb*c
		if math.Abs(det) < 1e-12 {
			return math.NaN(), math.NaN()
		}
		return distance*(x*d-bb*y)/det + halfW, distance*(a*y-x*c)/det + halfH
	})
}
