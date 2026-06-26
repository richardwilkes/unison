# Changes since v0.92.3

## New & Improved

- Added `Table.SyncRowHeights()` — a new public method that recalculates cached row heights based on current column
  widths. Previously this logic was inlined in the three SizeColumns* methods; it is now extracted so callers who adjust
  column widths directly (outside those methods) can trigger the same recalculation.

## Bug Fixes

- `LabelContentSizes` now reports the same height for an empty line as for a line containing text. Previously the
  empty-text height was taken from the passed-in `font` parameter while the height for text came from the text's own
  `TextDecoration` font, so the two could disagree when those fonts differed. The single-line height is now derived from
  the text itself (falling back to the `font` parameter only when there is no text object), guaranteeing that a line
  with text and an empty line are the same height.
