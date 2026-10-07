# Deluxe Paint II reference for Pixeluxe

## Sources and extraction

The main reference is the locally supplied disk in `previous/Deluxe_Paint_II_1987_Electronic_Arts_PAL.adf`: a 901,120-byte image using the Amiga OFS `DOS/0` filesystem, volume `DPaint`, with its root at block 880. SHA-256: `a519c2be8d89765d7198b908bdc7797f9c00071d4dedfdc03db31df4b50ff9a3`. The disk is intentionally excluded from Git; these inspection steps require a local copy.

The extracted `adf/dpaint` program identifies itself as “Deluxe Paint - Version 2.0P”, copyright 1986–1987 Daniel Silva and Electronic Arts. It is 191,428 bytes; SHA-256: `b775e59d9fdf9c6038be3383f10781b4b065b3d4b301d7907629781bb4b64581`. The string `Release 2.48` also appears but should not be confused with the program's displayed version.

Extraction is reproducible with the repository's standard Go utility:

```sh
go run cmd/adfextract/main.go previous/Deluxe_Paint_II_1987_Electronic_Arts_PAL.adf reference/adf
```

The utility validates OFS block checksums, file sizes, and the absence of cycles in file chains. All disk entries were extracted. `adf/` also contains the disk's Workbench commands, devices, and libraries for inspection; they are not needed by the Go application.

