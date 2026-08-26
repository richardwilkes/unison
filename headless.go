// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/thememode"
)

// A headless session replaces the operating system with an in-memory stand-in, so an application's real event loop,
// windows, focus handling, modal dialogs and drawing can be exercised on a machine with no windowing system at all.
// Rendering is forced onto the CPU raster path and the resulting pixels are kept rather than blitted to a screen, so
// what would have been shown can be read back as an image.
//
// A session is not tied to the life of the process: Start() returns when the session ends, so one test binary can run
// several sessions one after another. Everything a session touches that outlives it — the window list, the task queue,
// the startup option callbacks, the theme state, the built-in cursors — is reset when it ends, so the next session
// starts from the same clean slate the first one did.
//
// One thing deliberately survives: a timer armed with InvokeTaskAfter counts down on a goroutine of its own, which no
// teardown can reach, so it may fire after its session has ended and enqueue a task into a later one — or into none at
// all, in which case the task simply sits in a queue that the next session's beginStartup discards. That is harmless,
// because every closure this package arms a timer with acts solely on the one object it belongs to and either checks
// that the object is still usable or does something that has no effect once it is not:
//
//   - Field.blink (field.go) redraws the caret only while the field's window is still valid, and scheduleBlink asks the
//     same question before arming the next one.
//   - tooltipSequencer.show and tooltipSequencer.close (tooltip.go), armed from Window.updateTooltip and from show
//     itself, act only while the window's tooltipSequence still matches the one they captured. Destroying a headless
//     window bumps that sequence, so a tooltip left in flight when a session ends is inert before it ever fires.
//   - ProgressBar.animationTick (progress_bar.go) does nothing but mark its bar for redraw, which Panel.MarkForRedraw
//     ignores for a detached panel and Window.MarkForRedraw ignores for a window that is no longer valid.
//   - Table.EventuallySizeColumnsToFit and Table.EventuallySyncToModel (table.go) resize and resync the table's own
//     columns and row cache, which nothing outside the table can observe once it is detached.
//   - releaseImagesForContext (image.go) arms a timer that only runs the garbage collector.
//   - x11RunDialogModal (file_dialog_linux.go) arms one that starts the helper process for the window it is itself
//     about to run a modal loop for. No session ever arms that one, since a session's file dialogs are the pure-Go
//     in-window ones.
//
// The wake-up such a task performs is safe as well: the native postEmptyEvent implementations post nothing once the
// platform has been torn down, macOS and Windows because platformInited is clear and Linux because its atomic
// connection handle is nil.

// HeadlessConfig describes the stand-in screen a headless session renders onto.
type HeadlessConfig struct {
	// Width of the screen, in logical points. Required to be greater than zero.
	Width float32
	// Height of the screen, in logical points. Required to be greater than zero.
	Height float32
	// Scale is the backing scale (device pixels per logical point) of the screen. Zero or less means 1.
	Scale float32
	// Background is the color Capture() paints behind the windows. The zero value means an opaque mid-gray, so that
	// both light and dark window content stands out against it. It is ignored when TransparentBackground is set.
	Background Color
	// SyncTimeout bounds how long Sync(), and therefore every driver call that waits for the application to go quiet,
	// will wait. Zero or less means 10 seconds.
	SyncTimeout time.Duration
	// DarkMode is what IsDarkModeEnabled() reports while the theme mode is thememode.Auto.
	DarkMode bool
	// TransparentBackground makes Capture() paint nothing behind the windows, so that the alpha of what they present —
	// a window created with TransparentWindowOption, say — can be read back from the capture rather than being blended
	// into a background. It exists because the transparent color is also the zero value of Background, which asks for
	// the default instead.
	TransparentBackground bool
}

// headlessBackgroundLevel is the gray level used for HeadlessConfig.Background when it is left at its zero value.
const headlessBackgroundLevel = 0x40

// headlessDefaultSyncTimeout is how long Sync() waits for the application to go quiet when HeadlessConfig.SyncTimeout
// is left at its zero value. It is generous enough that no test doing real work will ever reach it, since reaching it
// means the application is misbehaving rather than merely slow.
const headlessDefaultSyncTimeout = 10 * time.Second

