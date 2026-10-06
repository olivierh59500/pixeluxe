package ui

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"pixeluxe/internal/formats"
	"pixeluxe/internal/paint"
)

func TestPaletteRequesterCancelAndCommit(t *testing.T) {
	g := New()
	g.FG = 2
	original := paint.CloneImage(g.Canvas.Image)
	g.paletteDialog()
	g.Canvas.Image.Palette[2] = color.RGBA{17, 34, 51, 255}
	g.FG = 3
	g.cancelRequester(g.dialog)
	if g.FG != 2 || g.Canvas.Dirty() {
		t.Fatal("cancelling the palette requester must restore palette and foreground")
	}
	if got, want := g.Canvas.Image.Palette[2], original.Palette[2]; got != want {
		gr, gg, gb, ga := got.RGBA()
		wr, wg, wb, wa := want.RGBA()
		if gr != wr || gg != wg || gb != wb || ga != wa {
			t.Fatal("cancelled palette colour was not restored")
		}
	}
	g.paletteDialog()
	g.Canvas.Image.Palette[2] = color.RGBA{17, 34, 51, 255}
	g.dialog.accept()
	if g.dialog != nil || !g.Canvas.Dirty() || !g.Canvas.Undo() || g.Canvas.Dirty() {
		t.Fatal("a confirmed palette change must be one undoable action")
	}
}

func TestSaveOverwriteReturnsToBrowserOnCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.png")
	original := paint.New(4, 4, nil)
	original.Clear(4)
	if err := formats.Save(path, original.Image); err != nil {
		t.Fatal(err)
	}
	g := New()
	g.Canvas = paint.New(4, 4, nil)
	g.Canvas.Clear(2)
	g.fileDialog(true, false)
	browser := g.dialog
	if !g.readDirectory(browser, dir) {
		t.Fatal(browser.err)
	}
	browser.fields[1].value = "existing.png"
	browser.accept()
	if g.dialog == browser || g.dialog.kind != "confirm" {
		t.Fatal("saving an existing file must ask before replacing it")
	}
	g.cancelRequester(g.dialog)
	if g.dialog != browser || browser.fields[1].value != "existing.png" || browser.directory != dir {
		t.Fatal("cancelling replacement must preserve the file requester")
	}
	unmodified, err := formats.Load(path)
	if err != nil || unmodified.ColorIndexAt(0, 0) != 4 {
		t.Fatal("cancelling replacement must preserve the existing picture")
	}
	browser.accept()
	g.dialog.accept()
	updated, err := formats.Load(path)
	if err != nil || updated.ColorIndexAt(0, 0) != 2 || g.dialog != nil || g.Canvas.Dirty() {
		t.Fatal("confirming replacement must save the current picture and close the requester")
	}
}

func TestFileRequesterKeepsErrorsAndFiltersEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "drawer"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"broken.iff", "ignored.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("invalid picture"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	g := New()
	g.fileDialog(false, false)
	d := g.dialog
	if !g.readDirectory(d, dir) {
		t.Fatal(d.err)
	}
	if len(d.files) != 3 || d.files[0].name != ".." || d.files[1].name != "drawer" || d.files[2].name != "broken.iff" {
		t.Fatalf("browser must show the parent, sorted drawers and supported files: %v", d.files)
	}
	d.fields[1].value = "broken.iff"
	d.accept()
	if g.dialog != d || d.err == "" {
		t.Fatal("load errors must be visible without dismissing the requester")
	}
	previousDirectory := d.directory
	if g.readDirectory(d, filepath.Join(dir, "missing")) || d.directory != previousDirectory || d.err == "" {
		t.Fatal("an invalid drawer must preserve the previous listing and report an error")
	}
}

func TestNewPictureIsUndoableAndValidatesDimensions(t *testing.T) {
	g := New()
	g.Canvas.Pixel(2, 3, 2)
	g.Canvas.MarkSaved()
	g.settingsDialog("new")
	d := g.dialog
	d.fields[0].value, d.fields[1].value, d.fields[2].value = "8", "6", "3"
	d.accept()
	if g.dialog != d || d.err == "" || g.Canvas.Image.Rect != image.Rect(0, 0, 320, 256) {
		t.Fatal("invalid palette sizes must leave the original canvas untouched")
	}
	d.fields[2].value = "4"
	d.accept()
	if g.dialog != nil || g.Canvas.Image.Rect != image.Rect(0, 0, 8, 6) || len(g.Canvas.Image.Palette) != 4 {
		t.Fatal("new requester must create the requested picture")
	}
	if !g.Canvas.Undo() || g.Canvas.Image.Rect != image.Rect(0, 0, 320, 256) || g.Canvas.Image.ColorIndexAt(2, 3) != 2 {
		t.Fatal("new picture dimensions and colours must participate in history")
	}
}

func TestBrushImportAndExportPreserveAlphaMask(t *testing.T) {
	src := image.NewPaletted(image.Rect(0, 0, 3, 1), color.Palette{color.Black, color.White, color.RGBA{}})
	src.Pix = []uint8{0, 2, 1}
	b := importedBrush(src, paint.DefaultPalette(), 0)
	if !b.Mask[0] || b.Mask[1] || !b.Mask[2] {
		t.Fatal("alpha transparency must preserve opaque black when alpha is a different palette entry")
	}
	im := brushExportImage(b)
	_, _, _, alpha := im.At(1, 0).RGBA()
	_, _, _, blackAlpha := im.At(0, 0).RGBA()
	if alpha != 0 || blackAlpha != 65535 {
		t.Fatal("brush export must encode the opacity mask without changing opaque black")
	}
	path := filepath.Join(t.TempDir(), "brush.iff")
	if err := formats.Save(path, im); err != nil {
		t.Fatal(err)
	}
	loaded, err := formats.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, alpha = loaded.At(1, 0).RGBA()
	if alpha != 0 {
		t.Fatal("IFF brush export must preserve its transparent pixels")
	}
}

func TestStencilRequesterCancelRestoresSettings(t *testing.T) {
	g := New()
	g.Canvas.Stencil[2] = true
	g.stencilDialog()
	g.Canvas.Stencil[2] = false
	g.Canvas.Stencil[3] = true
	g.Canvas.StencilEnabled = true
	g.cancelRequester(g.dialog)
	if !g.Canvas.Stencil[2] || g.Canvas.Stencil[3] || g.Canvas.StencilEnabled {
		t.Fatal("cancel must restore the previous stencil and its enabled state")
	}
}
