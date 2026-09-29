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
	"sync"
	"unsafe"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/errs"
)

// AccessibilityClientsStoppedCallback is invoked when VoiceOver or Switch Control was on and now neither is. KVO may
// deliver the change on any thread, so it may be invoked off the main thread.
var AccessibilityClientsStoppedCallback func()

// axClientState records which of the assistive technologies NSWorkspace reports on are running.
type axClientState struct {
	voiceOver     bool
	switchControl bool
}

var (
	axClientObserverOnce sync.Once
	// axClientObserver is the KVO observer, kept for the life of the process.
	axClientObserver objc.ID
	// axClientStateReader is a var so that tests can substitute it.
	axClientStateReader = readAXClientState
	// axClientsLock guards axClientsLast, since KVO may deliver on any thread.
	axClientsLock sync.Mutex
	axClientsLast axClientState
)

// IsVoiceOverEnabled reports whether VoiceOver is running.
func IsVoiceOverEnabled() bool {
	return workspaceFlag("isVoiceOverEnabled")
}

// IsSwitchControlEnabled reports whether Switch Control is running.
func IsSwitchControlEnabled() bool {
	return workspaceFlag("isSwitchControlEnabled")
}

func workspaceFlag(getter string) bool {
	var on bool
	WithPool(func() {
		on = objc.Send[bool](objc.ID(Cls("NSWorkspace")).Send(Sel("sharedWorkspace")), Sel(getter))
	})
	return on
}

func readAXClientState() axClientState {
	return axClientState{voiceOver: IsVoiceOverEnabled(), switchControl: IsSwitchControlEnabled()}
}

// axClientsStopped reports whether at least one client was running in prev and neither is in now.
func axClientsStopped(prev, now axClientState) bool {
	return (prev.voiceOver || prev.switchControl) && !now.voiceOver && !now.switchControl
}

// axClientsChanged re-reads the client state and invokes AccessibilityClientsStoppedCallback on the transition that
// axClientsStopped describes. A redundant notification finds the state it remembered and does nothing.
func axClientsChanged() {
	axClientsLock.Lock()
	now := axClientStateReader()
	stopped := axClientsStopped(axClientsLast, now)
	axClientsLast = now
	axClientsLock.Unlock()
	if stopped && AccessibilityClientsStoppedCallback != nil {
		AccessibilityClientsStoppedCallback()
	}
}

// InstallAXClientObserver starts watching VoiceOver and Switch Control for AccessibilityClientsStoppedCallback. Only
// the first call does anything.
func InstallAXClientObserver() {
	axClientObserverOnce.Do(func() {
		if err := registerAXClientObserver("macAXClientObserver"); err != nil {
			errs.Log(err)
		}
	})
}

// registerAXClientObserver registers the observer class under the given name and adds an instance of it as a KVO
// observer of NSWorkspace's voiceOverEnabled and switchControlEnabled. It is never removed. On class-registration
// failure it returns the error so the caller can log it and carry on without the observer.
func registerAXClientObserver(className string) error {
	cls, err := objc.RegisterClass(className, Cls("NSObject"), nil, nil, []objc.MethodDef{{
		Cmd: Sel("observeValueForKeyPath:ofObject:change:context:"),
		Fn: func(_ objc.ID, _ objc.SEL, _, _, _ objc.ID, _ unsafe.Pointer) {
			axClientsChanged()
		},
	}})
	if err != nil {
		return errs.NewWithCause("unable to register "+className+" class", err)
	}
	WithPool(func() {
		axClientObserver = objc.ID(cls).Send(Sel("new"))
		axClientsLock.Lock()
		axClientsLast = axClientStateReader()
		axClientsLock.Unlock()
		workspace := objc.ID(Cls("NSWorkspace")).Send(Sel("sharedWorkspace"))
		for _, keyPath := range []string{"voiceOverEnabled", "switchControlEnabled"} {
			// No options and no context.
			workspace.Send(Sel("addObserver:forKeyPath:options:context:"), axClientObserver, NSStringFromGo(keyPath),
				uint64(0), uintptr(0))
		}
	})
	return nil
}
