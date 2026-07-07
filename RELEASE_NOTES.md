# Changes since v0.94.0

## New & Improved

- `Markdown` now generates anchor IDs for headings and resolves in-document links. Clicking a link whose target begins
  with `#` scrolls the matching heading to the top of the view rather than invoking the external link handler. Matching
  is attempted case-sensitively first, then case-insensitively, and the anchor may be URL-escaped. This is also exposed
  programmatically via the new `Markdown.ScrollToAnchor` method.

## Bug Fixes

- Tab and Shift-Tab moving focus between fields compared the event modifiers without masking out the lock (sticky)
  modifiers, so having NumLock (on by default on most Windows keyboards) or CapsLock engaged prevented the match and
  suppressed focus traversal entirely. The comparison now masks with `mod.NonSticky`, consistent with the other key
  handling paths.
- `ScrollPanel` could recurse into infinite frame-change handling and overflow the stack when collapsing blank space at
  the edge of the content. It repositioned the content and headers directly, leaving the scroll bar values stale; the
  following `Sync` then derived positions from those bars and undid the move, re-triggering the handler. It now adjusts
  the scroll bar values (the single source of truth) so `Sync` settles in a single pass.
