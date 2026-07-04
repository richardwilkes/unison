# Changes since v0.93.0

## New & Improved

- `Field` now shows a context menu on right-click containing the standard Cut, Copy, Paste and Select All actions. Only
  the actions that can currently be performed are included, and if none of them apply, no menu is shown. The menu can
  also be triggered programmatically via the new `Field.ShowContextMenu` method, and the mouse up handling that
  triggers it is exposed as `Field.DefaultMouseUp`.
- Linux only: Windows now set the `WM_CLASS` property (the ICCCM instance and class names) when created. The instance
  name is the application's command name (falling back to the application name) and the class name is the application
  identifier (falling back to the instance name). This lets desktop environments associate a window with its `.desktop`
  file — whose `StartupWMClass` entry should match the class name — so the taskbar/dock shows the correct icon and
  groups the application's windows together.

## Bug Fixes

- Right-clicking a dock tab popped up its context menu during mouse down handling. The menu swallowed the matching
  mouse up event, leaving the window convinced the right button was still pressed, so all mouse movement was routed as
  a drag (and cursor and tooltip updates stalled) until the next click. The context menu is now shown on mouse up
  instead. As part of this, right-clicking a tab whose container holds only a single dockable no longer falls through
  to the left-click behavior of activating the tab.
- If Linux or Windows reports a list of monitors but marks none of them as the primary, the first one is now treated as
  the primary display. In addition, the places where unison itself uses `PrimaryDisplay()` now guard against a nil
  return.
- Linux only: The X11 modifier state translation reported Mod2 — which is NumLock on essentially all X11 configurations
  — as the Command modifier. With NumLock engaged, every key and mouse event carried a phantom Command modifier: menu
  command key sequences were ignored, character input was suppressed entirely, plain clicks in tables and lists behaved
  like Ctrl-clicks (toggling the row under the mouse rather than selecting it), and the keypad could not produce
  digits. Mod2 is now reported as NumLock, and Mod4 (the actual Super/Windows key) is now reported as Command, where
  previously it was not reported at all.
- Key handling paths that compared event modifiers for exact equality now mask out the lock (sticky) modifiers first,
  so having NumLock (Windows) or CapsLock (all platforms) engaged no longer prevents matches. This affected menu
  accelerator matching (menu command key sequences were ignored), key navigation within open menus, the fallback
  cut/copy/paste/select-all handling in `Field` when no menu is present, and Return/Enter in the file dialog's file
  name field.
