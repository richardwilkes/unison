# Changes since the last release

## New & Improved

- Added screen-reader support on macOS (VoiceOver), Windows (Narrator, NVDA, JAWS) and Linux (Orca). Every widget
  describes itself, so an existing application becomes largely accessible without being changed. Nothing runs until an
  assistive technology asks for it, so an application nothing is listening to pays essentially nothing.

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
