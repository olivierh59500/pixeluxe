package ui

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"

	"pixeluxe/internal/formats"
	"pixeluxe/internal/paint"
)

// decodePicture detects IFF by its contents and retains metadata that the
// ordinary image.Image API cannot represent. Other image formats use the
// standard PNG, GIF and JPEG decoders in formats.
func decodePicture(path string) (*image.Paletted, *formats.Metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	header, _ := r.Peek(4)
	var im *image.Paletted
	var meta *formats.Metadata
	if string(header) == "FORM" {
		im, meta, err = formats.DecodeILBM(r)
	} else {
		im, err = formats.Decode(r)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load %s: %w", filepath.Base(path), err)
	}
	return normalizePicture(im), meta, nil
}

// normalizePicture decouples picture coordinates from a source subimage's
// origin and stride. All editor tools address pixels relative to (0,0).
func normalizePicture(src *image.Paletted) *image.Paletted {
	if src == nil {
		return nil
	}
	cloned := paint.CloneImage(src)
	if cloned.Rect.Min == (image.Point{}) {
		return cloned
	}
	im := image.NewPaletted(image.Rect(0, 0, src.Rect.Dx(), src.Rect.Dy()), cloned.Palette)
	for y := 0; y < im.Rect.Dy(); y++ {
		for x := 0; x < im.Rect.Dx(); x++ {
			im.SetColorIndex(x, y, src.ColorIndexAt(x+src.Rect.Min.X, y+src.Rect.Min.Y))
		}
	}
	return im
}

func cloneMetadata(src *formats.Metadata) *formats.Metadata {
	if src == nil {
		return nil
	}
	copy := *src
	copy.ColorRanges = append([]formats.ColorRange(nil), src.ColorRanges...)
	return &copy
}

func (g *Game) Load(path string) error {
	im, meta, err := decodePicture(path)
	if err != nil {
		return err
	}
	g.installWithMetadata(im, path, meta)
	return nil
}

func (g *Game) installWithMetadata(im *image.Paletted, path string, meta *formats.Metadata) {
	g.install(normalizePicture(im), path)
	g.FileMetadata = cloneMetadata(meta)
	g.RangeChanged, g.MultiCycle = false, false
	g.cycleTick, g.cycleOffset, g.CycleSpeed = 0, 0, 8
	if meta == nil {
		return
	}
	for _, r := range meta.ColorRanges {
		if !r.Active() || int(r.High) >= len(g.Canvas.Image.Palette) {
			continue
		}
		g.CycleLow, g.CycleHigh = int(r.Low), int(r.High)
		g.CycleSpeed = max(1, min(600, int(math.Round(16384/float64(r.Rate)))))
		g.MultiCycle = true
		break
	}
}

func isILBMPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".iff", ".ilbm", ".lbm", ".brush":
		return true
	}
	return false
}

func (g *Game) saveOptions() formats.ILBMOptions {
	options := formats.ILBMOptions{Compressed: true, XAspect: 1, YAspect: 1}
	if meta := g.FileMetadata; meta != nil {
		options.XAspect, options.YAspect = max(1, meta.XAspect), max(1, meta.YAspect)
		options.ColorRanges = append([]formats.ColorRange(nil), meta.ColorRanges...)
		if meta.HasHotspot {
			hotspot := meta.Hotspot
			options.Hotspot = &hotspot
		}
	}
	if g.RangeChanged {
		r := formats.ColorRange{Rate: uint16(max(1, 16384/max(1, g.CycleSpeed))), Flags: 1, Low: uint8(g.CycleLow), High: uint8(g.CycleHigh)}
		replaced := false
		for i, old := range options.ColorRanges {
			if old.Active() {
				options.ColorRanges[i], replaced = r, true
				break
			}
		}
		if !replaced {
			options.ColorRanges = append(options.ColorRanges, r)
		}
	}
	valid := options.ColorRanges[:0]
	for _, r := range options.ColorRanges {
		if r.Low <= r.High && int(r.High) < len(g.Canvas.Image.Palette) {
			valid = append(valid, r)
		}
	}
	options.ColorRanges = valid
	return options
}