// normalized returns the configuration with its optional fields filled in, or an error if a required field is
// unusable.
func (c HeadlessConfig) normalized() (HeadlessConfig, error) {
	// Written as "not greater than zero" rather than "less than or equal to zero" so that NaN, which fails every
	// comparison, is rejected along with the negatives. A NaN or infinite dimension would otherwise reach the int
	// conversions in captureScreen and the display frame, whose result is implementation-defined.
	if !headlessFinitePositive(c.Width) || !headlessFinitePositive(c.Height) {
		return c, errs.Newf("headless screen size must be finite and positive, but was %vx%v", c.Width, c.Height)
	}
	switch {
	case math.IsNaN(float64(c.Scale)) || math.IsInf(float64(c.Scale), 0):
		return c, errs.Newf("headless screen scale must be finite, but was %v", c.Scale)
	case c.Scale <= 0:
		c.Scale = 1
	}
	switch {
	case c.TransparentBackground:
		c.Background = Transparent
	case c.Background == 0:
		c.Background = RGB(headlessBackgroundLevel, headlessBackgroundLevel, headlessBackgroundLevel)
	}
	if c.SyncTimeout <= 0 {
		c.SyncTimeout = headlessDefaultSyncTimeout
	}
	return c, nil
}

// headlessFinitePositive reports whether v is a usable dimension: greater than zero and not infinite. NaN fails the
// comparison and so is rejected too.
func headlessFinitePositive(v float32) bool {
	return v > 0 && !math.IsInf(float64(v), 0)
}

// HeadlessScreen is a running headless session: the in-memory screen the application is drawing onto, and the handle
// used to drive it. StartHeadless() creates one and hands it back; ActiveHeadlessScreen() returns the one currently
// standing in for the operating system, if there is one.
//
// The methods fall into five groups:
//
//   - Injecting input, the events an operating system would have delivered: MouseMove, MouseDown, MouseUp, Click,
//     ClickWith, DoubleClick, Drag, Wheel, KeyDown, KeyUp, KeyPress and Type. Positions are in the screen's logical
//     coordinate space; PanelCenter and PanelPoint convert a widget's own coordinates into it.
//   - Drag & drop arriving from outside the application: BeginExternalDrag, DropExternal and LastDrag. A drag the
//     application starts itself needs none of these — Drag() is all it takes.
//   - Running code on the UI thread: Do, Post and Sync. Anything a test wants to read out of the application, from a
//     widget's state to the window list, must be read inside Do, since it belongs to that thread.
//   - Looking at the result: Size, Scale, WindowAt, FocusedWindow, Cursor, Capture, CaptureWindow, Beeps and Errors,
//     plus SetDarkMode to change what the session tells the application about the theme.
//   - Ending the session: Quit, Stop, Wait, Done and Running.
//
// Everything other than Size, Scale, Beeps, Errors, Running and Done performs its work on the UI thread and waits for
// it, so once the session has ended they return zero values instead of blocking — except LastDrag, which goes on
// reporting how the final drag ended, since a session ending underneath a drag is itself one of the ways a drag ends.
// Sessions run one at a time within a process, never side by side, and a session owns most of this package's mutable
// globals while it runs, so tests that use one must not call t.Parallel.
type HeadlessScreen struct {
	// The fields are ordered for packing rather than by role. cfg never changes once the session has been created and
	// may therefore be read from any goroutine. The locks, channels and atomics are the cross-goroutine plumbing that
	// lets a test on its own goroutine talk to the UI thread. Everything else is UI-thread only and is touched solely
	// from within a closure that the UI thread is running.
	display *Display
	ready   chan struct{}
	done    chan struct{}
	wake    chan struct{}
	cursor  *Cursor
	focused *Window
	hover   *Window
	capture *Window
	drag    *headlessDragSession
	buttons map[int]bool
	input   []func()
	// recorded is what Errors() reports. It is named for its role rather than for its content, since this file also
	// calls into the errs package.
	recorded []error
	stack    []*Window
	// The cursors that existed before the session started, taken out of the way by beginStartup and put back by
	// finish(). See beginStartup for why a session must neither adopt nor destroy them.
	priorCursors                []*Cursor
	priorBuiltInCursors         []*Cursor
	priorCursorChangedCallbacks []*func()
	priorCursorSettings         *cursorSettings
	priorMenuFactory            MenuFactory
	clipboard                   []drag.Data
	lastDrag                    HeadlessDragResult
	inputLock                   sync.Mutex
	errLock                     sync.Mutex
	terminated                  atomic.Bool
	beeps                       atomic.Int32
	pointer                     geom.Point
	cfg                         HeadlessConfig
	// hover, capture, buttons, pointer and lastMods together make up the input router's state (headless_input.go):
	// which window the pointer is over, which one has grabbed it for the duration of a press, which buttons are down,
	// and where the pointer is with which modifiers held. drag, when it is not nil, takes the pointer away from all of
	// that for the duration of a drag & drop session (headless_drag.go), and lastDrag is how the one before it ended.
	lastMods mod.Modifiers
	// priorThemeMode is the theme mode in force before the session started, which beginStartup replaces with
	// thememode.Auto and finish() puts back, so that a session neither inherits a mode set before it nor leaves its own
	// behind.
	priorThemeMode   thememode.Enum
	darkMode         bool
	prevCPURendering bool
	readyClosed      bool
}

