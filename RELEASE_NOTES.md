# Changes since the last release

## New & Improved

- `Window.ToFront()` now performs the platform activation at the end of the current pass of the event loop rather than
  from inside the callback that asked for it, and collapses repeated requests made within one pass into the last one.
  Bringing a window to the front from a button, menu or other callback no longer races the event still being
  dispatched, and a sequence such as a menu closing and then a dialog opening activates only the dialog instead of
  briefly raising the window underneath it on the way. `ActiveWindow()` and `FrontmostWindow()` report a window whose
  activation is pending as the active one.
- Menu item handlers now run from the event loop once the menu has closed, rather than from inside the mouse or key
  event that chose the item, so a handler that opens a window or a modal dialog behaves the same on every platform.
  Wrapping such work in `InvokeTask()` is no longer necessary. On macOS, this means a popup's selection callback fires
  once `Popup()` has returned rather than from inside it; a handler must not retain the `MenuItem` it is handed.

## Bug Fixes

- A modal dialog opened from a callback could fail to take the keyboard focus, most often on Windows and sometimes on
  macOS: the press that opened it still owned the platform's mouse capture while the dialog's event loop ran, and the
  activation was requested from inside an event that had not finished being dispatched. Dialogs now cancel any press
  in progress before their loop starts, as the platform's own dialogs do.
- Windows: the mouse capture taken on button-down is now released before the final button-up is delivered rather than
  after, so a nested event loop started from a mouse-up handler routes input normally.
