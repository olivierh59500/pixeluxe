package ui

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"pixeluxe/internal/formats"
	"pixeluxe/internal/paint"
)

func writeMetadataPicture(t *testing.T, path string, im *image.Paletted, options formats.ILBMOptions) {
	t.Helper()
	var data bytes.Buffer
	if err := formats.EncodeILBMWithOptions(&data, im, options); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func metadataFixture() formats.ILBMOptions {
	hotspot := image.Pt(9, 8)
	return formats.ILBMOptions{
		Compressed: true, XAspect: 10, YAspect: 11, Hotspot: &hotspot,
		ColorRanges: []formats.ColorRange{
			{Rate: 0, Flags: 0, Low: 0, High: 1},
			{Rate: 2048, Flags: 3, Low: 2, High: 7},
			{Rate: 1024, Flags: 1, Low: 8, High: 15},
			{Rate: 36, Flags: 1, Low: 16, High: 23},
		},
	}
}

func TestPictureLoadSavePreservesMultipleRangesAspectAndHotspot(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "source.iff"), filepath.Join(dir, "roundtrip.iff")
	im := paint.New(24, 16, nil)
	im.Pixel(8, 4, 7)
	options := metadataFixture()
	writeMetadataPicture(t, src, im.Image, options)
	g := New()
	if err := g.Load(src); err != nil {
		t.Fatal(err)
	}
	if g.FileMetadata == nil || g.RangeChanged || !g.MultiCycle || g.CycleLow != 2 || g.CycleHigh != 7 || g.CycleSpeed != 8 {
		t.Fatal("loading must retain ranges and select the first active range")
	}
	g.Canvas.Begin()
	g.Canvas.Pixel(9, 5, 8)
	g.Canvas.Commit()
	if err := g.Save(dst); err != nil {
		t.Fatal(err)
	}
	loaded, meta, err := decodePicture(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(meta.ColorRanges, options.ColorRanges) || meta.XAspect != 10 || meta.YAspect != 11 || !meta.HasHotspot || meta.Hotspot != *options.Hotspot {
		t.Fatalf("IFF metadata did not survive editing and saving: %+v", meta)
	}
	if loaded.ColorIndexAt(8, 4) != 7 || loaded.ColorIndexAt(9, 5) != 8 || g.Canvas.Dirty() {
		t.Fatal("saving must preserve picture indices and mark the picture saved")
	}
}

func TestEditedRangeReplacesOnlyFirstActiveRange(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "source.iff"), filepath.Join(dir, "edited.iff")
	options := metadataFixture()
	writeMetadataPicture(t, src, paint.New(16, 16, nil).Image, options)
	g := New()
	if err := g.Load(src); err != nil {
		t.Fatal(err)
	}
	g.settingsDialog("range")
	g.dialog.fields[0].value, g.dialog.fields[1].value, g.dialog.fields[2].value = "3", "6", "4"
	g.dialog.accept()
	if !g.RangeChanged || g.MultiCycle {
		t.Fatal("editing the range must select explicit single-range settings")
	}
	if err := g.Save(dst); err != nil {
		t.Fatal(err)
	}
	_, meta, err := decodePicture(dst)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]formats.ColorRange(nil), options.ColorRanges...)
	want[1] = formats.ColorRange{Rate: 4096, Flags: 1, Low: 3, High: 6}
	if !reflect.DeepEqual(meta.ColorRanges, want) || g.RangeChanged {
		t.Fatalf("range update affected unrelated ranges: got %v, want %v", meta.ColorRanges, want)
	}
	if err := g.Save(filepath.Join(dir, "saved-again.iff")); err != nil {
		t.Fatal(err)
	}
}

