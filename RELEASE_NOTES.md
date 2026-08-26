# Changes since the last release

## New & Improved

- Added a headless mode for testing user interfaces without a display. `unison.Headless(cfg)` (or the more convenient
  `unison.StartHeadless(cfg, ...)`) runs the real event loop against an in-memory screen of the given size and scale,
  and the returned `*unison.HeadlessScreen` drives it from the test: it injects mouse and keyboard input in screen
  coordinates, simulates drag & drop (both drags started by the application and data arriving from outside it),
  captures the rendered screen or a single window as an image, and ends the session so that another can be started in
  the same process. Rendering uses the CPU raster path, so no OpenGL or window server is needed, which makes it usable
  in CI.

## Bug Fixes

- (none yet)
