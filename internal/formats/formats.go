// Package formats reads and writes the indexed image formats used by Pixeluxe.
// Its codecs use only the Go standard library.
package formats

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Limits bound both decoded allocations and input buffering. They comfortably
// include Amiga overscan pictures while refusing decompression bombs.
const (
	MaxDimension = 8192
	MaxPixels    = 16 * 1024 * 1024
	MaxFileSize  = 64 * 1024 * 1024
)

// Load detects the image format from its contents, rather than its extension.
func Load(path string) (*image.Paletted, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := Decode(f)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", filepath.Base(path), err)
	}
	return img, nil
}

// Decode reads PNG, GIF, JPEG, IFF ILBM, or IFF PBM. Indexed PNG and GIF
// retain their palette and pixels; truecolor pictures are quantized to at most
// 32 colors. Animated GIFs import their first frame.
func Decode(r io.Reader) (*image.Paletted, error) {
	data, err := readLimited(r)
	if err != nil {
		return nil, err
	}
	if len(data) >= 4 && string(data[:4]) == "FORM" {
		img, _, err := decodeILBM(data)
		return img, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("unsupported or invalid image: %w", err)
	}
	if err := checkDimensions(cfg.Width, cfg.Height); err != nil {
		return nil, err
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if indexed, ok := decoded.(*image.Paletted); ok {
		if err := validatePaletted(indexed); err != nil {
			return nil, err
		}
		return indexed, nil
	}
	return quantize(decoded, 32), nil
}

// Save writes PNG, GIF, or ILBM according to the extension. ILBM extensions
// are .iff, .ilbm, .lbm, and .brush. The file is replaced only after encoding
// succeeds, so an encoding error cannot destroy an existing picture.
func Save(path string, img *image.Paletted) error {
	if err := validatePaletted(img); err != nil {
		return err
	}
	var encode func(io.Writer) error
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		encode = func(w io.Writer) error { return png.Encode(w, img) }
	case ".gif":
		encode = func(w io.Writer) error { return gif.Encode(w, img, nil) }
	case ".iff", ".ilbm", ".lbm", ".brush":
		encode = func(w io.Writer) error { return EncodeILBM(w, img) }
	default:
		return fmt.Errorf("unsupported export extension %q (use .png, .gif, or .iff)", filepath.Ext(path))
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".pixeluxe-save-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := encode(f); err != nil {
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

func readLimited(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("nil image reader")
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	if len(data) > MaxFileSize {
		return nil, fmt.Errorf("image file exceeds %d bytes", MaxFileSize)
	}
	return data, nil
}

func checkDimensions(w, h int) error {
	if w <= 0 || h <= 0 || w > MaxDimension || h > MaxDimension || int64(w)*int64(h) > MaxPixels {
		return fmt.Errorf("invalid or oversized image dimensions %d x %d", w, h)
	}
	return nil
}

func validatePaletted(img *image.Paletted) error {
	if img == nil {
		return fmt.Errorf("nil indexed image")
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if err := checkDimensions(w, h); err != nil {
		return err
	}
	if len(img.Palette) == 0 || len(img.Palette) > 256 {
		return fmt.Errorf("indexed image needs 1 to 256 colors")
	}
	for i, c := range img.Palette {
		if c == nil {
			return fmt.Errorf("nil palette color at index %d", i)
		}
	}
	if img.Stride < w || len(img.Pix) < w || h-1 > (len(img.Pix)-w)/img.Stride {
		return fmt.Errorf("invalid indexed pixel storage")
	}
	for y := 0; y < h; y++ {
		for _, v := range img.Pix[y*img.Stride : y*img.Stride+w] {
			if int(v) >= len(img.Palette) {
				return fmt.Errorf("pixel index %d exceeds palette size %d", v, len(img.Palette))
			}
		}
	}
	return nil
}
