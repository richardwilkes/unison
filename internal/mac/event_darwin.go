// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package mac

import (
	"sync"
	"time"

	"github.com/ebitengine/purego/objc"
)

// EventModifierFlags holds the modifier key mask of an event, using AppKit's NSEventModifierFlags bit assignments.
type EventModifierFlags uint

// Possible EventModifierFlags values.
const (
	EventModifierFlagCapsLock EventModifierFlags = 1 << (16 + iota)
	EventModifierFlagShift
	EventModifierFlagControl
	EventModifierFlagOption
	EventModifierFlagCommand
)

const (
	// nsEventTypeApplicationDefined is NSEventType's NSEventTypeApplicationDefined.
	nsEventTypeApplicationDefined uint64 = 15
	// nsEventMaskKeyUp is NSEventMask's NSEventMaskKeyUp (1 << NSEventTypeKeyUp).
	nsEventMaskKeyUp uint64 = 1 << 11
	// nsEventMaskAny is NSEventMask's NSEventMaskAny.
	nsEventMaskAny = ^uint64(0)
)

// defaultRunLoopMode returns Foundation's NSDefaultRunLoopMode constant.
var defaultRunLoopMode = sync.OnceValue(func() objc.ID {
	return NSStringConstant("Foundation", "NSDefaultRunLoopMode")
})

// DoubleClickInterval returns the maximum time between clicks of a double-click.
func DoubleClickInterval() time.Duration {
	return time.Duration(objc.Send[float64](objc.ID(Cls("NSEvent")), Sel("doubleClickInterval"))*1000) *
		time.Millisecond
}

// CurrentModifierFlags returns the current state of the modifier keys, independent of the event stream.
func CurrentModifierFlags() EventModifierFlags {
	return EventModifierFlags(objc.Send[uint64](objc.ID(Cls("NSEvent")), Sel("modifierFlags")))
}

// PostEmptyEvent posts an application-defined event to the front of the event queue, waking up a blocked WaitEvents
// call. It may be called from any thread.
func PostEmptyEvent() {
	WithPool(func() {
		event := objc.ID(Cls("NSEvent")).Send(
			Sel("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:"),
			nsEventTypeApplicationDefined, NSPoint{}, uint64(0), float64(0), int64(0), objc.ID(0), int16(0),
			int64(0), int64(0))
		sharedApp().Send(Sel("postEvent:atStart:"), event, true)
	})
}

// PollEvents dispatches all pending events, returning once the event queue is empty.
func PollEvents() {
	WithPool(func() {
		app := sharedApp()
		distantPast := objc.ID(Cls("NSDate")).Send(Sel("distantPast"))
		for {
			event := app.Send(Sel("nextEventMatchingMask:untilDate:inMode:dequeue:"), nsEventMaskAny, distantPast,
				defaultRunLoopMode(), true)
			if event == 0 {
				break
			}
			app.Send(Sel("sendEvent:"), event)
		}
	})
}

// WaitEvents blocks until at least one event is available, then dispatches all pending events before returning.
func WaitEvents() {
	WithPool(func() {
		app := sharedApp()
		event := app.Send(Sel("nextEventMatchingMask:untilDate:inMode:dequeue:"), nsEventMaskAny,
			objc.ID(Cls("NSDate")).Send(Sel("distantFuture")), defaultRunLoopMode(), true)
		if event != 0 { // distantFuture only returns when an event arrives, so this is always true in practice
			app.Send(Sel("sendEvent:"), event)
		}
		PollEvents()
	})
}

// WaitEventsTimeout blocks until at least one event is available or the timeout (in seconds) expires, then
// dispatches all pending events before returning.
func WaitEventsTimeout(timeout float64) {
	WithPool(func() {
		app := sharedApp()
		date := objc.ID(Cls("NSDate")).Send(Sel("dateWithTimeIntervalSinceNow:"), timeout)
		event := app.Send(Sel("nextEventMatchingMask:untilDate:inMode:dequeue:"), nsEventMaskAny, date,
			defaultRunLoopMode(), true)
		if event != 0 {
			app.Send(Sel("sendEvent:"), event)
		}
		PollEvents()
	})
}

// StopMainEventLoop stops the main event loop started by [NSApp run].
func StopMainEventLoop() {
	sharedApp().Send(Sel("stop:"), objc.ID(0))
}
