# Changes since v0.95.0

## Bug Fixes

- Linux only: The hidden window used to run the modal event loop for file dialogs was placed off-screen, but Wayland
  ignores client-requested window positions and showed it as a tiny "phantom" window; it is now kept truly hidden. In
  addition, a window blocked by a modal no longer re-activates that modal when it gains focus, which created a feedback
  loop under focus-follows-mouse window managers and appeared to trap the cursor inside the modal on compositors that
  warp the pointer on activation (e.g. Hyprland).
- Linux only: Dialog windows didn't set `WM_TRANSIENT_FOR`, so under XWayland the compositor could treat them as
  unparented. Dialogs now set `WM_TRANSIENT_FOR` to the window they were raised from.
- Dialogs and new windows could open on the primary display rather than the display the user was working on when no
  window held the keyboard focus at that moment (e.g. immediately after a menu closed, before the focus notification
  for the underlying window arrived). Window placement now falls back to the frontmost visible window, which is exposed
  via the new `FrontmostWindow()` function.
- Windows only: The native open and save file dialogs were shown without an owner window, letting the system place
  them on the primary display. They now use the active (or frontmost) window as their owner, so they appear on the
  same display as the window they were raised from.
- Linux only: The modifier key bit indices used to read `QueryKeymap()`'s key-pressed bit vector were computed as
  `keycode - minKeyCode`, but `QueryKeymap()` indexes its bit vector by the raw keycode. This offset meant a pressed
  modifier key (Shift, Control, etc.) was probed at the wrong bit and went undetected.
