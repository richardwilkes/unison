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
	"bufio"
	"errors"
	"io"
	"net"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/atspi"
	"github.com/richardwilkes/unison/internal/dbus"
)

// These tests cover the part of the Linux accessibility wiring that can be exercised without an accessibility bus and a
// real screen reader: the decision about whether to serve one at all, what that decision costs an application nothing
// is watching, and the conversion that puts a node's window-local bounds where AT-SPI expects them. Everything past the
// point where the bus is dialed belongs to internal/atspi, which is tested in full on every platform, and the rest is
// checked by hand with Orca and accerciser.

// a11yTestTimeout is how long these tests wait for something that ought to happen at once. It only bounds how long a
// failure takes to report, so it is generous enough to ride out the multi-second stalls a loaded CI runner can suffer,
// as internal/x11's portalTestTimeout is.
const a11yTestTimeout = 10 * time.Second

// a11yTestSettle is how long these tests give something that should not happen at all a chance to happen anyway.
const a11yTestSettle = 250 * time.Millisecond

// a11yTestBusName is the unique name the fake buses hand out in reply to Hello.
const a11yTestBusName = ":1.99"

// a11yTestBusGUID is the server identifier the fake accessibility bus reports when it accepts the authentication
// handshake. A client has no use for it beyond its being there.
const a11yTestBusGUID = "0123456789abcdef0123456789abcdef"

// a11yTestAuthLineLimit bounds how many lines of the authentication handshake the fake accessibility bus will read, so
// that a client which never says BEGIN cannot keep its goroutine alive forever.
const a11yTestAuthLineLimit = 4

// a11yTestRegistryName is the bus name the fake accessibility bus answers Embed from, standing in for the registry.
const a11yTestRegistryName = ":1.7"

// a11yTestDesktopPath is the object the fake accessibility bus hands back as the desktop the application was embedded
// into, which is what the real registry's Embed replies with.
const a11yTestDesktopPath dbus.ObjectPath = "/org/a11y/atspi/accessible/desktop"

// a11yTestObjectRefSignature is the signature of the reference to an object on the accessibility bus, which is the one
// thing the fake accessibility bus has to encode.
const a11yTestObjectRefSignature dbus.Signature = "(so)"

// dbusPath is the path of a D-Bus daemon's own object.
const dbusPath dbus.ObjectPath = "/org/freedesktop/DBus"

// dbusPropertiesInterface is the interface every object's properties are read through.
const dbusPropertiesInterface = "org.freedesktop.DBus.Properties"

// saveA11yState records the process-wide accessibility state these tests change and puts it back afterwards. None of
// them may call t.Parallel, since all of that state is shared.
func saveA11yState(t *testing.T) {
	t.Helper()
	priorNo, priorEnv := noAccessibility.Load(), accessibilityEnv.Load()
	priorAdapter, priorCancel := linuxA11y, linuxA11yCancelWatch
	priorRejoin, priorInstalled, priorGrace := linuxA11yRejoinAttempted, linuxA11yInstalledAt, linuxA11yRejoinGrace
	priorAttempt, priorCurrent, priorJoining := linuxA11yAttempt, linuxA11yCurrentAttempt, linuxA11yJoining
	hadWatch := priorCancel != nil
	t.Cleanup(func() {
		if !hadWatch && linuxA11yCancelWatch != nil {
			linuxA11yCancelWatch()
		}
		noAccessibility.Store(priorNo)
		accessibilityEnv.Store(priorEnv)
		linuxA11y, linuxA11yCancelWatch = priorAdapter, priorCancel
		linuxA11yRejoinAttempted, linuxA11yInstalledAt, linuxA11yRejoinGrace = priorRejoin, priorInstalled, priorGrace
		linuxA11yAttempt, linuxA11yCurrentAttempt, linuxA11yJoining = priorAttempt, priorCurrent, priorJoining
	})
	noAccessibility.Store(false)
	accessibilityEnv.Store(0)
	linuxA11y = nil
	linuxA11yCancelWatch = nil
	linuxA11yRejoinAttempted = false
	linuxA11yInstalledAt = time.Time{}
	linuxA11yCurrentAttempt = 0
	linuxA11yJoining = false
	t.Setenv("NO_AT_BRIDGE", "")
}

// runQueuedA11yTasks runs the tasks the user interface thread has been handed, waiting until at least the expected
// number of them have arrived, and returns how many it ran. What provokes them here is a goroutine of the watch's or
// the join's own, so one has not necessarily been queued by the time the call that provoked it has returned.
//
// Running them inside the test that caused them is what keeps the process-wide state they write inside the window
// saveA11yState puts that state back in: no event loop runs during these tests, so a task left behind would be run by
// whatever came next, long after the cleanup had restored everything it touches.
func runQueuedA11yTasks(t *testing.T, expected int) int {
	t.Helper()
	deadline := time.Now().Add(a11yTestTimeout)
	ran := 0
	for {
		if length, head := taskQueueState(); length > head {
			processNextTask()
			ran++
			continue
		}
		if ran >= expected || time.Now().After(deadline) {
			return ran
		}
		time.Sleep(time.Millisecond)
	}
}

