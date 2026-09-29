# Changes since the last release

## New & Improved

- Added `SetFocusForReading()` and `FocusForReading()`. While an assistive technology is being served, the switch makes
  standalone static text and disabled controls tab stops, so that a person using a screen reader in its focus mode can
  reach them with Tab. A disabled control that takes the focus this way stays disabled in every other respect: no event
  is delivered to it, no menu command acts on it, `Enabled()` goes on reporting that it is disabled, and a screen reader
  announces it as unavailable. Nothing changes for anyone no assistive technology is serving, so the switch may be left
  on unconditionally. `AccessibilityInfo.FocusForReading` asks the same of one panel.
- A link created with `NewLink()` is now a tab stop for every keyboard user. It outlines itself while it holds the
  focus, is followed by Return, the keypad's Enter and the space bar as well as by a click, and takes the focus when
  clicked while an assistive technology is being served. Return pressed on a link in a dialog follows the link rather
  than pressing the default button. It also has a border that leaves room for the outline, which makes it 4 pixels
  wider than its text. Call `SetFocusable(false)` on a link that should be followed by a click alone, and
  `SetBorder(nil)` as well for one that should take up no more room than its text. A link that is, or is inside, a
  cell of a `Table`, `List` or `TableHeader` takes no focus, and the links of a `Markdown` are neither tab stops nor
  any wider than they were: the document follows the link under its reading caret, and a click on one now gives the
  document the focus while it can take it.
- `Panel.CanPerformCmd()` and `Panel.PerformCmd()` now pass over a panel that is not enabled and go on to its parent,
  as the delivery of a key does, so a menu command or its key equivalent no longer acts on a disabled panel that holds
  the keyboard focus.
- The hints that follow the fields of a `ColorEditor`, and the unit that follows the radius fields of a
  `GradientEditor`, are now given to an assistive technology as the description of the field they belong to rather than
  as unavailable text of their own.

## Bug Fixes

- The release of a key is no longer delivered to the `KeyUpCallback` of a panel that is disabled.
- The dropdown button of a disabled combo field no longer opens its menu, which let the value of the disabled field be
  changed.
- A `GradientEditor` field that holds the keyboard focus while disabled is now kept up to date with the gradient.
- A click on a button, check box, radio button, popup menu or well that cannot take the focus no longer clears the
  window's focus while an assistive technology is being served.
