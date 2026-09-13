// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// Package accessibility defines the snapshot schema unison hands to the platform assistive-technology adapters, along
// with the diffing that turns two successive snapshots into the events those adapters report.
//
// A [Tree] is one window's element hierarchy captured at a single moment, addressed by [NodeID]. A tree is immutable
// once it has been published: nothing in it is ever modified afterwards, so an adapter may answer queries from it on
// whatever thread the platform's assistive technology calls in on, without ever touching a live panel. [Diff] compares
// the previously published tree against a newly built one and returns the events an assistive technology needs to hear
// about. It is pure and deterministic — the same pair of trees always yields the same events in the same order — which
// makes the whole core testable without a display, an assistive technology, or even a window.
//
// Coordinates in a tree are window-local, with a top-left origin, in logical units. A node's bounds are its whole
// extent, not what its ancestors leave visible of it; Node.Offscreen says when none of it can be seen, and Tree.HitTest
// confines each node to its ancestors. Text offsets are rune indexes, not bytes and not UTF-16 code units; adapters
// convert as their platform requires.
//
// This package depends on nothing else in unison beyond the two enumerations it needs for roles and check states. It
// must never import the root unison package, and a test enforces that.
package accessibility