// headlessState is the same type as HeadlessScreen, named for its other role: the in-memory stand-in for the operating
// system that the api* wrappers in platform_api.go dispatch to.
type headlessState = HeadlessScreen

// newHeadlessScreen validates cfg and creates the session it describes. The session is inert until it is published via
// headlessActive.
func newHeadlessScreen(cfg HeadlessConfig) (*HeadlessScreen, error) {
	cfg, err := cfg.normalized()
	if err != nil {
		return nil, err
	}
	frame := geom.NewRect(0, 0, cfg.Width, cfg.Height)
	return &HeadlessScreen{
		display: &Display{
			Frame:  frame,
			Usable: frame,
			Scale:  geom.NewPoint(cfg.Scale, cfg.Scale),
			// There is no physical screen to ask, so assume the conventional 96 dpi at 1x and scale from there.
			PPI:      int(defaultDisplayPPI * cfg.Scale),
			Primary:  true,
			headless: true,
		},
		ready:    make(chan struct{}),
		done:     make(chan struct{}),
		wake:     make(chan struct{}, 1),
		buttons:  make(map[int]bool),
		cfg:      cfg,
		darkMode: cfg.DarkMode,
	}, nil
}

// Headless returns a startup option that runs the application against an in-memory screen of the given size rather
// than the operating system's. Use it when you want to run Start() yourself; StartHeadless() is the more convenient
// entry point for tests, since it runs Start() on its own goroutine and hands back the driver.
//
// The session is created and published when Start() applies the option, which is before any part of the application
// has had a chance to talk to the platform. Each application creates a session of its own, so one option value may be
// handed to Start() as many times as there are sessions to run; nothing of an earlier session is carried over. An
// invalid configuration, or a session that is still running, is reported as an option error, which Start() treats the
// same way it treats every other bad option.
func Headless(cfg HeadlessConfig) StartupOption {
	return func(_ startupOption) error {
		hs, err := newHeadlessScreen(cfg)
		if err != nil {
			return err
		}
		// Publish only if nothing else is published: a session that is still running owns the globals this one would
		// start writing into, and the check that Start() itself goes on to make would refuse the second application
		// anyway, so swapping its session out from under the first would only leave that one unable to finish.
		if !headlessActive.CompareAndSwap(nil, hs) {
			return errs.New("a headless session is already active")
		}
		return nil
	}
}

// headlessOption returns the startup option that publishes hs as the active session.
func headlessOption(hs *HeadlessScreen) StartupOption {
	return func(_ startupOption) error {
		headlessActive.Store(hs)
		return nil
	}
}