// TestLinuxA11yMode covers the decision that is made before the session bus is consulted at all. Only the ordinary
// case, where the desktop is asked, may reach the bus.
func TestLinuxA11yMode(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	for i, d := range []struct {
		noBridge string
		env      int32
		expected linuxA11yStartMode
		refused  bool
	}{
		{expected: linuxA11yAsk},
		{env: 1, expected: linuxA11yForced},
		{env: -1, expected: linuxA11yRefused},
		{refused: true, expected: linuxA11yRefused},
		{noBridge: "1", expected: linuxA11yRefused},
		{noBridge: "2", expected: linuxA11yRefused},
		// NO_AT_BRIDGE is a number, read the way the C toolkits read it, so anything that is not one, or is zero, is
		// not an instruction to stay away; neither is an unset one.
		{noBridge: "0", expected: linuxA11yAsk},
		{noBridge: "00", expected: linuxA11yAsk},
		{noBridge: "yes", expected: linuxA11yAsk},
		// The desktop's instruction wins over this application's request for support.
		{noBridge: "1", env: 1, expected: linuxA11yRefused},
	} {
		accessibilityEnv.Store(d.env)
		noAccessibility.Store(d.refused)
		t.Setenv("NO_AT_BRIDGE", d.noBridge)
		c.Equal(d.expected, linuxA11yMode(), "case %d", i)
	}
}

// TestLinuxA11yStatusInitWithoutSessionBus is half of the zero-cost-when-inactive test for this platform: a machine
// with no session bus has nothing to ask and nothing that could ever answer, so nothing is started, nothing is watched
// and nothing is retried.
func TestLinuxA11yStatusInitWithoutSessionBus(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	snapshots := axSnapshotCount
	wasActive := IsAccessibilityActive()
	t.Cleanup(dbus.SetSessionForTest(nil, errors.New("there is no session bus")))
	linuxA11yStatusInit()
	c.Nil(linuxA11y, "no adapter should have been created")
	c.Nil(linuxA11yCancelWatch, "nothing should be being watched")
	c.Equal(wasActive, IsAccessibilityActive(), "accessibility support should not have been turned on")
	c.Equal(snapshots, axSnapshotCount, "no snapshot should have been built")
}

// TestLinuxA11yStatusInitIsRefusedByEnvironment is the other half: an application that has been told to stay away from
// the accessibility bus must not even reach for the session bus.
//
// The session bus it is given is a working one that records everything asked of it, rather than a stub that fails every
// call. A stub would prove nothing: linuxA11yStatusInit gives a session bus it cannot have up just as quietly as it
// gives up on a refusal, so every assertion below would hold with the refusal deleted outright. What has to be shown is
// that the connection was not used at all — no match rule asked for, and the launcher's IsEnabled property never read.
func TestLinuxA11yStatusInitIsRefusedByEnvironment(t *testing.T) {
	for name, refuse := range map[string]func(t *testing.T){
		"NO_AT_BRIDGE":         func(t *testing.T) { t.Helper(); t.Setenv("NO_AT_BRIDGE", "1") },
		"UNISON_ACCESSIBILITY": func(_ *testing.T) { accessibilityEnv.Store(-1) },
		"NoAccessibility":      func(_ *testing.T) { noAccessibility.Store(true) },
	} {
		t.Run(name, func(t *testing.T) {
			c := check.New(t)
			saveA11yState(t)
			bus := newFakeSessionBus(t)
			t.Cleanup(dbus.SetSessionForTest(bus.conn, nil))
			refuse(t)
			linuxA11yStatusInit()
			c.Nil(linuxA11y)
			c.Nil(linuxA11yCancelWatch, "a refusal must not watch anything")
			// The read follows the match rules, and both are asked for from a goroutine of the watch's own, so the
			// settle is what gives either the chance to turn up before the count is believed.
			c.Equal(0, bus.settledReads(0), "a refusal must not read the launcher's IsEnabled property")
			c.Equal(0, len(bus.waitForRules(0)), "nor ask the session bus to deliver anything")
		})
	}
}

