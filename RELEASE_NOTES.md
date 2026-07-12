# Changes since v0.95.0

## Bug Fixes

- Linux only: Dialog windows didn't set `WM_TRANSIENT_FOR`, so under XWayland the compositor could treat them as
  unparented, mishandling focus and pointer confinement and trapping the cursor at the display origin. Dialogs now set
  `WM_TRANSIENT_FOR` to the window they were raised from.
- Linux only: The modifier key bit indices used to read `QueryKeymap()`'s key-pressed bit vector were computed as
  `keycode - minKeyCode`, but `QueryKeymap()` indexes its bit vector by the raw keycode. This offset meant a pressed
  modifier key (Shift, Control, etc.) was probed at the wrong bit and went undetected.
