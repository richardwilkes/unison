# Changes since v0.96.1

## Enhancements

- Cursors now have a runtime-configurable size and colors. `CursorSize()` and `SetCursorSize()` control the logical
  size the built-in cursors are built at, while the new `ThemeCursorForeground` and `ThemeCursorBackground` theme
  colors control the color of their body and linework and of their outline and halo, respectively. Changing any of
  them — including as part of a theme change — rebuilds the built-in cursors and immediately updates every open
  window. Client code can follow along by creating its own cursors with the new `NewThemedCursorFromSVG()` function
  and registering a `CursorChangedCallback()`, which is called after the built-in cursors have been rebuilt but before
  open windows are refreshed, so replacement cursors created within it are picked up immediately.
- Cursors are now rasterized from their vector art at each display's backing scale rather than being scaled from a
  single fixed-size bitmap, so they are sharper on Windows systems using fractional DPI scaling and on non-retina
  macOS displays.
- `SVG` gained `DrawInRectWithReplacementInks()` and `DrawInRectPreservingAspectRatioWithReplacementInks()`, which
  draw an SVG while substituting a different ink for each of a set of colors found in the source art. This is what
  allows the cursor art — drawn entirely with pure black for the foreground and pure white for the background — to be
  recolored.

## Bug Fixes

- The text cursor's artwork contained a stray zero-width segment at the top-left corner of its lower-left arm, which
  the outline stroke rendered as a small notch in the cursor's body, making that corner asymmetric with the rest of
  the cursor. The stray segment has been removed.
