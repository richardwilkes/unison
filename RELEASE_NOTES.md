# Changes since the last release

## New & Improved

- Added the `TableRowAccessibleText` interface, which a table row may implement to say what an assistive technology
  should call each of its cells. Otherwise a table names them from `CellDataForSort`, so a row that puts an ordering
  marker ahead of its text would have that read out too.
- A headless session now behaves the same on every host. Where the platforms differ it follows the convention Windows
  and Linux share: `mod.OSMenuCommand()` returns `Control`, `Modifiers.String()` produces `Ctrl+Shift+` rather than the
  macOS glyphs, `MouseWheelMultiplier` is the Windows and Linux value, and the quit menu item is titled `Exit`. Tests
  that asserted on the macOS forms of any of these will need updating. `mod.SetPlatformNeutral()` selects the same
  modifier convention outside a session, and `mod.PlatformNeutral()` reports whether it is selected.
- A headless session now describes windows to `AccessibilityTree()` as Linux does, whatever the host: a flat table has
  the `Table` role, headings and a focusable `Markdown` take the focus while an assistive technology is active, and a
  window with nothing focused is given one when accessibility support turns on. Tests that asserted on the macOS or
  Windows answers will need updating.
- Added `OpenBrowser()`, which `DefaultMarkdownLinkHandler` now calls. Call it in place of `xos.OpenBrowser()` from your
  own link handlers: a headless session records the request instead of launching a browser, and the new
  `HeadlessScreen.OpenedURLs()` hands the recorded URLs back to the test.

## Bug Fixes

- (none yet)