An additional primary source is the [Electronic Arts DeluxePaint II manual](https://d1yx3ys82bpsa0.cloudfront.net/atchm/documents/DeluxePaint_II_manual.pdf), available in the local inspection workspace as `DeluxePaint_II_manual.pdf`, with 152 pages. Printed page numbers differ from PDF page numbers. Figure 1.1 on printed page 1.5 corresponds to PDF page 25; the reference section starts on PDF page 105. `manual-toolbox.png` shows the figure and its caption. Data extracted from the executable takes precedence when it differs from the manual.

Reference extracts, JSON inspection reports, previews, and the downloaded manual are generated local artifacts and are excluded from Git. The original ADF, this document, and embedded runtime assets in `assets/` are tracked.

## Confirmed interface

The panel sits to the right of the painting area. At the top are ten preset brushes: four round, four square, and two dotted. Below them are two columns of tools, ordered from top to bottom:

| Left | Right |
|---|---|
| Dotted Freehand | Continuous Freehand |
| Straight Line | Curve |
| Fill | Airbrush |
| Unfilled/filled Rectangle | Unfilled/filled Circle |
| Unfilled/filled Ellipse | Unfilled/filled Polygon |
| Brush Selector | Text |
| Grid | Symmetry |
| Magnify | Zoom |
| UNDO | CLR |

Below the tools are a circular foreground indicator over the background color and a 32-color palette arranged in four columns and eight rows. The left button paints and selects the foreground color; the right button paints and selects the background color. Right-clicking the indicator opens the palette; right-clicking a preset brush adjusts its size.

Menus are accessed by holding the right button at the top of the screen. They open as the pointer passes over them, submenus extend to the right, and commands are selected on release. When idle, the bar shows the current mode, possibly `S` for the stencil, `B` for a fixed background, coordinates, or perspective angles. F9 hides the bar; F10 hides both the bar and panel.

## Menus

Confirmed order: **Picture, Brush, Mode, Effects, Font, Prefs**. The following labels come from the executable or primary manual; case-sensitive shortcuts are preserved.

- **Picture**: Load, Save, Delete, Print; Color Control → Palette (`p`), Use Brush Palette, Restore Palette, Default Palette, Cycle (`TAB`), Bg → Fg, Bg ↔ Fg, Remap; Spare → Swap (`j`), Copy To Spare, Merge in front, Merge in back, Delete this Page; Page Size, Show Page (`S`), Screen Format, Quit, About.
- **Brush**: Load, Save, Delete; Size → Stretch (`Z`), Halve (`h`), Double (`H`), Double Horiz, Double Vert; Flip → Horiz (`x`), Vert (`y`); Rotate → 90 Degrees (`z`), Any Angle, Shear; Change Color → Bg → Fg, Bg ↔ Fg, Remap; Bend → Horiz, Vert; Handle → Center, Corner.
- **Mode**: Matte (`F1`), Color (`F2`), Replc (`F3`), Smear (`F4`), Shade (`F5`), Blend (`F6`), Cycle (`F7`), Smooth (`F8`).
- **Effects**: Stencil → Make, Remake, Lock FG, Reverse, On/Off, Free, Load, Save, Delete; Background → Fix, Off; Perspective → Do, FillScreen, Reset, Center, Anti-Alias (None, Low, High), Rotation (Absolute, Relative).
- **Font**: Style → Bold, Italic, Underln; Load Font Dir, followed by families and sizes loaded from disk.
- **Prefs**: Coords, Fast FB, MultiCycle, Be Square, Workbench, ExclBrush.

The fixed background created by **Background → Fix** is a copy of the current picture. Additions are drawn over it, while **CLR** and right-button painting restore that background in erased areas. **Off** frees the copy and restores ordinary erasing across the page. **Stencil → Lock FG** protects areas painted since Fix regardless of their colors (manual, pages 4.17–4.18). The fixed background therefore cannot be represented as a single solid color.

`dpaint-strings.txt` records offsets for labels, requesters, and other useful strings in the executable, including the range `0x1c03e–0x1c379`. The initial Picture/Brush labels and Load/Save/Delete operations reuse requester strings instead of repeating them in that block.

## Tool shortcuts

| Key | Action |
|---|---|
| `s`, `d`, `D` | Dotted, continuous, and one-pixel continuous freehand |
| `v`, `q`, `f`, `F` | Line, curve, fill, fill settings |
| `r` / `R`, `c` / `C`, `e` / `E` | Rectangle, circle, ellipse: outlined / filled |
| `b`, `B`, `t` | Capture brush, previous captured brush, text |
| `u`, `K` | Undo, clear |
| `m`, `<`, `>` | Magnifier, zoom out/in |
| `g`, `G`, `/` | Grid, grid aligned to brush, symmetry |
| `,`, `.`, `[`, `]` | Eyedropper, one-pixel brush, previous/next range color |
| `-`, `=` | Smaller/larger brush |
| `n`, arrow keys | Center under cursor, pan page |
| `a`, Space | Repeat the last menu command, cancel the current operation |

Shift constrains lines and shapes; Ctrl leaves brush trails while drawing these tools. The manual assigns F8 to the cursor, but this disk explicitly assigns F8 to Smooth; Pixeluxe should follow the disk version here. The provided table does not establish a polygon shortcut.

## Original pictures and palettes

The fifteen graphical files are IFF `FORM ILBM` with five planes and 32 colors, using either uncompressed data or ByteRun1 compression. The inspection artifact `asset-manifest.json` records dimensions, palette, transparency, compression, `GRAB` handle coordinates, `CRNG` cycles, and SHA-256 hashes. `previews/` contains decoded PNG files for visual comparison. CMAP bytes align to four bits; previews map hardware levels 0–15 to 0–255 by multiplication by 17.

| File under `adf/` | Dimensions |
|---|---|
| Lo-Res/Seascape | 320 × 200 |
| Lo-Res/StencilSet | 320 × 200 |
| Lo-Res/Reference Palette | 320 × 200 |
| Brush/Archbrush | 147 × 37 |
| Brush/Bobsled | 24 × 27 |
| Brush/Building | 53 × 106 |
| Brush/Dolphin | 103 × 128 |
| Brush/Pattern1 | 18 × 15 |
| Brush/fireworks | 107 × 104 |
| Brush/anim1, anim2, anim3 | 32 × 19; 83 × 65; 49 × 200 |
| Brush/anim4, anim5, anim6 | 319 × 54; 81 × 70; 93 × 45 |

The Med-Res, Interlace, and Hi-Res directories contain no pictures on this disk. Lo-Res examples were saved at 320 × 200 with a 10:11 pixel aspect ratio; this does not imply that the application's PAL mode is limited to 200 lines. The executable contains NTSC/PAL heights of 200/400 and 256/512, and Building uses the 320 × 256 format.

## Fonts and research limits

The inspection artifact `fonts.json` contains the original bitmap glyphs, widths, advances, kerning, and baselines. Font specimens are in `previews/fonts/`. Families: ruby 8/12/15, opal 9/12, sapphire 14/19, diamond 12/20, garnet 9/16, emerald 17/20, and topaz 11. Character codes are Amiga/Latin-1, generally `0x20–0xff`.

The original program opens `topaz.font` for its interface. The ROM's 8-pixel system font is absent from the ADF; topaz 11 is the only disk variant. The disk alone therefore cannot directly recover the font for every menu or prove its colors and exact pixel placement without running the software in a suitable Amiga environment. The precise order of some items is reconstructed from the manual and strings; no screenshot of this ADF running has been claimed. Extracted pictures and fonts provide the reference for a native Go reimplementation without emulating the 68000 executable. Pixeluxe's modern interface is an adaptation; its optional Classic view retains the original remake's compact layout.
