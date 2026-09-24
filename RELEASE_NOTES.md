# Changes since the last release

## New & Improved

- The accessible name a control takes from the label laid out before it is now read the way that label names itself,
  so a label whose `Accessibility.Name` differs from the text it draws — one drawn in small caps, say — names its
  neighbor in the same words. A custom panel that declares `role.Label` and an `Accessibility.Name` now serves as such
  a label too, a popup menu titles its menu after its `Accessibility.LabeledBy` label, and an icon-only table column
  header is named by its tooltip.
- `NewSmallCapsText` now keeps the text as it was written and capitalizes only what it draws, through the new
  `TextDecoration.SmallCaps`, so `Text.String()` and what a screen reader is told say "Title" rather than "TITLE".
- Every panel can now have a contextual menu. Set the new `ContextMenuCallback` on `InputCallbacks` to build one, and
  the window opens it for a right-click on the panel, for the Menu key or shift+F10 while the panel holds the keyboard
  focus, and at the request of an assistive technology. The callback returns the menu, or nil when there is nothing to
  offer just now; the window pops it up and disposes of it, and never shows an empty one. Only the panel under the
  pointer or holding the focus is asked, never an ancestor of it, so a right-click on a panel without a menu is an
  ordinary press even within a panel that has one. The new `Panel.ShowContextMenu(where) bool` opens the menu from
  code, in the active window only, and reports whether one was shown.
- A menu opened from the keyboard or by an assistive technology has no pointer position, so it opens where the widget's
  new `ContextMenuAnchorer` says, or else at the center of the visible part of the panel, which the new
  `Panel.DefaultContextMenuAnchor` returns. A custom widget that must act on the right-click before its menu opens, as
  a table selects the row under the pointer, implements the new `ContextMenuPressHandler`; one that has a callback but,
  for a while, no menu to offer at all implements the new `ContextMenuWithholder` and is then treated as having no
  callback.
- A right-click on a panel with a `ContextMenuCallback` is not delivered to the panel's mouse callbacks: the panel takes
  the keyboard focus on the press if it can hold it, and the menu opens on the release. A right press that becomes a
  drag opens no menu; a widget that wants such a drag delivered to its mouse callbacks after all asks for it through
  `ContextMenuPressHandler`.
- The rows and cells of a `Table` and the rows of a `List` with a `ContextMenuCallback` offer the menu too. A
  right-click selects the row under the pointer, unless it is already selected, in which case the whole selection is
  kept for the menu to act on. A widget with a menu of its own in a `Table` cell, such as a `Field`, opens its own menu
  rather than the table's when it can take the focus.
- `Field` now builds its menu through the callback: `NewField` installs the new `Field.DefaultContextMenu` as the
  `ContextMenuCallback`, so an application can replace or wrap it. `Markdown` no longer has a contextual menu of its
  own: the Copy and Select All menu of a document being read is gone, and those commands remain available through the
  application's Edit menu. A menu given to a document through `ContextMenuCallback` opens beneath the reading caret.

## Breaking API changes

- `Field.ShowContextMenu(where)` and `Markdown.ShowContextMenu(where)` are gone in favor of
  `Panel.ShowContextMenu(where) bool`. A call against either still compiles but now goes through the
  `ContextMenuCallback`: a `Markdown` without one shows nothing, where it used to show Copy and Select All, and a
  `Field` shows nothing unless it holds the keyboard focus, where it used to show Cut, Copy, Paste and Select All
  regardless. A method value or interface written against the old `func(geom.Point)` signature no longer matches.
- Since `NewField` installs a `ContextMenuCallback`, a `MouseDownCallback`, `MouseDragCallback` or `MouseUpCallback`
  set on a field no longer sees the right button.

## Bug Fixes

- A dock tab alone in its container no longer swallows a right-click, and a right-click on a tab's close button no
  longer closes the dockable.
- `List`'s `DoubleClickCallback` now fires only for a left-button double-click, as `Table`'s does.
- A right or middle click on one of several selected rows of a `Table` or `List` no longer collapses the selection to
  that row; only a left click does.
- Clicking a `Table` while a `Field` in one of its cells is being edited no longer panics when the field, committing as
  it loses the focus, removes the row that was clicked.
