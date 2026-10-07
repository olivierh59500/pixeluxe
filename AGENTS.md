# Pixeluxe development

Pixeluxe is a native, pure Go pixel art editor inspired by Deluxe Paint II on
Amiga. Preserve the indexed image model, original assets, drawing tools, menus,
keyboard shortcuts, image formats, and undo behavior when changing the interface.

## Language

- Write source comments, documentation, UI labels, and Git commit messages in
  English.
- Use clear, conventional commit messages, such as
  `feat: modernize the editor interface` or `fix: preserve indexed brush colors`.
- Keep conversations with the user in their preferred language.

## Implementation and validation

- Keep application code in Go and support desktop builds with `CGO_ENABLED=0`.
- Keep codecs and painting operations independent of the desktop UI.
- Preserve palette indices and nearest-neighbor rendering during edits and zoom.
- Run `make check` after substantive code changes. It runs the Go tests and
  `go vet` with CGO disabled.
- Run `make preview` after visual changes and inspect `docs/pixeluxe.png`.
- Use a native smoke run when changing pointer routing or the window loop:
  `./bin/pixeluxe -demo -smoke 120 -capture /tmp/pixeluxe-smoke.png`.
- Document material compatibility limits instead of claiming complete Amiga
  parity.

## Local repository

- Use the local `main` branch and keep changes in focused commits.
- Do not configure a remote, push, or publish unless the user requests it.
- Track source files, embedded runtime assets, and documentation screenshots.
- Keep `previous/` local and ignored. Never commit reference disk images or
  reintroduce their removed history.
- Keep generated executables and app bundles in ignored `bin/`.
- Do not commit reference extracts, downloaded manuals, temporary files, or
  derived reference previews. They are reproducible inspection artifacts; the
  runtime assets in `assets/` remain tracked.
- Do not overwrite unrelated changes or use destructive Git cleanup commands.
