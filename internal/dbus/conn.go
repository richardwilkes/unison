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
	"maps"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/toolbox/v2/xos"
)

const (
	// busDestination is the bus's own well-known name.
	busDestination = "org.freedesktop.DBus"
	// busInterface is the interface the bus itself implements.
	busInterface = "org.freedesktop.DBus"
	// busObjectPath is the path of the bus's own object.
	busObjectPath ObjectPath = "/org/freedesktop/DBus"
)

const (
	// callTimeout is how long [Conn.Call] waits for a reply. The accessibility buses answer in microseconds, so a wait
	// this long only ever happens when the peer is wedged, and waiting forever would wedge us too.
	callTimeout = 5 * time.Second
	// dialTimeout bounds connecting and authenticating.
	dialTimeout = 5 * time.Second
	// maxQueued is how many messages may be waiting in one of a connection's queues before further ones are dropped.
	// Reaching it means the peer has stopped reading, or a handler has stopped returning, for long enough that the
	// backlog is no longer worth delivering.
	maxQueued = 16384
)

// ErrClosed is returned by operations on a connection that has been closed, and is what [Conn.OnDisconnect] callbacks
// receive when the connection was closed locally rather than lost.
var ErrClosed = errors.New("dbus: connection closed")

// SignalFilter selects the signals that a subscription receives. An empty field matches any value, so the zero
// SignalFilter matches every signal.
type SignalFilter struct {
	Sender    string
	Path      string
	Interface string
	Member    string
}

// matches returns true if msg satisfies every non-empty field of the filter.
func (f *SignalFilter) matches(msg *Message) bool {
	return (f.Sender == "" || f.Sender == msg.Sender) &&
		(f.Path == "" || f.Path == string(msg.Path)) &&
		(f.Interface == "" || f.Interface == msg.Interface) &&
		(f.Member == "" || f.Member == msg.Member)
}

// subscription is one registered signal handler.
type subscription struct {
	handler func(*Message)
	filter  SignalFilter
}

// subtree is one registered dynamic object range.
type subtree struct {
	resolve func(path ObjectPath) Object
	prefix  ObjectPath
}

// covers returns true if path is the subtree's prefix or falls below it.
func (s *subtree) covers(path ObjectPath) bool {
	if s.prefix == "/" {
		return true
	}
	return path == s.prefix || strings.HasPrefix(string(path), string(s.prefix)+"/")
}

// outgoing is one encoded message waiting to be written, plus the channel, if any, that the sender is waiting on.
type outgoing struct {
	done chan error
	data []byte
}

// finish reports the outcome of the write to whoever is waiting for it, if anyone is.
func (o *outgoing) finish(err error) {
	if o.done != nil {
		o.done <- err
	}
}

// Conn is a connection to a D-Bus server. It is safe for concurrent use.
//
// Three goroutines serve a connection. The reader decodes messages and either completes the pending [Conn.Call] whose
// serial they answer or hands them to the dispatcher, so nothing a handler does can stall it. The dispatcher runs the
// signal and method call handlers, one at a time and in the order the messages arrived, which means a handler that
// blocks delays every later message: handlers should do their work and return, replying to a method call later from
// another goroutine if they must. The writer, which is started only when there is something to write, drains an
// unbounded queue, so emitting a signal never blocks on the peer.
type Conn struct {
	rwc          io.ReadWriteCloser
	in           *bufio.Reader
	out          *queue[*outgoing]
	incoming     *queue[*Message]
	closed       chan struct{}
	pending      map[uint32]chan *Message
	objects      map[ObjectPath]Object
	err          error
	name         string
	subtrees     []*subtree
	subs         []*subscription
	onDisconnect []func(error)
	timeout      time.Duration // How long Call waits for a reply; only the tests ever change it
	serial       atomic.Uint32
	mu           sync.Mutex
	writerOnce   sync.Once
	dispatchOnce sync.Once
}

// Dial connects to the D-Bus server at the given address, authenticates, and performs the Hello handshake that gives
// the connection its unique name. The address may list several alternatives separated by ";", which are tried in order.
func Dial(address string) (*Conn, error) {
	targets, err := parseAddress(address)
	if err != nil {
		return nil, err
	}
	failures := make([]error, 0, len(targets))
	for _, target := range targets {
		conn, dialErr := dialTransport(target)
		if dialErr != nil {
			failures = append(failures, dialErr)
			continue
		}
		return conn, nil
	}
	return nil, errors.Join(failures...)
}

