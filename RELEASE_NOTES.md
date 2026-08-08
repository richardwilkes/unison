# Changes since v0.97.0

## Enhancements

- (none yet)

## Bug Fixes

- An SVG containing a `<use>` element that reached itself, either directly or through other definitions (e.g.
  `<g id="a"><use href="#a"/></g>`), recursed until the stack overflowed, killing the process outright rather than
  failing in a recoverable way. Such a document is now rejected with an error, as is one whose `<use>` elements nest
  more than 32 deep.
- `NewComposeColorFilter()` panicked with a nil pointer dereference when either of its filters was nil, even though
  this package's own constructors legitimately return a nil `*ColorFilter` for filters that would do nothing — for
  example, `NewBlendColorFilter()` with `blendmode.Dst`, or with a zero-alpha color and `blendmode.SrcOver`. Composing
  with such a filter now yields the other filter, and composing two of them yields nil.
- `DockState.Apply()` left `Dock.MaximizedContainer` pointing at a container it had just removed. Applying a saved
  state while a container was maximized therefore hid every restored container, since none of them was that stale
  pointer, leaving the dock blank with no visible way to recover. Any maximized container is now restored before the
  saved state is applied.
- Shift-clicking in a `Field` moved the selection anchor — the gesture's fixed end — to the click position, so a
  following drag or second shift-click extended from the wrong end and visibly dropped the previously selected range
  mid-gesture. The anchor now stays put; only an unshifted click moves it.
- Shift+Up and Shift+Down in a `Field` destroyed the selection whenever the moving end crossed the anchor — for
  example, Shift+Right followed by Shift+Up in a multi-line field — collapsing it to an empty caret instead of
  flipping it past the anchor. The selection now flips, in the by-word variants as well.
- In the common (non-native) open dialog, a multiple selection containing an item the dialog was not allowed to choose
  — such as a directory when `SetCanChooseDirectories(false)` — still enabled the OK button as long as some later item
  in the selection was valid, and that item's path was then returned from `Paths()`. Every selected item must now be
  choosable for OK to be enabled.
- In the common (non-native) save dialog, pressing Return in the file name field accepted the dialog based only on
  that field being non-empty, without rebuilding the path from it. Navigating to another directory, double-clicking
  into one, or selecting one in the file list all left the stored path stale, so `RunModal()` could report success
  while `Path()` returned a path in the previously shown directory, a directory itself, or an empty string — even
  though the OK button was visibly disabled. Return now rebuilds the path from the name field and the directory
  currently showing, and accepts only when the OK button would.
- `FontFamily.MatchStyle()` picked the wrong face from an internally registered family when a wider-than-Standard
  spacing was requested and no face matched all three of weight, spacing and slant. A face whose spacing matched
  exactly was scored as though it did not match at all, so any wider face — an UltraExpanded face against a request for
  SemiExpanded, say — outranked it in the spacing tier, which outranks slant and weight. Exact spacing matches now
  score as matches.
- Windows only: the URLs reported for dropped files were built by concatenating the raw path into a `file://` string
  and parsing it, so characters that are legal in a filename but significant in a URL came back wrong. A `#` or `?`
  cut the path short and reappeared as a fragment or query, a `%xx` sequence was decoded as an escape, and a `%`
  followed by anything else failed to parse at all, dropping the file from the results entirely. The path is now
  escaped rather than interpreted.
