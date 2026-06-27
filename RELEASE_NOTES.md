# Changes since v0.92.3

## New & Improved

- Added `Table.SyncRowHeights()` — a new public method that recalculates cached row heights based on current column
  widths. Previously this logic was inlined in the three SizeColumns* methods; it is now extracted so callers who adjust
  column widths directly (outside those methods) can trigger the same recalculation.
- Added additional table theme fields to determine whether the first & last divider lines are drawn. Default is to
  enable them both.

## Bug Fixes

- The dock header now calls `FlushDrawing()` after `MarkForRedraw()` during drag-update and drag-exit events, so the
  tab insertion indicator will actually show up, as there is no continuous redraw loop during native drag & drop.
- `LabelContentSizes` now reports the same height for an empty line as for a line containing text. Previously the
  empty-text height was taken from the passed-in `font` parameter while the height for text came from the text's own
  `TextDecoration` font, so the two could disagree when those fonts differed. The single-line height is now derived from
  the text itself (falling back to the `font` parameter only when there is no text object), guaranteeing that a line
  with text and an empty line are the same height.
- The dock divider position is now clamped only for layout purposes. Previously, it would be set to whatever value it
  had been clamped to as you sized the view. The new behavior means the divider will restore itself if you shrink the
  view down, the grow it back to where it was.
- Linux only: Window frame border widths are now detected more reliably. `_NET_REQUEST_FRAME_EXTENTS` is only sent (and
  waited on) when the window manager advertises support for it via `_NET_SUPPORTED`, avoiding a needless stall at window
  creation under window managers that ignore it (such as bare xwayland); when it is supported, the wait timeout was
  raised so a busy window manager has time to respond. Border widths are no longer cached as valid until the window
  manager has actually reported them, so the zero placeholder is no longer mistaken for a real frame size. The content
  rect is also held at its last known value while awaiting the `ConfigureNotify` for a pending resize, preventing reads
  of stale window geometry.
- Windows only: Fix mouse wheel events when on a display positioned to the left of or above the primary display.
- Windows only: Custom cursors are no longer baked at a single monitor's DPI. They were previously rasterized once at
  the primary display's scale and reused everywhere, so on a secondary monitor with a different DPI the cursor appeared
  the wrong size and never corrected. A correctly sized cursor is now produced for whichever monitor's DPI it is shown
  on, and refreshed when a window moves between monitors.
- Windows only: When a window is dragged between monitors with differing DPI, it now snaps to the exact correct size.
  The size computed in response to `WM_GETDPISCALEDSIZE` was scaling the whole window rect instead of just the client
  area, overshooting by roughly half the window frame.
- Windows only: Packaged apps now declare Per-Monitor V2 DPI awareness in their manifest, matching what the runtime
  requests. Previously the manifest declared system DPI awareness, which takes precedence over the runtime request and
  left distributed apps blurry and incorrectly scaled on secondary monitors with a different DPI.
