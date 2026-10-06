package paint

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"reflect"
	"testing"
)

func transformFixture() *Brush {
	img := image.NewPaletted(image.Rect(10, 20, 12, 23), color.Palette{
		color.Black, color.White, color.RGBA{R: 255, A: 255}, color.RGBA{G: 255, A: 255},
		color.RGBA{B: 255, A: 255}, color.RGBA{R: 255, B: 255, A: 255}, color.RGBA{R: 255, G: 255, A: 255},
	})
	copy(img.Pix, []byte{1, 2, 3, 4, 5, 6})
	return &Brush{Image: img, Transparent: 3, Mask: []bool{true, false, true, true, false, true}}
}

func TestRotateRightAngles(t *testing.T) {
	for _, tc := range []struct {
		angle float64
		w, h  int
		pix   []byte
		mask  []bool
	}{
		{0, 2, 3, []byte{1, 2, 3, 4, 5, 6}, []bool{true, false, true, true, false, true}},
		{90, 3, 2, []byte{5, 3, 1, 6, 4, 2}, []bool{false, true, true, true, true, false}},
		{180, 2, 3, []byte{6, 5, 4, 3, 2, 1}, []bool{true, false, true, true, false, true}},
		{270, 3, 2, []byte{2, 4, 6, 1, 3, 5}, []bool{false, true, true, true, true, false}},
		{-90, 3, 2, []byte{2, 4, 6, 1, 3, 5}, []bool{false, true, true, true, true, false}},
	} {
		t.Run(fmt.Sprintf("%g_degrees", tc.angle), func(t *testing.T) {
			b := transformFixture()
			palette := append(color.Palette(nil), b.Image.Palette...)
			b.Rotate(tc.angle)
			if b.Image.Rect != image.Rect(0, 0, tc.w, tc.h) || !reflect.DeepEqual(b.Image.Pix, tc.pix) {
				t.Fatalf("angle %v: image = %v %v", tc.angle, b.Image.Rect, b.Image.Pix)
			}
			if !reflect.DeepEqual(b.Mask, tc.mask) || b.Transparent != 3 {
				t.Fatalf("angle %v: mask or palette changed incorrectly: %v", tc.angle, b.Mask)
			}
			for i, col := range palette {
				r, g, bl, a := col.RGBA()
				r2, g2, bl2, a2 := b.Image.Palette[i].RGBA()
				if r != r2 || g != g2 || bl != bl2 || a != a2 {
					t.Fatalf("palette entry %d changed colour", i)
				}
			}
		})
	}
}

func TestRotateArbitraryAngleBoundsAndTransparency(t *testing.T) {
	b := &Brush{Image: image.NewPaletted(image.Rect(0, 0, 3, 3), color.Palette{color.Black, color.White}), Transparent: 1}
	// Palette index zero must remain opaque even in an uncovered-corner test.
	b.Rotate(45)
	if b.Image.Rect != image.Rect(0, 0, 5, 5) {
		t.Fatalf("bounds = %v, want 5x5", b.Image.Rect)
	}
	if b.Mask[0] || b.Image.Pix[0] != 1 {
		t.Fatalf("uncovered corner must use transparent index with an empty mask")
	}
	if !b.Mask[2*5+2] || b.Image.Pix[2*5+2] != 0 {
		t.Fatalf("centre should preserve opaque palette index zero")
	}
}

func TestShearAndBend(t *testing.T) {
	for _, tc := range []struct {
		name  string
		apply func(*Brush)
		w, h  int
	}{
		{"positive shear", func(b *Brush) { b.Shear(1) }, 5, 3},
		{"negative shear", func(b *Brush) { b.Shear(-1) }, 5, 3},
		{"horizontal bend", func(b *Brush) { b.Bend(2, true) }, 4, 3},
		{"vertical bend", func(b *Brush) { b.Bend(-2, false) }, 2, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := transformFixture()
			tc.apply(b)
			if b.Image.Rect != image.Rect(0, 0, tc.w, tc.h) || len(b.Mask) != tc.w*tc.h {
				t.Fatalf("bounds/mask = %v %v", b.Image.Rect, b.Mask)
			}
			for i, index := range b.Image.Pix {
				if index > 6 {
					t.Fatalf("new palette index %d", index)
				}
				if index == 2 || index == 5 {
					if b.Mask[i] {
						t.Fatalf("transparent source pixel %d became opaque", index)
					}
				}
			}
		})
	}
}

func TestPerspectiveIdentityAndZRotation(t *testing.T) {
	for _, angle := range []float64{0, 90, 180, 270} {
		a, b := transformFixture(), transformFixture()
		a.Rotate(angle)
		if err := b.Perspective(0, 0, angle); err != nil {
			t.Fatal(err)
		}
		if a.Image.Rect != b.Image.Rect || !reflect.DeepEqual(a.Image.Pix, b.Image.Pix) || !reflect.DeepEqual(a.Mask, b.Mask) {
			t.Fatalf("z rotation %v differs from affine rotation", angle)
		}
	}
}

