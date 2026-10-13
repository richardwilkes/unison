# Changes since the last release

## New & Improved

- (none yet)

## Bug Fixes

- A menu opened from a title on the in-window menu bar that is taller than the space beneath the bar is now shrunk to fit there and scrolls its items, rather than being moved up over the bar.
- `PopupMenu.Click()`, the dropdown of a field built with `NewComboField()` or `Field.InstallDropdown()` (whether opened by its button or by the down arrow key), and the "More Tabs" button of a dock header no longer open their menu when the widget's window is not the active one, and `PopupMenu.WillShowMenuCallback` is not run. The menu is built in the active window, so it previously appeared there, at coordinates taken from the widget's own window, when a screen reader acted on a background window or code opened one of these from it. A screen reader is no longer offered the press that would be refused.
- A menu item that has a sub-menu, including a title on the in-window menu bar, no longer opens it when the item's window is not the active one, whether asked by a screen reader to press or expand the item or by code. The sub-menu is built in the active window, so it previously appeared there at coordinates taken from the item's own window. A screen reader is no longer offered the press or expansion that would be refused; collapsing an open sub-menu is unaffected.
