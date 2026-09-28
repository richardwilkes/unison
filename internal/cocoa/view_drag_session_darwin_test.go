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
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
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

// TestBeginDraggingSessionReportsNoSession pins the result the window layer relies on to wind a drag up itself: with
// nothing to drag, or with no event to start a session from, BeginDraggingSession reports false and leaves the view not
// believing it is in a drag of its own, so AppKit will never report a session ending. The no-event half goes through
// the seam that stands in for the application's current event, since whether the real application has one depends on
// what ran before, and with one AppKit would be handed a real session to run.
func TestBeginDraggingSessionReportsNoSession(t *testing.T) {
	runOnMain(func() {
		w, v, cleanup := newTestWindowAndView(t)
		defer cleanup()
		ov := objc.ID(v)
		WithPool(func() {
			frame := geom.NewRect(0, 0, 10, 10)
			payload := []drag.Data{{Type: uti.UTF8PlainText, Data: []byte("payload")}}
			// With no data there is nothing to drag, however good the stashed event is.
			event := synthMouseEvent(nsEventTypeLeftMouseDragged, NSPoint{X: 5, Y: 5}, 0, w)
			ov.Send(Sel("setLastMouseDraggedEvent:"), event)
			if v.BeginDraggingSession(nil, frame, drag.Copy) {
				t.Error("BeginDraggingSession with no data reported a session")
			}
			if objc.Send[bool](ov, Sel("isInDragWeStarted")) {
				t.Error("the view believes it is in a drag after refusing to start one with no data")
			}
			// With nothing stashed and no current event, there is nothing to start a session from.
			ov.Send(Sel("setLastMouseDraggedEvent:"), objc.ID(0))
			consulted := 0
			noEvent := func() objc.ID {
				consulted++
				return 0
			}
			if v.beginDraggingSession(nil, frame, drag.Copy, noEvent, payload) {
				t.Error("beginDraggingSession with no event reported a session")
			}
			if consulted != 1 {
				t.Errorf("the current event was consulted %d times, want once", consulted)
			}
			if objc.Send[bool](ov, Sel("isInDragWeStarted")) {
				t.Error("the view believes it is in a drag after refusing to start one with no event")
			}
		})
	})
}