// StartHeadless runs the application against an in-memory screen on its own goroutine and returns once the application
// has settled, so that any windows the StartupFinishedCallback created already exist and anything it set is already
// visible to the caller. The returned HeadlessScreen is how the calling goroutine drives and inspects the running
// application.
//
// A StartupFinishedCallback that runs a nested event loop of its own — a first-run dialog put up with RunModal(), say —
// does not hold this up: what the caller is given back is then an application parked in that loop with nothing left to
// do, ready for the test to inject the input that dismisses it.
//
// Unlike Start(), which exits the process when it cannot get going, every failure here is reported as an error: a test
// binary must survive a session that refuses to start. End the session with Quit() or Stop(); until one of those has
// returned, the globals the session owns still belong to it, so a second session cannot be started. From a test, the
// reliable way to arrange that is t.Cleanup(screen.Stop), which ends the session however the test itself ends.
func StartHeadless(cfg HeadlessConfig, options ...StartupOption) (*HeadlessScreen, error) {
	hs, err := newHeadlessScreen(cfg)
	if err != nil {
		return nil, err
	}
	initTermLock.Lock()
	switch {
	case headlessActive.Load() != nil:
		err = errs.New("a headless session is already active")
	case initialized:
		err = errs.New("already initialized")
	case initializing:
		err = errs.New("initialization already in progress")
	case terminating:
		err = errs.New("termination in progress")
	default:
		// Publish the session here rather than leaving it to the startup option below, so that the gap between this
		// call and Start() actually applying the option isn't a window in which a second call could sneak past the
		// check above. Nothing else can observe the session yet, since Start() has not run.
		headlessActive.Store(hs)
	}
	initTermLock.Unlock()
	if err != nil {
		return nil, err
	}
	// Apply the caller's options here, on this goroutine, rather than handing them to Start(): Start() runs
	// xos.ExitIfErr on each one, which would take the test binary down with it, and the promise made above is an error
	// instead. Applying an option only writes into a package-level variable, and those belong to this session from the
	// moment it was published above, so there is nothing to race with. Start() is then given the headless option alone,
	// with the `go` statement ordering these writes ahead of every read it makes of them.
	for _, option := range options {
		if err = option(startupOption{}); err != nil {
			// Nothing has started, so put back everything the options that did run wrote, session included, leaving the
			// process as untouched as a failed size check would have.
			resetStartupOptions()
			headlessActive.Store(nil)
			return nil, err
		}
	}
	go Start(headlessOption(hs))
	if err = hs.awaitStartup(); err != nil {
		return nil, err
	}
	// ready is closed on the way into the StartupFinishedCallback rather than on the way out of it, so that a callback
	// which parks in a nested event loop cannot strand this wait. Settling is what turns that into the guarantee the
	// doc makes: every driver call goes through the task queue, and the probe Sync() posts can only run once
	// finishStartup — the task the callback is running inside — has returned, or inside a nested loop the callback
	// started. So by the time this comes back, either the callback has finished, with everything it did visible here
	// through the happens-before of the probe's done channel, or it is parked in a loop with nothing left to do and the
	// test is free to drive it. A session that ended during startup falls straight through, since Sync() returns
	// immediately when there is no session left to ask.
	hs.Sync()
	return hs, nil
}

// awaitStartup blocks until the session has released its caller, which lateInit does on the way into the
// StartupFinishedCallback, or has ended without ever doing so, which is reported as an error.
func (s *HeadlessScreen) awaitStartup() error {
	select {
	case <-s.ready:
	case <-s.done:
		// The session ended before startup finished. Both channels may be closed by now — a StartupFinishedCallback
		// that quits immediately still lets ready close on its way out — so recheck before calling this a failure.
		select {
		case <-s.ready:
		default:
			return errs.New("the headless session ended before startup completed")
		}
	}
	return nil
}

// ActiveHeadlessScreen returns the headless session that is currently standing in for the operating system, or nil if
// the application is running against the real one. It is safe to call from any goroutine.
func ActiveHeadlessScreen() *HeadlessScreen {
	return headlessActive.Load()
}

// resetStartupOptions puts every package-level variable a StartupOption writes back to the value it has in a process
// that has never applied one. Each option writes into one of these, so an option applied for a session would otherwise
// still be in place for the next one — or for an unrelated test that never asked for it at all.
func resetStartupOptions() {
	startupFinishedCallback = nil
	openFilesCallback = nil
	themeChangedCallback = nil
	recoveryCallback = nil
	quitAfterLastWindowClosedCallback = nil
	allowQuitCallback = nil
	quittingCallback = nil
	noGlobalMenuBar = false
	noPlatformFileDialogs = false
}