// dialTransport connects to and authenticates with one of the endpoints named by an address.
func dialTransport(target transport) (*Conn, error) {
	socket, err := net.DialTimeout(target.network, target.address, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("dbus: unable to connect to %s: %w", target.address, err)
	}
	// The handshake is the only part of the protocol with a deadline, since after it the connection may sit idle for
	// hours waiting for a signal.
	if err = socket.SetDeadline(time.Now().Add(dialTimeout)); err != nil {
		xio.CloseIgnoringErrors(socket)
		return nil, fmt.Errorf("dbus: unable to prepare the connection to %s: %w", target.address, err)
	}
	in := bufio.NewReader(socket)
	if err = authenticate(socket, in); err != nil {
		xio.CloseIgnoringErrors(socket)
		return nil, err
	}
	if err = socket.SetDeadline(time.Time{}); err != nil {
		xio.CloseIgnoringErrors(socket)
		return nil, fmt.Errorf("dbus: unable to clear the deadline on the connection to %s: %w", target.address, err)
	}
	conn := newConn(socket, in)
	if err = conn.Hello(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// Connect returns a connection that speaks D-Bus over rwc. The SASL handshake is assumed to have already been performed,
// or to be unnecessary, which is the case for the direct peer to peer connections the tests use, and no Hello is sent,
// so [Conn.Name] is empty until [Conn.Hello] is called. Use [Dial] to reach a real bus.
func Connect(rwc io.ReadWriteCloser) (*Conn, error) {
	if rwc == nil {
		return nil, errors.New("dbus: no transport was supplied")
	}
	return newConn(rwc, bufio.NewReader(rwc)), nil
}

// newConn creates a connection and starts its reader goroutine. in must be the only reader of rwc.
func newConn(rwc io.ReadWriteCloser, in *bufio.Reader) *Conn {
	c := &Conn{
		rwc:      rwc,
		in:       in,
		out:      newQueue[*outgoing](),
		incoming: newQueue[*Message](),
		closed:   make(chan struct{}),
		pending:  make(map[uint32]chan *Message),
		objects:  make(map[ObjectPath]Object),
		timeout:  callTimeout,
	}
	go c.readLoop()
	return c
}

// Hello performs the org.freedesktop.DBus.Hello handshake, which every connection to a bus must make before it may send
// anything else, and records the unique name the bus assigns. [Dial] calls this, so only a connection created by
// [Connect] needs it, and then only if the peer is a bus.
func (c *Conn) Hello() error {
	reply, err := c.Call(NewMethodCall(busDestination, busObjectPath, busInterface, "Hello"))
	if err != nil {
		return err
	}
	args, err := reply.Args()
	if err != nil {
		return err
	}
	name, ok := "", false
	if len(args) == 1 {
		name, ok = args[0].(string)
	}
	if !ok || name == "" {
		return fmt.Errorf("dbus: the bus did not return a unique name in reply to Hello (%s)", reply)
	}
	c.mu.Lock()
	c.name = name
	c.mu.Unlock()
	return nil
}

// Name returns the unique name the bus assigned to this connection, or an empty string if [Conn.Hello] has not been
// called.
func (c *Conn) Name() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.name
}

// Dropped returns how many messages have been dropped because a queue was full. It is only of interest for diagnostics:
// anything other than zero means the peer or a handler stopped keeping up.
func (c *Conn) Dropped() int {
	return c.out.dropped() + c.incoming.dropped()
}

// Call sends a method call and waits up to five seconds for the reply. An ERROR reply is returned as an [Error], never
// as the reply message.
func (c *Conn) Call(msg *Message) (*Message, error) {
	return c.CallWithFlags(msg, 0)
}

// CallWithFlags is [Conn.Call] with additional flags applied to the message. If the flags include
// [FlagNoReplyExpected], the message is sent and a nil reply is returned as soon as it has been written.
func (c *Conn) CallWithFlags(msg *Message, flags Flags) (*Message, error) {
	msg.Flags |= flags
	if msg.Flags&FlagNoReplyExpected != 0 {
		return nil, c.Send(msg)
	}
	msg.Serial = c.nextSerial()
	data, err := msg.Encode()
	if err != nil {
		return nil, err
	}
	ch := make(chan *Message, 1)
	c.mu.Lock()
	if c.err != nil {
		err = c.err
		c.mu.Unlock()
		return nil, err
	}
	c.pending[msg.Serial] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, msg.Serial)
		c.mu.Unlock()
	}()
	if err = c.writeAndWait(data); err != nil {
		return nil, err
	}
	timer := time.NewTimer(c.timeout)
	defer timer.Stop()
	select {
	case reply := <-ch:
		return replyOrError(reply)
	case <-c.closed:
		// The reply may have arrived just before the connection went away, in which case it is still the answer.
		select {
		case reply := <-ch:
			return replyOrError(reply)
		default:
		}
		return nil, c.failure()
	case <-timer.C:
		return nil, fmt.Errorf("dbus: timed out after %v waiting for a reply to %s", c.timeout, msg)
	}
}

