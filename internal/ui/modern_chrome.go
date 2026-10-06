package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"
)

func (g *Game) drawModernMenu(dst *image.RGBA) {
	g.drawModernMenuPanel(dst, g.menuRect(), menus[g.menu].Items, true)
	if g.submenu >= 0 {
		g.drawModernMenuPanel(dst, g.subRect(), menus[g.menu].Items[g.submenu].Children, false)
	}
}

func (g *Game) drawModernMenuPanel(dst *image.RGBA, r image.Rectangle, items []MenuItem, root bool) {
	modernFill(dst, r.Add(image.Pt(0, 6)), color.RGBA{13, 14, 19, 255}, 9)
	modernFill(dst, r, modernBorder, 9)
	modernFill(dst, r.Inset(1), modernRaised, 8)
	_, rowHeight, padding := g.menuDimensions()
	for i, it := range items {
		rr := image.Rect(r.Min.X+5, r.Min.Y+padding+i*rowHeight, r.Max.X-5, r.Min.Y+padding+(i+1)*rowHeight)
		selected := g.pointer.In(rr) || (root && i == g.submenu)
		fg := color.Color(modernText)
		if selected {
			modernFill(dst, rr, modernAccentSoft, 5)
			fg = modernAccent
		}
		if g.checked(it.Action) {
			modernCheck(dst, image.Rect(rr.Min.X+6, rr.Min.Y+9, rr.Min.X+18, rr.Min.Y+19), modernAccent)
		}
		keyWidth := 0
		if len(it.Children) > 0 {
			modernTextAt(dst, "›", rr.Max.X-19, rr.Min.Y+5, 16, modernMuted)
			keyWidth = 22
		} else if it.Key != "" {
			keyWidth = modernTextWidth(it.Key, 11) + 16
			modernTextAt(dst, it.Key, rr.Max.X-keyWidth+3, rr.Min.Y+8, 11, modernMuted)
		}
		label := modernClipText(it.Label, rr.Dx()-34-keyWidth, 13)
		modernTextAt(dst, label, rr.Min.X+26, rr.Min.Y+7, 13, fg)
	}
}

func modernCheck(dst *image.RGBA, r image.Rectangle, c color.Color) {
	drawLine(dst, r.Min.X+1, r.Min.Y+r.Dy()/2, r.Min.X+r.Dx()/3, r.Max.Y-2, c)
	drawLine(dst, r.Min.X+r.Dx()/3, r.Max.Y-2, r.Max.X-1, r.Min.Y+1, c)
}

