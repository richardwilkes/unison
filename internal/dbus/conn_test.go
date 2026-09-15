// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package dbus

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/unison/internal/testenv"
)

const (
	// testTimeout is how long a test waits for something that should happen at once. It only bounds how long a failure
	// takes to report, so it is generous: a loaded CI runner can stall the whole process for seconds at a time, and the
	// connection's own timeouts that some tests race against are milliseconds.
	testTimeout = 10 * time.Second
	// testService, testPath and testInterface name the peer that the connection under test calls.
	testService   = "org.example.Service"
	testPath      = ObjectPath("/org/example/object")
	testInterface = "org.example.Greeter"
)

// awaitOne waits for one value to arrive on ch, reporting whether one did before testTimeout passed. The channel is
// checked once more after the deadline: when a loaded machine stalls the whole process, the deadline and the value it
// was waiting for arrive together, and a select with both ready would pick between them at random.
func awaitOne[T any](ch <-chan T) (value T, ok bool) {
	timer := time.NewTimer(testTimeout)
	defer timer.Stop()
	select {
	case value = <-ch:
		return value, true
	case <-timer.C:
		select {
		case value = <-ch:
			return value, true
		default:
			return value, false
		}
	}
}

// fakeBus is the other end of a connection: it answers the handful of calls that the bus itself implements, records the
// method calls and signals it receives, and lets a test call into the objects the connection exports.
type fakeBus struct {
	t       *testing.T
	c       check.Checker
	client  *Conn
	side    net.Conn
	in      *bufio.Reader
	calls   chan *Message
	signals chan *Message
	gone    chan struct{}
	replies map[uint32]chan *Message
	rules   []string
	serial  uint32
	mu      sync.Mutex
}

// newFakeBus creates a connection under test whose peer is a fake bus. Both ends are closed when the test finishes.
func newFakeBus(t *testing.T) *fakeBus {
	t.Helper()
	c := check.New(t)
	clientSide, busSide := net.Pipe()
	client, err := Connect(clientSide)
	c.NoError(err)
	b := &fakeBus{
		t:       t,
		c:       c,
		client:  client,
		side:    busSide,
		in:      bufio.NewReader(busSide),
		calls:   make(chan *Message, 64),
		signals: make(chan *Message, 64),
		gone:    make(chan struct{}),
		replies: make(map[uint32]chan *Message),
	}
	go b.run()
	t.Cleanup(func() {
		client.Close()
		xio.CloseIgnoringErrors(busSide)
		// The fake bus's own goroutine reports what it cannot make sense of through the testing.T, which panics the
		// whole test binary with "Log in goroutine after Test... has completed" if it gets there after the test
		// function has returned, so the cleanup waits for it to be done rather than leaving it to race the teardown.
		<-b.gone
	})
	return b
}

// run reads messages from the connection under test until it goes away.
func (b *fakeBus) run() {
	defer close(b.gone)
	for {
		msg, err := Decode(b.in)
		if err != nil {
			return
		}
		switch msg.Type {
		case TypeMethodCall:
			if msg.Destination == busName && string(msg.Path) == busPath {
				b.answerBus(msg)
				continue
			}
			b.deliver(b.calls, msg)
		case TypeMethodReturn, TypeError:
			b.mu.Lock()
			ch := b.replies[msg.ReplySerial]
			delete(b.replies, msg.ReplySerial)
			b.mu.Unlock()
			if ch != nil {
				ch <- msg
			}
		case TypeSignal:
			b.deliver(b.signals, msg)
		default:
		}
	}
}

// deliver hands a message to a test without ever blocking the fake bus's reader.
func (b *fakeBus) deliver(ch chan *Message, msg *Message) {
	select {
	case ch <- msg:
	default:
		b.t.Errorf("the fake bus had nowhere to put %s", msg)
	}
}

// answerBus answers the calls that the bus itself implements.
func (b *fakeBus) answerBus(msg *Message) {
	switch msg.Member {
	case "Hello":
		b.replyTo(msg, "s", testSender)
	case "AddMatch", "RemoveMatch":
		args, err := msg.Args()
		if err != nil || len(args) != 1 {
			b.errorTo(msg, InvalidArgs, "a match rule is required")
			return
		}
		rule, _ := args[0].(string) //nolint:errcheck // A non-string cannot get here, since the signature was checked
		b.mu.Lock()
		if msg.Member == "AddMatch" {
			b.rules = append(b.rules, rule)
		} else if i := slices.Index(b.rules, rule); i >= 0 {
			b.rules = slices.Delete(b.rules, i, i+1)
		}
		b.mu.Unlock()
		b.replyTo(msg, "")
	default:
		b.errorTo(msg, UnknownMethod, msg.Member)
	}
}

// matchRules returns the match rules that the connection under test has asked for.
func (b *fakeBus) matchRules() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.rules)
}

// write sends a message to the connection under test.
func (b *fakeBus) write(msg *Message) {
	b.mu.Lock()
	b.serial++
	msg.Serial = b.serial
	b.mu.Unlock()
	data, err := msg.Encode()
	if err != nil {
		b.t.Errorf("the fake bus could not encode %s: %v", msg, err)
		return
	}
	if _, err = b.side.Write(data); err != nil {
		return // The connection under test has gone away, which some tests do on purpose
	}
}

// replyTo answers a method call that the connection under test made.
func (b *fakeBus) replyTo(call *Message, sig Signature, args ...any) {
	reply := NewReply(call)
	if len(args) != 0 {
		b.c.NoError(reply.SetBodyWithSignature(sig, args...))
	}
	b.write(reply)
}

