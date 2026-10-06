package ui

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"pixeluxe/internal/formats"
	"pixeluxe/internal/paint"
)

func prepareDeleteBrowser(t *testing.T, g *Game, path string, brush bool) *Dialog {
	t.Helper()
	g.deleteFileDialog(brush)
	d := g.dialog
	if !g.readDirectory(d, filepath.Dir(path)) {
		t.Fatal(d.err)
	}
	d.fields[1].value = filepath.Base(path)
	d.fields[1].cursor = len([]rune(d.fields[1].value))
	return d
}

func TestDeleteFileRequiresConfirmationAndCancelKeepsBrowser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "picture.iff")
	original := []byte("a file selected for deletion")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	g := New()
	d := prepareDeleteBrowser(t, g, path, false)
	d.accept()
	if g.dialog == d || g.dialog == nil || g.dialog.kind != "confirm" {
		t.Fatal("Delete must request explicit confirmation before removing a file")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatal("opening the deletion confirmation must preserve the file")
	}
	g.cancelRequester(g.dialog)
	if g.dialog != d || d.directory != filepath.Dir(path) || d.fields[1].value != filepath.Base(path) {
		t.Fatal("Cancel must restore the browser, drawer and selected filename")
	}
	data, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatal("Cancel must leave the selected file intact")
	}
}

func TestDeleteBrushConfirmsRemovesAndRefreshesList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "brush.iff")
	if err := os.WriteFile(path, []byte("brush"), 0644); err != nil {
		t.Fatal(err)
	}
	g := New()
	before := paint.CloneImage(g.Canvas.Image)
	d := prepareDeleteBrowser(t, g, path, true)
	if !d.brush || !d.delete || d.title != "Delete brush" || d.buttons[0].label != "Delete" {
		t.Fatal("brush deletion must expose the Delete requester")
	}
	d.accept()
	g.dialog.accept()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("confirmed deletion must remove the selected temporary file")
	}
	if g.dialog != d || d.fields[1].value != "" || d.err != "" {
		t.Fatal("successful deletion must return to the refreshed browser")
	}
	for _, f := range d.files {
		if f.name == filepath.Base(path) {
			t.Fatal("the refreshed browser still lists the deleted file")
		}
	}
	if !bytes.Equal(before.Pix, g.Canvas.Image.Pix) || g.Canvas.Dirty() {
		t.Fatal("deleting a stored brush must not change the open picture")
	}
}

func TestDeleteFailureIsVisibleAndDirectoriesNavigate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.png")
	g := New()
	d := prepareDeleteBrowser(t, g, path, false)
	d.accept()
	if g.dialog != d || d.err == "" {
		t.Fatal("a missing deletion target must report its error in the browser")
	}
	if err := os.WriteFile(path, []byte("temporary picture"), 0644); err != nil {
		t.Fatal(err)
	}
	d.accept()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	g.dialog.accept()
	if g.dialog != d || d.err == "" {
		t.Fatal("a deletion failure after confirmation must restore the browser with an inline error")
	}
	drawer := filepath.Join(dir, "drawer")
	if err := os.Mkdir(drawer, 0755); err != nil {
		t.Fatal(err)
	}
	d.fields[1].value = "drawer"
	d.accept()
	if g.dialog != d || d.directory != drawer {
		t.Fatal("selecting a drawer in Delete must navigate rather than delete it")
	}
	if info, err := os.Stat(drawer); err != nil || !info.IsDir() {
		t.Fatal("drawer navigation must preserve the directory")
	}
}