// TestLinuxA11yStatusInitAsksOnceAndWatches is the rest of the zero-cost-when-inactive test: on a desktop where nothing
// is listening, the whole cost of accessibility support is one property read and two match rules on the session
// connection the color-scheme watcher already holds. Nothing is dialed, nothing is exported and no snapshot is built.
func TestLinuxA11yStatusInitAsksOnceAndWatches(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	// So that the one task the watch queues below is the only one there is to run, whatever earlier work in this
	// process left behind. Those closures belong to tests that are over, as the ones a headless session drops on its
	// way up do.
	resetTaskQueue()
	snapshots := axSnapshotCount
	wasActive := IsAccessibilityActive()
	bus := newFakeSessionBus(t)
	t.Cleanup(dbus.SetSessionForTest(bus.conn, nil))

	linuxA11yStatusInit()

	c.NotNil(linuxA11yCancelWatch, "the launcher should be being watched")
	rules := bus.waitForRules(2)
	c.True(slices.ContainsFunc(rules, func(rule string) bool {
		return strings.Contains(rule, "member='PropertiesChanged'") && strings.Contains(rule, "path='/org/a11y/bus'")
	}), "the launcher's own property changes should be watched: %v", rules)
	c.True(slices.ContainsFunc(rules, func(rule string) bool {
		return strings.Contains(rule, "member='NameOwnerChanged'") && strings.Contains(rule, "arg0='org.a11y.Bus'")
	}), "the launcher appearing should be watched, since some desktops only start it on demand: %v", rules)

	// The property is read by the watch, once those rules are in place, so that a screen reader starting up in between
	// cannot be missed; it is still read only the once, and the answer — nothing is listening — starts nothing.
	c.Equal(1, bus.waitForReads(1), "the launcher's IsEnabled property should have been read")
	c.Equal(1, bus.settledReads(1), "and should have been read exactly once")

	// The watch hands the answer to the user interface thread, so it is run here rather than being left in the queue
	// for whatever runs next, which would write this test's state back after its cleanup had restored it.
	c.Equal(1, runQueuedA11yTasks(t, 1), "the answer should have been handed to the user interface thread")

	c.Nil(linuxA11y, "nothing is listening, so no adapter should have been created")
	c.Equal(wasActive, IsAccessibilityActive(), "accessibility support should not have been turned on")
	c.Equal(snapshots, axSnapshotCount, "no snapshot should have been built")

	// Nothing is left on the bus once the watch is dropped.
	linuxA11yCancelWatch()
	linuxA11yCancelWatch = nil
	c.Equal(0, len(bus.waitForRules(0)))
}

// TestLinuxA11yHooksAreInertWithoutAnAdapter covers the case the environment override creates: snapshot building can be
// turned on without an accessibility bus to publish to, and every path out of the root package must then do nothing.
func TestLinuxA11yHooksAreInertWithoutAnAdapter(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	w := &Window{}
	w.nativeAccessibilityPublish(&accessibility.Tree{}, nil)
	// Nothing on this platform reaches the wrapper this sits behind, since the X11 event loop hands the adapter the
	// geometry from the ConfigureNotify it is already handling, but it is what the shared code would call.
	c.NotPanics(w.nativeAccessibilityGeometryChanged)
	w.nativeAccessibilityShutdown()
	w.x11RefreshAccessibilityGeometry(geom.NewPoint(10, 20))
	nativeAccessibilityAnnounce("nobody is listening")
	linuxSetA11yEnabled(false) // Turning support off when it was never on does nothing
	linuxA11yTerminate()
	c.Nil(linuxA11y)
}

// TestLinuxA11yConnectionLostIgnoresAnAdapterThatIsGone covers the check that makes the report of an accessibility bus
// connection dying safe to act on: it arrives on that connection's own goroutines and is handed to the user interface
// thread, by which time the adapter it is about may have been stopped and replaced by one with a live connection of its
// own, which must not be torn down in its place.
func TestLinuxA11yConnectionLostIgnoresAnAdapterThatIsGone(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	current := &atspi.Adapter{}
	linuxA11y = current
	linuxA11yAttempt = 7
	linuxA11yCurrentAttempt = 7
	linuxA11yConnectionLost(0, errors.New("a connection that never produced an adapter"))
	linuxA11yConnectionLost(6, errors.New("a connection whose adapter has already been replaced"))
	c.True(linuxA11y == current, "an adapter that is no longer the current one must not take the current one down")
	c.False(linuxA11yRejoinAttempted, "nor use up the one rebuild a real loss is allowed")
}

// TestLinuxA11yForcedModeActivatesWithoutABus covers the promise the environment override makes: an application that
// asked for snapshots whatever the desktop reports keeps building them even when there is no accessibility bus to
// publish them on. Without this, support turned off and back on — or an accessibility bus connection lost and not
// recovered — would leave such an application with snapshots off for the rest of its life, while macOS and Windows
// both put it back where it started.
func TestLinuxA11yForcedModeActivatesWithoutABus(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	wasActive := IsAccessibilityActive()
	t.Cleanup(func() {
		if !wasActive {
			deactivateAccessibility()
		}
	})
	accessibilityEnv.Store(1)
	linuxA11yAttempt++
	linuxA11yJoining = true
	linuxFinishStartA11y(linuxA11yAttempt, nil, errors.New("there is no accessibility bus to join"))
	c.Nil(linuxA11y, "a join that failed must leave nothing behind")
	c.False(linuxA11yJoining, "and must leave the way clear for another attempt")
	c.True(IsAccessibilityActive(), "but snapshot building must be on, since the environment asked for it")

	// The ordinary case, where the desktop decides, has nothing to keep on for: the desktop said something was
	// listening, and there was nothing to reach.
	deactivateAccessibility()
	accessibilityEnv.Store(0)
	linuxA11yAttempt++
	linuxFinishStartA11y(linuxA11yAttempt, nil, errors.New("there is no accessibility bus to join"))
	c.False(IsAccessibilityActive())
}

