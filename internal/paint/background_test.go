package paint

import "testing"

// Restoration can keep the very same index used by the fill's connectivity
// test. Visiting locations explicitly must prevent an endless flood fill.
func TestFloodFillRestoresBackgroundWithUnchangedPixels(t *testing.T) {
	c := New(12, 8, DefaultPalette())
	c.Clear(1)
	background := New(12, 8, c.Image.Palette)
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			background.Pixel(x, y, uint8(1+(x+y)%2))
		}
	}
	c.RestoreImage = background.Image
	c.Erase = true
	c.Begin()
	c.FloodFill(3, 4, 0)
	c.Commit()
	for i, v := range c.Image.Pix {
		if v != background.Image.Pix[i] {
			t.Fatalf("pixel %d restored to %d, want %d", i, v, background.Image.Pix[i])
		}
	}
	if !c.Undo() {
		t.Fatal("background restore must be undoable")
	}
	for _, v := range c.Image.Pix {
		if v != 1 {
			t.Fatal("undo lost foreground pixels")
		}
	}
}
