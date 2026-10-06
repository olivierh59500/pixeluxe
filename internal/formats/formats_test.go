package formats

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testPalette(n int) color.Palette {
	p := make(color.Palette, n)
	for i := range p {
		p[i] = color.NRGBA{R: uint8(i * 37), G: uint8(i * 61), B: uint8(i * 103), A: 255}
	}
	return p
}

func assertSameImage(t *testing.T, got, want *image.Paletted) {
	t.Helper()
	if got.Bounds().Size() != want.Bounds().Size() {
		t.Fatalf("size: got %v want %v", got.Bounds(), want.Bounds())
	}
	for y := 0; y < want.Rect.Dy(); y++ {
		for x := 0; x < want.Rect.Dx(); x++ {
			g := color.NRGBAModel.Convert(got.At(got.Rect.Min.X+x, got.Rect.Min.Y+y)).(color.NRGBA)
			w := color.NRGBAModel.Convert(want.At(want.Rect.Min.X+x, want.Rect.Min.Y+y)).(color.NRGBA)
			if g.A == 0 && w.A == 0 {
				continue
			}
			if g != w {
				t.Fatalf("pixel (%d,%d): got %#v want %#v", x, y, g, w)
			}
		}
	}
}

func TestILBMRoundTripEveryDepth(t *testing.T) {
	for depth := 1; depth <= 8; depth++ {
		for _, compressed := range []bool{false, true} {
			name := string(rune('0'+depth)) + map[bool]string{false: "/plain", true: "/ByteRun1"}[compressed]
			t.Run(name, func(t *testing.T) {
				palette := testPalette(1 << depth)
				// Nonzero bounds and an odd width catch stride, origin and padding errors.
				img := image.NewPaletted(image.Rect(7, 11, 44, 28), palette)
				for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
					for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
						img.SetColorIndex(x, y, uint8((x*17+y*53)%len(palette)))
					}
				}
				var data bytes.Buffer
				if err := EncodeILBMWithOptions(&data, img, ILBMOptions{Compressed: compressed}); err != nil {
					t.Fatal(err)
				}
				got, meta, err := DecodeILBM(bytes.NewReader(data.Bytes()))
				if err != nil {
					t.Fatal(err)
				}
				if meta.Planes != uint8(depth) || meta.Compression != map[bool]uint8{false: 0, true: 1}[compressed] {
					t.Fatalf("incorrect metadata: %+v", meta)
				}
				assertSameImage(t, got, img)
				if len(got.Palette) != len(img.Palette) {
					t.Fatal("palette length changed")
				}
			})
		}
	}
}

// This fixture is independent of the encoder: plane 0 precedes plane 1,
// words are padded to 16 pixels, and pixels use the most significant bit first.
func TestILBMKnownPlanarFixture(t *testing.T) {
	bmhd := header(9, 2, 2, 0, 0)
	body := []byte{0x55, 0x80, 0x33, 0x00, 0xaa, 0x00, 0xcc, 0x80}
	data := iffFixture("ILBM", "BMHD", bmhd, "JUNK", []byte{7}, "CMAP", []byte{0, 0, 0, 255, 0, 0, 0, 255, 0, 0, 0, 255}, "BODY", body)
	got, _, err := DecodeILBM(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 1, 2, 3, 0, 1, 2, 3, 1, 3, 2, 1, 0, 3, 2, 1, 0, 2}
	if !bytes.Equal(got.Pix, want) {
		t.Fatalf("wrong plane order or bit order: got %v want %v", got.Pix, want)
	}
}

