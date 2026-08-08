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
- Linux only: a screen number in `DISPLAY` (e.g. `:0.1`) was only checked for being non-negative, never against the
  number of screens the X server actually reported, so pointing at a screen the server does not have crashed the
  application with an index-out-of-range panic while the connection was still being established. Such a display is now
  rejected with an error naming the missing screen.
- Linux only: ColormapNotify events were decoded two bytes off, because three bytes were skipped after the event code
  rather than one, so every such event delivered garbage for its sequence number, window, colormap, and installed
  state.
- Linux only: requesting a transparent window on a system whose driver offers no transparency-capable framebuffer
  configuration failed outright instead of falling back to an opaque one. The fallback existed but was guarded by a
  condition that could never be true.
- Linux only: the numeric keypad typed digits into text fields regardless of NumLock, because the navigation keysyms
  the key produces with NumLock off (KP_Home, KP_Left, KP_Insert, and the rest) were mapped to the digits printed on
  their keycaps. Those keys now type nothing while they are acting as navigation keys, so the NumLock state finally
  governs whether the keypad produces characters.
- `List.RowRect()` computed a row's vertical position by multiplying that row's own height by its index instead of
  summing the heights of the rows above it. In a list whose cells differ in height — the default, when the cell
  factory reports a height below 1 — every row after the first was reported at the wrong offset, so keyboard
  navigation scrolled to the wrong place and could leave the newly selected row off-screen.
- `ValidateSaveFilePath()` skipped its overwrite confirmation whenever the path handed to it existed, even when
  applying the required extension had changed the path into a different, also-existing file. The native dialog had
  only ever prompted about the original path, so the real target was overwritten with no warning. The confirmation is
  now skipped only for the exact path the dialog itself presented.
- A `ScrollPanel` with a border of its own laid its children out from its frame rect rather than its content rect,
  even though the sizes it reported already accounted for the border insets. The content view, the headers and the
  scroll bars were all positioned and sized as though the border were not there, drawing on top of it.
- An SVG `transform` attribute whose operations were separated by a comma — `transform="translate(1,2), scale(2)"`, a
  form the spec's comma-wsp separator permits — failed to parse, and that error rejected the entire document. The
  parser split the list on the closing parenthesis and left the leading comma attached to the following operation's
  name, where it matched nothing.
- `Table.DefaultMouseUp()` cleared only the interaction row captured at mouse down, leaving the interaction column
  set. A drag arriving after the release — which happens for the remainder of a gesture whenever a second mouse button
  is still down — was therefore taken as a continuation of a column resize and overwrote the column's width using
  resize state left over from the finished gesture.
- Linux only: `IsMinimized()` and `IsMaximized()` always returned false. The WM_STATE and _NET_WM_STATE property
  handlers recorded the state reported by the window manager in a platform-private copy of the flags rather than in
  the fields those methods read.
- Windows only: the window frame insets were converted to logical units and then applied to positions, which are raw
  screen pixels. On a monitor scaled above 100%, `ContentRectForFrameRect(FrameRect())` therefore disagreed with
  `ContentRect()`, and every round trip through `ContentRect()` and `SetContentRect()` — `Pack()`, for one — moved the
  window by the insets times the scale minus one, walking it a little further across the screen on each call. The
  position and size portions of the insets are now each applied in the space they belong to.
- Windows only: `HideCursor()` displayed a 1x1 black dot rather than hiding the cursor. The blank cursor was built
  from zeroed AND and XOR masks, a combination Win32 renders as an opaque black pixel — transparency requires the AND
  bits to be set — and the one-byte mask buffers were also shorter than the WORD-aligned scanline `CreateCursor()`
  reads. The cursor is now created at the system cursor size with masks that are actually transparent.