// TestLinuxA11yRejoinKeepsForcedModeBuilding covers the same promise for a loss the rebuild guard stops short of
// dialing again after: a connection that was taken away within moments of being made, which no thing the desktop has
// said stands between — forced mode creates no watch, so nothing reports one. Snapshot building, which the adapter took
// with it, still has to come back: it is the half of the promise that has nothing to do with there being a bus.
func TestLinuxA11yRejoinKeepsForcedModeBuilding(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	wasActive := IsAccessibilityActive()
	t.Cleanup(func() {
		if !wasActive {
			deactivateAccessibility()
		}
	})
	// Nothing on this path may reach the session bus: the rebuild is not attempted, and forced mode would not ask the
	// desktop in any case.
	t.Cleanup(dbus.SetSessionForTest(nil, errors.New("the session bus must not be reached")))
	// Every loss below is of a connection that was dropped as soon as it was accepted, which is the one case the guard
	// refuses a second dial for. The grace period is lengthened rather than left at what it really is, so that a
	// machine which stalls part way through cannot turn these into losses of a connection that lived.
	linuxA11yRejoinGrace = time.Hour
	linuxA11yInstalledAt = time.Now()
	accessibilityEnv.Store(1)
	linuxA11yRejoinAttempted = true // As the first loss left it
	linuxA11yRejoin()
	c.True(IsAccessibilityActive(), "the environment asked for snapshots whatever the desktop reports")
	c.Nil(linuxA11y, "but the one rebuild a loss is allowed has already been used up")
	c.False(linuxA11yJoining, "so nothing may be being joined")

	// The ordinary case, where the desktop decides, has nothing to keep on for.
	deactivateAccessibility()
	accessibilityEnv.Store(0)
	linuxA11yRejoinAttempted = true
	linuxA11yRejoin()
	c.False(IsAccessibilityActive())

	// Nor may an application the desktop has told to stay away from the accessibility bus have anything turned back
	// on. NO_AT_BRIDGE refuses the mode without refusing activateAccessibility, so the mode is all that stands between
	// a forced-on application and snapshots the desktop has said it does not want.
	accessibilityEnv.Store(1)
	t.Setenv("NO_AT_BRIDGE", "1")
	linuxA11yRejoinAttempted = true
	linuxA11yRejoin()
	c.False(IsAccessibilityActive())
}

// TestLinuxA11yRejoinDialsAgainOnTheFirstLoss covers the other half: the first loss in forced mode both turns snapshot
// building back on and dials the accessibility bus once more, since the launcher will have a new bus up by the time the
// dial arrives. There is nothing to reach here, so what the attempt produces is an error, which leaves snapshot
// building on and the adapter empty.
func TestLinuxA11yRejoinDialsAgainOnTheFirstLoss(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	resetTaskQueue()
	wasActive := IsAccessibilityActive()
	t.Cleanup(func() {
		if !wasActive {
			deactivateAccessibility()
		}
	})
	// There must be nothing to join: no session bus to ask for the address, no address in the environment, and no X11
	// connection to read the root window's property from during a test.
	t.Cleanup(dbus.SetSessionForTest(nil, errors.New("there is no session bus")))
	t.Setenv("AT_SPI_BUS_ADDRESS", "")
	accessibilityEnv.Store(1)
	linuxA11yRejoin()
	c.True(linuxA11yRejoinAttempted, "the one rebuild a loss is allowed must have been used up")
	c.True(linuxA11yJoining, "and must be in flight")

	// The join reports back through the user interface thread, which nothing else is running here.
	c.Equal(1, runQueuedA11yTasks(t, 1), "the join should have reported back")
	c.False(linuxA11yJoining, "the attempt must have been reported as finished")
	c.Nil(linuxA11y, "a join that failed must leave nothing behind")
	c.True(IsAccessibilityActive(), "but snapshot building must be on, since the environment asked for it")
}

// TestLinuxA11yRejoinRecoversFromEveryLongLivedLoss covers the accessibility bus that is restarted more than once while
// the application runs, which is what the whole of this machinery exists for: at-spi-bus-launcher keeps org.a11y.Bus
// across the restart, so the desktop reports nothing and the rebuild guard is never cleared by anything it says. Only a
// connection that dies within moments of being made is the bus the guard is aimed at; one that had been serving an
// assistive technology for far longer is a bus that really went away, and the launcher will have a new one up by the
// time the dial arrives, so every such loss is dialed again. Without this, the second restart would leave the
// application off the accessibility bus, and with snapshot building off, for the rest of its life.
func TestLinuxA11yRejoinRecoversFromEveryLongLivedLoss(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	resetTaskQueue()
	wasActive := IsAccessibilityActive()
	t.Cleanup(func() {
		if !wasActive {
			deactivateAccessibility()
		}
	})
	// As in the test above, there must be nothing to join, so that each rebuild is one dial that fails rather than an
	// adapter to take apart: no session bus to ask for the address, no address in the environment, and no X11
	// connection to read the root window's property from during a test. Forced mode is what has the rebuild happen
	// without the session bus being asked anything.
	t.Cleanup(dbus.SetSessionForTest(nil, errors.New("there is no session bus")))
	t.Setenv("AT_SPI_BUS_ADDRESS", "")
	accessibilityEnv.Store(1)

	for i := range 2 {
		// What the loss of a connection that had been up for far longer than the grace period leaves behind: the guard
		// the rebuild before it set, and the adapter that has just been thrown away recorded as installed long ago.
		linuxA11yRejoinAttempted = i > 0
		linuxA11yInstalledAt = time.Now().Add(-time.Hour)

		linuxA11yRejoin()
		c.True(linuxA11yJoining, "loss %d should have dialed the accessibility bus again", i)
		c.True(linuxA11yRejoinAttempted, "and used up the rebuild a loss that comes at once is refused, loss %d", i)
		c.Equal(1, runQueuedA11yTasks(t, 1), "the join should have reported back, loss %d", i)
		c.False(linuxA11yJoining, "the attempt must have been reported as finished, loss %d", i)
		c.Nil(linuxA11y, "a join that failed must leave nothing behind, loss %d", i)
		c.True(IsAccessibilityActive(), "snapshot building must be on, since the environment asked for it, loss %d", i)
	}

	// A connection that died within moments of being made is the case the guard is aimed at, and still costs one dial
	// rather than an endless succession of them. The grace period is lengthened rather than left at what it really is,
	// so that a machine which stalls between these two statements cannot turn this into one of the losses above.
	linuxA11yRejoinGrace = time.Hour
	linuxA11yInstalledAt = time.Now()
	linuxA11yRejoin()
	c.False(linuxA11yJoining, "a bus that accepts a connection and drops it must be dialed once rather than forever")
	c.True(IsAccessibilityActive(), "though snapshot building comes back whatever the guard decides")
}

