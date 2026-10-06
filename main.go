package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"pixeluxe/assets"
	"pixeluxe/internal/pixfont"
	"pixeluxe/internal/ui"
)

func writePNG(path string, im image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = png.Encode(f, im)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func main() {
	open := flag.String("open", "", "open a PNG, GIF, JPEG or Amiga IFF picture")
	demo := flag.Bool("demo", false, "open the original Amiga Seascape picture")
	preview := flag.String("preview", "", "write a screenshot without opening a window")
	pdf := flag.String("pdf", "", "export the picture to an A4 PDF without opening a window")
	smoke := flag.Int("smoke", 0, "run this many frames, then exit (desktop smoke test)")
	capture := flag.String("capture", "", "write a screenshot at the end of a smoke test")
	scale := flag.Int("scale", 2, "initial desktop window scale (1 to 3)")
	flag.Parse()
	fontData, err := assets.Files.ReadFile("fonts.json")
	if err != nil {
		log.Fatal(err)
	}
	if err = pixfont.LoadAmiga(fontData); err != nil {
		log.Fatal(err)
	}
	ui.InstallFontMenu()
	g := ui.New()
	g.DemoFS, _ = fs.Sub(assets.Files, "pictures")
	g.BrushFS, _ = fs.Sub(assets.Files, "brushes")
	if *demo {
		if err := g.LoadExample("Seascape.iff"); err != nil {
			log.Fatal(err)
		}
	}
	if *open != "" {
		if err := g.Load(*open); err != nil {
			log.Fatal(err)
		}
	} else if flag.NArg() > 0 {
		if err := g.Load(flag.Arg(0)); err != nil {
			log.Fatal(err)
		}
	}
	if *pdf != "" {
		if err := g.PrintPDF(*pdf); err != nil {
			log.Fatal(err)
		}
		fmt.Println(*pdf)
		return
	}
	if *preview != "" {
		if err := writePNG(*preview, g.Render()); err != nil {
			log.Fatal(err)
		}
		fmt.Println(*preview)
		return
	}
	g.SmokeFrames = *smoke
	if *capture != "" {
		g.OnSmoke = func(im *image.RGBA) error { return writePNG(*capture, im) }
	}
	s := max(1, min(3, *scale))
	ebiten.SetWindowSize(ui.Width*s, ui.Height*s)
	ebiten.SetWindowSizeLimits(640, 512, -1, -1)
	ebiten.SetWindowTitle("Pixeluxe — Deluxe Paint II in Go")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetScreenFilterEnabled(false)
	ebiten.SetWindowClosingHandled(true)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
