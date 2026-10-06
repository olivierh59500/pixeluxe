package ui

import (
	"fmt"
	"image"
	"image/color"
	"testing"
)

func rectCenter(r image.Rectangle) image.Point {
	return image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2)
}

func TestMenuGeometryRoutesBothInterfaces(t *testing.T) {
	for _, modern := range []bool{false, true} {
		t.Run(fmt.Sprintf("modern=%t", modern), func(t *testing.T) {
			g := New()
			g.Modern, g.menu, g.submenu = modern, 0, 6
			panel := g.menuRect()
			title := g.menuTitleRect(0)
			if panel.Min.X != title.Min.X || panel.Min.Y != title.Max.Y {
				t.Fatalf("menu must open directly below its title: %v, %v", title, panel)
			}
			_, rowHeight, padding := g.menuDimensions()
			for row, entry := range menus[0].Items {
				p := image.Pt(panel.Min.X+30, panel.Min.Y+padding+row*rowHeight+rowHeight/2)
				index := g.menuRowAt(panel, p, len(menus[0].Items))
				if index != row || menus[0].Items[index].Label != entry.Label {
					t.Fatalf("pointer must address menu row %d, got %d", row, index)
				}
			}
			sub := g.subRect()
			w, h := g.screenSize()
			if !sub.In(image.Rect(0, 0, w, h)) {
				t.Fatalf("submenu must fit the active interface: %v", sub)
			}
			children := menus[0].Items[g.submenu].Children
			p := image.Pt(sub.Min.X+30, sub.Min.Y+padding+2*rowHeight+rowHeight/2)
			if index := g.menuRowAt(sub, p, len(children)); index != 2 || children[index].Action != "restore-palette" {
				t.Fatalf("submenu pointer must address the original palette action, got %d", index)
			}
			if g.menuRowAt(panel, image.Pt(panel.Min.X+3, panel.Max.Y-1), len(menus[0].Items)) != -1 {
				t.Fatal("menu padding must not activate an item")
			}
			if modern && title.Min.Y != modernHeaderHeight {
				t.Fatal("modern menus must be below the document header")
			}
		})
	}
}

func TestRequesterGeometryAndListRouting(t *testing.T) {
	for _, modern := range []bool{false, true} {
		t.Run(fmt.Sprintf("modern=%t", modern), func(t *testing.T) {
			g := New()
			g.Modern = modern
			entries := make([]string, 40)
			for i := range entries {
				entries[i] = fmt.Sprintf("Option %d", i)
			}
			g.chooseDialog("Choose an option", entries, nil)
			d := g.dialog
			w, h := g.screenSize()
			center := rectCenter(d.rect)
			if !d.rect.In(image.Rect(0, 0, w, h)) || center.X < w/2-1 || center.X > w/2+1 || center.Y < h/2-1 || center.Y > h/2+1 {
				t.Fatalf("requester must be centered within the active screen: %v", d.rect)
			}
			for _, button := range d.buttons {
				if !button.rect.In(d.rect) {
					t.Fatalf("button must move with its requester: %v", button.rect)
				}
			}
			d.scroll = 3
			for row := 0; row < dialogListRows(d); row++ {
				p := image.Pt(d.list.Min.X+12, d.list.Min.Y+2+row*d.listRowHeight()+d.listRowHeight()/2)
				if index := d.listIndexAt(p); index != d.scroll+row {
					t.Fatalf("list row must use the rendered row height: got %d", index)
				}
			}
			if index := d.listIndexAt(image.Pt(d.list.Max.X-4, d.list.Min.Y+20)); index != -1 {
				t.Fatal("scrollbar must not activate a list entry")
			}
			d.selected = 39
			d.keepSelectionVisible()
			if d.scroll+dialogListRows(d) != 40 {
				t.Fatal("keyboard selection must scroll the last entry into view")
			}
			original := d.rect
			g.openRequester(d)
			if d.rect != original {
				t.Fatal("restoring a requester must not scale its controls a second time")
			}
		})
	}
}

func TestPaletteAndStencilPointerRouting(t *testing.T) {
	for _, modern := range []bool{false, true} {
		t.Run(fmt.Sprintf("modern=%t", modern), func(t *testing.T) {
			g := New()
			g.Modern = modern
			g.paletteDialog()
			d := g.dialog
			g.palettePointerDown(d, rectCenter(paletteSwatch(d, 7)))
			if g.FG != 7 {
				t.Fatal("the visible palette swatch must select its corresponding color")
			}
			g.Canvas.Image.Palette[7] = color.RGBA{17, 34, 51, 255}
			r := paletteSlider(d, 1)
			g.palettePointerDown(d, rectCenter(r))
			if d.slider != 1 {
				t.Fatal("the visible green slider must select the green channel")
			}
			g.applyPaletteSlider(d, image.Pt(r.Max.X-1, r.Min.Y))
			cr, cg, cb, _ := g.Canvas.Image.Palette[7].RGBA()
			if cr>>8 != 17 || cg>>8 != 255 || cb>>8 != 51 {
				t.Fatal("dragging the green slider must preserve the other channels")
			}
			g.cancelRequester(d)
			g.stencilDialog()
			d = g.dialog
			g.palettePointerDown(d, rectCenter(paletteSwatch(d, 11)))
			g.palettePointerDown(d, rectCenter(stencilCheckbox(d)))
			if !g.Canvas.Stencil[11] || !g.Canvas.StencilEnabled {
				t.Fatal("stencil swatches and checkbox must share their rendered coordinates")
			}
			g.cancelRequester(d)
			if g.Canvas.Stencil[11] || g.Canvas.StencilEnabled {
				t.Fatal("cancelling must restore stencil settings in both interfaces")
			}
		})
	}
}

func TestRequesterCursorUsesRenderedText(t *testing.T) {
	for _, modern := range []bool{false, true} {
		g := New()
		g.Modern = modern
		g.settingsDialog("grid-settings")
		d := g.dialog
		f := &d.fields[0]
		f.value, f.cursor = "1184", 4
		x := f.rect.Min.X + 5 + 2*6
		if modern {
			x = f.rect.Min.X + 10 + modernTextWidth("11", 13)
		}
		if index := d.fieldCursorAt(f, x); index != 2 {
			t.Fatalf("modern=%t: cursor must match the position after the second rendered character, got %d", modern, index)
		}
	}
}