func saveILBMAtomic(path string, im *image.Paletted, options formats.ILBMOptions) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".pixeluxe-save-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := formats.EncodeILBMWithOptions(f, im, options); err != nil {
		f.Close()
		return fmt.Errorf("encode image: %w", err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (g *Game) Save(path string) error {
	if isILBMPath(path) {
		options := g.saveOptions()
		if err := saveILBMAtomic(path, g.Canvas.Image, options); err != nil {
			return err
		}
		meta := cloneMetadata(g.FileMetadata)
		if meta == nil {
			meta = &formats.Metadata{}
		}
		meta.Format, meta.Width, meta.Height = "ILBM", g.Canvas.Image.Rect.Dx(), g.Canvas.Image.Rect.Dy()
		meta.XAspect, meta.YAspect = options.XAspect, options.YAspect
		meta.ColorRanges = append([]formats.ColorRange(nil), options.ColorRanges...)
		g.FileMetadata, g.RangeChanged = meta, false
	} else if err := formats.Save(path, g.Canvas.Image); err != nil {
		return err
	}
	g.Filename = path
	g.Canvas.MarkSaved()
	g.notice("Saved " + filepath.Base(path))
	return nil
}

func (g *Game) SaveBrush(path string) error {
	if g.Brush == nil || g.Brush.Image == nil {
		return fmt.Errorf("pick up a custom brush before saving it")
	}
	im := brushExportImage(g.Brush)
	if im == nil {
		return fmt.Errorf("brush uses 256 opaque colors; transparency needs a free palette entry")
	}
	if isILBMPath(path) {
		hotspot := image.Pt(im.Rect.Dx()/2, im.Rect.Dy()/2)
		if g.Handles == 1 {
			hotspot = image.Point{}
		}
		if err := saveILBMAtomic(path, im, formats.ILBMOptions{Compressed: true, Hotspot: &hotspot, XAspect: 1, YAspect: 1}); err != nil {
			return err
		}
	} else if err := formats.Save(path, im); err != nil {
		return err
	}
	g.notice("Saved brush " + filepath.Base(path))
	return nil
}

// SaveStencil writes a two-colour picture of the protected coordinates, even
// when protection is temporarily disabled. It does not modify the artwork or
// its file identity and saved state.
func (g *Game) SaveStencil(path string) error {
	b := g.Canvas.Image.Bounds()
	im := image.NewPaletted(image.Rect(0, 0, b.Dx(), b.Dy()), color.Palette{color.Black, color.White})
	spatial := len(g.Canvas.StencilMask) == b.Dx()*b.Dy()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			protected := g.Canvas.Stencil[g.Canvas.Image.ColorIndexAt(x+b.Min.X, y+b.Min.Y)]
			if spatial {
				protected = g.Canvas.StencilMask[y*b.Dx()+x]
			}
			if protected {
				im.SetColorIndex(x, y, 1)
			}
		}
	}
	if err := formats.Save(path, im); err != nil {
		return err
	}
	g.notice("Saved stencil " + filepath.Base(path))
	return nil
}

func (g *Game) LoadStencil(path string) error {
	im, _, err := decodePicture(path)
	if err != nil {
		return err
	}
	if err := g.installStencil(im); err != nil {
		return err
	}
	g.notice("Loaded stencil " + filepath.Base(path))
	return nil
}

func (g *Game) installStencil(im *image.Paletted) error {
	b := g.Canvas.Image.Bounds()
	if im == nil || im.Rect.Dx() != b.Dx() || im.Rect.Dy() != b.Dy() {
		return fmt.Errorf("stencil dimensions must match the picture (%d x %d)", b.Dx(), b.Dy())
	}
	mask := make([]bool, b.Dx()*b.Dy())
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			mask[y*b.Dx()+x] = im.ColorIndexAt(x+im.Rect.Min.X, y+im.Rect.Min.Y) != 0
		}
	}
	g.Canvas.StencilMask, g.Canvas.StencilEnabled = mask, true
	return nil
}

// resizePicture keeps the current artwork at the upper left and records both
// palette and dimensions in the existing canvas history. Reducing the palette
// maps removed indices to the closest retained colour instead of discarding
// their pixels.
func (g *Game) resizePicture(w, h, colors int) {
	src := g.Canvas.Image
	p := make(color.Palette, colors)
	defaults := paint.DefaultPalette()
	for i := range p {
		if i < len(src.Palette) {
			p[i] = src.Palette[i]
		} else if i < len(defaults) {
			p[i] = defaults[i]
		} else {
			v := uint8((i - 32) * 255 / max(1, colors-33))
			p[i] = color.RGBA{v, v, v, 255}
		}
	}
	dst := image.NewPaletted(image.Rect(0, 0, w, h), p)
	var indices [256]uint8
	for i, c := range src.Palette {
		if i < colors {
			indices[i] = uint8(i)
		} else {
			indices[i] = uint8(p.Index(c))
		}
	}
	for y := 0; y < min(h, src.Rect.Dy()); y++ {
		for x := 0; x < min(w, src.Rect.Dx()); x++ {
			dst.SetColorIndex(x, y, indices[src.ColorIndexAt(x+src.Rect.Min.X, y+src.Rect.Min.Y)])
		}
	}
	g.Canvas.Begin()
	g.Canvas.Image = dst
	g.Canvas.Commit()
	g.FG, g.BG = indices[g.FG], indices[g.BG]
	g.PanX, g.PanY = 0, 0
	g.CycleLow, g.CycleHigh = min(g.CycleLow, colors-1), min(g.CycleHigh, colors-1)
	g.cycleOffset = 0
}