func TestSaveFiltersRangesOutsideReducedPalette(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.iff")
	options := metadataFixture()
	writeMetadataPicture(t, src, paint.New(16, 16, nil).Image, options)
	g := New()
	if err := g.Load(src); err != nil {
		t.Fatal(err)
	}
	g.resizePicture(16, 16, 8)
	dst := filepath.Join(dir, "eight-colors.iff")
	if err := g.Save(dst); err != nil {
		t.Fatal(err)
	}
	_, meta, err := decodePicture(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(meta.ColorRanges, options.ColorRanges[:2]) {
		t.Fatalf("ranges beyond the new palette must be omitted: %v", meta.ColorRanges)
	}
}

func TestSaveFailureKeepsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.iff")
	original := []byte("keep the existing file on encoding failure")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	g := New()
	g.Canvas.Image.Palette[2] = color.NRGBA{R: 255, A: 128}
	if err := g.Save(path); err == nil {
		t.Fatal("IFF saving a translucent palette must fail")
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, original) {
		t.Fatal("an encoding failure must leave the original destination intact")
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 {
		t.Fatal("failed saves must clean up their temporary file")
	}
}

func TestPageSizeRetainsPixelsPaletteAndUndo(t *testing.T) {
	g := New()
	g.Canvas = paint.New(4, 3, color.Palette{color.Black, color.White, color.RGBA{17, 34, 51, 255}})
	g.Canvas.Pixel(3, 2, 2)
	g.Canvas.MarkSaved()
	g.FileMetadata = &formats.Metadata{XAspect: 10, YAspect: 11}
	g.Filename = "picture.iff"
	g.settingsDialog("page-size")
	d := g.dialog
	d.fields[0].value, d.fields[1].value, d.fields[2].value = "8", "6", "3"
	d.accept()
	if g.dialog != nil || g.Canvas.Image.Rect != image.Rect(0, 0, 8, 6) || g.Canvas.Image.ColorIndexAt(3, 2) != 2 || len(g.Canvas.Image.Palette) != 3 {
		t.Fatal("page resizing must preserve upper-left artwork and the requested palette")
	}
	r, gg, b, _ := g.Canvas.Image.Palette[2].RGBA()
	if r != 17*257 || gg != 34*257 || b != 51*257 || g.FileMetadata == nil || g.Filename != "picture.iff" {
		t.Fatal("page resizing must preserve colours and file identity")
	}
	if !g.Canvas.Dirty() || !g.Canvas.Undo() || g.Canvas.Image.Rect != image.Rect(0, 0, 4, 3) || g.Canvas.Image.ColorIndexAt(3, 2) != 2 || g.Canvas.Dirty() {
		t.Fatal("page resizing must be undoable back to the saved original")
	}
}

func TestInstallNormalizesOriginAndOwnsMetadata(t *testing.T) {
	g := New()
	src := image.NewPaletted(image.Rect(5, 7, 9, 10), paint.DefaultPalette())
	src.SetColorIndex(8, 9, 2)
	meta := &formats.Metadata{ColorRanges: []formats.ColorRange{{Rate: 2048, Flags: 1, Low: 1, High: 3}}}
	g.installWithMetadata(src, "offset.iff", meta)
	meta.ColorRanges[0].High = 7
	src.SetColorIndex(8, 9, 3)
	if g.Canvas.Image.Rect != image.Rect(0, 0, 4, 3) || g.Canvas.Image.ColorIndexAt(3, 2) != 2 || g.FileMetadata.ColorRanges[0].High != 3 {
		t.Fatal("install must normalize coordinates and own image and metadata storage")
	}
}

func TestSaveBrushPreservesPaletteMaskAndHandle(t *testing.T) {
	src := image.NewPaletted(image.Rect(0, 0, 5, 3), color.Palette{color.Black, color.RGBA{17, 34, 51, 255}, color.White})
	src.SetColorIndex(1, 1, 1)
	g := New()
	g.Brush = importedBrush(src, g.Canvas.Image.Palette, 0)
	r, gg, b, _ := g.Brush.Image.Palette[1].RGBA()
	if len(g.Brush.Image.Palette) != 3 || r != 17*257 || gg != 34*257 || b != 51*257 {
		t.Fatal("a loaded brush must retain its original palette")
	}
	for _, handles := range []int{0, 1} {
		g.Handles = handles
		path := filepath.Join(t.TempDir(), "brush.iff")
		if err := g.SaveBrush(path); err != nil {
			t.Fatal(err)
		}
		im, meta, err := decodePicture(path)
		if err != nil {
			t.Fatal(err)
		}
		want := image.Pt(2, 1)
		if handles == 1 {
			want = image.Point{}
		}
		_, _, _, clearAlpha := im.At(0, 0).RGBA()
		_, _, _, opaqueAlpha := im.At(1, 1).RGBA()
		if !meta.HasHotspot || meta.Hotspot != want || clearAlpha != 0 || opaqueAlpha != 65535 {
			t.Fatalf("brush mask and handle must survive save: %+v", meta)
		}
	}
}
