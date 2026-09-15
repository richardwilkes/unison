# Changes since the last release

## New & Improved

- Added screen-reader support on macOS (VoiceOver), Windows (Narrator, NVDA, JAWS) and Linux (Orca). Every widget
  describes itself, so an existing application becomes largely accessible without being changed. Nothing runs until an
  assistive technology asks for it, so an application nothing is listening to pays essentially nothing.
- Added `Panel.Accessibility`, through which an application supplies what cannot be derived from a panel: a `Name` for
  an icon-only control, a `LabeledBy` for a control whose label is not the sibling before it, a `Role` override, and so
  on. Set its fields individually rather than assigning the struct as a whole.
- Custom widgets describe themselves by implementing `AccessibilityProvider` and carry out requests from an assistive
  technology by implementing `AccessibilityActor`, both of which must be implemented by the widget type rather than the
  embedded `Panel`. The new `accessibility` and `enums/role` packages hold the types involved.
- Added `AnnounceForAccessibility()`, which speaks a message that no change to a window expresses, such as a background
  task finishing. It may be called unconditionally and from any goroutine.
- Added the `UNISON_ACCESSIBILITY` environment variable and the `NoAccessibility()` startup option to force
  accessibility support on or off, along with `SetAccessibilityEnabled()`, `AccessibilityEnabled()` and
  `IsAccessibilityActive()` for controlling and inspecting it at runtime.
- Added `HeadlessScreen.AccessibilityTree()`, `AccessibilityNodeFor()`, `AccessibilityEvents()`, `Announcements()` and
  `PerformAccessibilityAction()`, so tests can assert on what a screen reader would be told with no display involved.
- `Slider` is now focusable, so it is usable without a mouse and takes part in the tab order. It takes the keyboard
  focus when clicked or tabbed to, steps its value with the arrow keys and jumps to the ends of its range with Home and
  End. While focused, its edge is drawn with the new `SliderTheme.SelectionInk`. Its reported height now includes its
  edge thickness, so sliders using the default theme are two pixels taller.

## Bug Fixes

- `ComboField` no longer opens an empty dropdown when it was built with no options and is clicked or the Down arrow is
  pressed, and no longer describes itself to assistive technologies as expandable.
- `Table` no longer draws disclosure triangles, or indents rows, while a flat `ApplyFilter` is in effect. The rows a
  flat filter shows have no hierarchy, so the triangles opened and closed nothing.
- On Linux, `Window.MovedCallback` no longer fires spuriously under a reparenting window manager. A synthetic
  ConfigureNotify already carries a root-relative position, and translating it again added the frame's origin twice.
- On Linux, only the desktop portal can now change the application's light or dark appearance: the D-Bus match rule
  for the color-scheme setting names the sender, so another process on the session bus can no longer flip it.
- `ScrollPanel` no longer scrolls unnecessarily when asked to bring a rect into view and it has a column or row header.
  The headers were being set aside twice, so a row lying in the strip along the bottom of the view, in plain sight,
  was scrolled up out of it whenever it was selected.
