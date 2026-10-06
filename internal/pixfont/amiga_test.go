package pixfont

import (
	"bytes"
	"encoding/json"
	"image"
	"os"
	"reflect"
	"sync"
	"testing"
)

func fontFixture() []byte {
	return []byte(`[{"name":"Test","size":3,"height":3,"xsize":4,"baseline":1,"first_char":32,"last_char":233,"glyphs":[
	{"code":32,"width":0,"advance":2,"kern":0,"rows":["","",""]},
	{"code":63,"width":1,"advance":2,"kern":0,"rows":["1","1","1"]},
	{"code":65,"width":2,"advance":3,"kern":1,"rows":["10","01","11"]},
	{"code":74,"width":2,"advance":2,"kern":-1,"rows":["10","01","10"]},
	{"code":233,"width":3,"advance":4,"kern":0,"rows":["010","101","111"]}
	]}]`)
}

func loadFixture(t *testing.T) {
	t.Helper()
	if err := LoadAmiga(fontFixture()); err != nil {
		t.Fatal(err)
	}
}

func amigaPixels(name, text string, x, y, scale, style int) map[image.Point]bool {
	pixels := make(map[image.Point]bool)
	WalkAmiga(name, text, x, y, scale, style, func(x, y int) { pixels[image.Pt(x, y)] = true })
	return pixels
}

func TestAmigaKerningAdvanceAndOrigins(t *testing.T) {
	loadFixture(t)
	want := map[image.Point]bool{
		image.Pt(11, 20): true, image.Pt(12, 21): true, image.Pt(11, 22): true, image.Pt(12, 22): true,
		image.Pt(13, 20): true, image.Pt(14, 21): true, image.Pt(13, 22): true,
	}
	if got := amigaPixels("test/3", "AJ", 10, 20, 1, 0); !reflect.DeepEqual(got, want) {
		t.Fatalf("kerning or glyph origin changed: got %v want %v", got, want)
	}
	if width := WidthAmiga(" TEST/3 ", "AJ", 1); width != 5 {
		t.Fatalf("kerning must contribute to advance: got %d want 5", width)
	}
	if width := WidthAmiga("test/3", "AJé\nAA", 2); width != 18 {
		t.Fatalf("longest line width: got %d want 18", width)
	}
	if height := HeightAmiga("test/3", 3); height != 9 {
		t.Fatalf("font cell height: got %d want 9", height)
	}
	if WidthAmiga("test/3", "", 1) != 0 {
		t.Fatal("empty string has width")
	}
}

func TestAmigaIntegerScaleAndMultiline(t *testing.T) {
	loadFixture(t)
	plain := amigaPixels("test/3", "J\nA", 7, 11, 1, 0)
	scaled := amigaPixels("test/3", "J\nA", 7, 11, 3, 0)
	want := make(map[image.Point]bool)
	for p := range plain {
		for sy := 0; sy < 3; sy++ {
			for sx := 0; sx < 3; sx++ {
				want[image.Pt(7+(p.X-7)*3+sx, 11+(p.Y-11)*3+sy)] = true
			}
		}
	}
	if !reflect.DeepEqual(scaled, want) {
		t.Fatal("integer scaling changed glyph bitmap or newline position")
	}
	if !plain[image.Pt(6, 11)] || !plain[image.Pt(8, 16)] {
		t.Fatal("negative kern or height+2 interline was lost")
	}
}

func TestAmigaLatin1AndUnsupportedRune(t *testing.T) {
	loadFixture(t)
	accent := amigaPixels("test/3", "é", 0, 0, 1, 0)
	want := map[image.Point]bool{image.Pt(1, 0): true, image.Pt(0, 1): true, image.Pt(2, 1): true, image.Pt(0, 2): true, image.Pt(1, 2): true, image.Pt(2, 2): true}
	if !reflect.DeepEqual(accent, want) || WidthAmiga("test/3", "é", 1) != 4 {
		t.Fatal("UTF-8 accent was decoded as multiple characters")
	}
	if got := amigaPixels("test/3", "🖌", 0, 0, 1, 0); !reflect.DeepEqual(got, amigaPixels("test/3", "?", 0, 0, 1, 0)) {
		t.Fatal("unsupported Unicode rune should use '?' once")
	}
}

func TestAmigaBoldItalicUnderline(t *testing.T) {
	loadFixture(t)
	plain := amigaPixels("test/3", "A", 0, 0, 1, 0)
	wantBold := make(map[image.Point]bool)
	for p := range plain {
		wantBold[p] = true
		wantBold[image.Pt(p.X+1, p.Y)] = true
	}
	if got := amigaPixels("test/3", "A", 0, 0, 1, AmigaBold); !reflect.DeepEqual(got, wantBold) {
		t.Fatal("bold did not smear one source pixel right")
	}
	wantItalic := map[image.Point]bool{image.Pt(1, 0): true, image.Pt(2, 1): true, image.Pt(0, 2): true, image.Pt(1, 2): true}
	if got := amigaPixels("test/3", "A", 0, 0, 1, AmigaItalic); !reflect.DeepEqual(got, wantItalic) {
		t.Fatalf("italic did not shear around the baseline: %v", got)
	}
	underlined := amigaPixels("test/3", " A", 10, 20, 1, AmigaUnderline)
	if !underlined[image.Pt(10, 22)] || !underlined[image.Pt(11, 22)] || underlined[image.Pt(12, 22)] || underlined[image.Pt(15, 22)] {
		t.Fatal("underline did not preserve gaps around descenders")
	}
	combined := amigaPixels("test/3", "A", 0, 0, 1, AmigaBold|AmigaItalic)
	for p := range wantItalic {
		if !combined[p] || !combined[image.Pt(p.X+1, p.Y)] {
			t.Fatal("combined styles lost pixels")
		}
	}
}