// errorTo answers a method call that the connection under test made with an error.
func (b *fakeBus) errorTo(call *Message, name, message string) {
	b.write(NewError(call, name, message))
}

// replyFrom answers a method call that the connection under test made, stamping the reply with the sender that a real
// bus would have filled in for whichever connection produced it.
func (b *fakeBus) replyFrom(call *Message, sender string, sig Signature, args ...any) {
	reply := NewReply(call)
	reply.Sender = sender
	if len(args) != 0 {
		b.c.NoError(reply.SetBodyWithSignature(sig, args...))
	}
	b.write(reply)
}

// errorFrom answers a method call that the connection under test made with an error, stamped with a sender the same way
// [fakeBus.replyFrom] stamps a return.
func (b *fakeBus) errorFrom(call *Message, sender, name, message string) {
	reply := NewError(call, name, message)
	reply.Sender = sender
	b.write(reply)
}

// emit sends a signal to the connection under test.
func (b *fakeBus) emit(path ObjectPath, iface, member string, sig Signature, args ...any) {
	msg := NewSignal(path, iface, member)
	msg.Sender = testDestination
	if len(args) != 0 {
		b.c.NoError(msg.SetBodyWithSignature(sig, args...))
	}
	b.write(msg)
}

// call makes a method call on an object that the connection under test exports and returns its reply.
func (b *fakeBus) call(path ObjectPath, iface, member string, sig Signature, args ...any) *Message {
	b.t.Helper()
	msg := NewMethodCall("", path, iface, member)
	msg.Sender = testDestination
	if len(args) != 0 {
		b.c.NoError(msg.SetBodyWithSignature(sig, args...))
	}
	ch := make(chan *Message, 1)
	b.mu.Lock()
	b.serial++
	msg.Serial = b.serial
	b.replies[msg.Serial] = ch
	b.mu.Unlock()
	data, err := msg.Encode()
	b.c.NoError(err)
	_, err = b.side.Write(data)
	b.c.NoError(err)
	reply, ok := awaitOne(ch)
	if !ok {
		b.t.Fatalf("timed out waiting for a reply to %s", msg)
	}
	return reply
}

// writeRaw sends bytes to the connection under test exactly as they are, for the messages that [Message.Encode] would
// not produce.
func (b *fakeBus) writeRaw(data []byte) {
	b.t.Helper()
	if _, err := b.side.Write(data); err != nil {
		b.t.Fatalf("the fake bus could not write %d bytes: %v", len(data), err)
	}
}

// callNoReply makes a method call on an object that the connection under test exports, telling it that no reply is
// expected, and returns a function that reports whether it was answered anyway.
func (b *fakeBus) callNoReply(path ObjectPath, iface, member string, sig Signature, args ...any) func() bool {
	b.t.Helper()
	msg := NewMethodCall("", path, iface, member)
	msg.Sender = testDestination
	msg.Flags = FlagNoReplyExpected
	if len(args) != 0 {
		b.c.NoError(msg.SetBodyWithSignature(sig, args...))
	}
	ch := make(chan *Message, 1)
	b.mu.Lock()
	b.serial++
	msg.Serial = b.serial
	b.replies[msg.Serial] = ch
	b.mu.Unlock()
	data, err := msg.Encode()
	b.c.NoError(err)
	b.writeRaw(data)
	return func() bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}
}

// nextCall returns the next method call the connection under test made to something other than the bus.
func (b *fakeBus) nextCall() *Message {
	b.t.Helper()
	msg, ok := awaitOne(b.calls)
	if !ok {
		b.t.Fatal("timed out waiting for a method call")
	}
	return msg
}

// nextSignal returns the next signal the connection under test emitted.
func (b *fakeBus) nextSignal() *Message {
	b.t.Helper()
	msg, ok := awaitOne(b.signals)
	if !ok {
		b.t.Fatal("timed out waiting for a signal")
	}
	return msg
}

// callResult is the outcome of a call made from another goroutine.
type callResult struct {
	reply *Message
	err   error
}

// callAsync makes a method call from another goroutine, so that the test can answer it, and returns a function that
// waits for the outcome.
func callAsync(t *testing.T, client *Conn, msg *Message) func() (*Message, error) {
	t.Helper()
	ch := make(chan callResult, 1)
	go func() {
		reply, err := client.Call(msg)
		ch <- callResult{reply: reply, err: err}
	}()
	return func() (*Message, error) {
		t.Helper()
		result, ok := awaitOne(ch)
		if !ok {
			t.Fatal("timed out waiting for a call to finish")
		}
		return result.reply, result.err
	}
}

// greet builds the method call that most of the tests make.
func greet(c check.Checker, text string) *Message {
	msg := NewMethodCall(testService, testPath, testInterface, "Greet")
	c.NoError(msg.SetBody(text))
	return msg
}

func TestConnectRequiresATransport(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	conn, err := Connect(nil)
	c.Nil(conn)
	c.HasError(err)
}

func TestConnHello(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	c.Equal("", b.client.Name())
	c.NoError(b.client.Hello())
	c.Equal(testSender, b.client.Name())
}

func TestConnCall(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	wait := callAsync(t, b.client, greet(c, "hello"))
	call := b.nextCall()
	c.Equal(testService, call.Destination)
	c.Equal(testPath, call.Path)
	c.Equal(testInterface, call.Interface)
	c.Equal("Greet", call.Member)
	args, err := call.Args()
	c.NoError(err)
	c.Equal([]any{"hello"}, args)
	b.replyTo(call, "s", "hello yourself")
	reply, err := wait()
	c.NoError(err)
	c.Equal(TypeMethodReturn, reply.Type)
	args, err = reply.Args()
	c.NoError(err)
	c.Equal([]any{"hello yourself"}, args)
}

