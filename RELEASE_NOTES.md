# Changes since v0.92.3

## Bug Fixes

- `LabelContentSizes` now reports the same height for an empty line as for a line containing text. Previously the
  empty-text height was taken from the passed-in `font` parameter while the height for text came from the text's own
  `TextDecoration` font, so the two could disagree when those fonts differed. The single-line height is now derived from
  the text itself (falling back to the `font` parameter only when there is no text object), guaranteeing that a line
  with text and an empty line are the same height.
