// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// Package unison is a cross-platform GUI toolkit for Go desktop applications, rendering with an OpenGL context.
//
// # Threading model
//
// Unison is single-threaded. Almost all of its state — every [Panel] and the widgets built on it, every [Window], all
// drawing via [Canvas], and the objects behind images, fonts, paints, and the like — is owned by one dedicated OS
// thread, referred to throughout as the UI thread. None of these types are safe for concurrent use. Reading or mutating
// them from any other goroutine is a data race that will eventually crash or corrupt the application, even if it
// appears to work in testing.
//
// The UI thread is the program's main thread. The package's init function calls [runtime.LockOSThread] so that main is
// pinned to a single OS thread, and [Start] takes over that thread to run the event loop. Start never returns; the
// application is driven entirely by events and tasks dispatched on this thread.
//
// # Code that already runs on the UI thread
//
// You do not need to do anything special when your code is invoked by Unison as part of normal event handling. The
// following all run on the UI thread, so they may freely touch panels, windows, and drawing state:
//
//   - The function passed to [StartupFinishedCallback], where applications create their initial windows.
//   - Panel input callbacks (the fields of [InputCallbacks]: MouseDownCallback, KeyDownCallback, and so on), drawing
//     callbacks (DrawCallback, DrawOverCallback), layout (the [Layout] and [Sizer] interfaces), and the other Panel
//     and Window callbacks.
//   - Command "can-perform" and "perform" handlers installed via [Panel.InstallCmdHandlers].
//   - Functions handed to [InvokeTask], [InvokeTaskAfter] and [InvokeTaskAndWait].
//
// # Reaching the UI thread from another goroutine
//
// When work happens off the UI thread — a network fetch, a file scan, any goroutine you start — it must not touch UI
// objects directly. Instead, marshal the UI work back onto the UI thread:
//
//   - [InvokeTask] queues a function to run on the UI thread at the next opportunity.
//   - [InvokeTaskAfter] does the same after a delay.
//   - [InvokeTaskAndWait] queues a function and blocks the caller until the UI thread has run it.
//
// These three are the supported entry points that are safe to call from any goroutine; they are internally synchronized
// and wake the event loop as needed. A typical background operation computes its result off-thread, then calls
// InvokeTask with a closure that applies the result to the UI:
//
//	go func() {
//	    data, err := fetch()
//	    unison.InvokeTask(func() {
//	        // Runs on the UI thread; safe to update widgets here.
//	        if err != nil {
//	            label.SetTitle(err.Error())
//	        } else {
//	            label.SetTitle(string(data))
//	        }
//	        label.MarkForRedraw()
//	    })
//	}()
//
// # What is and isn't synchronized
//
// Most of Unison deliberately uses no locks, because the single-threaded contract makes them unnecessary on the UI
// thread. The handful of facilities meant to be reachable from other goroutines — the task queue behind InvokeTask and
// the internal image cache — are individually synchronized. Do not infer from this that other types are safe to share;
// assume any type not documented otherwise is UI-thread-only. In particular, window and panel methods such as
// MarkForRedraw, SetFrameRect, AddChild, and a widget's content setters operate on unsynchronized shared state and must
// only be called on the UI thread.
//
// # Accessibility
//
// Unison describes its windows to the assistive technologies the operating system provides, so that a screen reader can
// read an application's controls, follow the keyboard focus, report what is being typed and act on the user's behalf.
// It speaks NSAccessibility on macOS (VoiceOver), UI Automation on Windows (Narrator, NVDA, JAWS) and AT-SPI2 on Linux
// (Orca), and it is pure Go on all three: no C library and no accessibility bridge is involved.
//
// Every widget in the library describes itself, so an application is largely accessible without doing anything. What
// cannot be derived is what has no text to derive it from: a control whose appearance is its only label, such as an
// icon-only [Button] or a [DrawablePanel], has to be named, and a label that is not simply the sibling preceding the
// control it names has to say what it labels.
//
//	search.Accessibility.Name = "Search"  // Name a control that has no text of its own
//	field.Accessibility.LabeledBy = label // Point a control at the label that names it
//
// Everything an application says about a panel goes through [Panel.Accessibility], an [AccessibilityInfo] held in the
// panel by value, so its fields are set in place rather than allocated. All of them are optional:
//
//   - Name is what is announced for the panel, overriding whatever name would otherwise be derived from it.
//   - Description elaborates on the name. When it is empty, the panel's tooltip text is used instead.
//   - Role is what kind of element the panel is, from [github.com/richardwilkes/unison/enums/role]. The zero value,
//     role.Auto, derives it from the widget, and role.None hides the panel entirely, promoting its children into its
//     parent.
//   - LabeledBy is the panel whose text names this one, for a control whose label is not the sibling before it. The
//     association is reported whether or not it also supplies the name, since assistive technologies offer the label as
//     an element of its own.
//   - Callback runs last, after everything else about the panel's node has been decided, and may adjust anything on it.
//     It is how a fact with no field of its own, such as a heading's level, gets reported.
//   - ActionCallback handles requests from an assistive technology for a panel that has no widget type of its own.
//
// A custom widget describes itself by implementing [AccessibilityProvider], and carries out what an assistive
// technology asks of it by implementing [AccessibilityActor]. Both are looked for on Panel.Self, so they must be
// implemented by the widget type rather than by an embedded [Panel]. ProvideAccessibility is handed an
// [AccessibilityBuilder] whose node already holds everything derivable from the panel alone — its bounds, whether it is
// enabled, focusable and focused, and the actions those imply — so an implementation sets only what it knows better:
// the role, the value, the states that matter and the actions it can carry out. The builder also offers
// [AccessibilityBuilder.VisibleRect], for a widget with more content than it can show, and
// [AccessibilityBuilder.AddVirtualChild], for the elements a widget draws without a panel apiece, as the rows and cells
// of [Table] and [List] do.
//
// [AnnounceForAccessibility] speaks a message that no change to a window expresses — a background task that finished,
// say. It does nothing when no assistive technology is being served, so it may be called unconditionally, and it is
// safe to call from any goroutine.
//
// Until an assistive technology has actually asked for it, none of this costs anything beyond the AccessibilityInfo
// each panel carries and one atomic load per pass of the event loop that redrew something: no hierarchy is walked,
// nothing is allocated, and no goroutine or platform object exists. What counts as asking differs by platform:
//
//   - macOS: the first accessibility query AppKit delivers to a window's content view, which is what VoiceOver or
//     Accessibility Inspector sends on reaching the application. macOS never says that the last assistive technology
//     has gone, so support stays on afterwards.
//   - Windows: the first WM_GETOBJECT asking for the UI Automation root object. Events are raised only while UI
//     Automation reports a listening client, so a window that goes on being described after every client has gone costs
//     the snapshots and nothing more.
//   - Linux: the accessibility bus launcher's org.a11y.Status.IsEnabled property, read once at startup over the session
//     bus connection the color-scheme watcher already keeps, and then watched, so a screen reader started or stopped
//     while the application runs is followed both ways — which makes Linux the one platform where support is also torn
//     down again. NO_AT_BRIDGE=1 refuses it outright, exactly as it does for GTK.
//
// [AccessibilityEnvKey] overrides that decision: UNISON_ACCESSIBILITY=1 builds and publishes descriptions whether or
// not anything appears to be listening, which is useful for seeing what a screen reader would be told, while
// UNISON_ACCESSIBILITY=0 refuses activation entirely. The [NoAccessibility] startup option refuses it from the start,
// [SetAccessibilityEnabled] turns it off or back on while the application runs — off shuts down whatever is being
// served and frees everything built for it — and [AccessibilityEnabled] and [IsAccessibilityActive] report where things
// stand.
//
// On macOS, setting the UNISON_AX_TRACE environment variable to anything at all logs the accessibility traffic to
// standard error: every attribute an assistive technology reads, every request it makes and every notification it is
// sent, each stamped with the time. It costs nothing when unset and is meant for working out what a screen reader is
// doing with a window.
//
// Once something has asked, each window's panel hierarchy is captured after the window has been drawn — at most once
// every 50 milliseconds — into an immutable tree of the kind defined by
// [github.com/richardwilkes/unison/accessibility], and the difference between that tree and the one before it becomes
// the events an assistive technology is told about. This is what keeps the platform adapters clear of the threading
// model described above: a published tree is never modified, so an adapter answers queries from it on whatever thread
// its platform calls in on — any thread at all on Windows, the D-Bus dispatcher goroutine on Linux, the main thread on
// macOS — without ever touching a live panel. Requests coming the other way, to press a button or move the focus, are
// handed to the UI thread with [InvokeTask] and answered optimistically on Windows and Linux, since an adapter there
// must never wait on a UI thread that may be inside a modal loop or a drag. macOS is the exception: AppKit delivers
// accessibility callbacks on the main thread, which is the UI thread, and VoiceOver reads back the state its request
// produced the moment it has asked, so a request that only moves the focus, the selection or the view is carried out
// synchronously, from inside that callback. An [AccessibilityActor] or ActionCallback handling one of those actions may
// therefore find itself running within an AppKit accessibility callback, and must not do anything there that it would
// not do from inside a mouse event — running a modal dialog, most of all. Requests that activate something are queued
// on every platform for exactly that reason. Everything an application writes — ProvideAccessibility,
// PerformAccessibilityAction, Callback, ActionCallback — runs on the UI thread either way, like any other callback.
//
// What an assistive technology would be handed can be asserted on in tests, with no display and no screen reader
// involved: see [HeadlessScreen.AccessibilityTree], [HeadlessScreen.AccessibilityNodeFor],
// [HeadlessScreen.AccessibilityEvents], [HeadlessScreen.Announcements], [HeadlessScreen.PerformAccessibilityAction] and
// [HeadlessScreen.EnableAccessibility].
package unison