func TestConnCallReturnsAnError(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	wait := callAsync(t, b.client, greet(c, "hello"))
	b.errorTo(b.nextCall(), NotSupported, "not today")
	reply, err := wait()
	c.Nil(reply)
	c.HasError(err)
	var dbusErr *Error
	c.True(errors.As(err, &dbusErr))
	c.Equal(NotSupported, dbusErr.Name)
	c.Equal("not today", dbusErr.Message)
}

func TestConnCallTimesOut(t *testing.T) {
	testenv.SkipTimingSensitive(t)
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	b.client.timeout = 50 * time.Millisecond
	wait := callAsync(t, b.client, greet(c, "hello"))
	b.nextCall() // Received, but deliberately left unanswered
	reply, err := wait()
	c.Nil(reply)
	c.HasError(err)
	// The same timeout bounds the write, and a machine slow enough to take 50 ms handing the message to the pipe fails
	// the call with "timed out ... writing a message" instead, which is not the wait this test is about: the fuller
	// text is what says the reply was waited for.
	c.Contains(err.Error(), "waiting for a reply to")
}

func TestConnCallWithNoReplyExpected(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	reply, err := b.client.CallWithFlags(greet(c, "hello"), FlagNoReplyExpected)
	c.NoError(err)
	c.Nil(reply)
	call := b.nextCall()
	c.Equal(FlagNoReplyExpected, call.Flags&FlagNoReplyExpected)
}

func TestConnSendAndEnqueue(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	msg := NewMethodCall(testService, testPath, testInterface, "Greet")
	msg.Flags = FlagNoReplyExpected
	c.NoError(msg.SetBody("sent"))
	c.NoError(b.client.Send(msg))
	c.NotEqual(uint32(0), msg.Serial)
	first := b.nextCall()
	args, err := first.Args()
	c.NoError(err)
	c.Equal([]any{"sent"}, args)
	queued := NewMethodCall(testService, testPath, testInterface, "Greet")
	queued.Flags = FlagNoReplyExpected
	c.NoError(queued.SetBody("queued"))
	b.client.Enqueue(queued)
	second := b.nextCall()
	args, err = second.Args()
	c.NoError(err)
	c.Equal([]any{"queued"}, args)
	c.Equal(0, b.client.Dropped())
}

func TestConnEmit(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	b.client.Emit(testPath, eventInterface, "StateChanged", "focused", int32(1))
	signal := b.nextSignal()
	c.Equal(TypeSignal, signal.Type)
	c.Equal(testPath, signal.Path)
	c.Equal(eventInterface, signal.Interface)
	c.Equal("StateChanged", signal.Member)
	args, err := signal.Args()
	c.NoError(err)
	c.Equal([]any{"focused", int32(1)}, args)

	// An empty dictionary has no derivable type, so the signature has to be supplied.
	b.client.EmitWithSignature(testPath, eventInterface, "TextChanged", "s"+propertiesSig, "insert", Dict{})
	signal = b.nextSignal()
	c.Equal(Signature("s"+propertiesSig), signal.Signature)
	args, err = signal.Args()
	c.NoError(err)
	c.Equal([]any{"insert", Dict{}}, args)
}

func TestConnMatchRules(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	rule := "type='signal',interface='" + eventInterface + "'"
	c.NoError(b.client.AddMatch(rule))
	c.Equal([]string{rule}, b.matchRules())
	c.NoError(b.client.RemoveMatch(rule))
	c.Equal([]string{}, b.matchRules())
}

func TestConnSubscribe(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	filtered := make(chan *Message, 8)
	everything := make(chan *Message, 8)
	cancel := b.client.Subscribe(SignalFilter{
		Sender:    testDestination,
		Path:      testPath,
		Interface: eventInterface,
		Member:    "StateChanged",
	}, func(msg *Message) { filtered <- msg })
	b.client.Subscribe(SignalFilter{}, func(msg *Message) { everything <- msg })

	b.emit(testPath, eventInterface, "StateChanged", "s", "focused")
	b.emit(testPath, eventInterface, "SomethingElse", "s", "ignored")
	b.emit("/org/example/other", eventInterface, "StateChanged", "s", "elsewhere")

	// Every wait here is bounded, so a regression that stops delivering a signal fails with a message that says which
	// wait it was rather than hanging until the package-wide panic timeout.
	msg, ok := awaitOne(filtered)
	if !ok {
		t.Fatal("timed out waiting for the filtered signal")
	}
	args, err := msg.Args()
	c.NoError(err)
	c.Equal([]any{"focused"}, args)
	for i := range 3 {
		if _, ok = awaitOne(everything); !ok {
			t.Fatalf("timed out waiting for signal %d of 3", i+1)
		}
	}
	c.Equal(0, len(filtered))

	// Once canceled, the subscription hears nothing more. The catch-all was registered second, so its receipt of the
	// next signal proves the canceled one would already have been called.
	cancel()
	cancel() // Canceling twice is harmless
	b.emit(testPath, eventInterface, "StateChanged", "s", "after")
	if _, ok = awaitOne(everything); !ok {
		t.Fatal("timed out waiting for the signal that follows the cancellation")
	}
	c.Equal(0, len(filtered))
}

