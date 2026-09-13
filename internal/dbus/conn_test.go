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
	"io"
	"net"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xio"
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
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	b.client.timeout = 50 * time.Millisecond
	wait := callAsync(t, b.client, greet(c, "hello"))
	b.nextCall() // Received, but deliberately left unanswered
	reply, err := wait()
	c.Nil(reply)
	c.HasError(err)
	c.Contains(err.Error(), "timed out")
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
		Path:      string(testPath),
		Interface: eventInterface,
		Member:    "StateChanged",
	}, func(msg *Message) { filtered <- msg })
	b.client.Subscribe(SignalFilter{}, func(msg *Message) { everything <- msg })

	b.emit(testPath, eventInterface, "StateChanged", "s", "focused")
	b.emit(testPath, eventInterface, "SomethingElse", "s", "ignored")
	b.emit("/org/example/other", eventInterface, "StateChanged", "s", "elsewhere")

	msg := <-filtered
	args, err := msg.Args()
	c.NoError(err)
	c.Equal([]any{"focused"}, args)
	for range 3 {
		c.NotNil(<-everything)
	}
	c.Equal(0, len(filtered))

	// Once canceled, the subscription hears nothing more. The catch-all was registered second, so its receipt of the
	// next signal proves the canceled one would already have been called.
	cancel()
	cancel() // Canceling twice is harmless
	b.emit(testPath, eventInterface, "StateChanged", "s", "after")
	c.NotNil(<-everything)
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
	c.NoError(err)
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
