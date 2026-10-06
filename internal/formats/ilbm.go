package formats

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
)

const (
	camgHAM = 0x0800
	camgEHB = 0x0080
)

// ColorRange is a Deluxe Paint CRNG color-cycling range. Rate uses the
// original units: 16384 means 60 palette steps per second. Flags bit 0 enables
// cycling and bit 1 reverses its direction.
type ColorRange struct {
	Rate      uint16
	Flags     uint16
	Low, High uint8
}

func (r ColorRange) Active() bool {
	return r.Flags&1 != 0 && r.Rate != 0 && r.Rate != 36 && r.Low < r.High
}
func (r ColorRange) Reverse() bool           { return r.Flags&2 != 0 }
func (r ColorRange) StepsPerSecond() float64 { return float64(r.Rate) * 60 / 16384 }

// Metadata preserves display dimensions, brush hotspot and color-cycle
// ranges without changing the simple indexed image API.
type Metadata struct {
	Format                string
	Width, Height         int
	X, Y                  int16
	Planes                uint8
	Masking               uint8
	Compression           uint8
	TransparentColor      uint16
	XAspect, YAspect      uint8
	PageWidth, PageHeight int16
	CAMG                  uint32
	ColorRanges           []ColorRange
	Hotspot               image.Point
	HasHotspot            bool
}

// ILBMOptions controls optional ILBM data. A zero value emits an uncompressed
// ordinary indexed ILBM. EncodeILBM uses ByteRun1 compression by default.
type ILBMOptions struct {
	Compressed       bool
	ColorRanges      []ColorRange
	Hotspot          *image.Point
	XAspect, YAspect uint8
}

// DecodeILBM decodes ILBM or chunky PBM and returns original IFF metadata.
// HAM is deliberately rejected: its pixels cannot be represented by a fixed
// indexed palette. Extra Half-Brite expands the palette to 64 colors.
func DecodeILBM(r io.Reader) (*image.Paletted, *Metadata, error) {
	data, err := readLimited(r)
	if err != nil {
		return nil, nil, err
	}
	return decodeILBM(data)
}

