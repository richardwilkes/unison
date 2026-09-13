# Changes since the last release

## New & Improved

- Added screen-reader support on macOS, Windows and Linux. Unison now describes its windows to the assistive technology
  each platform provides — NSAccessibility on macOS, UI Automation on Windows, AT-SPI2 on Linux — so a screen reader
  such as VoiceOver, Narrator, NVDA or Orca can read an application's controls, follow the keyboard focus, report what
  is being typed and act on the user's behalf. It is pure Go on all three: no C library and no accessibility bridge is
  involved. Every widget in the library describes itself, so an existing application becomes largely accessible without
  being changed; what cannot be derived is what has no text to derive it from, such as an icon-only button, which has to
  be named.
- Nothing about accessibility support runs until an assistive technology has actually asked for it. Asking is the first
  accessibility query a window's content view receives on macOS, the first `WM_GETOBJECT` for the UI Automation root
  object on Windows, and the accessibility bus launcher's `org.a11y.Status.IsEnabled` property on Linux — read once at
  startup over the session bus connection the color-scheme watcher already keeps, and then watched, so a screen reader
  started or stopped while the application runs is followed both ways. `NO_AT_BRIDGE=1` is honored on Linux exactly as
  it is for GTK. Until something asks, the cost is the `AccessibilityInfo` each panel carries and one atomic load per
  pass of the event loop that redrew something: no hierarchy is walked and nothing is allocated. No goroutine or
  platform object exists either on macOS and Windows; Linux pays for the watch it needs to be reachable at all, which
  holds one goroutine and two match rules on a session bus connection that already existed.
- Added `Panel.Accessibility`, an `AccessibilityInfo` held by value, which is everything an application says about a
  panel. All of its fields are optional: `Name` is what is announced, overriding whatever name would otherwise be
  derived; `Description` elaborates on it, defaulting to the panel's tooltip text; `Role` says what kind of element the
  panel is, with `role.Auto` deriving it from the widget and `role.None` hiding the panel and promoting its children
  into its parent; `LabeledBy` names the panel that labels this one; `Callback` runs last and may adjust anything on the
  finished node, which is how a fact with no field of its own, such as a heading's level, is reported; and
  `ActionCallback` handles requests from an assistive technology for a panel with no widget type of its own.
- Added the `accessibility` package, which defines the description a platform adapter is handed: a `Tree` of `Node`s
  capturing one window at a single moment, addressed by `NodeID`, immutable once published, along with the `Diff` that
  turns two successive trees into the events an assistive technology is told about. `Diff` is pure and deterministic, so
  the whole core is testable without a display, an assistive technology, or even a window. Coordinates in a tree are
  window-local logical units and text offsets are rune indexes; the adapters convert as their platform requires. Added
  the generated `enums/role` package for the roles a node can have, with the `IsText()`, `IsContainer()`, `IsRowLike()`
  and `IsWindow()` helpers.
- Added `AccessibilityProvider` and `AccessibilityActor`, which a custom widget implements to describe itself and to
  carry out what an assistive technology asks of it. Both are looked for on `Panel.Self`, so they have to be implemented
  by the widget type rather than by the embedded `Panel`. `ProvideAccessibility` is handed an `AccessibilityBuilder`
  whose node already holds everything derivable from the panel alone — its bounds, whether it is enabled, focusable and
  focused, and the actions those imply — so an implementation sets only what it knows better. The builder's
  `VisibleRect()` lets a widget with more content than it can show describe only what can be seen, and
  `AddVirtualChild()` describes the elements a widget draws without a panel apiece, as the rows and cells of `Table` and
  `List` now do.
- Added `AnnounceForAccessibility()`, which asks the platform's assistive technology to speak a message that no change
  to a window expresses, such as a background task finishing. It does nothing when no assistive technology is being
  served, so it may be called unconditionally, and it is safe to call from any goroutine. `IsAccessibilityActive()`
  reports whether anything is being served.
- Added `AccessibilityEnvKey` (`UNISON_ACCESSIBILITY`) and the `NoAccessibility()` startup option, which override what
  the platform reports. Setting the variable to a true value builds and publishes descriptions whether or not anything
  appears to be listening, which is useful for seeing what a screen reader would be told; setting it to a false value,
  or using the startup option, refuses accessibility support entirely. `SetAccessibilityEnabled()` turns support off or
  back on while the application runs, and `AccessibilityEnabled()` reports whether it is permitted.
- Added accessibility support to headless sessions, so what an assistive technology would be handed can be asserted on
  with no display and no screen reader involved. `HeadlessScreen.AccessibilityTree()` turns support on if it is not on
  already, describes the window as it is now and hands back the tree a platform adapter would have been given;
  `AccessibilityNodeFor()` finds the node describing one panel within it; `AccessibilityEvents()` drains the events
  published for a window; `Announcements()` drains what `AnnounceForAccessibility()` was asked to speak;
  `PerformAccessibilityAction()` makes a request of a node exactly as a screen reader would; and
  `EnableAccessibility()` turns support on without asking for a tree. A session starts with support off, as an
  application whose platform has no assistive technology running does.
- `Slider` is now focusable, which it had to become to be usable without a mouse. It takes the keyboard focus when
  clicked or tabbed to, draws a focus ring, steps its value with the left/down and right/up arrow keys and jumps to the
  ends of its range with Home and End. `SliderTheme` gained `SelectionInk`, which is the ink the focus ring is drawn
  with.
- Replaced the hand-written D-Bus client in `internal/x11` with a new pure-Go `internal/dbus` package, which is what the
  accessibility bus on Linux is spoken to through and what the color-scheme watcher now uses. The two color-scheme
  functions behave exactly as they did, and no third-party library or `libdbus` is needed.

## Bug Fixes

- (none yet)
