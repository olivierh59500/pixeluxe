package ui

import (
	"bytes"
	"image/color"
	"reflect"
	"testing"

	"pixeluxe/internal/formats"
	"pixeluxe/internal/paint"
)

func cyclingTestGame() *Game {
	p := make(color.Palette, 16)
	for i := range p {
		p[i] = color.RGBA{uint8(17 * i), uint8(255 - 17*i), 85, 255}
	}
	g := New()
	g.Canvas = paint.New(8, 4, p)
	for i := range g.Canvas.Image.Pix {
		g.Canvas.Image.Pix[i] = uint8(i % len(p))
	}
	g.Canvas.MarkSaved()
	g.Cycle, g.MultiCycle = true, true
	g.CycleLow, g.CycleHigh, g.CycleSpeed = 2, 5, 2
	return g
}

func assertCyclePalette(t *testing.T, got, base color.Palette, expected []int) {
	t.Helper()
	if len(got) != len(expected) {
		t.Fatalf("display palette size: got %d, want %d", len(got), len(expected))
	}
	for i, source := range expected {
		gr, gg, gb, ga := got[i].RGBA()
		wr, wg, wb, wa := base[source].RGBA()
		if gr != wr || gg != wg || gb != wb || ga != wa {
			t.Fatalf("display colour %d must use base colour %d", i, source)
		}
	}
}

func cycleIdentity(count int) []int {
	indices := make([]int, count)
	for i := range indices {
		indices[i] = i
	}
	return indices
}

func TestCyclingIndependentRangesRatesAndReversePreservePicture(t *testing.T) {
	g := cyclingTestGame()
	g.FileMetadata = &formats.Metadata{ColorRanges: []formats.ColorRange{
		{Rate: 16384, Flags: 0, Low: 0, High: 1},
		{Rate: 8192, Flags: 1, Low: 2, High: 5},
		{Rate: 4096, Flags: 3, Low: 8, High: 11},
	}}
	original := paint.CloneImage(g.Canvas.Image)
	for _, tick := range []int{4, 7} {
		g.cycleTick = tick
		expected := cycleIdentity(16)
		if tick == 4 {
			copy(expected[2:6], []int{4, 5, 2, 3})
		} else {
			copy(expected[2:6], []int{5, 2, 3, 4})
		}
		copy(expected[8:12], []int{11, 8, 9, 10})
		assertCyclePalette(t, g.displayPalette(g.Canvas.Image.Palette), original.Palette, expected)
	}
	assertCyclePalette(t, g.Canvas.Image.Palette, original.Palette, cycleIdentity(16))
	if !bytes.Equal(g.Canvas.Image.Pix, original.Pix) || g.Canvas.Dirty() || g.Canvas.Undo() {
		t.Fatal("display cycling must preserve picture indices, editable palette and history")
	}
	if !reflect.DeepEqual(g.FileMetadata.ColorRanges[1], formats.ColorRange{Rate: 8192, Flags: 1, Low: 2, High: 5}) {
		t.Fatal("display cycling must preserve the original CRNG metadata")
	}
}

func TestCyclingSingleRangeUsesFirstActiveAfterInvalidAndDisabled(t *testing.T) {
	g := cyclingTestGame()
	g.MultiCycle, g.cycleTick = false, 4
	g.FileMetadata = &formats.Metadata{ColorRanges: []formats.ColorRange{
		{Rate: 8192, Flags: 1, Low: 20, High: 23},
		{Rate: 8192, Flags: 0, Low: 0, High: 1},
		{Rate: 36, Flags: 1, Low: 6, High: 7},
		{Rate: 8192, Flags: 1, Low: 2, High: 5},
		{Rate: 4096, Flags: 3, Low: 8, High: 11},
	}}
	expected := cycleIdentity(16)
	copy(expected[2:6], []int{4, 5, 2, 3})
	assertCyclePalette(t, g.displayPalette(g.Canvas.Image.Palette), g.Canvas.Image.Palette, expected)
}

func TestCyclingOffAndZeroTimeKeepBasePaletteIndependent(t *testing.T) {
	g := cyclingTestGame()
	g.FileMetadata = &formats.Metadata{ColorRanges: []formats.ColorRange{{Rate: 8192, Flags: 1, Low: 2, High: 5}}}
	g.cycleTick = 0
	assertCyclePalette(t, g.displayPalette(g.Canvas.Image.Palette), g.Canvas.Image.Palette, cycleIdentity(16))
	g.cycleTick, g.Cycle = 5, false
	out := g.displayPalette(g.Canvas.Image.Palette)
	assertCyclePalette(t, out, g.Canvas.Image.Palette, cycleIdentity(16))
	r, gg, b, a := g.Canvas.Image.Palette[0].RGBA()
	out[0] = color.White
	ar, ag, ab, aa := g.Canvas.Image.Palette[0].RGBA()
	if ar != r || ag != gg || ab != b || aa != a {
		t.Fatal("the returned display palette must not alias the editable palette slice")
	}
}

func TestCyclingFallbackUsesConfiguredRangeAndWraps(t *testing.T) {
	g := cyclingTestGame()
	g.FileMetadata = nil
	for _, tick := range []int{1, 2, 8, 10} {
		g.cycleTick = tick
		expected := cycleIdentity(16)
		switch tick {
		case 2, 10:
			copy(expected[2:6], []int{3, 4, 5, 2})
		}
		assertCyclePalette(t, g.displayPalette(g.Canvas.Image.Palette), g.Canvas.Image.Palette, expected)
	}
}

func TestCyclingCustomRangePreservesOtherOriginalRanges(t *testing.T) {
	g := cyclingTestGame()
	g.FileMetadata = &formats.Metadata{ColorRanges: []formats.ColorRange{
		{Rate: 8192, Flags: 3, Low: 2, High: 5},
		{Rate: 4096, Flags: 3, Low: 8, High: 11},
	}}
	g.RangeChanged, g.MultiCycle = true, true
	g.CycleLow, g.CycleHigh, g.CycleSpeed, g.cycleTick = 0, 3, 4, 4
	expected := cycleIdentity(16)
	copy(expected[0:4], []int{1, 2, 3, 0})
	copy(expected[8:12], []int{11, 8, 9, 10})
	assertCyclePalette(t, g.displayPalette(g.Canvas.Image.Palette), g.Canvas.Image.Palette, expected)
	if g.FileMetadata.ColorRanges[0].Low != 2 || g.FileMetadata.ColorRanges[0].Flags != 3 {
		t.Fatal("temporary custom cycling must not overwrite the original metadata before Save")
	}
}

func TestUnsavedRangeRequiresConfirmationWithoutPixelChanges(t *testing.T) {
	g := New()
	g.RangeChanged = true
	continued := false
	g.confirmUnsaved(func() { continued = true })
	if continued || g.dialog == nil || g.dialog.kind != "confirm" || g.Canvas.Dirty() {
		t.Fatal("changed CRNG settings must require confirmation even when picture pixels are clean")
	}
	g.cancelRequester(g.dialog)
	if continued || g.dialog != nil || !g.RangeChanged {
		t.Fatal("Cancel must preserve the range changes and abort the requested replacement")
	}
	g.confirmUnsaved(func() { continued = true })
	g.dialog.accept()
	if !continued || g.dialog != nil {
		t.Fatal("confirming discard must continue the requested operation")
	}
}