func TestConnDisconnectNotification(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	lost := make(chan error, 1)
	b.client.OnDisconnect(func(err error) { lost <- err })
	xio.CloseIgnoringErrors(b.side)
	err, ok := awaitOne(lost)
	if !ok {
		t.Fatal("timed out waiting for the disconnect notification")
	}
	c.HasError(err)
	c.Contains(err.Error(), "peer closed the connection")

	// A callback registered after the fact is called at once.
	late := make(chan error, 1)
	b.client.OnDisconnect(func(lateErr error) { late <- lateErr })
	c.HasError(<-late)

	// Everything else reports the failure rather than hanging.
	_, err = b.client.Call(greet(c, "hello"))
	c.HasError(err)
	c.HasError(b.client.Send(greet(c, "hello")))
}

func TestConnCloseWithAPendingCall(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	lost := make(chan error, 1)
	b.client.OnDisconnect(func(err error) { lost <- err })
	wait := callAsync(t, b.client, greet(c, "hello"))
	b.nextCall() // In flight, and never answered
	b.client.Close()
	b.client.Close() // Closing twice is harmless
	reply, err := wait()
	c.Nil(reply)
	c.True(errors.Is(err, ErrClosed))
	if lostErr, ok := awaitOne(lost); ok {
		c.True(errors.Is(lostErr, ErrClosed))
	} else {
		t.Fatal("timed out waiting for the disconnect notification")
	}
	<-b.gone // The fake bus sees the connection go away too
}

func TestConnEnqueueDropsWhenTheQueueFills(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	clientSide, busSide := net.Pipe()
	t.Cleanup(func() { xio.CloseIgnoringErrors(busSide) })
	client, err := Connect(clientSide)
	c.NoError(err)
	defer client.Close()
	// Nothing ever reads the other end of the pipe, so the writer goroutine blocks and the queue fills up behind it.
	// How many messages the writer takes before it blocks is not fixed, so keep going until the queue overflows.
	for range 4 * maxQueued {
		client.Emit(testPath, eventInterface, "StateChanged", "focused", int32(1))
		if client.Dropped() != 0 {
			break
		}
	}
	c.True(client.Dropped() > 0, "expected some messages to have been dropped")
}

func TestConnIgnoresUnexpectedReplies(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	// A reply to a serial nobody is waiting for is dropped rather than upsetting anything.
	b.write(NewReply(&Message{Serial: 9999, Sender: testDestination}))
	c.NoError(b.client.Hello()) // Still working
	c.Equal(testSender, b.client.Name())
}

// serveOneConnection accepts a single connection on the listener, completes the SASL handshake as a server would, and
// answers the Hello that follows.
func serveOneConnection(t *testing.T, listener net.Listener) {
	t.Helper()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer xio.CloseIgnoringErrors(conn)
		in := bufio.NewReader(conn)
		first, err := in.ReadByte()
		if err != nil || first != 0 {
			t.Errorf("the handshake did not start with a zero byte")
			return
		}
		line, err := readAuthLine(in)
		if err != nil {
			t.Errorf("unable to read the AUTH line: %v", err)
			return
		}
		if !strings.HasPrefix(line, "AUTH EXTERNAL ") {
			t.Errorf("unexpected AUTH line %q", line)
			return
		}
		if _, err = io.WriteString(conn, "OK 1234567890abcdef1234567890abcdef\r\n"); err != nil {
			return
		}
		if line, err = readAuthLine(in); err != nil || line != "BEGIN" {
			t.Errorf("expected BEGIN, but got %q (%v)", line, err)
			return
		}
		hello, err := Decode(in)
		if err != nil {
			t.Errorf("unable to read the Hello: %v", err)
			return
		}
		if hello.Member != "Hello" {
			t.Errorf("expected Hello, but got %s", hello)
			return
		}
		reply := NewReply(hello)
		reply.Serial = 1
		if err = reply.SetBodyWithSignature("s", testSender); err != nil {
			t.Errorf("unable to build the Hello reply: %v", err)
			return
		}
		data, err := reply.Encode()
		if err != nil {
			t.Errorf("unable to encode the Hello reply: %v", err)
			return
		}
		if _, err = conn.Write(data); err != nil {
			return
		}
		<-t.Context().Done() // Stay up until the test is over, so that the client does not see a disconnect
	}()
}

func TestDial(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	socket := filepath.Join(t.TempDir(), "bus")
	listener, err := net.Listen("unix", socket)
	c.NoError(err)
	t.Cleanup(func() { xio.CloseIgnoringErrors(listener) })
	serveOneConnection(t, listener)
	// The first alternative cannot be used and the second does not exist, so the third is the one that answers.
	conn, err := Dial("tcp:host=localhost,port=1;unix:path=" + socket + ".missing;unix:path=" + socket)
	if err != nil {
		// Everything below dereferences the connection, so a failure here has to end this test rather than be reported
		// and carried on from: check.Checker.NoError calls t.Error, and conn.Name() on a nil connection panics the
		// whole test binary instead of failing the one test.
		t.Fatal(err)
	}
	t.Cleanup(conn.Close)
	c.Equal(testSender, conn.Name())
}

func TestDialFailures(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	conn, err := Dial("tcp:host=localhost,port=1")
	c.Nil(conn)
	c.HasError(err)
	c.Contains(err.Error(), `the "tcp" transport`)
	conn, err = Dial("unix:path=" + filepath.Join(t.TempDir(), "not-there"))
	c.Nil(conn)
	c.HasError(err)
	c.Contains(err.Error(), "unable to connect")
}

