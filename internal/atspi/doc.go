// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// Package atspi implements the server side of AT-SPI2, the accessibility interface that assistive technologies on Linux
// desktops use (https://www.freedesktop.org/wiki/Accessibility/AT-SPI2). It publishes the immutable snapshots that
// [github.com/richardwilkes/unison/accessibility] produces as D-Bus objects on the accessibility bus, so a screen
// reader such as Orca can read a Unison window.
//
// Nothing here is specific to Linux, so the whole package is built and tested on every platform even though only the
// Linux wiring in the root package ever starts an [Adapter].
//
// # Shape of the server
//
// One object represents the application as a whole, at [RootPath]; its children are the windows. Every node of every
// published window is an object of its own at /org/a11y/atspi/accessible/<node id>, answered by a resolver handed to
// [github.com/richardwilkes/unison/internal/dbus.Conn.ExportSubtree]. Node ids come from one process-wide counter, so
// these flat paths are unambiguous even though the windows they belong to are unrelated. A cache object at [CachePath]
// hands an assistive technology the whole hierarchy in one call, which is how libatspi avoids a round trip per node.
//
// # Events
//
// Objects alone are not enough: an assistive technology waits to be told what has changed rather than asking again, and
// a screen reader that is not told goes on reading what the user has moved away from. Every event that comes with a
// published snapshot therefore becomes one or more signals, sent from the path of the object it concerns, with the
// interface, member and detail string that AT-SPI's own bridge for GTK's accessibility toolkit sends for the same
// change. The whole of a window's hierarchy is announced the first time it is published, and taken back when it goes
// away, so that the cache an assistive technology keeps stays in step with the objects.
//
// # Threading
//
// [Start], [Publish], [SetGeometry], [RemoveWindow], [Announce] and [Stop] are called from the user interface thread.
// Everything else runs on the connection's dispatcher goroutine, where the answers are read from the published
// snapshot, never from a live panel: a snapshot is immutable once published, so no locking beyond resolving a path to
// the window that holds it is needed. The two things that have to wait for the bus — rejoining the accessibility tree
// when the registry comes back, and telling it the application is going — are done on goroutines of their own, so that
// neither the user interface thread nor the dispatcher is ever held up by a peer that has stopped reading.
//
// An assistive technology may also ask for something to be done, such as pressing a button or moving the focus. Those
// requests are handed to [Config.Action], which the root package arranges to run on the user interface thread, and the
// reply is optimistic: nothing here ever waits for the user interface thread, because it may be inside a modal loop or
// a drag while libatspi's client-side timeout is only two seconds. Signals are queued rather than written for the same
// reason: a client that has stopped reading must not be able to hold a redraw up.
package atspi