func TestSaveStencilKeepsDisabledSpatialMaskAndPictureState(t *testing.T) {
	g := New()
	g.Canvas = paint.New(5, 3, paint.DefaultPalette()[:4])
	g.Canvas.Clear(2)
	g.Canvas.MarkSaved()
	g.Filename = "open-picture.iff"
	g.Canvas.Stencil[2] = true
	mask := []bool{false, true, false, false, false, false, false, true, false, false, true, false, false, false, true}
	g.Canvas.StencilMask = append([]bool(nil), mask...)
	g.Canvas.StencilEnabled = false
	before := paint.CloneImage(g.Canvas.Image)
	path := filepath.Join(t.TempDir(), "stencil.iff")
	if err := g.SaveStencil(path); err != nil {
		t.Fatal(err)
	}
	im, meta, err := decodePicture(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(im.Palette) != 2 || meta.Planes != 1 || im.Rect != image.Rect(0, 0, 5, 3) {
		t.Fatal("stencils must save as a two-colour indexed image of the page size")
	}
	for i, protected := range mask {
		if (im.Pix[i] != 0) != protected {
			t.Fatalf("disabled spatial stencil pixel %d was not preserved", i)
		}
	}
	if !bytes.Equal(before.Pix, g.Canvas.Image.Pix) || g.Canvas.Dirty() || g.Filename != "open-picture.iff" || g.Canvas.StencilEnabled {
		t.Fatal("saving a stencil must not alter the artwork, file identity or stencil enabled state")
	}
}

func TestStencilFallbackColorFlagsRoundTripAndEnable(t *testing.T) {
	g := New()
	g.Canvas = paint.New(5, 3, paint.DefaultPalette()[:4])
	g.Canvas.Pixel(1, 1, 2)
	g.Canvas.Pixel(4, 2, 2)
	g.Canvas.Stencil[2] = true
	g.Canvas.StencilEnabled = false
	path := filepath.Join(t.TempDir(), "stencil.png")
	if err := g.SaveStencil(path); err != nil {
		t.Fatal(err)
	}
	g.Canvas.Stencil = [256]bool{}
	if err := g.LoadStencil(path); err != nil {
		t.Fatal(err)
	}
	if !g.Canvas.StencilEnabled || !g.Canvas.Protected(1, 1) || !g.Canvas.Protected(4, 2) || g.Canvas.Protected(0, 0) {
		t.Fatal("loading a stencil must enable the saved protected locations")
	}
	for i, protected := range g.Canvas.StencilMask {
		want := i == 6 || i == 14
		if protected != want {
			t.Fatalf("fallback colour flags did not round-trip at pixel %d", i)
		}
	}
}

func TestStencilLoadRejectsDifferentDimensionsWithoutChangingMask(t *testing.T) {
	g := New()
	g.Canvas = paint.New(5, 3, paint.DefaultPalette()[:4])
	g.Canvas.StencilMask = make([]bool, 15)
	g.Canvas.StencilMask[3] = true
	before := append([]bool(nil), g.Canvas.StencilMask...)
	path := filepath.Join(t.TempDir(), "wrong-size.iff")
	if err := formats.Save(path, paint.New(4, 3, paint.DefaultPalette()[:2]).Image); err != nil {
		t.Fatal(err)
	}
	if err := g.LoadStencil(path); err == nil || !reflect.DeepEqual(g.Canvas.StencilMask, before) || g.Canvas.StencilEnabled {
		t.Fatal("a size mismatch must preserve the current stencil and report an error")
	}
	g.stencilFileDialog(false)
	d := g.dialog
	if !g.readDirectory(d, filepath.Dir(path)) {
		t.Fatal(d.err)
	}
	d.fields[1].value = filepath.Base(path)
	d.accept()
	if g.dialog != d || d.err == "" || !reflect.DeepEqual(g.Canvas.StencilMask, before) {
		t.Fatal("stencil browser errors must remain visible without replacing the current mask")
	}
}

func TestStencilBrowserSavesMaskAndDeletionRequesterIsNamed(t *testing.T) {
	g := New()
	g.Canvas = paint.New(5, 3, paint.DefaultPalette()[:4])
	g.Canvas.StencilMask = make([]bool, 15)
	g.Canvas.StencilMask[7] = true
	g.stencilFileDialog(true)
	d := g.dialog
	if !d.save || !d.stencil || d.title != "Save stencil" || d.fields[1].value != "stencil.iff" {
		t.Fatal("stencil Save must use its own requester and default filename")
	}
	dir := t.TempDir()
	if !g.readDirectory(d, dir) {
		t.Fatal(d.err)
	}
	d.accept()
	im, _, err := decodePicture(filepath.Join(dir, "stencil.iff"))
	if err != nil || im.ColorIndexAt(2, 1) != 1 || g.dialog != nil {
		t.Fatal("stencil browser Save did not write the current spatial mask")
	}
	g.deleteStencilDialog()
	if !g.dialog.delete || !g.dialog.stencil || g.dialog.title != "Delete stencil" || g.dialog.buttons[0].label != "Delete" {
		t.Fatal("stencil deletion must expose an explicit Delete requester")
	}
}
