# Progress

Running log of work sessions against [plan.md](plan.md) (removing all cgo usage from unison). Newest session first.

## Session 10 — 2026-07-10: Phase 2 code complete — panels + CF helpers ported; the module is now 100% cgo-free

The "Open/save panels" bullet and the delete-the-cgo cleanup bullet of Phase 2 are done. `all_darwin.go`,
`macos.h`, `open_panel_darwin.m`, and `save_panel_darwin.m` are deleted; **no `import "C"` remains anywhere in the
module**, and the whole module now builds *and tests* with `CGO_ENABLED=0` on macOS (also verified for
linux/amd64+arm64, windows/amd64, and darwin/amd64). Exported API effectively unchanged; the root package needed
zero edits. All that remains of Phase 2 is the final human verification pass (see plan.md); Phase 3 is untouched.

### What changed (session 10)

- **[internal/mac/foundation_darwin.go](internal/mac/foundation_darwin.go)** (new) — the exported CF-flavored
  wrapper types the root dialogs consume (`Array`, `String`, `URL`) are now `objc.ID`-based over the
  toll-free-bridged Foundation classes (NSMutableArray/NSString/NSURL msgSends). Ownership discipline ported
  mechanically: constructors return owned (+1) references (`NewArrayFromStringSlice` builds an
  alloc/initWithCapacity: NSMutableArray and releases each NSString after addObject:, same net counts as the old
  CFArrayCreateMutable flow), index accessors return borrowed references, `Release` balances one owned reference.
  `NewFileURL` uses `fileURLWithFileSystemRepresentation:isDirectory:relativeToURL:` — the documented NSURL
  counterpart of the old `CFURLCreateFromFileSystemRepresentation` — verified to produce **byte-identical absolute
  URL strings** (plain/space/CJK/trailing-slash/directory cases) by a compiled-and-run Objective-C program;
  `AbsoluteString` is `absoluteString` (equivalent to the old CFURLCopyAbsoluteURL+CFURLGetString, including for
  relative URLs — also verified). The `ArrayOfURLToStringSlice` scheme/host-discarding quirk is preserved verbatim.
  One deliberate difference, documented in the file comment: nil handles are now safe no-ops yielding zero values
  (objc nil messaging) where CFRelease(NULL)/CFArrayGetCount(NULL) crashed. The exported-but-unused-everywhere
  `MutableArray` type was dropped (nothing in the module ever referenced it).
- **[internal/mac/open_panel_darwin.go](internal/mac/open_panel_darwin.go)** /
  **[internal/mac/save_panel_darwin.go](internal/mac/save_panel_darwin.go)** (new) — `OpenPanel`/`SavePanel` are
  `objc.ID`-based; `NewOpenPanel`/`NewSavePanel` reproduce the old `[[... openPanel] retain]` (+1) inside a
  `WithPool`. `RunModal` compares `runModal` against `NSModalResponseOK` = 1 (verified empirically, along with
  Cancel = 0 and sizeof(NSModalResponse) = 8). The deprecated `allowedFileTypes` property is kept per the plan
  (set/get round-trip verified against this SDK with compiled Objective-C — it still works, mapping onto
  allowedContentTypes internally). One deliberate fix, documented on the methods: `AllowedFileTypes()` returns a
  **retained (+1)** reference where the old bridge returned a borrowed one, because the root dialogs release the
  result — the old contract was an over-release (and a CFRelease(NULL) crash when no types were set) had
  `AllowedExtensions` ever been used; retaining makes the existing root code balanced with zero edits.
  `DirectoryURL()`/`URLs()`/`URL()` stay borrowed exactly like the old bridge (root does not release those), and
  string getters/setters run inside `WithPool`.
- **Deleted**: `all_darwin.go` (the last cgo file in the module), `macos.h`, `open_panel_darwin.m`,
  `save_panel_darwin.m`. The plan's "split into per-area files" cleanup was already satisfied incrementally by
  sessions 3–9, so nothing else moved.