func TestSubtreeExportsAreSafeWhileDispatching(t *testing.T) {
	testenv.SkipTimingSensitive(t)
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	obj := newTestObject()
	const prefix = ObjectPath("/org/example/churn")
	// The dispatcher reads the subtree list after releasing the lock, so a concurrent export or unexport that shifted
	// the surviving entries down within the same backing array used to hand it a nil *subtree and panic in covers.
	var wg sync.WaitGroup
	done := make(chan struct{})
	for i := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path := prefix + ObjectPath("/"+strconv.Itoa(i))
			for {
				select {
				case <-done:
					return
				default:
				}
				if err := b.client.ExportSubtree(path, func(_ ObjectPath) Object { return obj }); err != nil {
					t.Errorf("unable to export %s: %v", path, err)
					return
				}
				b.client.Unexport(path)
			}
		}()
	}
	c.NoError(b.client.ExportSubtree(prefix, func(_ ObjectPath) Object { return obj }))
	for range 50 {
		c.Equal([]any{"hello churn"}, replyValues(t, b.call(prefix+"/0/leaf", testInterface, greetMember, "s", "churn")))
	}
	close(done)
	wg.Wait()
}

// panickingObject panics whenever it is asked what it implements, which is code of the exporter's own running on the
// dispatcher goroutine.
type panickingObject struct{}

func (panickingObject) Interfaces() []*Interface { panic("Interfaces blew up") }

func TestPanicsInExportedCodeCostOnlyTheOneCall(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	const prefix = ObjectPath("/org/example/panics")
	obj := newTestObject()
	// Any peer on the bus chooses the path, and therefore whether the resolver and the object it hands back are asked
	// anything at all, so a panic in either has to cost the one call rather than the whole process.
	c.NoError(b.client.ExportSubtree(prefix, func(path ObjectPath) Object {
		switch {
		case strings.HasSuffix(string(path), "/resolver"):
			panic("the resolver blew up")
		case strings.HasSuffix(string(path), "/interfaces"):
			return panickingObject{}
		default:
			return obj
		}
	}))
	reply := b.call(prefix+"/resolver", testInterface, greetMember, "s", "x")
	c.Equal(TypeError, reply.Type)
	c.Equal(UnknownObject, reply.ErrorName) // A resolver that panicked resolved nothing
	for _, one := range []struct {
		iface  string
		member string
		sig    Signature
		args   []any
	}{
		{iface: testInterface, member: greetMember, sig: "s", args: []any{"x"}},
		{iface: introspectableInterface, member: "Introspect"},
		{iface: propertiesInterface, member: getMember, sig: "ss", args: []any{testInterface, nameProperty}},
		{iface: propertiesInterface, member: getAllMember, sig: "s", args: []any{""}},
	} {
		reply = b.call(prefix+"/interfaces", one.iface, one.member, one.sig, one.args...)
		c.Equal(TypeError, reply.Type, one.member)
		c.Equal(Failed, reply.ErrorName, one.member)
	}
	// The connection carries on afterwards, which is the whole point.
	c.Equal([]any{"hello intact"}, replyValues(t, b.call(prefix+"/fine", testInterface, greetMember, "s", "intact")))
}

// malformedObject hands out an interface list with a nil hole in it, which is what an object built on demand looks like
// when the code that builds it has a bug of its own.
type malformedObject struct {
	ifaces []*Interface
}

func (o malformedObject) Interfaces() []*Interface { return o.ifaces }

func TestMalformedInterfaceListsCostOnlyTheOneCall(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	const prefix = ObjectPath("/org/example/holes")
	obj := newTestObject()
	// Only Export checks what an object declares, and an object a resolver hands out is never seen until it is used,
	// so a nil hole anywhere in its list reaches the walk that dispatch makes over it. Any peer on the bus chooses the
	// path, and therefore which object is asked, so that walk has to cost the one call rather than the dispatcher
	// goroutine and with it every message the connection would have delivered afterwards.
	c.NoError(b.client.ExportSubtree(prefix, func(path ObjectPath) Object {
		switch {
		case strings.HasSuffix(string(path), "/interface"):
			return malformedObject{ifaces: []*Interface{nil}}
		case strings.HasSuffix(string(path), "/method"):
			return malformedObject{ifaces: []*Interface{{Name: testInterface, Methods: []*Method{nil}}}}
		default:
			return obj
		}
	}))
	for _, name := range []string{"interface", "method"} {
		for _, one := range []struct {
			iface  string
			member string
		}{
			{iface: testInterface, member: greetMember},
			{iface: introspectableInterface, member: "Introspect"},
		} {
			reply := b.call(prefix+ObjectPath("/"+name), one.iface, one.member, "")
			c.Equal(TypeError, reply.Type, "%s %s", name, one.member)
			c.Equal(Failed, reply.ErrorName, "%s %s", name, one.member)
		}
	}
	// The connection carries on afterwards, which is the whole point.
	c.Equal([]any{"hello intact"}, replyValues(t, b.call(prefix+"/fine", testInterface, greetMember, "s", "intact")))
}

func TestAPanicOutsideTheGuardsCostsOnlyTheOneMessage(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	c.NoError(b.client.Export(testPath, newTestObject()))
	// Nothing in the connection's own dispatch is meant to panic, so this reaches in and breaks it: a nil subscription
	// and a nil subtree are what a slice modified in place under the dispatcher's feet used to hand it, and neither is
	// touched inside one of the guards that the code an exporter supplies is called through.
	b.client.mu.Lock()
	b.client.subs = append(b.client.subs, nil)
	b.client.subtrees = append(b.client.subtrees, nil)
	b.client.mu.Unlock()
	// A signal has nobody to answer, so the fault is logged and the messages behind it are delivered anyway...
	b.emit(testPath, eventInterface, "StateChanged", "s", "focused")
	// ...while a method call is answered with an error rather than left waiting for a reply that will never come.
	reply := b.call("/org/example/nowhere", testInterface, greetMember, "s", "x")
	c.Equal(TypeError, reply.Type)
	c.Equal(Failed, reply.ErrorName)
	// The dispatcher is still running afterwards, which is the whole point.
	c.Equal([]any{"hello intact"}, replyValues(t, b.call(testPath, testInterface, greetMember, "s", "intact")))
}