func TestTransparentColorAndMaskRoundTrip(t *testing.T) {
	for _, indices := range [][]int{{0}, {0, 3}} {
		palette := testPalette(5)
		for _, i := range indices {
			c := palette[i].(color.NRGBA)
			c.A = 0
			palette[i] = c
		}
		img := image.NewPaletted(image.Rect(0, 0, 19, 3), palette)
		for i := range img.Pix {
			img.Pix[i] = uint8(i % len(palette))
		}
		var data bytes.Buffer
		if err := EncodeILBM(&data, img); err != nil {
			t.Fatal(err)
		}
		got, meta, err := DecodeILBM(bytes.NewReader(data.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		wantMask := uint8(2)
		if len(indices) > 1 {
			wantMask = 1
		}
		if meta.Masking != wantMask {
			t.Fatalf("got masking %d want %d", meta.Masking, wantMask)
		}
		assertSameImage(t, got, img)
	}
}

func TestEightPlaneMaskReusesUnusedColor(t *testing.T) {
	bmhd := header(2, 1, 8, 1, 0)
	body := make([]byte, 18)
	body[0] = 0x40  // second pixel is index 1
	body[16] = 0x80 // first pixel opaque, second transparent
	data := iffFixture("ILBM", "BMHD", bmhd, "CMAP", paletteBytes(testPalette(256)), "BODY", body)
	got, _, err := DecodeILBM(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Palette) != 256 {
		t.Fatal("palette exceeds 256 entries")
	}
	_, _, _, alpha := got.At(1, 0).RGBA()
	if alpha != 0 {
		t.Fatal("mask was lost")
	}
	_, _, _, alpha = got.At(0, 0).RGBA()
	if alpha != 65535 {
		t.Fatal("opaque pixel was masked")
	}
}

func TestPBMChunkyAndPadding(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		compression := byte(0)
		body := []byte{3, 2, 1, 99, 0, 1, 2, 99}
		if compressed {
			compression = 1
			body = []byte{3, 3, 2, 1, 99, 3, 0, 1, 2, 99}
		}
		data := iffFixture("PBM ", "BMHD", header(3, 2, 2, 0, compression), "CMAP", paletteBytes(testPalette(4)), "BODY", body)
		got, meta, err := DecodeILBM(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if meta.Format != "PBM " || !bytes.Equal(got.Pix, []byte{3, 2, 1, 0, 1, 2}) {
			t.Fatalf("bad PBM pixels: %v", got.Pix)
		}
	}
}

func TestEHBAndHAM(t *testing.T) {
	palette := testPalette(32)
	palette[1] = color.NRGBA{R: 201, G: 101, B: 51, A: 255}
	body := make([]byte, 12)
	body[0] = 0x80  // plane 0, index 1
	body[10] = 0x80 // plane 5, half-brite flag
	data := iffFixture("ILBM", "BMHD", header(1, 1, 6, 0, 0), "CAMG", []byte{0, 0, 0, 0x80}, "CMAP", paletteBytes(palette), "BODY", body)
	got, _, err := DecodeILBM(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Palette) != 64 || got.ColorIndexAt(0, 0) != 33 {
		t.Fatalf("bad EHB index or palette: %d/%d", got.ColorIndexAt(0, 0), len(got.Palette))
	}
	if c := got.Palette[33].(color.NRGBA); c != (color.NRGBA{R: 100, G: 50, B: 25, A: 255}) {
		t.Fatalf("half-brite was not expanded: %#v", c)
	}
	ham := iffFixture("ILBM", "BMHD", header(1, 1, 6, 0, 0), "CAMG", []byte{0, 0, 8, 0}, "BODY", body)
	if _, _, err := DecodeILBM(bytes.NewReader(ham)); err == nil || !strings.Contains(err.Error(), "HAM") {
		t.Fatalf("HAM must be rejected explicitly, got %v", err)
	}
}

func TestILBMColorRangesAndHotspot(t *testing.T) {
	img := image.NewPaletted(image.Rect(0, 0, 3, 2), testPalette(4))
	ranges := []ColorRange{{Rate: 8192, Flags: 3, Low: 1, High: 3}, {Rate: 36, Flags: 1, Low: 0, High: 2}}
	point := image.Pt(-2, 7)
	var data bytes.Buffer
	if err := EncodeILBMWithOptions(&data, img, ILBMOptions{Compressed: true, ColorRanges: ranges, Hotspot: &point, XAspect: 10, YAspect: 11}); err != nil {
		t.Fatal(err)
	}
	_, meta, err := DecodeILBM(bytes.NewReader(data.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(meta.ColorRanges, ranges) || !meta.HasHotspot || meta.Hotspot != point || meta.XAspect != 10 || meta.YAspect != 11 {
		t.Fatalf("metadata changed: %+v", meta)
	}
	if !ranges[0].Active() || !ranges[0].Reverse() || ranges[0].StepsPerSecond() != 30 || ranges[1].Active() {
		t.Fatal("incorrect CRNG semantics")
	}
}

func TestByteRun1RowsAndBoundaries(t *testing.T) {
	rows := [][]byte{{0, 0}, {1, 2, 3, 4}, bytes.Repeat([]byte{9}, 128), bytes.Repeat([]byte{7}, 129), bytes.Repeat([]byte{1, 1, 2, 2, 3, 3}, 50)}
	for _, row := range rows {
		encoded := encodeByteRun1(row)
		got := make([]byte, len(row))
		pos, err := decodeRow(encoded, 0, got, 1)
		if err != nil || pos != len(encoded) || !bytes.Equal(row, got) {
			t.Fatalf("row roundtrip failed: size %d: %v", len(row), err)
		}
	}
	// -128 is a no-op, -1 repeats the following byte twice.
	got := make([]byte, 2)
	if _, err := decodeRow([]byte{128, 255, 23}, 0, got, 1); err != nil || !bytes.Equal(got, []byte{23, 23}) {
		t.Fatal("ByteRun1 no-op or repeat failed")
	}
	for _, bad := range [][]byte{{2, 1, 2, 3}, {254, 1}, {1, 7}, {255}, {128}} {
		if _, err := decodeRow(bad, 0, make([]byte, 2), 1); err == nil {
			t.Fatalf("accepted invalid row %v", bad)
		}
	}
}

func TestIndexedPNGAndGIFPreservePalette(t *testing.T) {
	palette := testPalette(64)
	palette[7] = color.NRGBA{R: 10, G: 20, B: 30, A: 0}
	img := image.NewPaletted(image.Rect(0, 0, 17, 7), palette)
	for i := range img.Pix {
		img.Pix[i] = uint8(i % 64)
	}
	for _, format := range []string{"png", "gif"} {
		var data bytes.Buffer
		var err error
		if format == "png" {
			err = png.Encode(&data, img)
		} else {
			err = gif.Encode(&data, img, nil)
		}
		if err != nil {
			t.Fatal(err)
		}
		got, err := Decode(bytes.NewReader(data.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Pix, img.Pix) || len(got.Palette) != 64 {
			t.Fatalf("indexed %s was unnecessarily quantized", format)
		}
		assertSameImage(t, got, img)
	}
}

func TestTruecolorPNGAndJPEGQuantization(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 80, 51))
	for y := 0; y < 51; y++ {
		for x := 0; x < 80; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 3), G: uint8(y * 5), B: uint8(x * y), A: 255})
		}
	}
	img.SetNRGBA(0, 0, color.NRGBA{})
	for _, format := range []string{"png", "jpeg"} {
		var data bytes.Buffer
		var err error
		if format == "png" {
			err = png.Encode(&data, img)
		} else {
			err = jpeg.Encode(&data, img, nil)
		}
		if err != nil {
			t.Fatal(err)
		}
		got, err := Decode(bytes.NewReader(data.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Palette) > 32 || got.Rect != img.Rect {
			t.Fatalf("incorrect quantized %s output", format)
		}
		again, err := Decode(bytes.NewReader(data.Bytes()))
		if err != nil || !reflect.DeepEqual(again, got) {
			t.Fatalf("%s quantization is not deterministic", format)
		}
		if format == "png" {
			_, _, _, a := got.At(0, 0).RGBA()
			if a != 0 {
				t.Fatal("PNG transparency lost")
			}
		}
	}
	// A small truecolor PNG should keep exact colors, including partial alpha.
	exact := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	exact.SetNRGBA(0, 0, color.NRGBA{R: 7, G: 13, B: 201, A: 123})
	exact.SetNRGBA(1, 0, color.NRGBA{R: 99, G: 3, B: 2, A: 255})
	exact.SetNRGBA(2, 0, color.NRGBA{})
	var data bytes.Buffer
	if err := png.Encode(&data, exact); err != nil {
		t.Fatal(err)
	}
	got, err := Decode(bytes.NewReader(data.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 3; x++ {
		if color.NRGBAModel.Convert(got.At(x, 0)) != color.NRGBAModel.Convert(exact.At(x, 0)) {
			t.Fatalf("exact quantization changed color %d", x)
		}
	}
}

func TestSaveAndLoadExtensions(t *testing.T) {
	img := image.NewPaletted(image.Rect(0, 0, 17, 3), testPalette(8))
	for i := range img.Pix {
		img.Pix[i] = uint8(i % 8)
	}
	dir := t.TempDir()
	for _, ext := range []string{".png", ".GIF", ".iff", ".ilbm", ".lbm", ".brush"} {
		path := filepath.Join(dir, "picture"+ext)
		if err := Save(path, img); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		assertSameImage(t, got, img)
	}
	path := filepath.Join(dir, "existing.jpeg")
	if err := os.WriteFile(path, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, img); err == nil {
		t.Fatal("unsupported extension accepted")
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "unchanged" {
		t.Fatal("failed save destroyed existing file")
	}
	// Encoding an unsupported translucent ILBM must also preserve an existing file.
	path = filepath.Join(dir, "existing.iff")
	if err := os.WriteFile(path, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	img.Palette[0] = color.NRGBA{A: 42}
	if err := Save(path, img); err == nil {
		t.Fatal("translucent ILBM accepted")
	}
	contents, err = os.ReadFile(path)
	if err != nil || string(contents) != "unchanged" {
		t.Fatal("encoding failure destroyed existing file")
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".pixeluxe-save-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("save left temporary files")
	}
}

func TestMalformedIFFAndStorage(t *testing.T) {
	valid := iffFixture("ILBM", "BMHD", header(1, 1, 1, 0, 0), "BODY", []byte{0, 0})
	for n := 0; n < len(valid); n++ {
		if _, _, err := DecodeILBM(bytes.NewReader(valid[:n])); err == nil {
			t.Fatalf("accepted truncation at %d", n)
		}
	}
	cases := [][]byte{
		iffFixture("ILBM", "BODY", []byte{0, 0}),
		iffFixture("ILBM", "BMHD", header(0, 1, 1, 0, 0), "BODY", []byte{0, 0}),
		iffFixture("ILBM", "BMHD", header(65535, 65535, 8, 0, 0), "BODY", []byte{0, 0}),
		iffFixture("ILBM", "BMHD", header(1, 1, 0, 0, 0), "BODY", []byte{0, 0}),
		iffFixture("ILBM", "BMHD", header(1, 1, 9, 0, 0), "BODY", []byte{0, 0}),
		iffFixture("ILBM", "BMHD", header(1, 1, 1, 3, 0), "BODY", []byte{0, 0}),
		iffFixture("ILBM", "BMHD", header(1, 1, 1, 0, 2), "BODY", []byte{0, 0}),
		iffFixture("ILBM", "BMHD", header(1, 1, 1, 0, 1), "BODY", []byte{254, 0}),
		iffFixture("ILBM", "BMHD", header(1, 1, 1, 0, 0), "CMAP", []byte{0}, "BODY", []byte{0, 0}),
		iffFixture("ILBM", "BMHD", header(1, 1, 1, 0, 0), "BODY", []byte{0, 0}, "BODY", []byte{0, 0}),
		iffFixture("8SVX", "BODY", []byte{0, 0}),
	}
	for i, data := range cases {
		if _, _, err := DecodeILBM(bytes.NewReader(data)); err == nil {
			t.Fatalf("accepted malformed fixture %d", i)
		}
	}
	if _, err := Decode(nil); err == nil {
		t.Fatal("nil reader accepted")
	}
	for _, img := range []*image.Paletted{nil, {}, {Rect: image.Rect(0, 0, 2, 2), Stride: math.MaxInt, Pix: []byte{0, 0}, Palette: testPalette(2)}, {Rect: image.Rect(0, 0, 1, 1), Stride: 1, Pix: []byte{2}, Palette: testPalette(2)}, {Rect: image.Rect(0, 0, 1, 1), Stride: 1, Pix: []byte{0}, Palette: color.Palette{nil}}} {
		if err := EncodeILBM(io.Discard, img); err == nil {
			t.Fatal("invalid pixel storage accepted")
		}
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestEncoderReportsShortWrites(t *testing.T) {
	img := image.NewPaletted(image.Rect(0, 0, 1, 1), testPalette(2))
	if err := EncodeILBM(shortWriter{}, img); err != io.ErrShortWrite {
		t.Fatalf("short write: got %v", err)
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("FORM\x00\x00\x00\x04ILBM"))
	f.Add(iffFixture("ILBM", "BMHD", header(3, 2, 2, 0, 1), "BODY", []byte{255, 0, 255, 0, 255, 0, 255, 0}))
	f.Add(iffFixture("PBM ", "BMHD", header(3, 1, 2, 0, 0), "BODY", []byte{0, 1, 2, 0}))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1024*1024 {
			t.Skip()
		}
		img, err := Decode(bytes.NewReader(data))
		if err == nil {
			if err := validatePaletted(img); err != nil {
				t.Fatalf("decoder returned an invalid image: %v", err)
			}
		}
	})
}

func header(w, h uint16, planes, masking, compression byte) []byte {
	b := make([]byte, 20)
	binary.BigEndian.PutUint16(b[:2], w)
	binary.BigEndian.PutUint16(b[2:4], h)
	b[8], b[9], b[10] = planes, masking, compression
	b[14], b[15] = 1, 1
	return b
}

func paletteBytes(p color.Palette) []byte {
	b := make([]byte, len(p)*3)
	for i, c := range p {
		v := color.NRGBAModel.Convert(c).(color.NRGBA)
		b[i*3], b[i*3+1], b[i*3+2] = v.R, v.G, v.B
	}
	return b
}

func iffFixture(kind string, chunks ...any) []byte {
	var form, output bytes.Buffer
	form.WriteString(kind)
	for i := 0; i < len(chunks); i += 2 {
		writeChunk(&form, chunks[i].(string), chunks[i+1].([]byte))
	}
	output.WriteString("FORM")
	binary.Write(&output, binary.BigEndian, uint32(form.Len()))
	output.Write(form.Bytes())
	return output.Bytes()
}
