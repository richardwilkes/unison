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
// drawing via [Canvas], and the objects behind images, fonts, paints, and the like — is owned by
// one dedicated OS thread, referred to throughout as the UI thread. None of these types are safe for concurrent use.
// Reading or mutating them from any other goroutine is a data race that will eventually crash or corrupt the
// application, even if it appears to work in testing.
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
//   - Functions handed to [InvokeTask] and [InvokeTaskAfter].
//
// # Reaching the UI thread from another goroutine
//
// When work happens off the UI thread — a network fetch, a file scan, any goroutine you start — it must not touch UI
// objects directly. Instead, marshal the UI work back onto the UI thread:
//
//   - [InvokeTask] queues a function to run on the UI thread at the next opportunity.
//   - [InvokeTaskAfter] does the same after a delay.
//   - [ReleaseOnUIThread] queues a function used specifically to free native resources on the UI thread.
//
// These three are the supported entry points that are safe to call from any goroutine; they are internally synchronized
// and wake the event loop as needed. A typical background operation computes its result off-thread, then calls
// InvokeTask with a closure that applies the result to the UI:
//
//	go func() {
//		data, err := fetch()
//		unison.InvokeTask(func() {
//			// Runs on the UI thread; safe to update widgets here.
//			if err != nil {
//				label.SetTitle(err.Error())
//			} else {
//				label.SetTitle(string(data))
//			}
//			label.MarkForRedraw()
//		})
//	}()
//
// # What is and isn't synchronized
//
// Most of Unison deliberately uses no locks, because the single-threaded contract makes them unnecessary on the UI
// thread. The handful of facilities meant to be reachable from other goroutines — the task queue behind InvokeTask, the
// release queue behind ReleaseOnUIThread, and the internal image cache — are individually synchronized. Do not infer
// from this that other types are safe to share; assume any type not documented otherwise is UI-thread-only. In
// particular, window and panel methods such as MarkForRedraw, SetFrameRect, AddChild, and a widget's content setters
// operate on unsynchronized shared state and must only be called on the UI thread.
package unison