// countingObject records how many times it has been asked what it implements.
type countingObject struct {
	obj   *testObject
	count atomic.Int32
}

func (o *countingObject) Interfaces() []*Interface {
	o.count.Add(1)
	return o.obj.Interfaces()
}

func TestTheInterfaceListIsAskedForOncePerCall(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	obj := &countingObject{obj: newTestObject()}
	c.NoError(b.client.Export(testPath, obj))
	// Dispatch asks the object what it implements in order to find the method, and each of the builtin handlers that
	// walks the list asks for it again, so an object that builds its interfaces on demand built them twice for every
	// property an assistive technology read. The list is kept for the length of the call instead.
	for i, one := range []struct {
		iface  string
		member string
		sig    Signature
		args   []any
	}{
		{iface: testInterface, member: greetMember, sig: "s", args: []any{"x"}},
		{iface: propertiesInterface, member: getMember, sig: "ss", args: []any{testInterface, nameProperty}},
		{iface: propertiesInterface, member: getAllMember, sig: "s", args: []any{""}},
		{
			iface:  propertiesInterface,
			member: setMember,
			sig:    setPropertySig,
			args:   []any{testInterface, nameProperty, Variant{Sig: "s", Value: changedName}},
		},
		{iface: introspectableInterface, member: "Introspect"},
	} {
		obj.count.Store(0)
		reply := b.call(testPath, one.iface, one.member, one.sig, one.args...)
		c.Equal(TypeMethodReturn, reply.Type, "case %d: %s", i, one.member)
		c.Equal(int32(1), obj.count.Load(), "case %d: %s", i, one.member)
	}
}

func TestConnCarriesOnAfterAMalformedBody(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	received := make(chan *Message, 2)
	b.client.Subscribe(SignalFilter{Member: malformedMember}, func(msg *Message) { received <- msg })
	// A body that does not match the signature its message declares is a fault in that one message, whichever byte
	// order the peer chose. Converting a big-endian body inside Decode made it a fault in the stream instead, since
	// the reader ends the connection for any error Decode reports.
	for i, bigEndian := range []bool{false, true} {
		b.writeRaw(malformedBodySignal(t, uint32(i+1), bigEndian))
		msg, ok := awaitOne(received)
		if !ok {
			t.Fatalf("timed out waiting for the malformed signal (big-endian: %v)", bigEndian)
		}
		_, err := msg.Args()
		c.HasError(err, "big-endian: %v", bigEndian)
	}
	c.NoError(b.client.Hello()) // The connection is still usable, which is the whole point
}

func TestConnCarriesOnAfterAMessageThatIsNotValid(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	received := make(chan *Message, 2)
	b.client.Subscribe(SignalFilter{Member: paddedMember}, func(msg *Message) { received <- msg })
	lost := make(chan error, 1)
	b.client.OnDisconnect(func(err error) { lost <- err })
	wait := callAsync(t, b.client, greet(c, "hello"))
	call := b.nextCall()
	// A correctly framed signal with no INTERFACE field is a fault in that one message: every byte it declared has
	// been read, so the stream is still positioned at the start of the next message. Ending the connection for it took
	// every pending call and every subscription with it, and since a Session is never reconnected, accessibility with
	// them.
	b.writeRaw(interfacelessSignal(t, 1))
	// The signal that follows it proves the reader picked up exactly where it should have, and that the invalid one
	// was skipped rather than delivered.
	b.emit(testPath, eventInterface, paddedMember, "s", "after")
	msg, ok := awaitOne(received)
	if !ok {
		t.Fatal("timed out waiting for the signal that followed the invalid one")
	}
	c.Equal(eventInterface, msg.Interface)
	// The call that was in flight is still waiting for its answer rather than having been failed.
	b.replyTo(call, "s", "hi")
	reply, err := wait()
	c.NoError(err)
	c.NotNil(reply)
	select {
	case err = <-lost:
		t.Fatalf("the connection ended over one invalid message: %v", err)
	default:
	}
	c.NoError(b.client.Hello())
}

