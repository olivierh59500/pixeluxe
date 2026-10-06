package formats

import (
	"image"
	"image/color"
	"sort"
)

type histogramColor struct {
	key     uint16
	count   int
	r, g, b uint64
}

func bucketKey(c color.NRGBA) uint16 {
	if c.A < 128 {
		return 32768
	}
	return uint16(c.R>>3)<<10 | uint16(c.G>>3)<<5 | uint16(c.B>>3)
}

// quantize uses weighted median cut over a bounded RGB histogram. It first
// tries an exact palette, so small truecolor pixel-art images lose no colors.
// Images requiring reduction use binary transparency, as Amiga brushes do.
func quantize(src image.Image, limit int) *image.Paletted {
	bounds := src.Bounds()
	exact := make(map[color.NRGBA]uint8, limit)
	palette := make(color.Palette, 0, limit)
	for y := bounds.Min.Y; y < bounds.Max.Y && len(exact) <= limit; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			if _, ok := exact[c]; !ok {
				exact[c] = uint8(len(palette))
				palette = append(palette, c)
				if len(palette) > limit {
					break
				}
			}
		}
	}
	if len(palette) <= limit {
		dst := image.NewPaletted(image.Rect(0, 0, bounds.Dx(), bounds.Dy()), palette)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
				dst.SetColorIndex(x-bounds.Min.X, y-bounds.Min.Y, exact[c])
			}
		}
		return dst
	}

	hist := make(map[uint16]*histogramColor)
	transparent := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			key := bucketKey(c)
			if key == 32768 {
				transparent = true
				continue
			}
			h := hist[key]
			if h == nil {
				h = &histogramColor{key: key}
				hist[key] = h
			}
			h.count++
			h.r += uint64(c.R)
			h.g += uint64(c.G)
			h.b += uint64(c.B)
		}
	}
	entries := make([]histogramColor, 0, len(hist))
	for _, h := range hist {
		entries = append(entries, *h)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	palette = nil
	if transparent {
		palette = append(palette, color.NRGBA{})
		limit--
	}
	boxes := [][]histogramColor{entries}
	for len(boxes) < limit {
		best, bestChannel, bestScore := -1, 0, int64(0)
		for i, box := range boxes {
			if len(box) < 2 {
				continue
			}
			channel, span, weight := boxRange(box)
			score := int64(span) * int64(weight)
			if score > bestScore {
				best, bestChannel, bestScore = i, channel, score
			}
		}
		if best < 0 {
			break
		}
		box := boxes[best]
		sort.Slice(box, func(i, j int) bool {
			a, b := component(box[i].key, bestChannel), component(box[j].key, bestChannel)
			if a == b {
				return box[i].key < box[j].key
			}
			return a < b
		})
		weight := 0
		for _, c := range box {
			weight += c.count
		}
		mid, sum := 1, box[0].count
		for mid < len(box)-1 && sum < (weight+1)/2 {
			sum += box[mid].count
			mid++
		}
		boxes[best] = box[:mid]
		boxes = append(boxes, box[mid:])
	}
	for _, box := range boxes {
		if len(box) == 0 {
			continue
		}
		var r, g, b, n uint64
		for _, h := range box {
			r += h.r
			g += h.g
			b += h.b
			n += uint64(h.count)
		}
		palette = append(palette, color.NRGBA{R: uint8((r + n/2) / n), G: uint8((g + n/2) / n), B: uint8((b + n/2) / n), A: 255})
	}
	dst := image.NewPaletted(image.Rect(0, 0, bounds.Dx(), bounds.Dy()), palette)
	indices := make(map[uint16]uint8, len(hist)+1)
	for key, h := range hist {
		c := color.NRGBA{R: uint8(h.r / uint64(h.count)), G: uint8(h.g / uint64(h.count)), B: uint8(h.b / uint64(h.count)), A: 255}
		indices[key] = uint8(palette.Index(c))
	}
	indices[32768] = 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			dst.SetColorIndex(x-bounds.Min.X, y-bounds.Min.Y, indices[bucketKey(c)])
		}
	}
	return dst
}

func component(key uint16, channel int) int {
	return int((key >> uint(10-channel*5)) & 31)
}

func boxRange(box []histogramColor) (channel, span, weight int) {
	minimum, maximum := [3]int{31, 31, 31}, [3]int{}
	for _, c := range box {
		weight += c.count
		for ch := 0; ch < 3; ch++ {
			v := component(c.key, ch)
			if v < minimum[ch] {
				minimum[ch] = v
			}
			if v > maximum[ch] {
				maximum[ch] = v
			}
		}
	}
	for ch := 0; ch < 3; ch++ {
		if maximum[ch]-minimum[ch] > span {
			channel, span = ch, maximum[ch]-minimum[ch]
		}
	}
	return
}