// TestLinuxA11yFinishStartIgnoresASupersededAttempt covers the race joining the accessibility bus off the user
// interface thread creates: support may be turned off, or asked for again, while a join is in flight, and the answer to
// a question nobody is waiting for any more must not be installed behind the back of what was decided since.
func TestLinuxA11yFinishStartIgnoresASupersededAttempt(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	wasActive := IsAccessibilityActive()
	t.Cleanup(func() {
		if !wasActive {
			deactivateAccessibility()
		}
	})
	accessibilityEnv.Store(1)
	linuxA11yAttempt = 4
	linuxA11yJoining = true
	linuxFinishStartA11y(3, nil, errors.New("an answer nobody is waiting for"))
	c.True(linuxA11yJoining, "the attempt that is still in flight must not have been reported as finished")
	c.False(IsAccessibilityActive(), "nor must a superseded answer decide anything")

	// Stopping disowns whatever is in flight, which is what makes the answer that arrives afterwards a superseded one.
	before := linuxA11yAttempt
	linuxStopA11y()
	c.True(linuxA11yAttempt != before, "stopping must disown the attempt that is in flight")
	c.False(linuxA11yJoining)
}

// TestLinuxA11yFinishStartRefusesADeadAdapter covers the connection that dies while the join to the accessibility bus
// is still in flight. atspi.Start hands back an adapter all the same: the loss is reported through Config.Lost from the
// moment Start decides to return, which is before the task that installs the adapter has even been queued, so
// linuxA11yConnectionLost rejects that report as naming no current attempt — and Lost is called at most once. An
// adapter that is not asked whether its connection is still alive would therefore sit in linuxA11y for the rest of the
// process, dropping every snapshot published on it, refusing to be replaced, and never reporting the loss again.
func TestLinuxA11yFinishStartRefusesADeadAdapter(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	wasActive := IsAccessibilityActive()
	bus := newFakeA11yBus(t)
	adapter, err := atspi.Start(atspi.Config{BusAddress: bus.address, ToolkitVersion: "test"})
	c.NoError(err)
	if adapter == nil {
		t.Fatal("the fake accessibility bus should have produced an adapter")
	}

	// The bus goes away while the answer to the join is still on its way to the user interface thread, exactly as one
	// restarted behind a launcher that keeps org.a11y.Bus does. The adapter learns of it on a goroutine of the
	// connection's own, so it is waited for rather than assumed.
	bus.disconnect()
	deadline := time.Now().Add(a11yTestTimeout)
	for adapter.Err() == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	c.HasError(adapter.Err(), "the adapter should have noticed that its connection had ended")

	linuxA11yAttempt++
	linuxA11yJoining = true
	linuxFinishStartA11y(linuxA11yAttempt, adapter, nil)
	c.Nil(linuxA11y, "an adapter whose connection has already ended must not be installed")
	c.False(linuxA11yJoining, "and must leave the way clear for another attempt")
	c.Equal(uint64(0), linuxA11yCurrentAttempt, "nor may anything claim to be the adapter that is being served")
	c.Equal(wasActive, IsAccessibilityActive(), "the desktop decides here, and there was nothing to reach")
}

// TestLinuxA11yGeometry verifies the geometry a window actually hands the adapter, which is what
// Window.accessibilityGeometry produces rather than anything this test works out for itself. Unlike Windows, where the
// origin of a window rect is already in the raw global pixel space, an X11 window's position is divided by the backing
// scale on the way out of nativeContentRect, so multiplying it back is what recovers the device pixels the X server,
// and therefore AT-SPI, works in: a window whose content area is at (100,50) on a 2x display sits at (200,100) as far
// as an assistive technology is concerned.
func TestLinuxA11yGeometry(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 600, Scale: 2},
		StartupFinishedCallback(func() {
			wnd = axNewTestWindow(t, "geometry", geom.NewRect(100, 50, 200, 150), NewPanel())
		}))
	c.NotNil(wnd)
	var geometry atspi.Geometry
	var contentOrigin geom.Point
	screen.Do(func() {
		geometry = wnd.linuxA11yGeometry()
		contentOrigin = wnd.ContentRect().Point
	})
	c.Equal(geom.NewPoint(100, 50), contentOrigin, "the content area is where it was put, in logical units")
	c.Equal(geom.NewPoint(200, 100), geometry.Origin, "and is reported to AT-SPI in the X server's pixels")
	c.Equal(geom.NewPoint(2, 2), geometry.Scale, "along with the scale node bounds have to be multiplied by")

	// A window on a display to the left of or above the primary one sits at a negative origin, which the conversion
	// has to carry through rather than clamping: an assistive technology places its highlight from these numbers.
	screen.Do(func() {
		wnd.SetContentRect(geom.NewRect(-960, -20, 200, 150))
		geometry = wnd.linuxA11yGeometry()
	})
	c.Equal(geom.NewPoint(-1920, -40), geometry.Origin, "a negative origin must survive the conversion")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestLinuxA11yWindowManagerMinimizeMarksForPublish covers the minimize nothing in the application asked for: a
