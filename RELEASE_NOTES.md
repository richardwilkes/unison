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
