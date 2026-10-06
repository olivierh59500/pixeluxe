package ui

import (
	"image/color"

	"pixeluxe/internal/formats"
)

// displayPalette rotates colors for display only. Picture indices and the
// editable base palette stay intact, including while a requester is open.
func (g *Game) displayPalette(p color.Palette) color.Palette {
	out := append(color.Palette{}, p...)
	if !g.Cycle {
		return out
	}
	ranges := g.saveOptions().ColorRanges
	if len(ranges) == 0 {
		ranges = []formats.ColorRange{{Rate: uint16(max(1, 16384/max(1, g.CycleSpeed))), Flags: 1, Low: uint8(g.CycleLow), High: uint8(g.CycleHigh)}}
	}
	for _, r := range ranges {
		if !r.Active() || int(r.High) >= len(out) {
			continue
		}
		n := int(r.High - r.Low + 1)
		step := int(uint64(max(0, g.cycleTick))*uint64(r.Rate)/16384) % n
		if r.Reverse() {
			step = (n - step) % n
		}
		src := append(color.Palette{}, out...)
		for i := int(r.Low); i <= int(r.High); i++ {
			out[i] = src[int(r.Low)+(i-int(r.Low)+step)%n]
		}
		if !g.MultiCycle {
			break
		}
	}
	return out
}