// taskbar, switching workspaces and "show desktop" all iconify a window that need not hold the keyboard focus, and only
// one that does is rescued by the FocusOut that follows it. Marking the window for publish is what has the event loop
// find a window it cannot draw and withdraw it from the accessibility tree, so without it a window nobody can see goes
// on being listed as showing and visible.
func TestLinuxA11yWindowManagerMinimizeMarksForPublish(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	swapRedrawSet(t)
	wasActive := IsAccessibilityActive()
	t.Cleanup(func() {
		if !wasActive {
			deactivateAccessibility()
		}
	})
	c.True(activateAccessibility(), "snapshot building should have been turned on")
	w := newRedrawTestWindow()
	var minimizedCalls []bool
	w.MinimizedCallback = func(minimized bool) { minimizedCalls = append(minimizedCalls, minimized) }
	marked := func() bool {
		_, pending := redrawSet[w]
		delete(redrawSet, w)
		return pending
	}
	c.True(marked(), "installing the window's content marks it for its first draw")

	w.x11SetMinimized(true)
	c.True(marked(), "a window the window manager iconified must be described again, so the event loop withdraws it")

	w.x11SetMinimized(false)
	c.True(marked(), "and so must one it restored, which is what puts the window back into the tree")
	c.Equal([]bool{true, false}, minimizedCalls)

	// A report of the state already in effect changes nothing, so there is nothing to describe.
	w.x11SetMinimized(false)
	c.False(marked())
	c.Equal([]bool{true, false}, minimizedCalls)

	// An application nothing is listening to pays one atomic load for this and nothing else.
	deactivateAccessibility()
	w.x11SetMinimized(true)
	c.False(marked())
	c.Equal([]bool{true, false, true}, minimizedCalls, "the callback is not an accessibility matter")
}

// TestLinuxToolkitVersionFrom verifies the version reported to an assistive technology. Unison is normally one of the
// modules the application depends on; the main module's version is right only when the binary being run is Unison's
// own.
func TestLinuxToolkitVersionFrom(t *testing.T) {
	c := check.New(t)
	c.Equal(unknownToolkitVersion, linuxToolkitVersionFrom(nil))
	c.Equal(unknownToolkitVersion, linuxToolkitVersionFrom(&debug.BuildInfo{}))
	c.Equal("v1.2.3", linuxToolkitVersionFrom(&debug.BuildInfo{
		Main: debug.Module{Path: "example.com/app", Version: "v9.9.9"},
		Deps: []*debug.Module{
			{Path: "example.com/other", Version: "v0.1.0"},
			{Path: unisonModulePath, Version: "v1.2.3"},
		},
	}))
	c.Equal("v4.5.6", linuxToolkitVersionFrom(&debug.BuildInfo{
		Deps: []*debug.Module{
			{Path: unisonModulePath, Version: "v1.2.3", Replace: &debug.Module{
				Path:    unisonModulePath,
				Version: "v4.5.6",
			}},
		},
	}))
	c.Equal("v7.8.9", linuxToolkitVersionFrom(&debug.BuildInfo{
		Main: debug.Module{Path: unisonModulePath, Version: "v7.8.9"},
	}))
	// The version a real build reports has to be something, whatever the binary was built from.
	c.NotEqual("", linuxToolkitVersion())
}

// fakeSessionBus is the least of a D-Bus daemon that the startup path needs: it hands out a name, records match rules,
// and answers the accessibility bus launcher's IsEnabled property with "nothing is listening". A call it does not know
// about fails, which is how a test learns that something reached for more of the bus than it should have.
//
// It is a cut-down cousin of the fake peer in internal/atspi, which cannot be shared because it lives in that package's
// tests.
type fakeSessionBus struct {
	t      *testing.T
	conn   *dbus.Conn
	in     *bufio.Reader
	side   net.Conn
	rules  []string
	lock   sync.Mutex
	reads  int
	serial uint32
}

// newFakeSessionBus connects a session bus that answers on the far end of a pipe. Both ends are closed when the test
// finishes.
func newFakeSessionBus(t *testing.T) *fakeSessionBus {
	t.Helper()
	c := check.New(t)
	clientSide, busSide := net.Pipe()
	conn, err := dbus.Connect(clientSide)
	c.NoError(err)
	b := &fakeSessionBus{t: t, conn: conn, side: busSide, in: bufio.NewReader(busSide)}
	go b.run()
	t.Cleanup(func() {
		conn.Close()
		xio.CloseIgnoringErrors(busSide)
	})
	c.NoError(conn.Hello())
	return b
}