// replyOrError turns an ERROR reply into an [Error] and passes any other reply through.
func replyOrError(reply *Message) (*Message, error) {
	if err := reply.AsError(); err != nil {
		return nil, err
	}
	return reply, nil
}

// Send assigns the message a serial, queues it, and returns once it has been handed to the transport, so a failure to
// write it is reported. Use [Conn.Enqueue] for messages that must never block the caller.
func (c *Conn) Send(msg *Message) error {
	msg.Serial = c.nextSerial()
	data, err := msg.Encode()
	if err != nil {
		return err
	}
	return c.writeAndWait(data)
}

// Enqueue assigns the message a serial and queues it for the writer goroutine without waiting for it to be written. It
// never blocks. A message that arrives when the queue has reached its limit, or after the connection has ended, is
// dropped; the ones dropped because the queue was full are counted by [Conn.Dropped].
func (c *Conn) Enqueue(msg *Message) {
	msg.Serial = c.nextSerial()
	data, err := msg.Encode()
	if err != nil {
		errs.Log(errs.NewWithCause("dbus: unable to encode a message to enqueue", err), "message", msg.String())
		return
	}
	c.enqueue(&outgoing{data: data})
}

// Emit queues a signal, deriving its signature from the values passed. It never blocks. Use
// [Conn.EmitWithSignature] when the signature cannot be derived, which is the case for an empty array or dictionary.
func (c *Conn) Emit(path ObjectPath, iface, member string, args ...any) {
	c.EmitWithSignature(path, iface, member, "", args...)
}

// EmitWithSignature queues a signal whose body is marshaled with the given signature. An empty signature derives the
// signature from the values, exactly as [Conn.Emit] does. It never blocks.
func (c *Conn) EmitWithSignature(path ObjectPath, iface, member string, sig Signature, args ...any) {
	msg := NewSignal(path, iface, member)
	var err error
	if sig == "" {
		err = msg.SetBody(args...)
	} else {
		err = msg.SetBodyWithSignature(sig, args...)
	}
	if err != nil {
		errs.Log(errs.NewWithCause("dbus: unable to marshal a signal", err), "path", string(path), "interface", iface,
			"member", member)
		return
	}
	c.Enqueue(msg)
}

// AddMatch asks the bus to deliver the signals that satisfy the match rule, e.g.
// "type='signal',interface='org.freedesktop.portal.Settings'". Matching signals still have to be routed to a handler
// with [Conn.Subscribe].
func (c *Conn) AddMatch(rule string) error {
	return c.matchRule("AddMatch", rule)
}

// RemoveMatch asks the bus to stop delivering the signals that satisfy a match rule previously added with
// [Conn.AddMatch].
func (c *Conn) RemoveMatch(rule string) error {
	return c.matchRule("RemoveMatch", rule)
}

// matchRule calls one of the bus's match rule methods.
func (c *Conn) matchRule(member, rule string) error {
	msg := NewMethodCall(busDestination, busObjectPath, busInterface, member)
	if err := msg.SetBody(rule); err != nil {
		return err
	}
	_, err := c.Call(msg)
	return err
}

