# Changes since v0.94.0

## New & Improved

- `Markdown` now generates anchor IDs for headings and resolves in-document links. Clicking a link whose target begins
  with `#` scrolls the matching heading to the top of the view rather than invoking the external link handler. Matching
  is attempted case-sensitively first, then case-insensitively, and the anchor may be URL-escaped. This is also exposed
  programmatically via the new `Markdown.ScrollToAnchor` method.

## Bug Fixes