- **New tests**: [foundation_darwin_test.go](internal/mac/foundation_darwin_test.go) covers the string-slice array
  round-trip (ASCII/CJK, empty, nil-handle no-ops), `ArrayOfURLToStringSlice` with file URLs + a non-file URL
  (asserting the path-only quirk), the String wrapper round-trip, and `NewFileURL` (existing file vs directory via
  the real file system, trailing-slash and nonexistent-path handling, exact absolute-string values).
  [open_panel_darwin_test.go](internal/mac/open_panel_darwin_test.go) covers all four bool accessors both ways,
  the directory-URL round-trip, the **root-dialog-shaped allowedFileTypes flow** (unset → 0-handle safe use,
  set/copy semantics, double read-back each followed by Release proving the +1 contract, clear via 0), empty
  `URLs()`, and **`RunModal` itself**: a delayed `cancel:` scheduled in NSModalPanelRunLoopMode ends the real
  modal session (~0.3s) and RunModal reports false, with an `abortModal` backstop at 10s so a wedged modal
  session fails fast instead of hanging the suite. [save_panel_darwin_test.go](internal/mac/save_panel_darwin_test.go)
  covers the name-field round-trip (ASCII/CJK/empty), directory URL, allowedFileTypes, and the canceled modal run
  including the post-run `URL()` composition check (directory + name field, the value root's `Path()` parses).

### Discoveries (session 10)

1. **Panel `runModal` is fully testable headlessly** (and in a locked session, on both arches): schedule `cancel:`
   with `performSelector:withObject:afterDelay:inModes:` in **NSModalPanelRunLoopMode** — timers in that mode fire
   inside the modal session's run loop — and the session ends with NSModalResponseCancel ~0.3s in. Because
   `runModal` blocks the test suite's main-thread pump, every such test also schedules a 10s `abortModal` backstop
   on NSApp and cancels both pending performs (`cancelPreviousPerformRequestsWithTarget:`) after runModal returns.
2. **`NSSavePanel.URL` does not recompose from `setDirectoryURL:`/`setNameFieldStringValue:` until the panel has
   been presented at least once** — before that it reports the panel's defaults (e.g. `~/Desktop/Untitled`); after
   even a *canceled* run it reflects directory + name field. Verified identical with a compiled Objective-C
   program, so it is AppKit behavior, not a port difference. Production is unaffected: root only reads `Path()`
   after `RunModal` returns true, i.e. always after presentation.
3. `go vet ./internal/mac/` is down to 1 pre-existing unsafeptr finding (objc_darwin.go's NSStringConstant deref);
   the other one lived in all_darwin.go's cgo sections and went with the file.

### CI followup (session 10): panel tests skip where the panel XPC service can't start (Intel runner)

The Build workflow passed on the macos-26 arm64 runner but failed on macos-26-intel: all 9 panel tests failed with
`NewOpenPanel`/`NewSavePanel` returning 0, each burning ~60s first (~402s package total). On modern macOS both panels
are backed by the remote view service `com.apple.appkit.xpc.openAndSavePanelService`; on that headless Intel VM the
service cannot start, so `+[NSOpenPanel openPanel]` blocks for the ~60s XPC timeout and then returns nil. The arm64
runner creates panels fine despite also being headless, so a plain headless check would wrongly skip machines where
panels work. Fix (same shape as the session-7 GL skip): the panel tests now call `requirePanelService(t)` from the
test goroutine — never inside `runOnMain`, where `t.Skip`'s Goexit is misuse — which probes `openPanel` via a raw
msgSend (independent of `NewOpenPanel`'s Retain/WithPool handling, so a real port regression still fails wherever
panels work) and caches the result in a `sync.OnceValue`, so a broken environment pays the ~60s stall once for the
whole suite instead of once per test. One probe covers both panel kinds (NSOpenPanel subclasses NSSavePanel; same XPC
service). Verified by simulation: with the probe temporarily forced false, exactly the 9 panel tests SKIP (only the
first pays the probe) and everything else passes with no pump deadlock; unmodified, all 9 still run and pass locally.
Production is unaffected by a nil panel: real user machines have a WindowServer and a working panel service, and even
if creation ever did fail, the port's nil-messaging semantics make the root dialogs degrade gracefully (RunModal
false, empty getters, no-op setters) where the old CF bridge could crash (e.g. `AllowedExtensions`'s CFRelease(NULL)).

### Verification performed (session 10)

- `./build.sh --test` green; `golangci-lint run ./...` 0 issues; `golangci-lint fmt internal/mac/` and `gofmt -l`
  clean; root package needed zero edits (untouched per git status, and the full build passed).
- **`CGO_ENABLED=0 go build ./...` and `CGO_ENABLED=0 go test ./internal/mac/` pass natively on darwin/arm64** —
  the plan's headline goal for macOS — and `grep -rln 'import "C"'` over the module finds nothing.
- `go test ./internal/mac/` 10/10 fresh processes; `-count=5` single process; `-race`.
- darwin/amd64 under Rosetta 2, now built with `CGO_ENABLED=0 GOARCH=amd64 go test -c` (no cgo left to enable):
  5/5 fresh runs + `-test.count=3`, plus a verbose run of the 13 new tests (proves the runModal int64 return, the
  performSelector float64-delay marshaling, and the bool/pointer accessor shapes on SysV; no new struct-arg call
  shapes, so the straddle constraint is not in play).
- `GOOS=linux GOARCH={amd64,arm64} CGO_ENABLED=0 go build ./...`, windows/amd64, and darwin/amd64 all pass.
- `cmd/example` smoke-run twice — the normal build **and an explicitly `CGO_ENABLED=0` build** — both alive after
  8s with empty stderr/stdout (SIGKILL per the session-3 note). Startup does not open dialogs, so the
  production-shape proof for the panels is the test suite driving the exact call sequences the root dialogs make
  (including a real modal session ending in cancel), through the same exported API.
- Not covered (need a human session): interactively choosing files in the dialogs (the OK path of `RunModal` — the
  constant is verified, and cancel proves the modal plumbing — plus `URLs()` with a real selection), and the
  standing Phase 2 manual list (IME, popup-menu tracking, cross-app drag & drop and clipboard, minimize/zoom/
  focus, multi-monitor, transparency).

## Session 9 — 2026-07-10: Phase 2 — pasteboard + drag info ported to purego; pasteboard_darwin.m and drag_darwin.m deleted

The "Pasteboard, drag & drop" bullet of Phase 2 is done (the dragging-destination view overrides and
`beginDraggingSessionWithItems:` were already ported with the view in session 6, so this session covered the
remaining half: NSPasteboard read/write, NSPasteboardItem, and the DragInfo accessors). Exported API unchanged; the
root package needed zero edits. Only two `.m` files remain (open_panel, save_panel), and `all_darwin.go` is down to
the CF Array/String/URL helpers plus the two panels (~250 lines).

### What changed (session 9)

- **[internal/mac/drag_darwin.go](internal/mac/drag_darwin.go)** (new) — `DragInfo` is `objc.ID`-based (was
  `C.NSDraggingInfoRef`; both uintptr-kinded, so `var _ drag.Info = DragInfo(0)` and the view/root usage compile
  unchanged). `DragOp` is now `uint64` with baked-in constants (None=0, Copy=1, Move=16 — verified by compiling and
  running an Objective-C program against the SDK, along with sizeof(NSDragOperation)=8);
  `DragOpFromUnison`/`ToUnisonDragOp` moved verbatim. Every accessor is a direct msgSend to the sender
  (`draggingPasteboard`, `draggingSourceOperationMask`) bracketed by `WithPool` where autoreleased objects flow.
  Behavior parity kept deliberately: `FilePaths` still goes through each URL's `fileSystemRepresentation` (not
  `path`), and `URLs` still reduces each absolute URL string to its path component before re-parsing — the old
  bridge's `ArrayOfURLToStringSlice` quirk that discards scheme/host — documented in the method comment. One
  mechanical improvement: the old `dragFilePaths` leaked a +1 CFArray per call (Go never released it); the pure-Go
  version builds the []string directly, so nothing leaks. `NSPasteboardURLReadingFileURLsOnlyKey` is resolved once
  via `sync.OnceValue` + `NSStringConstant` and the `readObjectsForClasses:options:` NSDictionary is built with
  `dictionaryWithObject:forKey:` + `numberWithBool:`.
- **[internal/mac/pasteboard_darwin.go](internal/mac/pasteboard_darwin.go)** (new) — `Pasteboard`/`PasteboardItem`
  are `objc.ID`-based. `PasteboardGeneral` is the `generalPasteboard` singleton; `WriteItems` builds an autoreleased
  NSArray inside a pool (the old code's owned CFArray + release, same net ownership); `NewPasteboardItem` keeps the
  old bridge's +1-never-released item discipline (ownership effectively transfers to the pasteboard; the one extra
  reference per item is a pre-existing leak kept for bring-up parity, documented on the type). `SetData` creates the
  NSData with `dataWithBytes:length:` (copies during the call, so passing Go memory is safe); `Bytes` reads back
  through the shared `GoBytesFromNSData`, copying once instead of the old bridge's malloc+copy+copy+free. The
  `NSPasteboardTypeString` constant is resolved once via `sync.OnceValue` (its value, "public.utf8-plain-text", plus
  both dlsym resolutions were verified with compiled Objective-C programs).
- **[internal/mac/objc_darwin.go](internal/mac/objc_darwin.go)** — two new shared helpers: `GoStringFromCString`
  (NUL-terminated C string → Go string, for `fileSystemRepresentation`) and `GoBytesFromNSData` (NSData → copied
  []byte, nil for nil/empty, matching the old bridge's nil-on-zero-length behavior).
- **all_darwin.go / macos.h** — DragInfo and Pasteboard sections removed (~170 lines; the `uti` and `drag` imports
  went with them); `NSDraggingInfoRef`/`NSPasteboardRef`/`NSPasteboardItemRef` typedefs and both declaration blocks
  removed from macos.h; `pasteboard_darwin.m` and `drag_darwin.m` deleted.
- **New tests**: [pasteboard_darwin_test.go](internal/mac/pasteboard_darwin_test.go) exercises the Pasteboard
  methods against a **uniquely named pasteboard** (`pasteboardWithUniqueName`, released via `releaseGlobally`), so
  the user's clipboard is never touched: write/read-back of custom-UTI data + string (ASCII/CJK) through
  AppKit's own string channel (`stringForType:` readback proves `SetString` used the real NSPasteboardTypeString),
  AvailableDataTypes/HasDataType/Bytes including absent types, multi-item writes, zero-length data (present but
  Bytes = nil, matching the old bridge), empty WriteItems no-op, Clear, and PasteboardGeneral singleton identity
  against AppKit's own `generalPasteboard`. [drag_darwin_test.go](internal/mac/drag_darwin_test.go) extends the
  session-6 fake NSDraggingInfo class with a `draggingPasteboard` method backed by a unique pasteboard and drives
  every `drag.Info` accessor through real objc_msgSend dispatch: DataTypes/HasString/Text/HasDataType/Data with
  string+data payloads, file-URL handling (HasFilePaths/FilePaths asserted against AppKit's own
  `fileSystemRepresentation` recorded at write time, so path canonicalization can't flake the test), non-file URLs
  (HasURLs true, HasFilePaths false, and the URLs() path-only parity quirk asserted explicitly), the negative cases
  for a URL-less pasteboard, SourceDragOpMask for three masks, and the DragOp conversion table (including AppKit
  bits unison doesn't model being dropped). Helper tests for `GoStringFromCString`/`GoBytesFromNSData` added to
  objc_darwin_test.go.

### Notes (session 9)

1. NSPasteboard is fully testable headlessly (and in a locked session) via uniquely named pasteboards — writes,
   reads, and `readObjectsForClasses:options:` URL extraction all work without the app being active, and nothing
   touches the user's clipboard.
2. The new-test threading convention held: whole test bodies run inside a single `runOnMain` closure, and helpers
   that need the main thread (`newUniquePasteboard`, `newDragInfoWithPasteboard`) must be called from *within* a
   runOnMain closure — nesting runOnMain deadlocks by design.
3. `go vet ./internal/mac/` is down to 2 pre-existing unsafeptr findings (from 3): the removed pasteboard cgo
   section contained one of them. The remaining two (objc_darwin.go's NSStringConstant deref, all_darwin.go's
   NewArrayFromStringSlice) are unchanged.

### Verification performed (session 9)

- `./build.sh --test` green; `golangci-lint run ./...` 0 issues; `golangci-lint fmt internal/mac/` and `gofmt -l`
  clean; root package needed zero edits (verified by the untouched build and git status).
- `go test ./internal/mac/` 10/10 fresh processes; `-count=5` single process; `-race`.
- darwin/amd64 under Rosetta 2 (`CGO_ENABLED=1 GOARCH=amd64 go test -c`): 5/5 fresh runs + `-test.count=3`, plus a
  verbose run of the nine new tests (covers the amd64 side of the *byte returns from `bytes`/`UTF8String`/
  `fileSystemRepresentation` and the bool/uint64/pointer msgSend arg shapes; no new struct-arg call shapes were
  introduced, so the straddle constraint is not in play).
- `GOOS=linux GOARCH={amd64,arm64} CGO_ENABLED=0 go build ./...` and windows/amd64 pass.
- `cmd/example` smoke-run: alive after 8s with empty stderr/stdout (SIGKILL per the session-3 note). Startup does
  not exercise clipboard/drag, so the production-shape proof for this bullet is the test suite driving the same
  methods root's clipboard_darwin.go calls, plus the PasteboardGeneral identity test tying the tested unique
  pasteboard paths to the singleton production actually uses.
- Not covered (need a human session): real user-initiated drag & drop between apps (the DragInfo accessors are
  proven against real pasteboard contents, but not a live drag session's NSDraggingInfo), and clipboard interchange
  with other running applications (copy in unison → paste in TextEdit and vice versa — same NSPasteboard API, but
  worth a manual sanity pass alongside the existing Phase 2 final checklist).

## Session 8 — 2026-07-10: Phase 2 — menus ported to purego; menu_darwin.m, menu_item_darwin.m, and the last three //export callbacks deleted

The "Menus" bullet of Phase 2 is done. `MenuDelegate` and `MenuItemDelegate` are now Go-registered Objective-C
classes, the ~25 menu/menu-item functions are direct msgSends, and `menuPopup` is pure Go. The three `//export`
functions these delegates called (`goUpdateMenuCallback`, `goMenuItemValidateCallback`, `goMenuItemHandleCallback`)
were the last C→Go callbacks in the module — session 6's "no exports remain" claim covered only the view shims —
so **the cgo bridge is now exclusively plain Go→C calls** (pasteboard/drag-info accessors and the open/save panels).
Exported API unchanged; the root package needed zero edits. Four `.m` files remain (drag, open_panel, pasteboard,
save_panel).

### What changed (session 8)

- **[internal/mac/menu_darwin.go](internal/mac/menu_darwin.go)** (new) — `Menu` is `objc.ID`-based (was
  `C.NSMenuRef`; both uintptr-kinded, so root's `== 0` checks still compile). `MenuDelegate` is a Go-registered
  class (`NSMenuDelegate` protocol, nil-guarded lookup) with a single shared instance created on first use,
  mirroring the old file-static; `menuNeedsUpdate:` routes to the `menuUpdaters` map exactly as the export shim
  did. Ownership parity kept deliberately: `NewMenu` reproduces the old bridge's **double retain** (alloc/init +
  retain, so Release never deallocates a menu — documented on the type), and `Menu.Popup` reproduces the
  NSPopUpButtonCell flow verbatim (`initTextCell:pullsDown:` + retain, autoenables/altersState off, setMenu/
  selectItem, `performClickWithFrame:inView:` — a 32-byte NSRect arg, same proven shape as `setFrame:display:` —
  then a single release, keeping the old code's one-reference-per-popup leak for bring-up parity, noted in a
  comment). Popup and the string getters run inside `WithPool` so they are safe regardless of ambient pools.
- **[internal/mac/menu_item_darwin.go](internal/mac/menu_item_darwin.go)** (new) — `MenuItem` is `objc.ID`-based;
  `ControlStateValue` and the validator/handler maps moved over verbatim. `MenuItemDelegate` is a Go-registered
  class (`NSMenuItemValidation` protocol) with a shared instance that is every item's target: `handleMenuItem:`
  (the action) routes to `menuItemHandlers`, and the bool-returning `validateMenuItem:` routes to
  `menuItemValidators` with the same default-true fallback. `NewMenuItem` keeps the old +2 retain and the exact
  setup order (init with action selector → setTag: → setKeyEquivalentModifierMask: → setTarget:);
  `NewSeparatorMenuItem` retains the autoreleased `separatorItem` inside a pool.
- **all_darwin.go / macos.h** — Menu and Menu Item sections removed (~180 lines incl. the 3 exports and the
  now-unused `rectToCGRect`/`geom` import); `NSMenuRef`/`NSMenuItemRef`/`NSWindowRef` typedefs and both declaration
  blocks removed from macos.h; `menu_darwin.m` and `menu_item_darwin.m` deleted.
- **New tests**: [menu_darwin_test.go](internal/mac/menu_darwin_test.go) covers menu basics (title round-trip
  ASCII/CJK/empty, shared-delegate wiring on every menu), structure (insert/remove/removeAll/count/item identity,
  separator placement, item→menu back-pointers, submenu set/get), `menuNeedsUpdate:` routing through the delegate
  via msgSend (right updater for the right menu; updater-less menus are safe no-ops), item accessors (tag, title,
  key equivalent + modifier round-trips, action/target wiring, all three states), **AppKit-initiated validation** —
  `[menu update]` drives auto-enablement through the Go `validateMenuItem:` IMP (false → disabled, true → enabled,
  no validator → default enabled) — **AppKit-routed actions** — `performActionForItemAtIndex:` sends
  `handleMenuItem:` through NSApplication's action dispatch to the Go handler (and is a safe no-op without one) —
  and Release's registration-map cleanup contract.

### Notes (session 8)

1. `[menu update]` really does drive `NSMenuItemValidation` for items with an explicit target, and
   `performActionForItemAtIndex:` really does fire the target/action pair, both headlessly and in a locked
   session — so the whole menu behavior surface except live tracking is testable without a human.
2. AppKit only sends `menuNeedsUpdate:` at the start of a real user-interactive tracking session, so that path is
   driven via direct msgSend to the delegate (the same dispatch AppKit uses); `Menu.Popup`
   (`performClickWithFrame:inView:` blocks in a tracking run loop) cannot run headlessly at all and stays on the
   Phase 2 final manual-verification list.
3. `go vet ./internal/mac/` still reports exactly the 3 pre-existing unsafeptr findings from session 7 (lines
   shifted); nothing new.

### Verification performed (session 8)

- `./build.sh --test` green; `golangci-lint run ./...` 0 issues; `golangci-lint fmt internal/mac/` and `gofmt -l`
  clean; root package needed zero edits (verified by the untouched build).
- `go test ./internal/mac/` 10/10 fresh processes; `-count=5` single process; `-race`.
- darwin/amd64 under Rosetta 2 (`CGO_ENABLED=1 GOARCH=amd64 go test -c`): 5/5 fresh runs + `-test.count=3`, plus a
  verbose run of the seven menu tests (covers the SysV side of the bool-returning `validateMenuItem:` Go IMP and
  the int64/uint64/SEL/NSString msgSend arg shapes).
- `GOOS=linux GOARCH={amd64,arm64} CGO_ENABLED=0 go build ./...` and windows/amd64 pass.
- `cmd/example` smoke-run: alive after 8s with empty stderr/stdout (SIGKILL per the session-3 note). This is the
  production-shape proof: the demo's `BarForWindow`/`InsertStdMenus` startup path builds the entire macOS menu bar
  through NewMenu/NewMenuItem/InsertItemAtIndex/SetSubMenu/SetKeyBinding plus SetMainMenu/SetServicesMenu/
  SetWindowsMenu/SetHelpMenu, all via the ported msgSend path.
- Not covered (need a human session): `Menu.Popup` tracking (popup menus over a window, incl. during modal
  dialogs — the reason the NSPopUpButtonCell approach exists), real menu-bar interaction (opening menus, choosing
  items, key equivalents through the live event stream), and `menuNeedsUpdate:` delivery from real tracking.

## Session 7 — 2026-07-10: Phase 2 — OpenGL context + pixel format ported to purego; their .m files and the NSViewRef typedef deleted

The "OpenGL context + pixel format" bullet of Phase 2 is done. A previous run of this session was interrupted after
writing the port but **before any verification**; this session reviewed that work critically (it turned out correct
and complete, but its code comments claimed empirical constant verification that had not actually been performed —
now it has), added the missing tests, and ran the full verification matrix. Exported API unchanged; the root package
([glcontext_darwin.go](glcontext_darwin.go)) needed zero edits. Six `.m` files remain (drag, menu, menu_item,
open_panel, pasteboard, save_panel).

### What changed (session 7)

- **[internal/mac/opengl_context_darwin.go](internal/mac/opengl_context_darwin.go)** (new) — `OpenGLContextRef` is
  `objc.ID`-based (was `C.NSOpenGLContextRef`; both uintptr-kinded, so root's `== 0` checks still compile).
  `NewOpenGLContext` reproduces the old bridge step for step: `initWithFormat:shareContext:` → nil check → (when
  transparent) `setValues:forParameter:` with a GLint 0 for `NSOpenGLContextParameterSurfaceOpacity` (=236) →
  `setWantsBestResolutionOpenGLSurface:YES` → `setView:`. `MakeCurrent` on a 0 handle still clears the current
  context (the old `openGLMakeCurrent(nil)` branch), now via `ClearOpenGLCurrentContext`. One deliberate mechanical
  difference: `Release` sends `release` instead of calling `CFRelease` — identical for a non-nil ObjC object, and
  the nil case (a crash under CFRelease) is unreachable through unison, which guards every Release with `!= 0`.
- **[internal/mac/opengl_pixel_format_darwin.go](internal/mac/opengl_pixel_format_darwin.go)** (new) — same
  attribute list as the old `newOpenGLPixelFormat` (accelerated, closest-policy, 3.2 core profile, 24/8/24/8), as a
  `[...]uint32` passed to `initWithAttributes:` by pointer (NSOpenGLPixelFormatAttribute is uint32).
- **all_darwin.go / macos.h** — OpenGL Context and OpenGL Pixel Format sections removed (~41 lines);
  `NSOpenGLContextRef`/`NSOpenGLPixelFormatRef`/`NSViewRef` typedefs and the GL declarations removed from macos.h;
  `opengl_context_darwin.m` and `opengl_pixel_format_darwin.m` deleted.
- **All baked-in constants re-verified empirically** (the interrupted run's comments claimed this but hadn't done
  it): a compiled-and-run Objective-C program confirmed SurfaceOpacity=236, PFAColorSize=8, AlphaSize=11,
  DepthSize=12, StencilSize=13, Accelerated=73, ClosestPolicy=74, OpenGLProfile=99, ProfileVersion3_2Core=0x3200,
  and the ABI sizes (NSOpenGLPixelFormatAttribute=4, NSOpenGLContextParameter=8, GLint=4 bytes).
- **New tests**: [opengl_darwin_test.go](internal/mac/opengl_darwin_test.go) covers the pixel format's requested
  attributes read back via `getValues:forAttribute:forVirtualScreen:` (≥24/8/24/8, accelerated, profile == 0x3200
  exactly); context creation against a real window/view pair (context view identity, the view's
  wantsBestResolutionOpenGLSurface flag, default surface opacity 1, and the share-context creation path); the
  transparent path reading back surface opacity 0 through `getValues:forParameter:`; and the make-current contract
  (MakeCurrent binds, MakeCurrent(0) clears, ClearOpenGLCurrentContext clears) plus a one-frame
  Update/MakeCurrent/FlushBuffer smoke with the window ordered in.

### Notes (session 7)

1. `getValues:forParameter:` reads the surface-opacity value back correctly even before the window has ever been
   shown (no live drawable needed), so the transparent path is testable headlessly.
2. `go vet ./internal/mac/` reports 3 pre-existing `unsafeptr` warnings on HEAD (objc_darwin.go's NSStringConstant
   deref and two in all_darwin.go's cgo sections — the latter disappear with the cgo preamble). `go test`'s default
   vet subset doesn't include unsafeptr, and build.sh doesn't run vet, so nothing fails today; worth deciding in
   Phase 3 whether to gate on vet.

### CI followup (session 7): headless runners deadlocked the package — pump made Goexit-proof, GL tests skip without a GPU

The Build workflow failed identically on both macOS runners (macos-26 arm64 and macos-26-intel) while every local run
passed. Two stacked causes, both fixed in the working tree:

1. **Environment**: headless CI VMs have no hardware-accelerated OpenGL renderer, so `NewOpenGLPixelFormat` (whose
   attribute list requires `NSOpenGLPFAAccelerated`) legitimately returns 0 there. The four GL tests now call
   `requireAcceleratedGL(t)` first — from the test goroutine, where `t.Skip` is legal — which probes for a minimal
   `{NSOpenGLPFAAccelerated, 0}` pixel format via raw msgSends, independent of `NewOpenGLPixelFormat`'s attribute
   handling. They skip only where no accelerated renderer exists at all (the session-3 `TestDisplayFunctions`
   pattern); a real marshaling regression still fails on any machine with a GPU.
2. **Deadlock**: `newTestPixelFormat` called `t.Fatal` from *inside* a `runOnMain` closure, i.e. on TestMain's pump
   goroutine. `FailNow` calls `runtime.Goexit`, which killed the pump (the wrapper's `defer close(done)` ran during
   the Goexit, so the failing test was still reported, but the loop goroutine was gone); the next test's `runOnMain`
   then blocked forever on the unbuffered channel send until the 10-minute package timeout. This hazard was latent in
   every `t.Fatal`/`t.Skip` inside every `runOnMain` closure since session 3 — it just had never fired. The pump now
   runs each closure through `runPumped`, whose deferred handler re-enters the pump loop when a closure Goexits
   (deferred functions run during Goexit; the re-entered loop never returns, so the Goexit stays parked in that frame
   while the main thread keeps servicing work). Real panics are re-raised so crashes stay loud.
   `TestRunOnMainSurvivesGoexit` locks the contract in: it Goexits a closure on purpose and proves `runOnMain` still
   works afterward.

Verified by simulation on top of the normal matrix: with a temporary first-in-package test doing `t.Fatal` inside
`runOnMain`, the whole suite now runs to completion (one FAIL, zero deadlocks); with the probe temporarily forced to
report no acceleration, all four GL tests SKIP with a clear message and everything else passes.

### Verification performed (session 7)

- `./build.sh --test` green; `golangci-lint run ./...` 0 issues; `golangci-lint fmt internal/mac/` made no changes.
- `go test ./internal/mac/` 10/10 fresh processes; `-count=5` single process; `-race`.
- darwin/amd64 under Rosetta 2 (`CGO_ENABLED=1 GOARCH=amd64 go test -c`): 5/5 fresh runs + `-test.count=3`, plus an
  explicit verbose run of the four new GL tests (the new msgSend shapes are all plain integer/pointer args — no
  struct-arg straddle exposure — but the GLint out-pointer readbacks and uint32/int64 arg marshaling are now proven
  on both arches).
- `GOOS=linux GOARCH={amd64,arm64} CGO_ENABLED=0 go build ./...` and windows/amd64 pass.
- `cmd/example` smoke-run: alive after 8s with empty stderr/stdout (SIGKILL per the session-3 note). This is the
  production-shape proof for this bullet: apiCreate builds the pixel format + context through the ported path at
  startup, and every rendered frame goes through the ported MakeCurrent/FlushBuffer.
- Not covered (need a human session): visually confirming a transparent window composites correctly over the
  desktop (the opacity parameter round-trip is proven; the pixels are not), and rendering across a
  retina/non-retina monitor move (Update is exercised, but only single-screen).

## Session 6 — 2026-07-10: Phase 2 — view/IME ported to purego; view_darwin.m and all 17 export shims deleted

The "View" bullet of Phase 2 (flagged in the plan as the riskiest file) is done. `macContentView` is now a
Go-registered Objective-C class implementing every override the `.m` file had — mouse/key/tracking/drawing, the full
`NSTextInputClient` protocol (IME), and both drag & drop directions — and the last `//export` shims are gone from
all_darwin.go, so **internal/mac no longer has any C→Go callbacks at all** (the remaining cgo is plain Go→C calls
for menus, pasteboard/drag-info accessors, panels, and NSOpenGL*). Exported API unchanged; the root package needed
zero edits.

### What changed (session 6)

- **[internal/mac/view_darwin.go](internal/mac/view_darwin.go)** (new, ~590 lines) — `View` is `objc.ID`-based. The
  17 `Window*Callback` vars moved here verbatim from all_darwin.go. `macContentView` extends NSView, declares the
  `NSTextInputClient` and `NSDraggingSource` protocols (nil-guarded lookups, same as macWindowDelegate), and carries
  the old class's six ivars as purego `FieldDef`s: `wnd`, `trackingArea`, `markedText`, `lastMouseDraggedEvent`
  (all `objc.ID` — encodes as `@`), `dragMask` (uint64), `inDragWeStarted` (bool). Ownership discipline is ported
  mechanically: `trackingArea`/`markedText` are owned (+1) with a Go-implemented `dealloc` override releasing them
  before `SendSuper(dealloc)`; `wnd`/`lastMouseDraggedEvent` are assign-only, as before. `NewView` reproduces
  `initWithWindow:` step for step (super init → set wnd → fresh NSMutableAttributedString → updateTrackingAreas).
  The old shim-only logic (drag-op masking against `SourceDragOpMask`, insertText's Command-modifier gate and
  0xF700–0xF7FF function-key filtering, the `markedRange` = {0, length-1} quirk, mouseDragged's
  lastMouseDraggedEvent stash) is preserved exactly; insertText's per-character UTF-32 extraction loop became Go
  rune iteration (same code points). `View.BeginDraggingSession` builds the `NSDraggingItem` with direct msgSends,
  so `newDraggingItem` was deleted from pasteboard_darwin.m/macos.h ahead of the pasteboard bullet (the
  NSPasteboardItem still comes from the cgo `NewPasteboardItem` — package-internal calls across the cgo/purego
  boundary are plain Go calls). Matching the old bridge, the pasteboard item and dragging item are deliberately not
  released (pre-existing leak, kept for bring-up parity; noted in the method comment). `RegisterDraggedTypes` now
  autoreleases its NSArray/NSStrings inside a `WithPool`, fixing the old path's leaked CFArray+CFStrings per call.
- **[internal/mac/all_darwin.go](internal/mac/all_darwin.go)** — View section, all 17 `//export goWindow*` shims,
  and the now-unused CGPoint/CGRect→Go converters removed (~230 lines). `NewOpenGLContext` keeps passing
  `C.NSViewRef(view)` (View is uintptr-kinded, so the conversion stays legal — same pattern as Window/Menu).
- **macos.h / pasteboard_darwin.m / view_darwin.m** — View section, `newDraggingItem`, and the
  `NSDraggingItemRef`/`NSImageRef` typedefs removed; `view_darwin.m` deleted (`NSViewRef` typedef stays for
  `newOpenGLContext` until the GL bullet lands).
- **New tests**: [view_darwin_test.go](internal/mac/view_darwin_test.go) covers the class basics (protocol
  conformance, the constant bool overrides, isOpaque tracking the window, frame/MouseInRect/BackingScale geometry,
  and the tracking-area remove/release/re-add cycle keeping exactly one view-owned area); mouse events through real
  objc_msgSend dispatch with synthesized NSEvents (flipped coordinates, buttons, modifiers, the mouseDragged
  forwarding contract incl. suppression during a drag we started); **the full IME loop** — keyDown: →
  `interpretKeyEvents:` → AppKit's text input system calling back into our `insertText:replacementRange:` → typed
  callback; the NSTextInputClient surface (struct returns via markedRange/firstRectForCharacterRange,
  `setMarkedText:selectedRange:replacementRange:` driven through an **NSInvocation with a corrected
  `{_NSRange=QQ}` signature** — Foundation's marshaling calls our IMP the way AppKit would while sidestepping the
  purego amd64 caller-side straddle bug — plus both NSString/NSAttributedString branches, unmarkText, and the
  function-key filter); drag & drop (a Go-registered `unisonTestDraggingInfo` fake proves destination methods,
  source-mask intersection, coordinate flipping, and that **cgo-initiated calls dispatch into Go-implemented
  methods** via the dragSourceOperationMask accessor); registered-drag-types round-trip against AppKit's own
  bookkeeping; draw callbacks; and dealloc for never-installed views (installed views are deallocated by every
  test's apiDestroy-shaped cleanup).

### Discoveries (session 6)

1. **`[view display]` never calls `drawRect:`/`updateLayer` for a non-layer-backed view whose `wantsUpdateLayer`
   returns YES** — needsDisplay is simply cleared. Verified byte-for-byte identical behavior with a compiled
   Objective-C view of the old macContentView's shape (throwaway clang program), so this is pre-existing AppKit
   behavior, not a port difference: with the GL-based rendering, unison's draw callbacks in practice arrive via
   `updateLayer` (layer-backed windows) or the dirty-region machinery, not forced `display` calls. The draw test
   uses `displayRectIgnoringOpacity:inContext:` with a bitmap-backed NSGraphicsContext instead, which
   deterministically routes AppKit-initiated drawing into `updateLayer` headlessly.
2. `graphicsContextWithBitmapImageRep:` returns nil for non-premultiplied-alpha bitmap reps (bitmapFormat must be
   0, not `NSBitmapFormatAlphaNonpremultiplied`) — cost a puzzled half hour; noted in the test.
3. purego `FieldDef` ivars of type `objc.ID` work as cleanly as the session-5 bools (encode as `@`, generated
   getter/setter do plain assign — exactly the old MRC assign semantics; retain/release stays explicit in the
   methods that own their values).
4. The purego-generated method type encodings use `L` for uint64 (Apple treats `l`/`L` as 32-bit in encodings), so
   `methodSignatureForSelector:`-based marshaling against a Go-registered method would mis-parse NSRange args.
   Irrelevant for AppKit's direct-IMP dispatch (the spike + these tests prove IME works), but any NSInvocation
   aimed at our methods must build its signature from a corrected type string — as the setMarkedText test does.
   Worth remembering if some AppKit subsystem ever forwards to these methods via NSInvocation.

### Verification performed (session 6)

- `./build.sh --test` green; `golangci-lint run ./...` 0 issues (re-run after `golangci-lint fmt`); root package
  needed zero edits (verified by the untouched build).
- `go test ./internal/mac/` 10/10 fresh processes; `-count=5` single process; `-race`.
- darwin/amd64 under Rosetta 2 (`CGO_ENABLED=1 GOARCH=amd64 go test -c`): 5/5 fresh runs + `-test.count=3`, re-run
  after the formatting pass (covers amd64 stret returns, the SysV struct-arg callback classification for
  drawRect:/setMarkedText:/draggingSession:endedAtPoint:, and the NSPoint HFA/SSE returns from the Go fake
  dragging-info class).
- `GOOS=linux GOARCH={amd64,arm64} CGO_ENABLED=0 go build ./...` and windows/amd64 pass.
- `cmd/example` smoke-run: alive after 8s with empty stderr/stdout (SIGKILL per the session-3 note), in a locked
  session. Startup exercises the ported view in production shape: NewView → SetContentView/MakeFirstResponder →
  tracking-area installation → event pump delivering updateLayer draws through the Go IMPs.
- Not covered (need a human session): real CJK IME composition through a system input source (the
  interpretKeyEvents→insertText loop is proven for plain input; marked-text is proven via NSInvocation), real
  user-initiated drag & drop between apps (both directions are proven at the method level), scrollWheel: with a
  real device event, and live cursor-update/enter/exit from real pointer movement.

## Session 5 — 2026-07-10: Phase 2 — window + window delegate ported to purego; window_darwin.m and window_delegate_darwin.m deleted

The "Window + window delegate" bullet of Phase 2 is done. `macWindow` (NSWindow subclass) and `macWindowDelegate` are
now Go-registered Objective-C classes; all ~30 window functions are direct msgSends. Exported API unchanged — the
root package needed zero edits (verified: full build + `./build.sh --test` green with no root-package changes).

### What changed (session 5)

- **[internal/mac/window_darwin.go](internal/mac/window_darwin.go)** (new) — `Window` is now `objc.ID`-based (was
  `C.NSWindowRef`; both uintptr-kinded, so root's `nw == 0` check still compiles). `macWindow` uses purego
  `FieldDef` ivars (first use in shipped code) for the two `canBe*Window` bools: `NewWindow` sets them through the
  generated `setCanBeKeyWindow:`/`setCanBeMainWindow:` accessors right after init (same ordering as the old custom
  initializer, where super's init also ran before the ivars were assigned), and the `canBecomeKeyWindow`/
  `canBecomeMainWindow` overrides read them back via the generated `isCanBe*Window` getters — no unsafe ivar-offset
  arithmetic. The `WindowStyleMask`/`WindowCollectionBehavior`/`WindowLevel`/`WindowTabbingMode` aliases became
  `= uint64`/`= int64`; every enum constant value was verified by compiling and running a throwaway Objective-C
  program against the SDK (borderless=0, titled=1, closable=2, miniaturizable=4, resizable=8; managed=4,
  fsPrimary=128, fsNone=512; levels 0/3/101; tabbing 0/1/2; NSBackingStoreBuffered=2). The old bridge's CGRect
  by-pointer out-parameter style is gone: `Frame`/`ContentRectForFrameRect`/`FrameRectForContentRect` use
  `objc.Send[NSRect]` struct returns (stret on amd64), same shape Screen already uses.
- **[internal/mac/window_delegate_darwin.go](internal/mac/window_delegate_darwin.go)** (new) — `macWindowDelegate`
  with the 7 NSWindowDelegate methods. Design change (behavior identical): the old ObjC delegate stored its window
  in an ivar set at init; the Go delegate instead derives the window from each delegate message
  (`windowShouldClose:`'s sender IS the window; every `windowDid*:` notification's `object` is the window), so
  `NewWindowDelegate`'s Window parameter is now unused but kept for API compatibility. The six
  `Window*Callback` vars for delegate events moved here from all_darwin.go.
- **[internal/mac/all_darwin.go](internal/mac/all_darwin.go)** — Window and WindowDelegate sections removed
  (~380 lines incl. the 6 `//export goWindowDid*` funcs). The 17 view/drag callbacks (key/mouse/scroll/draw/drag)
  **stay** as `//export` shims until the view port, but their first parameter changed from `Window` (now a pure-Go
  type cgo can't export) to `C.NSWindowRef` with a `Window(w)` conversion inside — legal because cgo maps
  CFTypeRef-family typedefs to uintptr. `NewView` and `Menu.Popup` still pass `C.NSWindowRef(w)` unchanged.
- **macos.h** — Window/Window Delegate declarations and the `NSWindowDelegateRef` typedef removed; `newView` moved
  under the View section (`NSWindowRef` typedef stays for the remaining view/menu/export declarations).
- **[internal/mac/objc_darwin.go](internal/mac/objc_darwin.go)** — new helper `NewNSString` (owned +1 NSString via
  alloc/initWithBytes:length:encoding:, safe to use outside any autorelease pool, unlike the autoreleased
  `NSStringFromGo`). `Window.SetTitle` uses it so titles work from any call context, mirroring the old bridge's
  owned CFString discipline.
- **New tests**: [window_darwin_test.go](internal/mac/window_darwin_test.go) covers all four canBeKey/canBeMain
  combinations through real `canBecomeKeyWindow` dispatch (including overriding NSWindow's borderless=NO default in
  both directions), StyleMask round-trip, title round-trip (ASCII/CJK), SetTransparent (isOpaque/hasShadow),
  SetFrame/Frame round-trip, content/frame rect math inversion, visibility through MakeKeyAndOrderFront/OrderOut,
  and the delegate end-to-end: real AppKit-initiated `windowDidResize:`/`windowDidMove:` delivered synchronously
  from `setFrame:display:`, the `windowShouldClose:` nil/non-nil callback contract via msgSend, and the
  notification-object derivation driven with constructed NSNotifications. `TestNewNSString` added to
  objc_darwin_test.go.

### Discoveries (session 5)

1. **AppKit posts `windowDidMove:` only for origin-only frame changes** — a `setFrame:display:` that changes the
   size posts just `windowDidResize:` even when the origin also moved (empirical; the first test draft expected
   both). Same AppKit behavior under the old bridge, so nothing user-visible changes; worth knowing when the view
   port writes move/resize tests.
2. **A locked login session empties `CGGetActiveDisplayList`** (this machine was locked during this session:
   probe showed screenLocked=true, main display asleep, 0 active but 3 online displays). Session 3's
   `TestDisplayFunctions` failed on HEAD because of it (verified pre-existing via `git stash`). The test now skips
   when the list is empty and the main display is asleep; the misleading doc comment on `ActiveDisplayList` (it
   described the *online* list semantics) was corrected. Everything else — window creation, delegate delivery,
   AppKit dispatch, the example app — works fine in a locked session.

### Verification performed (session 5)

- `./build.sh --test` green; `golangci-lint run ./...` 0 issues; `golangci-lint fmt` applied (drive-by w32
  reformats it wanted were reverted to keep the diff focused).
- `go test ./internal/mac/` 10/10 fresh processes; `-count=5` single process; `-race`.
- darwin/amd64 under Rosetta 2 (`CGO_ENABLED=1 GOARCH=amd64 go test -c`): 5/5 fresh runs + `-test.count=3`
  (covers the amd64 stret paths for Frame/ContentRect math and the Go bool-returning `canBecome*` IMPs).
- `GOOS=linux GOARCH={amd64,arm64} CGO_ENABLED=0 go build ./...` and windows/amd64 pass.
- `cmd/example` smoke-run: alive after 8s with empty stderr/stdout (killed with SIGKILL per the session-3 note),
  despite the locked session. Startup exercises the ported path in production shape: NewWindow → NewWindowDelegate/
  SetDelegate → SetContentView/MakeFirstResponder/SetTitle → frame math → MakeKeyAndOrderFront, with the delegate's
  didResize feeding unison's layout.
- Not covered (need a human session): actually miniaturizing/zooming a window (the toggles are ported verbatim but
  only the getters and the delegate notifications are exercised — real minimize animates through the Dock and needs
  an unlocked, active session), key-window focus transitions (app activation would steal focus), and live-resize
  from a real user drag.

## Session 4 — 2026-07-10: Phase 2 — app + event loop ported to purego; app_darwin.m and event_darwin.m deleted

The "App + event loop" bullet of Phase 2 is done. `macAppDelegate` is now a Go-registered Objective-C class, the
Cmd+keyUp local event monitor is an Objective-C **block created from Go** (`objc.NewBlock` — first block in shipped
code; purego v0.11.0-alpha.6 has full block support via `__NSMallocBlock__`, which lives in libsystem so blocks work
before AppKit loads), and the event pump (`PollEvents`/`WaitEvents`/`WaitEventsTimeout`/`PostEmptyEvent`/
`StopMainEventLoop`) is pure msgSend. Exported API unchanged; the root package needed zero edits.

### What changed (session 4)

- **[internal/mac/app_darwin.go](internal/mac/app_darwin.go)** (new) — `InstallMacAppDelegate` registers
  `macAppDelegate` (6 delegate methods calling the App*Callback vars directly, incl. the uint64-returning
  `applicationShouldTerminate:` → NSTerminateCancel and `application:openURLs:` with NSURL→path conversion);
  class registration is `sync.Once`-guarded so install/uninstall/reinstall cycles work (the class can only be
  registered once per process; instances are per-install). `FinishLaunching` runs `[NSApp run]` until the
  didFinishLaunching callback stops it, then sets activation policy Regular — same flow as the ObjC bridge.
  Hide/unhide/activate and the four menu setters are one-line msgSends; `Menu` (still cgo-typed) converts to
  `objc.ID` legally because cgo maps CFTypeRef-family typedefs to uintptr.
- **[internal/mac/event_darwin.go](internal/mac/event_darwin.go)** (new) — `EventModifierFlags` + constants moved
  here from all_darwin.go; `DoubleClickInterval`, `CurrentModifierFlags`, `PostEmptyEvent` (struct NSPoint arg is
  all-float/SSE-class, so the amd64 straddle constraint does not apply), the pump functions, and
  `StopMainEventLoop`. `NSDefaultRunLoopMode` is resolved once via `NSStringConstant`. Autorelease pools bracket
  exactly the regions the old `@autoreleasepool` blocks did.
- **all_darwin.go / macos.h** — App and Event sections removed (~140 + ~40 lines, all 6 `//export goApp*` funcs);
  `app_darwin.m` and `event_darwin.m` deleted.
- **[internal/mac/objc_darwin.go](internal/mac/objc_darwin.go)** — **real bug found and fixed by the amd64 test
  run**: `WithPool` pushed/popped an autorelease pool without pinning the OS thread, but pools are per-thread and
  a Go goroutine can migrate between push and pop (the cgo bridge never had this hazard — its `@autoreleasepool`
  blocks lived inside single C calls). `PostEmptyEvent` is called from arbitrary goroutines in production, so this
  was a real crash waiting to happen (reproduced as a SIGSEGV in `objc_autoreleasePoolPop` under Rosetta,
  `-test.count=3`). `WithPool` now does `runtime.LockOSThread`/`UnlockOSThread` around the pool; `PoolPush`/
  `PoolPop` docs state the same-thread requirement for direct users.
- **Test infrastructure**: [testmain_darwin_test.go](internal/mac/testmain_darwin_test.go) (new) pins the main
  goroutine to the main OS thread (`init` + `runtime.LockOSThread`) and turns `TestMain` into a main-thread
  dispatcher: tests run on a secondary goroutine and submit main-thread work via `runOnMain`, while the dispatcher
  also pumps the main run loop between work items. The theme observer is now installed from the main thread in
  `TestMain` (the session-3 dedicated pump thread is gone — see discovery 1); later window/view test sessions can
  reuse `runOnMain` as-is.
- **New tests**: [app_darwin_test.go](internal/mac/app_darwin_test.go) covers the full delegate lifecycle
  (install → `FinishLaunching` through a real `[NSApp run]` with watchdog → willFinish/didFinish delivered →
  `terminate:` round-trip through the uint64-returning delegate method → `application:openURLs:` driven via
  objc_msgSend → keyUp-monitor block logic via `objc.InvokeBlock` plus a synthesized Cmd+keyUp NSEvent routed
  through the real queue → uninstall/reinstall) and the four menu setters against their AppKit getters (which also
  proves cgo Menu handles interoperate with the purego side). [event_darwin_test.go](internal/mac/event_darwin_test.go)
  proves the production wake-up contract (PostEmptyEvent → blocked WaitEvents returns) and both WaitEventsTimeout
  paths (expiry, and mid-wait wake by a posted event). Launch-once-per-process effects are handled: on `-count=N`
  reruns the launch notifications can't fire again and post-launch AppKit substitutes its own services/windows
  menus, so those asserts are gated on `isFinishedLaunching`.

### Discoveries

1. **The session-3 "dedicated pump thread" for NSDistributedNotificationCenter only worked by accident.** With the
   main goroutine unlocked (old TestMain), the pump goroutine's `LockOSThread` evidently landed on the real main
   thread (m0) while the main goroutine was parked. Once session 4's `init` locked the main goroutine to m0, the
   pump got a genuinely different thread and delivery silently stopped (theme test failed even in isolation, while
   HEAD passed). Delivery requires the default center to be created from and pumped on the **process main thread**
   — which is exactly the production configuration, so production was never affected. The test infrastructure now
   does it that way explicitly.
2. **Autorelease pools + goroutine migration** (the `WithPool` bug above): any purego port that brackets work with
   `objc_autoreleasePoolPush/Pop` from Go must pin the OS thread for the pool's lifetime. Worth remembering for
   every remaining Phase 2 file.
3. `-count=N` in one process is a genuinely different regime for AppKit lifecycle code (launch-once, AppKit-managed
   menus after launch). Keep running it — it caught both of the above along with the Rosetta run.

### Verification performed (session 4)

- `./build.sh --test` green (twice — after the port and after the WithPool fix); `golangci-lint run ./...` 0
  issues; `golangci-lint fmt` applied (no changes beyond the edits themselves).
- `go test ./internal/mac/` 10/10 fresh processes + 5 more after the fixes; `-count=5` single process; `-race`.
- darwin/amd64 under Rosetta 2 (`CGO_ENABLED=1 GOARCH=amd64 go test -c`): 5/5 fresh runs + 3/3 `-test.count=3`
  runs (this configuration exposed the WithPool bug before the fix).
- `GOOS=linux GOARCH={amd64,arm64} CGO_ENABLED=0 go build ./...` and windows/amd64 pass.
- `cmd/example` smoke-run: alive after 8s with empty stderr/stdout, killed with SIGKILL per the session-3 note.
  Startup exercises the entire ported path in production shape: InstallMacAppDelegate → FinishLaunching
  (`[NSApp run]` → delegate → stop) → WaitEvents/PollEvents pumping with real windows rendering.
- Not covered (need a human or a real session): applicationDidHide/hideOtherApplications side effects (would hide
  the user's other apps), ActivateIgnoringOtherApps (would steal focus), Cmd+keyUp forwarding to a real key window
  (needs the view/window port; the block logic and AppKit→block dispatch are covered).

## Session 3 — 2026-07-10: Phase 2 — sound, theme, screen (+display), image, and cursor ported to purego; their .m files deleted

The "five trivial files" bullet of Phase 2 is done. Each area moved from the cgo bridge to a pure-Go `_darwin.go`
file with the exported API unchanged; the corresponding `.m` files, `macos.h` declarations, and `all_darwin.go`
sections are gone. The CGDirectDisplay functions (the old "Display" section of `all_darwin.go`, plain C calls into
CoreGraphics) were ported alongside screen since `ScreenForDisplayID` needs `CGDisplayUnitNumber`.

### What changed

- **[internal/mac/sound_darwin.go](internal/mac/sound_darwin.go)** — `Beep` via `NSBeep` (dlsym from AppKit).
- **[internal/mac/theme_darwin.go](internal/mac/theme_darwin.go)** — `ThemeDelegate` is now a Go-registered
  Objective-C class (`objc.RegisterClass`, the first in shipped code); observer install is `sync.Once`-guarded.
  `IsDarkModeEnabled` reads AppleInterfaceStyle through NSUserDefaults instead of `CFPreferencesCopyAppValue` —
  same domains searched, same result, and it fixes a small CFString leak the old code had.
- **[internal/mac/display_darwin.go](internal/mac/display_darwin.go)** — the six CG functions via
  `purego.RegisterLibFunc` against the CoreGraphics framework; `DisplayID` is now `= uint32` (was
  `= C.CGDirectDisplayID`, same underlying type). `CGDisplayBounds`/`CGDisplayScreenSize` return CGRect/CGSize by
  value — first use of struct returns through RegisterLibFunc, verified on both arches.
- **[internal/mac/screen_darwin.go](internal/mac/screen_darwin.go)** — `Screen` is now `objc.ID`-based (was
  `C.NSScreenRef`; both uintptr-kinded, so root-package `== 0` checks still compile). Frame/VisibleFrame use
  `objc.Send[NSRect]` struct returns; `ConvertRectToBacking` passes NSRect by value (all-float structs, so the
  amd64 straddle constraint from Session 2 does not apply).
- **[internal/mac/image_darwin.go](internal/mac/image_darwin.go)** — `newNSImage` (unexported, +1 retain) builds
  the NSBitmapImageRep/NSImage pair; the colorSpaceName uses the real `NSCalibratedRGBColorSpace` constant via
  dlsym rather than a lookalike string. The still-cgo drag path in `all_darwin.go` now calls it and converts the
  handle with `C.NSImageRef(imgRef)` (legal: cgo maps CFTypeRef-family typedefs to uintptr).
- **[internal/mac/cursor_darwin.go](internal/mac/cursor_darwin.go)** — `Cursor` is `objc.ID`-based. `NewCursor`
  reproduces the old bridge's +2 retain count (alloc/init + retain) and the int truncation of the hot spot; the
  shared cursors keep their +1 retain so `Release` balances.
- **[internal/mac/objc_darwin.go](internal/mac/objc_darwin.go)** — helper additions: `LoadFramework` (generalizes
  `LoadAppKit`, returns the dlopen handle for symbol lookup), `NSStringConstant` (dlsym + deref of exported
  NSString* constants), and geom converters (`PointFromNSPoint`, `NSPointFromPoint`, `SizeFromNSSize`,
  `RectFromNSRect`, `NSRectFromRect`).
- Per-area `_darwin_test.go` files cover every ported function that can be exercised headlessly, including a
  delivery test for the full ThemeDelegate path (class registration → distributed notification → Go callback).

### Discoveries (all verified empirically)

1. **NSDistributedNotificationCenter delivers on the run loop of the thread that FIRST created the default
   center — the `addObserver:` thread is irrelevant.** Proven with a standalone control/poisoned repro: touching
   `defaultCenter` from a throwaway thread before registering breaks delivery permanently (0/3 vs 3/3). This was
   the cause of a ~40% full-suite test flake (another test's transient thread would win the first-touch race).
   Fixed in tests via `TestMain` starting a dedicated, permanently locked pump thread before any test runs, and
   documented on `InstallSystemThemeChangedCallback`: production is safe because unison installs the observer from
   the main thread at startup before anything else touches the center — same ordering the cgo bridge relied on.
2. **`NSCursor arrowCursor`/`IBeamCursor` return nil until NSApplication is initialized** (the other four shared
   cursors work regardless). Identical under the old ObjC bridge — unison always creates the shared application
   first, so only the tests needed a `sharedApplication` call.
3. **Pre-existing SIGTERM shutdown crash, NOT a regression**: `xos.Exit` runs `quitting()` on the signal-handler
   goroutine, so `Window.OrderOut` → AppKit `orderOut:` executes off the main thread while the main thread sits in
   `waitEvents`, and AppKit SIGTRAPs. Reproduced 3/3 with identical stacks on the *unmodified* tree (via
   `git stash`). Session 2's "exits cleanly on SIGTERM" observation does not hold today; the app runs fine and the
   crash is only in process teardown. Worth fixing separately (marshal shutdown to the main thread) — out of scope
   for the port.

### Verification performed (session 3)

- `./build.sh --test` green; `golangci-lint run ./...` 0 issues; `golangci-lint fmt` applied to internal/mac.
- `go test -race ./internal/mac/` 10/10 in fresh processes (was flaky ~40% before the TestMain fix), plus
  `-count=5` in a single process (validates the observer surviving reruns).
- darwin/amd64: `CGO_ENABLED=1 GOARCH=amd64 go test -c` binary run under Rosetta 2 — all tests pass, covering the
  amd64 hidden-pointer struct returns from C (CGDisplayBounds) and objc_msgSend_stret paths.
- `GOOS=linux GOARCH={amd64,arm64} CGO_ENABLED=0 go build ./...` and windows/amd64 still pass.
- `cmd/example` smoke-run: launches, renders, alive after 6s, empty stderr — startup exercises the ported
  display/screen (window placement), cursor, and theme (apiLateInit) paths in-process.

## Session 2 — 2026-07-10: Phase 0 complete (purego/objc feasibility spike proven on both arches) + Phase 2 foundation helpers

The spike turned out to be runnable without a human at the keyboard: windows open on this machine from a normal
session, drawRect: fires via AppKit, and mouse/key events can be synthesized as real NSEvents posted to the app's own
queue and dispatched through `sendEvent:`. All Phase 0 exit criteria were met on darwin/arm64 (native) and
darwin/amd64 (Rosetta 2).

### Headline finding: the port requires purego v0.11.0-alpha.6, not v0.10.1

- **purego v0.10.1 cannot do the port at all.** Its `NewCallback`/`NewIMP` panic on struct arguments and only allow
  integer/pointer/bool returns, and the arm64 callback assembly does not preserve x8 (the indirect struct-return
  register). `drawRect:` (NSRect arg) and `markedRange` (NSRange return) are unimplementable.
- **v0.11.0-alpha.6 (tagged 2026-07-02) adds full struct support for callbacks** on amd64/arm64: SysV eightbyte
  classification for args (including reading >16-byte structs from the stack and the "whole struct spills to stack
  when registers run out" rule), arm64 HFA args/returns in d0–d3 (NSRect return is an HFA — no x8 needed),
  16-byte returns in x0/x1 / rax/rdx, x8 indirect returns, and the amd64 hidden-pointer return (the callee skips the
  hidden first register arg). `go.mod` was bumped accordingly; `go mod tidy` run.
- The plan's "Ebitengine is fully cgo-free on macOS" prior-art claim is **wrong**: ebiten v2.9.9 still ships `.m`
  files in `internal/glfw` and uses cgo `//export` in `exp/textinput` on darwin. Feasibility no longer rests on that
  claim — the spike proved everything directly.

### Spike results (throwaway program, per plan not committed; all checks on BOTH arches)

PASS on darwin/arm64 and darwin/amd64:

- `RegisterClass` of an NSView subclass declaring the NSTextInputClient protocol; `conformsToProtocol:` true.
- IMPs written in Go, invoked through real objc_msgSend dispatch: `markedRange` (16-byte NSRange return),
  `firstRectForCharacterRange:actualRange:` (32-byte NSRect return + NSRange arg + out-pointer writeback),
  `characterIndexForPoint:` (NSPoint HFA arg), `insertText:replacementRange:` (id + NSRange args),
  `initWithFrame:` calling `objc.SendSuper` with a struct arg.
- AppKit-initiated calls: `drawRect:` received a correct NSRect after `orderFront` + `setNeedsDisplay:`;
  synthesized `NSEvent` mouse/key events posted via `postEvent:atStart:` were dispatched to `mouseDown:`/`keyDown:`
  through `nextEventMatchingMask:...` + `sendEvent:` loop turns wrapped in explicit autorelease pools.
- **IME text path**: `keyDown:` → `interpretKeyEvents:` → AppKit called our `insertText:replacementRange:` with the
  typed character — and it worked even with the app *inactive* under the accessory activation policy. (Real CJK
  input-source testing still needs a human; see Phase 2 verification bullet.)
- msgSend calling side: NSRange/NSRect/NSPoint pass+return round-trips via NSValue and `rangeOfString:`;
  `purego.NewCallback` plain-C callback (qsort comparator); `objc_autoreleasePoolPush`/`Pop`.

Two gotchas discovered, both worked around and documented:

1. **purego alpha.6 call-side bug (amd64)**: a 16-byte struct argument that no longer fits entirely in the remaining
   integer registers is *split* across r9 and the stack (`tryPlaceRegister` never checks remaining-register count;
   `addInt` silently overflows per-eightbyte). SysV requires the whole struct to go to memory. Hit by
   `setMarkedText:selectedRange:replacementRange:` (id + 2×NSRange) when *purego is the caller*. Proven callee-side
   correct with an NSInvocation (Foundation's compiled marshaling) as the caller — so AppKit→Go, the direction IME
   actually uses, is fine. Constraint documented in the header comment of
   [internal/mac/objc_darwin.go](internal/mac/objc_darwin.go): don't `Send` struct args after ≥4 preceding
   integer-register args on amd64 (audit each Phase 2 call; almost no bridge calls have that shape). Worth reporting
   upstream to ebitengine/purego.
2. **Click-through**: clicks on an inactive window are swallowed (used only to activate) unless the view implements
   `acceptsFirstMouse:` returning YES. Also, AppKit recomputes `locationInWindow` for synthesized events at delivery
   time, so exact coordinate round-trips can be off by a couple of points — irrelevant for real user events.

### Committed to the working tree

- **go.mod/go.sum**: purego v0.10.1 → v0.11.0-alpha.6 (hard requirement, see above). Verified the bump against
  everything that already uses purego: full `./build.sh --test`, linux amd64+arm64 and windows amd64
  `CGO_ENABLED=0 go build ./...`, and a 6-second live smoke-run of `cmd/example` (canvas's purego-based GL loading
  works under alpha.6).
- **[internal/mac/objc_darwin.go](internal/mac/objc_darwin.go)** — the Phase 0 "shared helper set" decision,
  realized as the first Phase 2 file (foundation helpers, coexists with the cgo bridge until the port lands):
  reuse `purego/objc` directly for classes/selectors/msgSend, plus a thin helper layer: NSPoint/NSSize/NSRect/NSRange
  ABI structs (Go type names chosen so purego's derived @encode strings resemble the real ones), `Sel`/`Cls` caches
  (`Cls` panics on unknown class names and lazily dlopens AppKit), `LoadAppKit`, `Retain`/`Release`/`Autorelease`,
  `PoolPush`/`PoolPop`/`WithPool`, NSString⇄Go string, NSArray⇄[]objc.ID, NSNumber int64/float64, NSURL⇄file path.
- **[internal/mac/objc_darwin_test.go](internal/mac/objc_darwin_test.go)** — permanent unit tests for the helper
  layer (string/array/number/URL round-trips, retain/release across nested pools) plus a struct-msgSend guard test
  (NSRect/NSPoint/NSRange round-trips) that will catch ABI regressions in future purego bumps on every
  `go test ./...` run on macOS. Found during testing: macOS returns NFD-normalized path strings from `NSURL.path`
  (pre-existing Cocoa behavior, same under the old cgo bridge) — documented on `FilePathFromNSURL`.

### Verification performed (session 2)

- `./build.sh --test` green (includes the new internal/mac tests); `go test -race ./internal/mac/` green.
- `golangci-lint run ./...` — 0 issues (note: `.golangci.yml` excludes `internal/mac/` from linting).
- `GOOS=linux GOARCH={amd64,arm64} CGO_ENABLED=0 go build ./...` and `GOOS=windows GOARCH=amd64 CGO_ENABLED=0
  go build ./...` pass with alpha.6 (Phase 1 GLX port unaffected).
- Spike binary: SPIKE OK on darwin/arm64 (native) and darwin/amd64 (Rosetta 2, `GOARCH=amd64 CGO_ENABLED=0`).
- `cmd/example` smoke-run: launches, renders, stays alive, exits cleanly on SIGTERM with purego v0.11.0-alpha.6.

## Session 1 — 2026-07-10: Phase 1 complete (Linux GLX → purego) + purego promoted to direct dependency

Phase 1 was done first (the plan marks it "small, do first") because it is fully verifiable from this machine via
cross-compilation, whereas the Phase 0 spike needs an interactive macOS GUI run.

### Done

- **Promoted `github.com/ebitengine/purego` v0.10.1 from indirect to direct** in `go.mod` (Phase 0, first bullet) via
  `go mod tidy` after adding the import. Checked for newer releases per the plan's risk note: v0.10.1 is still the
  latest stable; only v0.11.0 alphas exist, so we stay on v0.10.1.
- **Rewrote [internal/x11/glx_linux.go](internal/x11/glx_linux.go) with no `import "C"`** (was ~216 lines of cgo with
  `#cgo linux pkg-config: x11 gl`):
  - libX11 (`libX11.so.6` → `libX11.so`) and libGL (`libGL.so.1` → `libGL.so`) are dlopen'd on first `NewGLX` call
    (`sync.Once`), with clear errors naming the distro package to install when a library is missing. Missing symbols
    surface as errors, not panics (manual `Dlsym` + `RegisterFunc` instead of `purego.RegisterLibFunc`).
  - All 13 entry points from the plan's inventory are registered with typed Go func vars.
    `glXCreateContextAttribsARB` is resolved through `glXGetProcAddressARB` (not dlsym) and called with the same
    3.2-core attribs array as the old C `createContext` helper; if resolution fails, `CreateContext` returns nil,
    matching the old behavior.
  - `XVisualInfo` is a Go struct (`xVisualInfo`) matching the C layout on both 32-bit and 64-bit Linux (uintptr for
    pointers/`unsigned long`, int32 for `int`; unread members are blank `_` fields so the `unused` linter stays quiet).
  - Exported API is unchanged: `Display`, `FBConfig`, `GLXContext`, `GLXWindow`, `GLX` and all its methods,
    `Conn.NewGLX`. **Deviation from the plan's letter**: the plan said "uintptr-based named types" for all four, but
    `Display`/`FBConfig`/`GLXContext` are defined over `unsafe.Pointer` instead (only `GLXWindow`, an XID, is
    uintptr-based). Reason: [glcontext_linux.go](glcontext_linux.go) compares/assigns `x11.GLXContext` against `nil`,
    so a uintptr-based type would have forced caller changes the plan explicitly wanted to avoid; purego handles
    UnsafePointer-kind args/returns fine. glcontext_linux.go needed zero changes.
  - Two-connection design, transparent-visual selection logic, and the NVIDIA BadMatch comment preserved verbatim.
  - **Fixed a pre-existing double-free**: the old code both `defer C.XFree(configs)`'d and explicitly freed `configs`
    on the "no suitable framebuffer configuration" error path. The port keeps only the defer.

### Verification performed

- `GOOS=linux CGO_ENABLED=0 go build ./...` passes on amd64 and arm64 (was failing before with `undefined: x11.GLX`
  etc., since the cgo file was excluded); `internal/x11` also compiles for linux/386 and linux/arm.
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c ./internal/x11/` compiles the test binary.
- `GOOS=linux GOARCH=amd64 golangci-lint run ./internal/x11/` — 0 issues (also ran `golangci-lint fmt` for gofumpt).
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` passes (sanity check).
- Native macOS: `./build.sh --test` green; `golangci-lint run ./...` — 0 issues.

### Live Linux verification — done (2026-07-10)

- Rich ran the checked-in code on a live Linux machine after Session 2 and reports it working, closing the last
  Phase 1 bullet. (The finer-grained scenarios — XWayland vs X11, transparent window, NVIDIA driver,
  `clipboard_live_test.go` — were not individually itemized; re-check them if a Linux display regression appears.)

## What remains (in plan order)

1. **Phase 2 final manual verification (needs a human at the keyboard)**: all Phase 2 code is ported — the module
   is 100% cgo-free as of session 10 — but the plan's last Phase 2 bullet requires exercising the app by hand:
   real CJK input source (IME), popup-menu tracking (`Menu.Popup`), live drag & drop between apps, clipboard
   interchange with other apps, interactively choosing files in the open/save dialogs (the OK path), window
   minimize/zoom/focus in an active session, multi-monitor + retina scale changes, and transparent-window
   compositing. Each session's "Not covered" list has the per-area details.
2. **Software-GL fallback for the CI-skipped GL tests (requested by Rich, 2026-07-10)**: extend
   `requireAcceleratedGL` in `internal/mac/opengl_darwin_test.go` so that when the accelerated probe fails, the GL
   tests fall back to the Apple software renderer instead of skipping — either drop `NSOpenGLPFAAccelerated` and
   let `ClosestPolicy` choose, or explicitly request `NSOpenGLPFARendererID = kCGLRendererGenericFloatID`
   (0x00020400). A CGL probe on the M4 Max (2026-07-10) confirmed the software renderer still exists on Apple
   Silicon macOS 26 (`CGLQueryRendererInfo` lists id 0x01020400 with accelerated=0, majorGL=4) and that an explicit
   3.2-core pixel format requesting it succeeds, so it very likely works on the headless runners too. Keep the
   accelerated path preferred wherever it exists (production attributes unchanged; the fallback is test-only), make
   the test log which renderer it ran on, and keep a skip as the last resort if even software GL is unavailable.
3. **Phase 3**: build.sh/CI enforcement of `CGO_ENABLED=0`, `import "C"` guard, README setup-section rewrite
   (no C toolchain needed anywhere now; document runtime libX11/libGL dlopen on Linux and the new
   cross-compilation ability), `.claude/CLAUDE.md` architecture-note update, upack audit.

## Notes for future sessions

- **purego is now v0.11.0-alpha.6** (required — see session 2). When a stable v0.11.0 ships, bump to it and check
  whether the amd64 call-side struct-straddle bug (session 2) was fixed; the guard test in
  `internal/mac/objc_darwin_test.go` plus a re-run of the spike checks would validate the bump.
- purego `RegisterFunc` supports args/returns of Kind Uintptr, Ptr, UnsafePointer, ints, string, bool, floats, and
  Func — named types are matched by Kind, so the pattern used in glx_linux.go (typed handle types over
  unsafe.Pointer/uintptr) carries over directly to the Phase 2 mac port. As of v0.11.0-alpha.6, struct args/returns
  also work in both directions on amd64/arm64 (subject to the caller-side straddle constraint documented in
  internal/mac/objc_darwin.go).
- Phase 2 method-registration recipe proven by the spike: `objc.RegisterClass` with `MethodDef` Go funcs taking
  `(objc.ID, objc.SEL, ...)`; declare protocol conformance via `objc.GetProtocol("NSTextInputClient")`; name the Go
  ABI structs `NSPoint`/`NSSize`/`NSRect`/`NSRange` so derived @encode strings look right; call super with
  `objc.SendSuper` (struct args work). The content view must implement `acceptsFirstMouse:` → the old ObjC bridge got
  click-through behavior implicitly; check what `view_darwin.m` does today before changing behavior.
- Windows and the event loop work headlessly-ish from an agent session: accessory activation policy + synthesized
  `NSEvent`s posted with `postEvent:atStart:` + a `nextEventMatchingMask:`/`sendEvent:` pump wrapped in explicit
  autorelease pools. AppKit recomputes `locationInWindow` at delivery, so assert approximate coordinates. The
  darwin/amd64 side runs fine under Rosetta 2 (`GOARCH=amd64 CGO_ENABLED=0 go build`, run the binary directly) —
  re-verify both arches after each risky Phase 2 step.
- macOS returns NFD-normalized strings from `NSURL.path` — do not compare Cocoa-sourced paths to Go literals
  byte-for-byte in tests; use ASCII/CJK test data (see `TestNSURLFilePathRoundTrip`).
- `.golangci.yml` excludes `internal/mac/` from linting entirely, so lint passes say nothing about that directory —
  review new code there manually (gofumpt formatting still applies via `golangci-lint fmt` if desired).
- golangci-lint can cross-lint: `GOOS=linux golangci-lint run ./internal/x11/` works from macOS and caught real issues
  (gofumpt formatting, govet fieldalignment) that the native run never sees. Use `GOOS=<os> golangci-lint run` on any
  platform-suffixed files touched in future sessions.
- NSDistributedNotificationCenter delivery requires the default center to be created from and pumped on the
  **process main thread** (Session 3 discovery 1, sharpened by Session 4 discovery 1 — a non-main "first-touch"
  thread is not enough). Any future code observing distributed notifications must be installed from the main
  thread before other threads can touch the center. In internal/mac tests, `TestMain`
  ([testmain_darwin_test.go](internal/mac/testmain_darwin_test.go)) owns the locked main thread: it installs the
  theme observer first, pumps the main run loop, and services `runOnMain` closures — use `runOnMain` for anything
  AppKit requires on the main thread ([NSApp run], event pumping, and the upcoming window/view work).
- `WithPool` locks the goroutine to its OS thread for the pool's lifetime (Session 4 discovery 2). Direct
  `PoolPush`/`PoolPop` pairs are only safe on an already-locked thread — prefer `WithPool`.
- The GitHub macOS runners (both arches) have a WindowServer session (windows, views, events, and drawing all work)
  but **no hardware-accelerated OpenGL renderer**, so any test needing a real GL config must gate on
  `requireAcceleratedGL(t)` (session 7 CI followup). Relatedly, `t.Fatal`/`t.Skip` inside a `runOnMain` closure
  Goexits the main-thread pump; `runPumped` in testmain_darwin_test.go makes that survivable, but skips must still
  be issued from the test goroutine (a skip from inside a closure marks the test skipped, yet FailNow/SkipNow off
  the test goroutine remains documented misuse — gate before entering `runOnMain` when possible).
- While cgo and purego coexist in internal/mac, a Go-registered Objective-C class name must not collide with one
  still compiled from a `.m` file (objc_allocateClassPair fails). Delete the `.m` file in the same step that
  registers the class from Go, as done for ThemeDelegate. (Moot since session 10 — no cgo remains — but relevant
  if a `.m` file ever comes back.)
- Application-modal sessions are testable headlessly (session 10): schedule the ending action with
  `performSelector:withObject:afterDelay:inModes:` in **NSModalPanelRunLoopMode** so the timer fires inside the
  modal run loop, and always add an `abortModal` backstop — `runModal` blocks the test suite's main-thread pump,
  so a wedged modal session would otherwise hang the package to its timeout. Cancel pending performs with
  `cancelPreviousPerformRequestsWithTarget:` once the session ends. See `cancelModalAfter` in
  [open_panel_darwin_test.go](internal/mac/open_panel_darwin_test.go).
- The SIGTERM shutdown crash (Session 3 discovery 3) is pre-existing and unrelated to the port: `xos.Exit` calls
  window teardown from the signal-handler goroutine, off the main thread. Smoke-testing the example app should
  assert liveness and use SIGKILL, not judge success by SIGTERM exit status. Consider a separate fix that marshals
  quitting() to the main event loop.
- purego `objc.FieldDef` ivars work well for Go-registered classes (proven by macWindow): a `ReadWrite` bool field
  named `foo` generates `setFoo:` and `isFoo` accessor methods, so instance state needs no unsafe offset arithmetic
  and no Go-side handle→state maps. Non-bool getters use the plain field name.
- Everything ported so far works in a **locked login session** (this machine was locked for all of Session 5):
  window creation/ordering, Go-registered class dispatch, delegate delivery, and the example app all behave
  normally. Only `CGGetActiveDisplayList` degrades (empty while displays sleep — `TestDisplayFunctions` skips
  itself in that state). Unattended agent sessions can keep testing AppKit paths without an unlocked screen, but
  focus/miniaturize/zoom behavior still needs a human with an active session.
- When a Go export shim must keep receiving an object handle from remaining `.m` code while the Go-side type has
  already moved off cgo (Session 5: `Window`), declare the shim parameter as the CFTypeRef-based C typedef
  (`C.NSWindowRef`) and convert — cgo maps CFTypeRef-family typedefs to uintptr, so `Window(w)` is legal, and cgo
  cannot export parameters of pure-Go named types like `objc.ID`. (As of Session 6 no export shims remain; the
  same uintptr-kinded conversion trick still carries purego-side handles INTO remaining cgo calls, e.g.
  `C.NSViewRef(view)` in NewOpenGLContext and `DragInfo(sender)` from the Go drag methods.)
- The purego-generated Objective-C type encodings for Go-registered methods use `L` where the real encoding would
  be `Q` (Apple parses `l`/`L` as 32-bit), so never drive a Go-registered method through
  `methodSignatureForSelector:`-derived NSInvocations — build the signature from a hand-written correct type
  string instead (see the setMarkedText test in view_darwin_test.go). AppKit's normal direct-IMP dispatch is
  unaffected.
- Headless AppKit draw testing: `[view display]` clears needsDisplay without calling `drawRect:`/`updateLayer` on
  a non-layer-backed view whose `wantsUpdateLayer` is YES (identical under compiled ObjC — Session 6 discovery 1).
  Use `displayRectIgnoringOpacity:inContext:` with a bitmap-backed NSGraphicsContext (bitmapFormat 0 —
  premultiplied — or the context comes back nil) to force AppKit-initiated drawing deterministically.