func modernClipText(s string, width, size int) string {
	if modernTextWidth(s, size) <= width {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && modernTextWidth(string(runes)+"…", size) > width {
		runes = runes[:len(runes)-1]
	}
	if modernTextWidth("…", size) > width {
		return ""
	}
	return string(runes) + "…"
}

func modernWrapText(s string, width, size int) []string {
	width = max(1, width)
	var lines []string
	for _, paragraph := range strings.Split(s, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		if modernTextWidth(paragraph, size) <= width {
			lines = append(lines, paragraph)
			continue
		}
		line := ""
		for _, word := range strings.Fields(paragraph) {
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if line != "" && modernTextWidth(candidate, size) > width {
				lines = append(lines, line)
				line = ""
			}
			for modernTextWidth(word, size) > width && len([]rune(word)) > 1 {
				runes := []rune(word)
				end := 1
				for end < len(runes) && modernTextWidth(string(runes[:end+1]), size) <= width {
					end++
				}
				lines = append(lines, string(runes[:end]))
				word = string(runes[end:])
			}
			if line != "" {
				line += " "
			}
			line += word
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func (g *Game) drawModernDialog(dst *image.RGBA) {
	d := g.dialog
	for y := dst.Rect.Min.Y; y < dst.Rect.Max.Y; y++ {
		for x := dst.Rect.Min.X; x < dst.Rect.Max.X; x++ {
			p := dst.RGBAAt(x, y)
			p.R, p.G, p.B = uint8(uint16(p.R)*2/5), uint8(uint16(p.G)*2/5), uint8(uint16(p.B)*2/5)
			dst.SetRGBA(x, y, p)
		}
	}
	modernFill(dst, d.rect.Add(image.Pt(0, 9)), color.RGBA{8, 9, 13, 255}, 12)
	modernFill(dst, d.rect, modernBorder, 12)
	modernFill(dst, d.rect.Inset(1), modernPanel, 11)
	modernTextAt(dst, modernClipText(d.title, d.rect.Dx()-54, 20), d.rect.Min.X+27, d.rect.Min.Y+17, 20, modernText)
	fill(dst, image.Rect(d.rect.Min.X+1, d.rect.Min.Y+52, d.rect.Max.X-1, d.rect.Min.Y+53), modernBorder)
	if d.body != "" {
		maxLines := max(1, (d.rect.Dy()-124)/19)
		if len(d.fields) > 0 {
			maxLines = 2
		}
		p := d.localPoint(18, 47)
		for i, line := range modernWrapText(d.body, d.rect.Dx()-54, 13) {
			if i >= maxLines {
				break
			}
			modernTextAt(dst, line, p.X, p.Y+i*19, 13, modernMuted)
		}
	}
	if d.kind == "file" {
		p := d.localPoint(18, 76)
		modernTextAt(dst, "Double-click to open  ·  Wheel to scroll  ·  Enter to select", p.X, p.Y, 12, modernMuted)
		modernTextAt(dst, "PNG  /  GIF  /  IFF-ILBM  /  LBM  /  BRUSH", p.X, d.rect.Max.Y-156, 12, modernMuted)
	}
	if !d.list.Empty() {
		g.drawModernRequesterList(dst, d)
	}
	for i := range d.fields {
		f := &d.fields[i]
		modernTextAt(dst, f.label, d.rect.Min.X+27, f.rect.Min.Y+8, 13, modernMuted)
		border := modernBorder
		if d.active == i {
			border = modernAccent
		}
		modernFill(dst, f.rect, border, 5)
		modernFill(dst, f.rect.Inset(1), modernBG, 4)
		chars, start, end := d.fieldWindow(f)
		if d.active == i && f.selected {
			modernFill(dst, f.rect.Inset(3), modernAccentSoft, 3)
		}
		modernTextAt(dst, string(chars[start:end]), f.rect.Min.X+10, f.rect.Min.Y+8, 13, modernText)
		if d.active == i && !f.selected && g.Frames%60 < 35 {
			x := f.rect.Min.X + 10 + modernTextWidth(string(chars[start:f.cursor]), 13)
			fill(dst, image.Rect(x, f.rect.Min.Y+6, x+1, f.rect.Max.Y-6), modernAccent)
		}
	}
	if d.kind == "palette" || d.kind == "stencil" {
		g.drawModernPaletteRequester(dst, d)
	}
	if d.err != "" {
		modernTextAt(dst, modernClipText(d.err, d.rect.Dx()-54, 12), d.rect.Min.X+27, d.rect.Max.Y-78, 12, color.RGBA{255, 131, 131, 255})
	}
	for i, button := range d.buttons {
		modernButton(dst, button.rect, button.label, i == 0, g.pointer.In(button.rect))
	}
}

func (g *Game) drawModernRequesterList(dst *image.RGBA, d *Dialog) {
	modernFill(dst, d.list, modernBorder, 6)
	modernFill(dst, d.list.Inset(1), modernBG, 5)
	rows := dialogListRows(d)
	for row := 0; row < rows; row++ {
		index := d.scroll + row
		if index >= d.itemCount() {
			break
		}
		y := d.list.Min.Y + 2 + row*d.listRowHeight()
		r := image.Rect(d.list.Min.X+3, y, d.list.Max.X-d.listScrollbarWidth()-2, y+d.listRowHeight())
		fg := color.Color(modernText)
		if d.selected == index {
			modernFill(dst, r, modernAccentSoft, 4)
			fg = modernAccent
		} else if g.pointer.In(r) {
			modernFill(dst, r, modernRaised, 4)
		}
		label := ""
		if d.kind == "file" {
			f := d.files[index]
			label = "    " + f.name
			if f.dir {
				label = "+   " + f.name
			}
		} else {
			label = d.entries[index]
		}
		modernTextAt(dst, modernClipText(label, r.Dx()-16, 13), r.Min.X+8, y+5, 13, fg)
	}
	bar := image.Rect(d.list.Max.X-d.listScrollbarWidth(), d.list.Min.Y+2, d.list.Max.X-2, d.list.Max.Y-2)
	x := (bar.Min.X + bar.Max.X) / 2
	drawLine(dst, x-3, bar.Min.Y+9, x, bar.Min.Y+6, modernMuted)
	drawLine(dst, x, bar.Min.Y+6, x+3, bar.Min.Y+9, modernMuted)
	drawLine(dst, x-3, bar.Max.Y-9, x, bar.Max.Y-6, modernMuted)
	drawLine(dst, x, bar.Max.Y-6, x+3, bar.Max.Y-9, modernMuted)
	track := image.Rect(bar.Min.X+6, bar.Min.Y+18, bar.Max.X-6, bar.Max.Y-18)
	modernFill(dst, track, modernRaised, 3)
	if d.itemCount() > 0 {
		thumbHeight := max(16, track.Dy()*min(rows, d.itemCount())/d.itemCount())
		thumbY := track.Min.Y
		if d.itemCount() > rows {
			thumbY += (track.Dy() - thumbHeight) * d.scroll / (d.itemCount() - rows)
		}
		modernFill(dst, image.Rect(track.Min.X, thumbY, track.Max.X, thumbY+thumbHeight), modernMuted, 3)
	}
}

func (g *Game) drawModernPaletteRequester(dst *image.RGBA, d *Dialog) {
	p := g.Canvas.Image.Palette
	for i := 0; i < 32; i++ {
		r := paletteSwatch(d, i)
		index := d.palettePage*32 + i
		if index >= len(p) {
			modernFill(dst, r, modernBG, 4)
			continue
		}
		selected := index == int(g.FG)
		if d.kind == "stencil" {
			selected = g.Canvas.Stencil[index]
		}
		border := modernBorder
		if selected {
			border = modernAccent
		}
		modernFill(dst, r, border, 4)
		modernFill(dst, r.Inset(2), p[index], 2)
		if selected {
			badge := image.Rect(r.Max.X-17, r.Min.Y+3, r.Max.X-3, r.Min.Y+17)
			modernFill(dst, badge, modernPanel, 3)
			modernCheck(dst, badge.Inset(2), modernAccent)
		}
	}
	if d.kind == "stencil" {
		caption := d.localPoint(56, 158)
		modernTextAt(dst, "Click colors to protect their pixels from drawing.", caption.X, caption.Y, 12, modernMuted)
		checkbox := stencilCheckbox(d)
		modernButton(dst, checkbox, "", g.Canvas.StencilEnabled, g.pointer.In(checkbox))
		if g.Canvas.StencilEnabled {
			modernCheck(dst, checkbox.Inset(9), modernAccent)
		}
		modernTextAt(dst, "Stencil enabled", checkbox.Max.X+18, checkbox.Min.Y+9, 13, modernText)
		return
	}
	cr, cg, cb, _ := p[g.FG].RGBA()
	channels := [3]int{int(cr >> 12), int(cg >> 12), int(cb >> 12)}
	caption := d.localPoint(62, 158)
	modernTextAt(dst, fmt.Sprintf("Color %d  ·  RGB %X%X%X  ·  12-bit Amiga", g.FG, channels[0], channels[1], channels[2]), caption.X, caption.Y, 13, modernMuted)
	for channel, name := range []string{"Red", "Green", "Blue"} {
		r := paletteSlider(d, channel)
		modernTextAt(dst, name, d.rect.Min.X+39, r.Min.Y+7, 13, modernText)
		for value := 0; value < 16; value++ {
			c := color.RGBA{A: 255}
			switch channel {
			case 0:
				c.R = uint8(value * 17)
			case 1:
				c.G = uint8(value * 17)
			case 2:
				c.B = uint8(value * 17)
			}
			fill(dst, image.Rect(r.Min.X+value*r.Dx()/16, r.Min.Y, r.Min.X+(value+1)*r.Dx()/16, r.Max.Y), c)
		}
		outline(dst, r, modernBorder)
		x := r.Min.X + channels[channel]*(r.Dx()-1)/15
		thumb := image.Rect(x-4, r.Min.Y-3, x+5, r.Max.Y+3)
		modernFill(dst, thumb, modernText, 3)
		modernTextAt(dst, fmt.Sprintf("%2d", channels[channel]), r.Max.X+27, r.Min.Y+7, 13, modernMuted)
	}
}
