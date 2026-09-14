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
	"net"
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

// a11yTestTimeout is how long these tests wait for something that should happen at once.
const a11yTestTimeout = 2 * time.Second

// a11yTestSettle is how long these tests give something that should not happen at all a chance to happen anyway.
const a11yTestSettle = 250 * time.Millisecond

// a11yTestBusName is the unique name the fake session bus hands out in reply to Hello.
const a11yTestBusName = ":1.99"

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
	priorRejoin := linuxA11yRejoinAttempted
	priorAttempt, priorCurrent, priorJoining := linuxA11yAttempt, linuxA11yCurrentAttempt, linuxA11yJoining
	hadWatch := priorCancel != nil
	t.Cleanup(func() {
		if !hadWatch && linuxA11yCancelWatch != nil {
			linuxA11yCancelWatch()
		}
		noAccessibility.Store(priorNo)
		accessibilityEnv.Store(priorEnv)
		linuxA11y, linuxA11yCancelWatch = priorAdapter, priorCancel
		linuxA11yRejoinAttempted = priorRejoin
		linuxA11yAttempt, linuxA11yCurrentAttempt, linuxA11yJoining = priorAttempt, priorCurrent, priorJoining
	})
	noAccessibility.Store(false)
	accessibilityEnv.Store(0)
	linuxA11y = nil
	linuxA11yCancelWatch = nil
	linuxA11yRejoinAttempted = false
	linuxA11yCurrentAttempt = 0
	linuxA11yJoining = false
	t.Setenv("NO_AT_BRIDGE", "")
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
// the accessibility bus must not even reach for the session bus, which is checked by making any attempt to use it fail
// the test.
func TestLinuxA11yStatusInitIsRefusedByEnvironment(t *testing.T) {
	for name, refuse := range map[string]func(t *testing.T){
		"NO_AT_BRIDGE":         func(t *testing.T) { t.Helper(); t.Setenv("NO_AT_BRIDGE", "1") },
		"UNISON_ACCESSIBILITY": func(_ *testing.T) { accessibilityEnv.Store(-1) },
		"NoAccessibility":      func(_ *testing.T) { noAccessibility.Store(true) },
	} {
		t.Run(name, func(t *testing.T) {
			c := check.New(t)
			saveA11yState(t)
			restore := dbus.SetSessionForTest(nil, errors.New("the session bus must not be reached"))
			t.Cleanup(restore)
			refuse(t)
			linuxA11yStatusInit()
			c.Nil(linuxA11y)
			c.Nil(linuxA11yCancelWatch, "a refusal must not watch anything")
		})
	}
}

// TestLinuxA11yStatusInitAsksOnceAndWatches is the rest of the zero-cost-when-inactive test: on a desktop where nothing
// is listening, the whole cost of accessibility support is one property read and two match rules on the session
// connection the color-scheme watcher already holds. Nothing is dialed, nothing is exported and no snapshot is built.
func TestLinuxA11yStatusInitAsksOnceAndWatches(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
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
	w.nativeAccessibilityGeometryChanged()
	w.nativeAccessibilityShutdown()
	w.x11RefreshAccessibilityGeometry()
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
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestLinuxA11yGeometryFor covers the conversion itself for the cases a window cannot be put in, such as a display to
// the left of the primary one, whose negative origin must survive.
func TestLinuxA11yGeometryFor(t *testing.T) {
	c := check.New(t)
	geometry := linuxA11yGeometryFor(geom.NewPoint(-1920, 0), geom.NewPoint(1, 1))
	c.Equal(geom.NewPoint(-1920, 0), geometry.Origin)
	c.Equal(geom.NewPoint(1, 1), geometry.Scale)
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