// recordError appends err to the list Errors() reports. It is installed as the recovery callback when the application
// did not supply one of its own, so that a panic inside a task or a widget callback becomes something a test can
// assert on instead of a line in the log. It may be called from any goroutine, since xos.SafeCall reports panics on
// whichever goroutine they happened on.
func (s *headlessState) recordError(err error) {
	s.errLock.Lock()
	s.recorded = append(s.recorded, err)
	s.errLock.Unlock()
}

// finish returns every piece of package-level state the session touched to the value it had before the session
// started, then releases the goroutines waiting on the session. It runs on the UI goroutine after the event loop has
// exited, which is the last moment at which that state is still exclusively ours.
func (s *headlessState) finish() {
	windowList = nil
	modalStack = nil
	// A fresh map rather than clear(), so that a stray reference to the old one cannot resurrect entries here.
	redrawSet = make(map[*Window]struct{})
	wndWithCurrentCtx = nil
	s.stack = nil
	s.focused = nil
	s.hover = nil
	s.capture = nil
	// lastDrag is deliberately left alone: it is the session's own rather than a global, and terminate() has already
	// recorded a drag that was still in flight as canceled, which LastDrag() goes on reporting after the end.
	s.drag = nil
	clear(s.buttons)
	s.inputLock.Lock()
	clear(s.input)
	s.input = nil
	s.inputLock.Unlock()
	taskQueueLock.Lock()
	clear(taskQueue)
	taskQueue = nil
	taskQueueHead = 0
	taskQueueLock.Unlock()
	initTermLock.Lock()
	initialized = false
	initializing = false
	terminating = false
	initTermLock.Unlock()
	platformInited.Store(false)
	uiGoroutineID.Store(0)
	quitLock.Lock()
	calledAtExit = false
	quitLock.Unlock()
	resetStartupOptions()
	// The in-window file dialogs remember the directory they were last used in, which the next session must no more
	// inherit from this one than this one inherited it from whatever ran before.
	lastWorkingDir = ""
	// The mode goes back to whatever beginStartup found, as the cursors do, rather than to Auto: what the session set
	// is the session's own, and what was set before it belongs to whatever set it.
	currentThemeMode.Store(int32(s.priorThemeMode))
	needPlatformDarkModeUpdate = true
	platformDarkModeEnabled = false
	MarkDynamicColorsForRebuild()
	cpuRenderingActive = s.prevCPURendering
	// The session's own factory is the in-window one, which nothing outside the session should go on using, so what
	// goes back is whatever beginStartup found: nil in the ordinary case, and the native factory something built
	// before the session otherwise.
	defaultMenuFactory = s.priorMenuFactory
	s.priorMenuFactory = nil
	// The built-in cursors a headless session hands out are inert and deliberately not recorded in cursorList, so
	// finishQuit's teardown loop over that list never sees them and never clears the singletons that point at them.
	// Dropping the session's own is therefore this loop's job, or the next session would keep using cursors belonging
	// to the one that just ended. What goes back in their place is whatever beginStartup took out of the way, which is
	// nil for each of these in the ordinary case of nothing having existed before the session, and the cursors and
	// callbacks of whatever ran earlier in the process otherwise. Those were never the session's to destroy, and
	// nothing it did can have reached them.
	for i, p := range builtInCursors() {
		if i < len(s.priorBuiltInCursors) {
			*p = s.priorBuiltInCursors[i]
		} else {
			*p = nil
		}
	}
	builtCursorSettings = s.priorCursorSettings
	cursorList = s.priorCursors
	cursorChangedCallbacks = s.priorCursorChangedCallbacks
	s.priorBuiltInCursors = nil
	s.priorCursors = nil
	s.priorCursorChangedCallbacks = nil
	s.priorCursorSettings = nil
	// A drag that was still in flight when the session ended leaves these pointing at what it picked up. Each is
	// normally cleared by the cleanup the drag source registered, which does not run if the session ended first.
	dragTableData = nil
	dragDockable = nil
	headlessActive.Store(nil)
	close(s.done)
}