// TestAReplyIsMatchedOnItsSenderAsWellAsItsSerial covers the check that keeps a call from being completed by whoever
// guesses its serial first. Serials start at one and count up, so a peer that can reach this connection's unique name
// could otherwise address a METHOD_RETURN to it with a guessed REPLY_SERIAL and hand a caller forged values, the
// genuine reply arriving afterwards to be discarded as unmatched. A rejected reply must leave the call pending rather
// than failing it, which is what makes the genuine reply that follows the one that completes it.
func TestAReplyIsMatchedOnItsSenderAsWellAsItsSerial(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	// Every case below waits for a reply that only arrives after a rejected one has been ignored, so the connection's
	// own five second deadline would be the thing under test rather than the check.
	b.client.timeout = time.Hour

	// A call addressed to a unique name is answered by that connection and by nobody else, so a reply from another one
	// is dropped and the genuine one still completes the call.
	msg := NewMethodCall(":1.99", testPath, testInterface, "Greet")
	c.NoError(msg.SetBody("hello"))
	wait := callAsync(t, b.client, msg)
	call := b.nextCall()
	b.replyFrom(call, ":1.17", "s", "forged")
	b.replyFrom(call, ":1.99", "s", "genuine")
	reply, err := wait()
	c.NoError(err)
	args, err := reply.Args()
	c.NoError(err)
	c.Equal([]any{"genuine"}, args)

	// The bus answers for a unique name that has gone away, and for a message it refuses to deliver, so its own name is
	// accepted for such a call as well: rejecting it would leave the call waiting for a reply that is never coming.
	msg = NewMethodCall(":1.99", testPath, testInterface, "Greet")
	c.NoError(msg.SetBody("hello"))
	wait = callAsync(t, b.client, msg)
	call = b.nextCall()
	b.errorFrom(call, busName, ServiceUnknown, "no such connection")
	reply, err = wait()
	c.Nil(reply)
	c.HasError(err)

	// A call addressed to the bus itself is answered by the bus, so a reply from another connection is dropped here
	// too. The path is one of the test object's rather than the bus's own, since the fake bus answers everything
	// addressed to its own object for itself; only the destination decides what is accepted.
	msg = NewMethodCall(busName, testPath, testInterface, "Greet")
	c.NoError(msg.SetBody("hello"))
	wait = callAsync(t, b.client, msg)
	call = b.nextCall()
	b.replyFrom(call, ":1.17", "s", "forged")
	b.replyFrom(call, busName, "s", "genuine")
	reply, err = wait()
	c.NoError(err)
	args, err = reply.Args()
	c.NoError(err)
	c.Equal([]any{"genuine"}, args)

	// A call addressed to a well-known name is answered by whichever connection owns it, under the unique name it was
	// given, which the caller has no way of knowing without asking. Enforcing anything here would reject every
	// legitimate reply, so the reply is accepted whoever it says it came from. That is why the callers ask the bus who
	// owns the name and address the unique name it gives back, which turns their calls into the first case above; only
	// a call made when nothing owns the name yet, which is what starts the service, is still left like this.
	wait = callAsync(t, b.client, greet(c, "hello"))
	call = b.nextCall()
	b.replyFrom(call, ":1.17", "s", "the owner of the name")
	reply, err = wait()
	c.NoError(err)
	args, err = reply.Args()
	c.NoError(err)
	c.Equal([]any{"the owner of the name"}, args)
}

// TestAReplyWithNoSenderCompletesACall covers the peer to peer case: a connection with no bus on the other end has
// nobody to fill the SENDER field in, so a reply that carries none is the only kind there is. Rejecting those would
// break every connection made with [Connect], which here is the tests and nothing else: a connection that reaches a
// bus is dialed, and [Dial] performs the Hello that gives it the unique name every reply to it is then stamped with.
func TestAReplyWithNoSenderCompletesACall(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	b.client.timeout = time.Hour
	msg := NewMethodCall(":1.99", testPath, testInterface, "Greet")
	c.NoError(msg.SetBody("hello"))
	wait := callAsync(t, b.client, msg)
	call := b.nextCall()
	b.replyTo(call, "s", "unstamped")
	reply, err := wait()
	c.NoError(err)
	args, err := reply.Args()
	c.NoError(err)
	c.Equal([]any{"unstamped"}, args)
}

func TestCallRemovesTheSerialItRegistered(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	// The write and the wait for the reply are both bounded by this, and the test deliberately touches the message
	// while the call is in flight, which only the timeout path would ever look at again.
	b.client.timeout = time.Hour
	msg := greet(c, "hello")
	wait := callAsync(t, b.client, msg)
	call := b.nextCall()
	// Sending the same *Message again overwrites the serial the call in flight registered, which is what two
	// goroutines sharing one message do. This entry stands in for that second call: the first call's cleanup has to
	// remove the serial it registered rather than whatever the field holds by the time it runs, or it removes the
	// other call's channel instead, leaving its own entry in the map forever and the other call's reply to be
	// discarded on arrival and time out five seconds later.
	const offset = 1000
	other := &pending{reply: make(chan *Message, 1)}
	b.client.mu.Lock()
	b.client.pending[call.Serial+offset] = other
	b.client.mu.Unlock()
	msg.Serial = call.Serial + offset
	b.replyTo(call, "s", "hi")
	reply, err := wait()
	c.NoError(err)
	c.NotNil(reply)
	b.client.mu.Lock()
	_, stillPending := b.client.pending[call.Serial+offset]
	delete(b.client.pending, call.Serial+offset)
	leftOver := len(b.client.pending)
	b.client.mu.Unlock()
	c.True(stillPending, "the cleanup removed another call's entry from the pending map")
	c.Equal(0, leftOver, "the cleanup left its own entry in the pending map")
}

func TestQueueIsBoundedByItemsAndBytes(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	q := newQueue[string](8, 10)
	c.True(q.push("abcde", 5))
	c.True(q.push("fghij", 5))
	c.False(q.push("k", 1)) // The byte bound is reached long before the item bound
	c.Equal(1, q.dropped())
	items, ok := q.drain()
	c.True(ok)
	c.Equal([]string{"abcde", "fghij"}, items)
	c.True(q.push("lmnop", 5)) // Draining releases the bytes as well as the items
	// An item larger than the entire byte bound still goes into an empty queue, since refusing it would mean never
	// delivering it at all.
	oversized := newQueue[string](8, 10)
	c.True(oversized.push("enormous", 1000))
	c.False(oversized.push("x", 1))
	// The item bound still does the work when the items are small.
	many := newQueue[string](2, 1<<20)
	c.True(many.push("a", 1))
	c.True(many.push("b", 1))
	c.False(many.push("c", 1))
	c.Equal(1, many.dropped())
	many.close()
	c.False(many.push("d", 1))
	c.Equal(1, many.dropped()) // A push after the queue has closed is not a drop, since nothing is going anywhere
	items, ok = many.drain()
	c.False(ok)
	c.Nil(items)
}