// run answers messages until the connection goes away.
func (b *fakeSessionBus) run() {
	for {
		msg, err := dbus.Decode(b.in)
		if err != nil {
			return
		}
		if msg.Type != dbus.TypeMethodCall {
			continue
		}
		switch {
		case msg.Path == dbusPath && msg.Member == "Hello":
			b.reply(msg, "s", a11yTestBusName)
		case msg.Path == dbusPath && (msg.Member == "AddMatch" || msg.Member == "RemoveMatch"):
			b.match(msg)
		case msg.Path == atspi.BusPath && msg.Interface == dbusPropertiesInterface && msg.Member == "Get":
			b.isEnabled(msg)
		default:
			b.write(dbus.NewError(msg, dbus.ServiceUnknown, "the fake session bus has no "+msg.Member))
		}
	}
}

// match records or forgets a match rule.
func (b *fakeSessionBus) match(msg *dbus.Message) {
	args, err := msg.Args()
	if err != nil || len(args) != 1 {
		b.write(dbus.NewError(msg, dbus.InvalidArgs, "a match rule is required"))
		return
	}
	rule, ok := args[0].(string)
	if !ok {
		b.write(dbus.NewError(msg, dbus.InvalidArgs, "a match rule is required"))
		return
	}
	b.lock.Lock()
	if msg.Member == "AddMatch" {
		b.rules = append(b.rules, rule)
	} else if i := slices.Index(b.rules, rule); i >= 0 {
		b.rules = slices.Delete(b.rules, i, i+1)
	}
	b.lock.Unlock()
	b.reply(msg, "")
}

// isEnabled answers the accessibility bus launcher's IsEnabled property with false, counting the reads so that a test
// can insist there was only one.
func (b *fakeSessionBus) isEnabled(msg *dbus.Message) {
	args, err := msg.Args()
	if err != nil || len(args) != 2 || args[0] != atspi.StatusInterface || args[1] != "IsEnabled" {
		b.write(dbus.NewError(msg, dbus.InvalidArgs, "only IsEnabled may be read"))
		return
	}
	b.lock.Lock()
	b.reads++
	b.lock.Unlock()
	b.reply(msg, "v", dbus.Variant{Sig: "b", Value: false})
}

// isEnabledReads returns how many times the launcher's IsEnabled property has been read.
func (b *fakeSessionBus) isEnabledReads() int {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.reads
}

// waitForReads returns the number of times the launcher's IsEnabled property has been read once it has been read the
// expected number of times, or whatever it has reached when waiting has gone on too long. The property is read from a
// goroutine of the watch's own, once the bus has taken the match rules, so it has not necessarily been read by the time
// the call that started the watch has returned.
func (b *fakeSessionBus) waitForReads(expected int) int {
	b.t.Helper()
	deadline := time.Now().Add(a11yTestTimeout)
	for {
		reads := b.isEnabledReads()
		if reads >= expected || time.Now().After(deadline) {
			return reads
		}
		time.Sleep(time.Millisecond)
	}
}

// settledReads gives a further read of the launcher's IsEnabled property time to arrive and returns the count it
// settled on. Waiting is the only way to test "read exactly once": waitForReads comes back the instant the read it was
// waiting for lands, so a second one sent a moment behind it would not have been counted yet. Nothing is being waited
// for here, so a slow machine only makes the wait more generous rather than making the test fail.
func (b *fakeSessionBus) settledReads(expected int) int {
	b.t.Helper()
	deadline := time.Now().Add(a11yTestSettle)
	for {
		reads := b.isEnabledReads()
		if reads > expected || time.Now().After(deadline) {
			return reads
		}
		time.Sleep(time.Millisecond)
	}
}

// waitForRules returns the match rules in place once there are the expected number of them, or whatever there is when
// waiting has gone on too long. The rules are asked for from a goroutine of the watch's own, so they are not
// necessarily in place by the time the call that started the watch has returned.
func (b *fakeSessionBus) waitForRules(expected int) []string {
	b.t.Helper()
	deadline := time.Now().Add(a11yTestTimeout)
	for {
		b.lock.Lock()
		rules := slices.Clone(b.rules)
		b.lock.Unlock()
		if len(rules) == expected || time.Now().After(deadline) {
			return rules
		}
		time.Sleep(time.Millisecond)
	}
}

// reply answers a method call.
func (b *fakeSessionBus) reply(call *dbus.Message, sig dbus.Signature, args ...any) {
	reply := dbus.NewReply(call)
	if len(args) != 0 {
		if err := reply.SetBodyWithSignature(sig, args...); err != nil {
			b.t.Errorf("the fake session bus could not build a reply to %s: %v", call, err)
			return
		}
	}
	b.write(reply)
}

// write sends a message to the connection under test.
func (b *fakeSessionBus) write(msg *dbus.Message) {
	b.lock.Lock()
	b.serial++
	msg.Serial = b.serial
	b.lock.Unlock()
	data, err := msg.Encode()
	if err != nil {
		b.t.Errorf("the fake session bus could not encode %s: %v", msg, err)
		return
	}
	_, _ = b.side.Write(data) //nolint:errcheck // The connection under test has gone away, which is not this end's problem
}

