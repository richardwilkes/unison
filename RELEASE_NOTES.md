# Changes since the last release

## New & Improved

- (none yet)

## Bug Fixes

- Closing the last tab of a Dock while the keyboard focus was within it no longer leaves the window's focus naming a
  panel that is no longer in the window. The panel is told it lost the focus and the window reports nothing focused,
  so the application can place the focus afresh.
- Moving a Dock tab to another place in the same window, whether by dragging it or through Dock.DockTo or
  DockContainer.Stack, now keeps the keyboard focus on the panel within the tab that held it, rather than putting it
  on the tab's first focusable panel. A tab moved to a Dock in another window still releases the focus in the window
  it left, and the receiving Dock places it within the tab as before.
- Tab and shift-Tab now move the focus into the window when nothing holds it, seeding it at the first or last tab
  stop, instead of being ignored. Window.FocusNext and Window.FocusPrevious, the seeding a window does as it becomes
  the active one, and the seeding done when a screen reader arrives, all now treat a focus that names a panel removed
  from the window as no focus at all, so the usual seeding rules apply to it: a real tab stop is preferred over a
  panel that takes the focus only for an assistive technology, and a focusable content panel is passed over.
