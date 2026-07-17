# Changelog

## v0.1.13 - 2026-07-17

- add `RenderInline` / `renderer.NewInlineTerminal`: a NO-ALT-SCREEN render
  mode that renders on the MAIN screen and never emits the alt-screen
  enter/leave (mode 1049), so a host that already owns an alternate screen
  (e.g. a PTY wrapper around a full-screen child) keeps it untouched. Cleans
  up its own main-screen lines on exit.
- readLoop no longer leaks: `Close` signals the stdin reader (via a `done`
  channel) so a goroutine parked on delivering an event exits instead of
  blocking forever on a channel nobody reads after Render returns.

## v0.1.12 - 2026-07-04

- maintenance release (no notable changes)

## v0.1.11 - 2026-07-04

- demo.tape

## v0.1.10 - 2026-07-04

- maintenance release (no notable changes)

## v0.1.9 - 2026-07-04

- release.sh

## v0.1.8 - 2026-07-04

- maintenance release (no notable changes)

## v0.1.7 - 2026-07-04

- release v0.1.6
- release v0.1.6
- release v0.1.5
- release v0.1.4
- release v0.1.3
- docs: add README
- release v0.1.2
- release v0.1.1
- 4th of July
- 4th of July
- Initial commit
