// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// TestDragSessionEvent covers the guard that keeps a nil event from reaching
// beginDraggingSessionWithItems:event:source:. AppKit raises NSInvalidArgumentException for a nil event, and as an
// uncaught Objective-C exception that aborts the process rather than surfacing as anything Go can handle, so the
// fallback to the application's current event and the refusal to proceed without any event at all are what stand
// between a drag started outside a mouse-dragged callback and a crash.
func TestDragSessionEvent(t *testing.T) {
	c := check.New(t)

	// The stashed event wins and the fallback is never consulted.
	currentCalls := 0
	current := func() objc.ID {
		currentCalls++
		return objc.ID(99)
	}
	event, ok := dragSessionEvent(objc.ID(42), current)
	c.True(ok)
	c.Equal(objc.ID(42), event)
	c.Equal(0, currentCalls, "the current event should not be consulted when one was stashed")

	// With nothing stashed -- a drag started from a plain mouseMoved: -- the current event stands in.
	event, ok = dragSessionEvent(0, current)
	c.True(ok)
	c.Equal(objc.ID(99), event)
	c.Equal(1, currentCalls)

	// With no event available at all, the caller must abandon the drag rather than hand AppKit a nil.
	event, ok = dragSessionEvent(0, func() objc.ID { return 0 })
	c.False(ok, "a drag must not be started without an event")
	c.Equal(objc.ID(0), event)
}

// TestViewMouseMovedStashesDragEvent is the regression test for a crash on macOS: moving the mouse while the window
// believed a button was held reached Window.mouseDrag, whose callback started a drag, but mouseMoved: -- unlike
// mouseDragged: -- left lastMouseDraggedEvent nil, so BeginDraggingSession handed AppKit a nil event and the process
// aborted with NSInvalidArgumentException. A move must expose its own event for the duration of the callback, and
// must leave the ivar as it found it so the mouseDragged: forwarding path is unaffected.
func TestViewMouseMovedStashesDragEvent(t *testing.T) {
	defer func() { WindowMouseMovedCallback = nil }()
	runOnMain(func() {
		w, v, cleanup := newTestWindowAndView(t)
		defer cleanup()
		ov := objc.ID(v)

		var stashedDuringMove objc.ID
		WindowMouseMovedCallback = func(_ Window, _ geom.Point, _ uint) {
			stashedDuringMove = ov.Send(Sel("lastMouseDraggedEvent"))
		}
		WithPool(func() {
			moveEvent := synthMouseEvent(nsEventTypeMouseMoved, NSPoint{X: 10, Y: 20}, 0, w)
			ov.Send(Sel("mouseMoved:"), moveEvent)
			if stashedDuringMove != moveEvent {
				t.Errorf("lastMouseDraggedEvent during mouseMoved = %#x, want the move event %#x", stashedDuringMove,
					moveEvent)
			}
			if got := ov.Send(Sel("lastMouseDraggedEvent")); got != 0 {
				t.Errorf("lastMouseDraggedEvent = %#x after mouseMoved returned, want 0", got)
			}
		})
	})
}