// fakeA11yBus is the least of an accessibility bus that atspi.Start needs in order to hand an adapter back: it listens
// on a Unix socket, performs the server half of the EXTERNAL authentication handshake, hands out a unique name, takes
// the match rule the adapter asks for and answers the registry's Embed with a desktop reference. Nothing is published
// over it, since everything an adapter says once it is running belongs to internal/atspi's own tests; what it is for is
// producing a real adapter whose connection can then be taken away.
//
// Unlike fakeSessionBus, which the root package hands to the code under test ready made, this has to be dialed: the
// connection an adapter is built on is one atspi.Start makes for itself, and only an address can say where.
type fakeA11yBus struct {
	t        *testing.T
	listener net.Listener
	conn     net.Conn
	address  string
	lock     sync.Mutex
	serial   uint32
}

// newFakeA11yBus starts a fake accessibility bus on a Unix socket in the test's own directory. Everything it holds is
// closed when the test finishes.
func newFakeA11yBus(t *testing.T) *fakeA11yBus {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bus")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("unable to listen on %s: %v", path, err)
	}
	b := &fakeA11yBus{t: t, listener: listener, address: "unix:path=" + path}
	go b.serve()
	t.Cleanup(func() {
		xio.CloseIgnoringErrors(listener)
		b.disconnect()
	})
	return b
}

// serve answers one connection until it goes away, which is all the single adapter a test builds ever needs.
func (b *fakeA11yBus) serve() {
	accepted, acceptErr := b.listener.Accept()
	if acceptErr != nil {
		return
	}
	b.lock.Lock()
	b.conn = accepted
	b.lock.Unlock()
	in := bufio.NewReader(accepted)
	if !b.authenticate(accepted, in) {
		b.disconnect()
		return
	}
	for {
		msg, decodeErr := dbus.Decode(in)
		if decodeErr != nil {
			return
		}
		if msg.Type != dbus.TypeMethodCall {
			continue
		}
		switch {
		case msg.Path == dbusPath && msg.Member == "Hello":
			b.reply(msg, "s", a11yTestBusName)
		case msg.Path == dbusPath && (msg.Member == "AddMatch" || msg.Member == "RemoveMatch"):
			b.reply(msg, "")
		case msg.Path == atspi.RootPath && msg.Interface == atspi.InterfaceSocket && msg.Member == "Embed":
			b.reply(msg, a11yTestObjectRefSignature,
				dbus.ObjectRef{Name: a11yTestRegistryName, Path: a11yTestDesktopPath})
		case msg.Path == atspi.RootPath && msg.Interface == atspi.InterfaceSocket && msg.Member == "Unembed":
			// The registry has nothing to say to an application that is leaving, and none was asked for.
		default:
			b.write(dbus.NewError(msg, dbus.ServiceUnknown, "the fake accessibility bus has no "+msg.Member))
		}
	}
}

// authenticate performs the server half of the handshake dbus.Dial makes: the connection opens with a zero byte and an
// AUTH line that the credentials of the socket itself answer for, and the BEGIN that follows the acceptance is where
// the message stream starts.
func (b *fakeA11yBus) authenticate(conn net.Conn, in *bufio.Reader) bool {
	leading, err := in.ReadByte()
	if err != nil || leading != 0 {
		return false
	}
	for range a11yTestAuthLineLimit {
		line, lineErr := in.ReadString('\n')
		if lineErr != nil {
			return false
		}
		switch {
		case strings.HasPrefix(line, "AUTH "):
			if _, err = io.WriteString(conn, "OK "+a11yTestBusGUID+"\r\n"); err != nil {
				return false
			}
		case strings.HasPrefix(line, "BEGIN"):
			return true
		default:
			return false
		}
	}
	return false
}

// disconnect takes the connection away without a word, which is what an accessibility bus that has been restarted does
// to the application it was serving.
func (b *fakeA11yBus) disconnect() {
	b.lock.Lock()
	conn := b.conn
	b.conn = nil
	b.lock.Unlock()
	if conn != nil {
		xio.CloseIgnoringErrors(conn)
	}
}

// reply answers a method call.
func (b *fakeA11yBus) reply(call *dbus.Message, sig dbus.Signature, args ...any) {
	reply := dbus.NewReply(call)
	if len(args) != 0 {
		if err := reply.SetBodyWithSignature(sig, args...); err != nil {
			b.t.Errorf("the fake accessibility bus could not build a reply to %s: %v", call, err)
			return
		}
	}
	b.write(reply)
}

// write sends a message to the connection under test, and does nothing once that connection has been taken away.
func (b *fakeA11yBus) write(msg *dbus.Message) {
	b.lock.Lock()
	b.serial++
	msg.Serial = b.serial
	conn := b.conn
	b.lock.Unlock()
	if conn == nil {
		return
	}
	data, err := msg.Encode()
	if err != nil {
		b.t.Errorf("the fake accessibility bus could not encode %s: %v", msg, err)
		return
	}
	_, _ = conn.Write(data) //nolint:errcheck // The connection under test has gone away, which is not this end's problem
}