// Subscribe routes the signals that satisfy the filter to the handler and returns a function that stops doing so. The
// handler runs on the dispatcher goroutine, so it must not block; see [Conn]. Subscribing does not by itself ask the bus
// for anything, so a subscription for signals from another connection also needs [Conn.AddMatch].
func (c *Conn) Subscribe(filter SignalFilter, handler func(*Message)) (cancel func()) {
	sub := &subscription{handler: handler, filter: filter}
	c.mu.Lock()
	// The dispatcher reads the slice without the lock, so it is never modified in place.
	c.subs = append(slices.Clip(c.subs), sub)
	c.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			if i := slices.Index(c.subs, sub); i >= 0 {
				c.subs = slices.Delete(slices.Clone(c.subs), i, i+1)
			}
			c.mu.Unlock()
		})
	}
}

// Export publishes an object at a path, replacing whatever was there. Every exported object also answers the
// org.freedesktop.DBus.Introspectable, org.freedesktop.DBus.Properties and org.freedesktop.DBus.Peer interfaces; see
// [Object].
func (c *Conn) Export(path ObjectPath, obj Object) error {
	if err := path.Validate(); err != nil {
		return err
	}
	if obj == nil {
		return errors.New("dbus: no object was supplied to export")
	}
	c.mu.Lock()
	c.objects[path] = obj
	c.mu.Unlock()
	return nil
}

// ExportSubtree publishes the objects at and below a path, resolving each one as it is called. resolve is called on the
// dispatcher goroutine and returns nil for a path that has nothing at it, which is answered with an UnknownObject
// error. An object exported at an exact path with [Conn.Export] takes precedence, as does a subtree registered at a
// longer prefix.
func (c *Conn) ExportSubtree(prefix ObjectPath, resolve func(path ObjectPath) Object) error {
	if err := prefix.Validate(); err != nil {
		return err
	}
	if resolve == nil {
		return errors.New("dbus: no resolver was supplied to export a subtree")
	}
	c.mu.Lock()
	c.subtrees = append(slices.DeleteFunc(c.subtrees, func(s *subtree) bool { return s.prefix == prefix }),
		&subtree{resolve: resolve, prefix: prefix})
	c.mu.Unlock()
	return nil
}

// Unexport removes whatever [Conn.Export] or [Conn.ExportSubtree] published at the path.
func (c *Conn) Unexport(path ObjectPath) {
	c.mu.Lock()
	delete(c.objects, path)
	c.subtrees = slices.DeleteFunc(c.subtrees, func(s *subtree) bool { return s.prefix == path })
	c.mu.Unlock()
}

// OnDisconnect registers a function to call once, with the reason, when the connection is lost or closed. A connection
// that is already gone calls it immediately, on the calling goroutine; otherwise it is called on a goroutine of the
// connection's own. Either way it must not block.
func (c *Conn) OnDisconnect(f func(error)) {
	if f == nil {
		return
	}
	c.mu.Lock()
	if err := c.err; err != nil {
		c.mu.Unlock()
		f(err)
		return
	}
	c.onDisconnect = append(slices.Clip(c.onDisconnect), f)
	c.mu.Unlock()
}

// Close shuts the connection down. Pending calls fail with [ErrClosed], queued messages are discarded, the goroutines
// exit and the [Conn.OnDisconnect] callbacks run. It may be called more than once and from any goroutine.
func (c *Conn) Close() {
	c.fail(ErrClosed)
}

// nextSerial returns the serial to use for the next outgoing message. Zero is not a valid serial, so it is skipped when
// the counter wraps.
func (c *Conn) nextSerial() uint32 {
	for {
		if serial := c.serial.Add(1); serial != 0 {
			return serial
		}
	}
}

// failure returns the error that ended the connection, or nil while it is still usable.
func (c *Conn) failure() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// enqueue hands an encoded message to the writer goroutine, starting it if this is the first thing to be written. It
// returns false if the message was dropped because the connection has ended or the queue is full.
func (c *Conn) enqueue(item *outgoing) bool {
	if c.failure() != nil {
		return false
	}
	c.writerOnce.Do(func() { go c.writeLoop() })
	return c.out.push(item, maxQueued)
}