func decodeILBM(data []byte) (*image.Paletted, *Metadata, error) {
	if len(data) < 12 || string(data[:4]) != "FORM" {
		return nil, nil, fmt.Errorf("not an IFF FORM image")
	}
	formSize := uint64(binary.BigEndian.Uint32(data[4:8]))
	if formSize < 4 || formSize+8 > uint64(len(data)) {
		return nil, nil, fmt.Errorf("truncated or invalid IFF FORM size")
	}
	kind := string(data[8:12])
	if kind != "ILBM" && kind != "PBM " {
		return nil, nil, fmt.Errorf("unsupported IFF FORM type %q", kind)
	}
	meta := &Metadata{Format: kind}
	var cmap, body []byte
	haveHeader, haveBody := false, false
	end := int(formSize + 8)
	for pos := 12; pos < end; {
		if end-pos < 8 {
			return nil, nil, fmt.Errorf("truncated IFF chunk header")
		}
		id := string(data[pos : pos+4])
		size := uint64(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		start := pos + 8
		next := uint64(start) + size + size%2
		if next > uint64(end) {
			return nil, nil, fmt.Errorf("truncated IFF %s chunk", id)
		}
		chunk := data[start : start+int(size)]
		if id == "BODY" {
			if haveBody {
				return nil, nil, fmt.Errorf("duplicate IFF BODY chunk")
			}
			if !haveHeader {
				return nil, nil, fmt.Errorf("IFF BODY precedes BMHD")
			}
			body, haveBody = chunk, true
		} else if !haveBody {
			switch id {
			case "BMHD":
				if len(chunk) != 20 {
					return nil, nil, fmt.Errorf("invalid BMHD length %d", len(chunk))
				}
				meta.Width, meta.Height = int(binary.BigEndian.Uint16(chunk[:2])), int(binary.BigEndian.Uint16(chunk[2:4]))
				meta.X, meta.Y = int16(binary.BigEndian.Uint16(chunk[4:6])), int16(binary.BigEndian.Uint16(chunk[6:8]))
				meta.Planes, meta.Masking, meta.Compression = chunk[8], chunk[9], chunk[10]
				meta.TransparentColor = binary.BigEndian.Uint16(chunk[12:14])
				meta.XAspect, meta.YAspect = chunk[14], chunk[15]
				meta.PageWidth, meta.PageHeight = int16(binary.BigEndian.Uint16(chunk[16:18])), int16(binary.BigEndian.Uint16(chunk[18:20]))
				haveHeader = true
			case "CMAP":
				if len(chunk)%3 != 0 || len(chunk) > 256*3 {
					return nil, nil, fmt.Errorf("invalid CMAP length %d", len(chunk))
				}
				cmap = chunk
			case "CAMG":
				if len(chunk) != 4 {
					return nil, nil, fmt.Errorf("invalid CAMG length")
				}
				meta.CAMG = binary.BigEndian.Uint32(chunk)
			case "CRNG":
				if len(chunk) != 8 {
					return nil, nil, fmt.Errorf("invalid CRNG length")
				}
				if len(meta.ColorRanges) >= 256 {
					return nil, nil, fmt.Errorf("too many CRNG ranges")
				}
				r := ColorRange{Rate: binary.BigEndian.Uint16(chunk[2:4]), Flags: binary.BigEndian.Uint16(chunk[4:6]), Low: chunk[6], High: chunk[7]}
				if r.Low > r.High {
					return nil, nil, fmt.Errorf("invalid CRNG color range")
				}
				meta.ColorRanges = append(meta.ColorRanges, r)
			case "GRAB":
				if len(chunk) != 4 {
					return nil, nil, fmt.Errorf("invalid GRAB length")
				}
				meta.Hotspot = image.Pt(int(int16(binary.BigEndian.Uint16(chunk[:2]))), int(int16(binary.BigEndian.Uint16(chunk[2:]))))
				meta.HasHotspot = true
			}
		}
		pos = int(next)
	}
	if !haveHeader || !haveBody {
		return nil, nil, fmt.Errorf("IFF image needs BMHD and BODY chunks")
	}
	if err := checkDimensions(meta.Width, meta.Height); err != nil {
		return nil, nil, err
	}
	if meta.CAMG&camgHAM != 0 {
		return nil, nil, fmt.Errorf("HAM images are unsupported: a fixed indexed palette cannot represent hold-and-modify colors")
	}
	if meta.Planes < 1 || meta.Planes > 8 {
		return nil, nil, fmt.Errorf("unsupported IFF depth %d (need 1 to 8 planes)", meta.Planes)
	}
	if meta.Compression > 1 {
		return nil, nil, fmt.Errorf("unsupported IFF compression %d", meta.Compression)
	}
	if meta.Masking > 2 {
		return nil, nil, fmt.Errorf("unsupported IFF masking %d", meta.Masking)
	}
	if kind == "PBM " && meta.Masking == 1 {
		return nil, nil, fmt.Errorf("PBM mask planes are unsupported; use transparent-color masking")
	}
	if meta.CAMG&camgEHB != 0 && (kind != "ILBM" || meta.Planes != 6) {
		return nil, nil, fmt.Errorf("Extra Half-Brite requires a six-plane ILBM")
	}
	palette := make(color.Palette, len(cmap)/3)
	for i := range palette {
		palette[i] = color.NRGBA{R: cmap[i*3], G: cmap[i*3+1], B: cmap[i*3+2], A: 255}
	}
	if len(palette) == 0 {
		palette = make(color.Palette, 1<<meta.Planes)
		for i := range palette {
			v := uint8(i * 255 / (len(palette) - 1))
			palette[i] = color.NRGBA{R: v, G: v, B: v, A: 255}
		}
	}
	if meta.CAMG&camgEHB != 0 {
		palette = extendPalette(palette, 64)
		for i := 0; i < 32; i++ {
			c := color.NRGBAModel.Convert(palette[i]).(color.NRGBA)
			palette[i+32] = color.NRGBA{R: c.R / 2, G: c.G / 2, B: c.B / 2, A: 255}
		}
	}
	img := image.NewPaletted(image.Rect(0, 0, meta.Width, meta.Height), palette)
	var mask []byte
	if meta.Masking == 1 {
		mask = make([]byte, (meta.Width*meta.Height+7)/8)
	}
	used := [256]bool{}
	maxIndex, pos := 0, 0
	rowBytes := (meta.Width + 15) / 16 * 2
	if kind == "PBM " {
		rowBytes = (meta.Width + 1) &^ 1
	}
	row := make([]byte, rowBytes)
	for y := 0; y < meta.Height; y++ {
		nrows := int(meta.Planes)
		if kind == "PBM " {
			nrows = 1
		}
		if meta.Masking == 1 {
			nrows++
		}
		for plane := 0; plane < nrows; plane++ {
			var err error
			pos, err = decodeRow(body, pos, row, meta.Compression)
			if err != nil {
				return nil, nil, fmt.Errorf("IFF row %d plane %d: %w", y, plane, err)
			}
			for x := 0; x < meta.Width; x++ {
				i := y*img.Stride + x
				if kind == "PBM " {
					if int(row[x]) >= 1<<meta.Planes {
						return nil, nil, fmt.Errorf("PBM pixel index exceeds bit depth")
					}
					img.Pix[i] = row[x]
				} else if plane == int(meta.Planes) {
					if row[x/8]&(128>>uint(x%8)) == 0 {
						mask[i/8] |= 128 >> uint(i%8)
					}
				} else if row[x/8]&(128>>uint(x%8)) != 0 {
					img.Pix[i] |= 1 << uint(plane)
				}
			}
		}
		for x := 0; x < meta.Width; x++ {
			i := y*img.Stride + x
			v := img.Pix[i]
			if int(v) > maxIndex {
				maxIndex = int(v)
			}
			if mask == nil || mask[i/8]&(128>>uint(i%8)) == 0 {
				used[v] = true
			}
		}
	}
	if meta.Compression == 1 {
		for pos < len(body) && body[pos] == 128 {
			pos++
		}
	}
	if pos != len(body) {
		return nil, nil, fmt.Errorf("unexpected trailing IFF BODY bytes")
	}
	img.Palette = extendPalette(img.Palette, maxIndex+1)
	if meta.Masking == 2 {
		if meta.TransparentColor >= 1<<meta.Planes {
			return nil, nil, fmt.Errorf("IFF transparent index exceeds bit depth")
		}
		img.Palette = extendPalette(img.Palette, int(meta.TransparentColor)+1)
		c := color.NRGBAModel.Convert(img.Palette[meta.TransparentColor]).(color.NRGBA)
		c.A = 0
		img.Palette[meta.TransparentColor] = c
	} else if mask != nil {
		haveTransparent := false
		for _, m := range mask {
			if m != 0 {
				haveTransparent = true
				break
			}
		}
		if haveTransparent {
			index := -1
			for i := len(img.Palette); i < 256; i++ {
				index = i
				break
			}
			if index < 0 {
				for i := range used {
					if !used[i] {
						index = i
						break
					}
				}
			}
			if index < 0 {
				return nil, nil, fmt.Errorf("masked image needs more than 256 indexed colors")
			}
			img.Palette = extendPalette(img.Palette, index+1)
			img.Palette[index] = color.NRGBA{}
			for i := range img.Pix {
				if mask[i/8]&(128>>uint(i%8)) != 0 {
					img.Pix[i] = uint8(index)
				}
			}
		}
	}
	return img, meta, nil
}

func extendPalette(palette color.Palette, n int) color.Palette {
	for len(palette) < n {
		palette = append(palette, color.NRGBA{A: 255})
	}
	return palette
}

func decodeRow(body []byte, pos int, row []byte, compression uint8) (int, error) {
	if compression == 0 {
		if len(body)-pos < len(row) {
			return pos, io.ErrUnexpectedEOF
		}
		copy(row, body[pos:pos+len(row)])
		return pos + len(row), nil
	}
	for dst := 0; dst < len(row); {
		if pos >= len(body) {
			return pos, io.ErrUnexpectedEOF
		}
		code := int(int8(body[pos]))
		pos++
		if code == -128 {
			continue
		}
		if code >= 0 {
			n := code + 1
			if n > len(row)-dst {
				return pos, fmt.Errorf("ByteRun1 literal crosses a row boundary")
			}
			if n > len(body)-pos {
				return pos, io.ErrUnexpectedEOF
			}
			copy(row[dst:dst+n], body[pos:pos+n])
			pos, dst = pos+n, dst+n
		} else {
			n := 1 - code
			if n > len(row)-dst {
				return pos, fmt.Errorf("ByteRun1 run crosses a row boundary")
			}
			if pos >= len(body) {
				return pos, io.ErrUnexpectedEOF
			}
			for i := 0; i < n; i++ {
				row[dst+i] = body[pos]
			}
			pos++
			dst += n
		}
	}
	return pos, nil
}

// EncodeILBM writes an indexed ILBM with ByteRun1 row compression and 1 to
// 8 bitplanes. Transparent palettes use a transparent color or a mask plane.
func EncodeILBM(w io.Writer, img *image.Paletted) error {
	return EncodeILBMWithOptions(w, img, ILBMOptions{Compressed: true})
}

// EncodeILBMWithOptions writes an ILBM, including optional CRNG and GRAB data.
func EncodeILBMWithOptions(w io.Writer, img *image.Paletted, options ILBMOptions) error {
	if w == nil {
		return fmt.Errorf("nil image writer")
	}
	if err := validatePaletted(img); err != nil {
		return err
	}
	if len(options.ColorRanges) > 256 {
		return fmt.Errorf("too many CRNG ranges")
	}
	for _, r := range options.ColorRanges {
		if r.Low > r.High || int(r.High) >= len(img.Palette) {
			return fmt.Errorf("CRNG range exceeds palette")
		}
	}
	planes := uint8(1)
	for 1<<planes < len(img.Palette) {
		planes++
	}
	transparent, transparentCount := 0, 0
	cmap := make([]byte, len(img.Palette)*3)
	for i, pc := range img.Palette {
		c := color.NRGBAModel.Convert(pc).(color.NRGBA)
		if c.A != 0 && c.A != 255 {
			return fmt.Errorf("ILBM supports binary transparency; palette index %d is translucent", i)
		}
		if c.A == 0 {
			transparent, transparentCount = i, transparentCount+1
		}
		cmap[i*3], cmap[i*3+1], cmap[i*3+2] = c.R, c.G, c.B
	}
	masking := uint8(0)
	if transparentCount == 1 {
		masking = 2
	}
	if transparentCount > 1 {
		masking = 1
	}
	wid, hei := img.Rect.Dx(), img.Rect.Dy()
	bmhd := make([]byte, 20)
	binary.BigEndian.PutUint16(bmhd[:2], uint16(wid))
	binary.BigEndian.PutUint16(bmhd[2:4], uint16(hei))
	bmhd[8], bmhd[9] = planes, masking
	if options.Compressed {
		bmhd[10] = 1
	}
	binary.BigEndian.PutUint16(bmhd[12:14], uint16(transparent))
	bmhd[14], bmhd[15] = options.XAspect, options.YAspect
	if bmhd[14] == 0 {
		bmhd[14] = 1
	}
	if bmhd[15] == 0 {
		bmhd[15] = 1
	}
	binary.BigEndian.PutUint16(bmhd[16:18], uint16(wid))
	binary.BigEndian.PutUint16(bmhd[18:20], uint16(hei))
	var body bytes.Buffer
	row := make([]byte, (wid+15)/16*2)
	for y := 0; y < hei; y++ {
		nrows := int(planes)
		if masking == 1 {
			nrows++
		}
		for plane := 0; plane < nrows; plane++ {
			clear(row)
			for x := 0; x < wid; x++ {
				index := img.Pix[y*img.Stride+x]
				on := index&(1<<uint(plane)) != 0
				if plane == int(planes) {
					_, _, _, a := img.Palette[index].RGBA()
					on = a != 0
				}
				if on {
					row[x/8] |= 128 >> uint(x%8)
				}
			}
			if options.Compressed {
				body.Write(encodeByteRun1(row))
			} else {
				body.Write(row)
			}
		}
	}
	var form bytes.Buffer
	form.WriteString("ILBM")
	writeChunk(&form, "BMHD", bmhd)
	writeChunk(&form, "CMAP", cmap)
	for _, r := range options.ColorRanges {
		crng := make([]byte, 8)
		binary.BigEndian.PutUint16(crng[2:4], r.Rate)
		binary.BigEndian.PutUint16(crng[4:6], r.Flags)
		crng[6], crng[7] = r.Low, r.High
		writeChunk(&form, "CRNG", crng)
	}
	if options.Hotspot != nil {
		if options.Hotspot.X < -32768 || options.Hotspot.X > 32767 || options.Hotspot.Y < -32768 || options.Hotspot.Y > 32767 {
			return fmt.Errorf("ILBM hotspot exceeds signed 16-bit coordinates")
		}
		grab := make([]byte, 4)
		binary.BigEndian.PutUint16(grab[:2], uint16(options.Hotspot.X))
		binary.BigEndian.PutUint16(grab[2:], uint16(options.Hotspot.Y))
		writeChunk(&form, "GRAB", grab)
	}
	writeChunk(&form, "BODY", body.Bytes())
	var output bytes.Buffer
	output.WriteString("FORM")
	binary.Write(&output, binary.BigEndian, uint32(form.Len()))
	output.Write(form.Bytes())
	n, err := w.Write(output.Bytes())
	if err == nil && n != output.Len() {
		err = io.ErrShortWrite
	}
	return err
}

func writeChunk(w *bytes.Buffer, id string, data []byte) {
	w.WriteString(id)
	binary.Write(w, binary.BigEndian, uint32(len(data)))
	w.Write(data)
	if len(data)%2 != 0 {
		w.WriteByte(0)
	}
}

func encodeByteRun1(row []byte) []byte {
	out := make([]byte, 0, len(row)+len(row)/128+1)
	for pos := 0; pos < len(row); {
		run := 1
		for run < 128 && pos+run < len(row) && row[pos+run] == row[pos] {
			run++
		}
		if run >= 3 {
			out = append(out, byte(1-run), row[pos])
			pos += run
			continue
		}
		start := pos
		pos += run
		for pos < len(row) && pos-start < 128 {
			run = 1
			for run < 3 && pos+run < len(row) && row[pos+run] == row[pos] {
				run++
			}
			if run >= 3 {
				break
			}
			if run > 128-(pos-start) {
				run = 128 - (pos - start)
			}
			pos += run
		}
		out = append(out, byte(pos-start-1))
		out = append(out, row[start:pos]...)
	}
	return out
}