func TestUnknownAmigaUsesCompactFont(t *testing.T) {
	loadFixture(t)
	for _, name := range []string{"", "missing/8"} {
		want := make(map[image.Point]bool)
		Walk("Pixeluxe\n123", 7, 9, 2, func(x, y int) { want[image.Pt(x, y)] = true })
		if got := amigaPixels(name, "Pixeluxe\n123", 7, 9, 2, 0); !reflect.DeepEqual(got, want) {
			t.Fatal("unknown name changed default font")
		}
		if WidthAmiga(name, "Pixeluxe\n123", 2) != Width("Pixeluxe\n123", 2) || HeightAmiga(name, 2) != 14 {
			t.Fatal("incorrect fallback metrics")
		}
		if styled := amigaPixels(name, "A", 0, 0, 1, AmigaBold|AmigaItalic|AmigaUnderline); len(styled) == 0 {
			t.Fatal("styled fallback emitted no pixels")
		}
	}
	for _, scale := range []int{-1, 0} {
		if len(amigaPixels("test/3", "A", 0, 0, scale, 7)) != 0 || WidthAmiga("test/3", "A", scale) != 0 || HeightAmiga("test/3", scale) != 0 {
			t.Fatal("nonpositive scale should be empty")
		}
	}
	WalkAmiga("test/3", "A", 0, 0, 1, 0, nil)
}

func TestLoadOriginalAmigaFonts(t *testing.T) {
	data, err := os.ReadFile("../../assets/fonts.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := LoadAmiga(data); err != nil {
		t.Fatal(err)
	}
	want := []string{"diamond/12", "diamond/20", "emerald/17", "emerald/20", "garnet/9", "garnet/16", "opal/9", "opal/12", "ruby/8", "ruby/12", "ruby/15", "sapphire/14", "sapphire/19", "topaz/11"}
	if got := AmigaNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected extracted fonts: %v", got)
	}
	names := AmigaNames()
	names[0] = "mutated"
	if AmigaNames()[0] != want[0] {
		t.Fatal("callers can mutate the font registry")
	}
	// Exact original ruby/8 é, including the accent above its lowercase body.
	pixels := amigaPixels("ruby/8", "é", 0, 0, 1, 0)
	if !pixels[image.Pt(4, 0)] || !pixels[image.Pt(5, 0)] || !pixels[image.Pt(3, 1)] || len(pixels) != 20 || WidthAmiga("ruby/8", "é", 1) != 8 {
		t.Fatalf("original ruby/8 accent or metrics changed: %v", pixels)
	}
	if WidthAmiga("topaz/11", "ÀÉéèêçù", 1) != 7*8 || HeightAmiga("topaz/11", 1) != 11 {
		t.Fatal("original Latin-1 topaz metrics changed")
	}
}

func TestLoadAmigaValidationIsAtomic(t *testing.T) {
	loadFixture(t)
	var original []amigaFontJSON
	if err := json.Unmarshal(fontFixture(), &original); err != nil {
		t.Fatal(err)
	}
	bad := [][]byte{nil, []byte(`null`), []byte(`[]`), []byte(`{}`), []byte(`[{]`)}
	mutations := []func(*amigaFontJSON){
		func(f *amigaFontJSON) { f.Name = "../oops" },
		func(f *amigaFontJSON) { f.Height = 0 },
		func(f *amigaFontJSON) { f.Baseline = f.Height },
		func(f *amigaFontJSON) { f.FirstChar = 256 },
		func(f *amigaFontJSON) { f.Glyphs[0].Code = 1 },
		func(f *amigaFontJSON) { f.Glyphs[1].Width = 257 },
		func(f *amigaFontJSON) { f.Glyphs[1].Rows[0] = "x" },
		func(f *amigaFontJSON) { f.Glyphs[1].Rows[0] = "11" },
		func(f *amigaFontJSON) { f.Glyphs = append(f.Glyphs, f.Glyphs[0]) },
	}
	for _, mutate := range mutations {
		var fonts []amigaFontJSON
		if err := json.Unmarshal(fontFixture(), &fonts); err != nil {
			t.Fatal(err)
		}
		mutate(&fonts[0])
		data, err := json.Marshal(fonts)
		if err != nil {
			t.Fatal(err)
		}
		bad = append(bad, data)
	}
	duplicate, err := json.Marshal(append(original, original[0]))
	if err != nil {
		t.Fatal(err)
	}
	bad = append(bad, duplicate, bytes.Repeat([]byte{' '}, 8*1024*1024+1))
	for i, data := range bad {
		if err := LoadAmiga(data); err == nil {
			t.Fatalf("invalid font input %d accepted", i)
		}
		if !reflect.DeepEqual(AmigaNames(), []string{"test/3"}) {
			t.Fatal("invalid load corrupted the registry")
		}
	}
}

func TestAmigaConcurrentRegistryAccess(t *testing.T) {
	loadFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 30; n++ {
				if err := LoadAmiga(fontFixture()); err != nil {
					t.Error(err)
					return
				}
				WidthAmiga("test/3", "Aé", 1)
				HeightAmiga("test/3", 1)
				AmigaNames()
				WalkAmiga("test/3", "AJé", 0, 0, 1, 7, func(int, int) {})
			}
		}()
	}
	wg.Wait()
}