// writeAndWait queues an encoded message and returns once it has been handed to the transport, reporting any error.
func (c *Conn) writeAndWait(data []byte) error {
	item := &outgoing{data: data, done: make(chan error, 1)}
	if !c.enqueue(item) {
		if err := c.failure(); err != nil {
			return err
		}
		return errors.New("dbus: the outgoing queue is full")
	}
	timer := time.NewTimer(c.timeout)
	defer timer.Stop()
	select {
	case err := <-item.done:
		return err
	case <-c.closed:
		return c.failure()
	case <-timer.C:
		return fmt.Errorf("dbus: timed out after %v writing a message", c.timeout)
	}
}

// writeLoop writes queued messages until the connection ends. Everything queued at the same time is written through one
// buffered writer and flushed together, so a burst of signals costs one write on the socket.
func (c *Conn) writeLoop() {
	w := bufio.NewWriter(c.rwc)
	for {
		items, ok := c.out.drain()
		if !ok {
			return
		}
		err := writeAll(w, items)
		for _, item := range items {
			item.finish(err)
		}
		if err != nil {
			c.fail(err)
			return
		}
	}
}

// writeAll writes and flushes a batch of messages.
func writeAll(w *bufio.Writer, items []*outgoing) error {
	for _, item := range items {
		if _, err := w.Write(item.data); err != nil {
			return fmt.Errorf("dbus: unable to write a message: %w", err)
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("dbus: unable to write a message: %w", err)
	}
	return nil
}

// readLoop decodes messages until the connection ends.
func (c *Conn) readLoop() {
	for {
		msg, err := Decode(c.in)
		if err != nil {
			if errors.Is(err, io.EOF) {
				c.fail(errors.New("dbus: the peer closed the connection"))
			} else {
				c.fail(fmt.Errorf("dbus: unable to read a message: %w", err))
			}
			return
		}
		c.route(msg)
	}
}

// route completes the call a reply answers or queues the message for the dispatcher. It must not block, since it runs on
// the reader goroutine.
func (c *Conn) route(msg *Message) {
	switch msg.Type {
	case TypeMethodReturn, TypeError:
		c.mu.Lock()
		ch := c.pending[msg.ReplySerial]
		delete(c.pending, msg.ReplySerial)
		c.mu.Unlock()
		if ch != nil {
			ch <- msg // Buffered, and only ever written to once, so this cannot block
		}
	case TypeSignal, TypeMethodCall:
		c.dispatchOnce.Do(func() { go c.dispatchLoop() })
		c.incoming.push(msg, maxQueued)
	default: // A reply to nothing, or a type we do not know, is ignored, as the specification requires
	}
}

// dispatchLoop runs the handlers for incoming signals and method calls, one at a time, until the connection ends.
func (c *Conn) dispatchLoop() {
	for {
		items, ok := c.incoming.drain()
		if !ok {
			return
		}
		for _, msg := range items {
			switch msg.Type {
			case TypeSignal:
				c.dispatchSignal(msg)
			case TypeMethodCall:
				c.dispatchMethodCall(msg)
			default: // route never queues anything else
			}
		}
	}
}

// dispatchSignal hands a signal to every subscription that wants it.
func (c *Conn) dispatchSignal(msg *Message) {
	c.mu.Lock()
	subs := c.subs // Never modified in place, so it is safe to use after the lock is released
	c.mu.Unlock()
	for _, sub := range subs {
		if sub.filter.matches(msg) {
			handler := sub.handler
			xos.SafeCall(func() { handler(msg) }, func(err error) {
				errs.Log(err, "signal", msg.String())
			})
		}
	}
}

// dispatchMethodCall answers an incoming method call, either from one of the object's own interfaces or from one of the
// standard interfaces that every object implements.
func (c *Conn) dispatchMethodCall(msg *Message) {
	obj := c.objectAt(msg.Path)
	call := &Call{conn: c, Message: msg, obj: obj}
	if obj == nil {
		call.Error(UnknownObject, fmt.Sprintf("no object is exported at %s", msg.Path))
		return
	}
	iface, method, ifaceFound := findMethod(obj.Interfaces(), msg.Interface, msg.Member)
	if method == nil {
		var builtinFound bool
		if iface, method, builtinFound = findMethod(builtinInterfaces, msg.Interface, msg.Member); method == nil {
			if msg.Interface != "" && !ifaceFound && !builtinFound {
				call.Error(UnknownInterface, fmt.Sprintf("%s does not implement %s", msg.Path, msg.Interface))
				return
			}
			call.Error(UnknownMethod, fmt.Sprintf("%s has no %s method", msg.Path, msg.Member))
			return
		}
	}
	if method.In != msg.Signature {
		call.Error(InvalidArgs, fmt.Sprintf("%s.%s takes (%s), not (%s)", iface.Name, method.Name, method.In,
			msg.Signature))
		return
	}
	call.out = method.Out
	handle := method.Handle
	if handle == nil {
		call.Error(NotSupported, fmt.Sprintf("%s.%s is declared but not implemented", iface.Name, method.Name))
		return
	}
	xos.SafeCall(func() { handle(call) }, func(err error) {
		errs.Log(err, "call", msg.String())
		call.Error(Failed, err.Error())
	})
}

// objectAt returns the object exported at the path, or nil if there is none. An exact export wins over a subtree, and a
// longer subtree prefix wins over a shorter one.
func (c *Conn) objectAt(path ObjectPath) Object {
	c.mu.Lock()
	obj, exists := c.objects[path]
	subtrees := c.subtrees
	c.mu.Unlock()
	if exists {
		return obj
	}
	var best *subtree
	for _, one := range subtrees {
		if one.covers(path) && (best == nil || len(one.prefix) > len(best.prefix)) {
			best = one
		}
	}
	if best == nil {
		return nil
	}
	return best.resolve(path) // Called without the lock, on the dispatcher goroutine
}

// childNodes returns the names of the immediate children of the path that have something exported at or below them, for
// the sake of introspection. A subtree contributes only its own prefix, since a resolver cannot be enumerated.
func (c *Conn) childNodes(path ObjectPath) []string {
	prefix := string(path)
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	names := make(map[string]struct{})
	add := func(candidate ObjectPath) {
		if rest, found := strings.CutPrefix(string(candidate), prefix); found && rest != "" {
			name, _, _ := strings.Cut(rest, "/")
			names[name] = struct{}{}
		}
	}
	c.mu.Lock()
	for exported := range c.objects {
		add(exported)
	}
	for _, one := range c.subtrees {
		add(one.prefix)
	}
	c.mu.Unlock()
	return slices.Sorted(maps.Keys(names))
}

// fail ends the connection, if it has not already ended, recording why.
func (c *Conn) fail(err error) {
	if err == nil {
		err = ErrClosed
	}
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return
	}
	c.err = err
	handlers := c.onDisconnect
	c.onDisconnect = nil
	c.mu.Unlock()
	close(c.closed) // Releases everything waiting for a reply or for a write
	c.out.close()
	c.incoming.close()
	xio.CloseIgnoringErrors(c.rwc) // Unblocks the reader and the writer
	if len(handlers) != 0 {
		go func() {
			for _, handler := range handlers {
				xos.SafeCall(func() { handler(err) }, func(panicErr error) { errs.Log(panicErr) })
			}
		}()
	}
}

// queue is an unbounded first in, first out queue that a producer may add to without blocking and that one consumer
// drains in batches.
type queue[T any] struct {
	cond      *sync.Cond
	items     []T
	dropCount int
	mu        sync.Mutex
	closed    bool
}

// newQueue creates a new, empty queue.
func newQueue[T any]() *queue[T] {
	q := &queue[T]{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// push adds an item unless the queue is closed or already holds limit items, in which case it returns false and counts
// the item as dropped. It never blocks.
func (q *queue[T]) push(item T, limit int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || len(q.items) >= limit {
		if !q.closed {
			q.dropCount++
		}
		return false
	}
	q.items = append(q.items, item)
	q.cond.Signal()
	return true
}

// drain waits for at least one item and returns everything the queue holds, or returns false once the queue is closed.
func (q *queue[T]) drain() (items []T, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if q.closed {
		return nil, false
	}
	items, q.items = q.items, nil
	return items, true
}

// close discards whatever the queue holds and releases the consumer.
func (q *queue[T]) close() {
	q.mu.Lock()
	q.closed = true
	q.items = nil
	q.cond.Broadcast()
	q.mu.Unlock()
}

// dropped returns how many items have been dropped because the queue was full.
func (q *queue[T]) dropped() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.dropCount
}
