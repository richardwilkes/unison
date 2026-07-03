# Changes since v0.93.0

## New & Improved

## Bug Fixes

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
