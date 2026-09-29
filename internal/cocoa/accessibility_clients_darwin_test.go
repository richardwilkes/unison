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
)

func TestAXClientsEnabled(_ *testing.T) {
	// The results depend on the user's settings; this exercises the NSWorkspace reads for crashes only.
	runOnMain(func() {
		_ = IsVoiceOverEnabled()
		_ = IsSwitchControlEnabled()
	})
}

// TestAXClientsStopped checks every pairing of the four client states: only going from at least one running to neither
// running is a stop.
func TestAXClientsStopped(t *testing.T) {
	states := []axClientState{{}, {voiceOver: true}, {switchControl: true}, {voiceOver: true, switchControl: true}}
	for _, prev := range states {
		for _, now := range states {
			want := prev != axClientState{} && now == axClientState{}
			if got := axClientsStopped(prev, now); got != want {
				t.Errorf("axClientsStopped(%+v, %+v) = %v, want %v", prev, now, got, want)
			}
		}
	}
}

// TestRegisterAXClientObserverNameCollision proves a class-name collision degrades to an error instead of panicking
// during startup. NSObject is used as a name guaranteed to already exist.
func TestRegisterAXClientObserverNameCollision(t *testing.T) {
	if err := registerAXClientObserver("NSObject"); err == nil {
		t.Error("registerAXClientObserver with an already-registered class name returned nil, want an error")
	}
}

// stubAXClients installs the client observer and substitutes the state it reads, starting with neither client running,
// and the callback it reports to, which counts into the returned int. Everything is restored afterwards.
func stubAXClients(t *testing.T) (state *axClientState, stops *int) {
	t.Helper()
	runOnMain(InstallAXClientObserver)
	if axClientObserver == 0 {
		t.Fatal("the client observer was not installed")
	}
	savedReader := axClientStateReader
	savedCallback := AccessibilityClientsStoppedCallback
	axClientsLock.Lock()
	savedLast := axClientsLast
	axClientsLast = axClientState{}
	axClientsLock.Unlock()
	t.Cleanup(func() {
		axClientStateReader = savedReader
		AccessibilityClientsStoppedCallback = savedCallback
		axClientsLock.Lock()
		axClientsLast = savedLast
		axClientsLock.Unlock()
	})
	state = &axClientState{}
	stops = new(int)
	axClientStateReader = func() axClientState { return *state }
	AccessibilityClientsStoppedCallback = func() { *stops++ }
	return state, stops
}

// TestAXClientObserverNotifications sends the KVO callback to the observer and checks that only the transition to
// neither client running reaches AccessibilityClientsStoppedCallback, and only once for it.
func TestAXClientObserverNotifications(t *testing.T) {
	state, stops := stubAXClients(t)
	for _, step := range []struct {
		name  string
		stops int
		state axClientState
	}{
		{name: "off to off", stops: 0, state: axClientState{}},
		{name: "VoiceOver on", stops: 0, state: axClientState{voiceOver: true}},
		{name: "on to on", stops: 0, state: axClientState{voiceOver: true}},
		{name: "VoiceOver off", stops: 1, state: axClientState{}},
		{name: "repeated off", stops: 1, state: axClientState{}},
		{name: "both on", stops: 1, state: axClientState{voiceOver: true, switchControl: true}},
		{name: "VoiceOver off, Switch Control on", stops: 1, state: axClientState{switchControl: true}},
		{name: "Switch Control off", stops: 2, state: axClientState{}},
	} {
		*state = step.state
		runOnMain(func() {
			WithPool(func() {
				axClientObserver.Send(Sel("observeValueForKeyPath:ofObject:change:context:"),
					NSStringFromGo("voiceOverEnabled"), objc.ID(Cls("NSWorkspace")).Send(Sel("sharedWorkspace")), 0, 0)
			})
		})
		if *stops != step.stops {
			t.Errorf("%s: stopped callback ran %d times in all, want %d", step.name, *stops, step.stops)
		}
	}
}

// TestAXClientObserverReceivesKVO proves the observer is registered with NSWorkspace for both key paths, by raising
// each KVO change notification by hand rather than by turning the real VoiceOver or Switch Control on and off.
func TestAXClientObserverReceivesKVO(t *testing.T) {
	_, stops := stubAXClients(t)
	for i, keyPath := range []string{"voiceOverEnabled", "switchControlEnabled"} {
		axClientsLock.Lock()
		axClientsLast = axClientState{voiceOver: true, switchControl: true}
		axClientsLock.Unlock()
		runOnMain(func() {
			WithPool(func() {
				workspace := objc.ID(Cls("NSWorkspace")).Send(Sel("sharedWorkspace"))
				key := NSStringFromGo(keyPath)
				workspace.Send(Sel("willChangeValueForKey:"), key)
				workspace.Send(Sel("didChangeValueForKey:"), key)
			})
		})
		if *stops != i+1 {
			t.Errorf("a change of %s reached the stopped callback %d times in all, want %d", keyPath, *stops, i+1)
		}
	}
}