func TestIncomingQueueIsBoundedByBytes(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	defer close(release)
	b.client.Subscribe(SignalFilter{Member: "Flood"}, func(_ *Message) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
	})
	payload := make([]byte, 1<<20)
	b.emit(testPath, eventInterface, "Flood", "ay", payload)
	if _, ok := awaitOne(entered); !ok {
		t.Fatal("timed out waiting for the handler to be called")
	}
	// The dispatcher is now wedged in the handler, so everything that follows piles up behind it. Far fewer than
	// maxQueued messages arrive, so only the bound on their total size can stop the backlog from growing.
	for range 2 + maxQueuedBytes/len(payload) {
		b.emit(testPath, eventInterface, "Flood", "ay", payload)
	}
	c.True(b.client.Dropped() > 0, "expected some messages to have been dropped")
}

func TestConnEndsWhenAMessageCannotBeDecoded(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	lost := make(chan error, 1)
	b.client.OnDisconnect(func(err error) { lost <- err })
	wait := callAsync(t, b.client, greet(c, "hello"))
	b.nextCall() // In flight, and never answered

	// Anything else on a shared bus may write nonsense, and there is nothing to be done with a stream that no longer
	// makes sense but end it: this is a fault in the framing rather than in one message, so the length that says where
	// the next message would start is part of what cannot be believed and there is nothing to resynchronize to. A
	// message that arrives whole and is merely wrong about itself costs only itself; see
	// TestConnCarriesOnAfterAMessageThatIsNotValid.
	b.writeRaw([]byte("Xnot a message at all, whatever else it may be"))
	err, ok := awaitOne(lost)
	if !ok {
		t.Fatal("timed out waiting for the disconnect notification")
	}
	c.HasError(err)
	c.Contains(err.Error(), "unable to read a message")
	c.Contains(err.Error(), "not a valid endianness flag")
	// The call that was in flight fails rather than waiting for the five seconds it is otherwise given.
	reply, callErr := wait()
	c.Nil(reply)
	c.HasError(callErr)
	// Everything afterwards reports the failure too.
	c.HasError(b.client.Send(greet(c, "hello")))
}

func TestIncomingQueueIsBoundedByWhatAMessageKeepsAlive(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	defer close(release)
	b.client.Subscribe(SignalFilter{Member: paddedMember}, func(_ *Message) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
	})
	const padding = 1 << 20
	b.writeRaw(paddedSignal(t, 1, padding))
	if _, ok := awaitOne(entered); !ok {
		t.Fatal("timed out waiting for the handler to be called")
	}
	// The dispatcher is now wedged in the handler, so everything that follows piles up behind it. Each of these
	// messages decodes to a few dozen bytes of parts and a body of six, yet holds a megabyte: charging the queue only
	// what the parts add up to let far more than the byte bound in, which is the whole of what keeps a peer that
	// floods while a handler is blocked costing messages rather than memory.
	for i := range 2 + maxQueuedBytes/padding {
		b.writeRaw(paddedSignal(t, uint32(2+i), padding))
	}
	c.True(b.client.Dropped() > 0, "expected some messages to have been dropped")
}

// reentrantRule is the match rule that a [reentrantObject] asks the bus for while it is answering a call.
const reentrantRule = "type='signal',interface='org.example.Reentrant'"

// reentrantObject answers a call by calling back into the connection from the dispatcher goroutine, which is what the
// documentation on [Conn] promises a handler may do.
type reentrantObject struct {
	failures chan error
}

func (o *reentrantObject) Interfaces() []*Interface {
	return []*Interface{
		{
			Name:    testInterface,
			Methods: []*Method{{Name: greetMember, In: "s", Out: "s", Handle: o.greet}},
		},
	}
}

// greet emits a signal, asks the bus for a match rule and makes a call of its own, all before answering the call it was
// given. None of it may deadlock: the reader routes the replies, so nothing a handler waits for is waiting on the
// dispatcher that the handler is occupying.
func (o *reentrantObject) greet(call *Call) {
	conn := call.Conn()
	conn.Emit(testPath, eventInterface, "StateChanged", "focused", int32(1))
	if err := conn.AddMatch(reentrantRule); err != nil {
		o.failures <- err
	}
	reply, err := conn.Call(NewMethodCall(testService, testPath, testInterface, "Ping"))
	if err != nil {
		o.failures <- err
		call.Error(Failed, err.Error())
		return
	}
	args, err := reply.Args()
	if err != nil {
		o.failures <- err
		call.Error(Failed, err.Error())
		return
	}
	call.ReplyWithSignature("", fmt.Sprintf("hello %v", args[0]))
}

func TestHandlerMayCallBackIntoTheConnection(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	obj := &reentrantObject{failures: make(chan error, 4)}
	c.NoError(b.client.Export(testPath, obj))
	// The call the handler makes has to be answered by someone other than the goroutine waiting for the outer reply.
	go func() {
		if nested, ok := awaitOne(b.calls); ok {
			b.replyTo(nested, "s", "pong")
		}
	}()
	c.Equal([]any{"hello pong"}, replyValues(t, b.call(testPath, testInterface, greetMember, "s", "you")))
	c.Equal("StateChanged", b.nextSignal().Member)
	c.Equal([]string{reentrantRule}, b.matchRules())
	close(obj.failures)
	for err := range obj.failures {
		c.NoError(err)
	}
}