func TestPerspectiveForeshortening(t *testing.T) {
	b := &Brush{Image: image.NewPaletted(image.Rect(0, 0, 20, 12), color.Palette{color.Black, color.White})}
	for i := range b.Image.Pix {
		b.Image.Pix[i] = 1
	}
	if err := b.Perspective(0, 60, 0); err != nil {
		t.Fatal(err)
	}
	if b.Image.Rect.Dx() >= 20 || b.Image.Rect.Dy() < 12 {
		t.Fatalf("expected horizontal foreshortening and depth-dependent height: %v", b.Image.Rect)
	}
	for i, index := range b.Image.Pix {
		if b.Mask[i] && index != 1 {
			t.Fatalf("perspective changed a source index")
		}
	}
}

func TestTransformsRejectUnsafeParametersWithoutMutation(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), math.MaxFloat64} {
		for _, tc := range []struct {
			name  string
			apply func(*Brush)
		}{
			{"shear", func(b *Brush) { b.Shear(value) }},
			{"bend", func(b *Brush) { b.Bend(value, true) }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				b := transformFixture()
				original := b.Image
				tc.apply(b)
				if b.Image != original {
					t.Fatalf("unsafe transform mutated brush")
				}
			})
		}
	}
	for _, angles := range [][3]float64{{90, 0, 0}, {0, 90, 0}, {math.NaN(), 0, 0}, {0, math.Inf(1), 0}} {
		b := transformFixture()
		original := b.Image
		if err := b.Perspective(angles[0], angles[1], angles[2]); !errors.Is(err, ErrInvalidBrushTransform) {
			t.Fatalf("angles %v error = %v", angles, err)
		}
		if b.Image != original {
			t.Fatalf("rejected perspective mutated brush")
		}
	}
	for _, malformed := range []*Brush{
		nil, {}, {Image: &image.Paletted{Rect: image.Rect(0, 0, 8193, 1)}},
		{Image: &image.Paletted{Rect: image.Rect(0, 0, 8000, 8000)}},
		{Image: &image.Paletted{Rect: image.Rect(0, 0, 2, 2), Stride: math.MaxInt, Pix: make([]byte, 4)}},
		{Image: &image.Paletted{Rect: image.Rect(0, 0, 2, 2), Stride: 2, Pix: make([]byte, 3)}},
	} {
		malformed.Rotate(45)
		malformed.Shear(1)
		malformed.Bend(1, false)
		if err := malformed.Perspective(10, 20, 30); err == nil {
			t.Fatal("malformed brush accepted")
		}
	}
}

func TestTransformAllocationLimits(t *testing.T) {
	b := &Brush{Image: image.NewPaletted(image.Rect(0, 0, 8192, 1), color.Palette{color.Black, color.White})}
	original := b.Image
	b.Shear(1)
	if b.Image != original {
		t.Fatal("oversized dimension was allocated")
	}
	b = &Brush{Image: image.NewPaletted(image.Rect(0, 0, 4000, 4000), color.Palette{color.Black, color.White})}
	original = b.Image
	b.Rotate(45)
	if b.Image != original {
		t.Fatal("oversized pixel count was allocated")
	}
}

func TestTransformSubimageWithStride(t *testing.T) {
	parent := image.NewPaletted(image.Rect(0, 0, 5, 4), color.Palette{color.Black, color.White})
	parent.SetColorIndex(3, 2, 1)
	b := &Brush{Image: parent.SubImage(image.Rect(2, 1, 4, 3)).(*image.Paletted)}
	b.Rotate(180)
	if !reflect.DeepEqual(b.Image.Pix, []byte{1, 0, 0, 0}) || !reflect.DeepEqual(b.Mask, []bool{true, false, false, false}) {
		t.Fatalf("subimage with larger stride transformed incorrectly: %v %v", b.Image.Pix, b.Mask)
	}
}

func TestFiniteHugeAnglesRemainSafe(t *testing.T) {
	b := transformFixture()
	b.Rotate(math.MaxFloat64)
	if b.Image.Rect.Dx() > 5 || b.Image.Rect.Dy() > 5 {
		t.Fatalf("angle normalization produced excessive bounds: %v", b.Image.Rect)
	}
	if err := b.Perspective(math.MaxFloat64, -math.MaxFloat64, math.MaxFloat64); err != nil && !errors.Is(err, ErrInvalidBrushTransform) {
		t.Fatalf("large finite angle failed unexpectedly: %v", err)
	}
}
