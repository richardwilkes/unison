// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

import (
	"errors"
	"io"
	"log/slog"

	"github.com/richardwilkes/toolbox/v2/errs"
)

var _ requestProcessor = &Request[*GetInputFocusReply]{}

type requestProcessor interface {
	processRequest(seq uint16, r *Reader, err error) bool
	setSequenceID(seq uint16)
}

// Request holds data for managing a request to the server.
type Request[T protoReader] struct {
	conn      *Conn
	replyChan chan *Reader
	errorChan chan error
	pingChan  chan bool
	replyData T
	sequence  uint16
}

func newRequest[T protoReader](conn *Conn, checked, reply bool, replyData T) *Request[T] {
	r := Request[T]{
		conn:      conn,
		replyData: replyData,
	}
	if checked {
		r.errorChan = make(chan error, 1)
		if !reply {
			r.pingChan = make(chan bool, 1)
		}
	}
	if reply {
		r.replyChan = make(chan *Reader, 1)
		if !checked {
			r.pingChan = make(chan bool, 1)
		}
	}
	return &r
}

// Reply waits for a reply to the request and returns it, or an error if one is received. If the request was created
// with checked=true, then the error channel will be used to receive any errors from the server. If the request was
// created with reply=true, then the reply channel will be used to receive the reply data from the server. If the
// request was created with checked=false and reply=false, then neither channel will be used and an error will be
// returned if either is attempted to be read from. If the request was created with checked=true and reply=false, then
// the ping channel will be used to signal when a reply is received without an error, and an error will be returned if
// an error is received or if a reply is received without an error. If the request was created with checked=false and
// reply=true, then the ping channel will be used to signal when a reply is received, and an error will be returned if a
// reply is received without an error.
func (r *Request[T]) Reply() (T, error) {
	if r.errorChan != nil {
		return r.replyChecked()
	}
	return r.replyUnchecked()
}

func (r *Request[T]) replyChecked() (T, error) {
	if r.replyChan == nil || r.errorChan == nil {
		return r.replyData, errs.New("not expecting a reply or an error")
	}
	select {
	case in := <-r.replyChan:
		r.replyData.protoRead(in)
		return r.replyData, nil
	case err := <-r.errorChan:
		return r.replyData, err
	case <-r.conn.doneRead:
		return r.replyData, io.EOF
	}
}

func (r *Request[T]) replyUnchecked() (T, error) {
	if r.replyChan == nil {
		return r.replyData, errs.New("not expecting a reply")
	}
	select {
	case in := <-r.replyChan:
		r.replyData.protoRead(in)
		return r.replyData, nil
	case <-r.pingChan:
		return r.replyData, nil
	case <-r.conn.doneRead:
		return r.replyData, io.EOF
	}
}

// Check waits for a reply to the request and returns an error if one is received, or nil if a reply is received without
// an error. If the request was created with checked=true, then the error channel will be used to receive any errors
// from the server. If the request was created with checked=false, then the error channel will not be used and an error
// will be returned if an attempt is made to read from it. If the request was created with checked=true and reply=false,
// then the ping channel will be used to signal when a reply is received without an error, and an error will be returned
// if an error is received or if a reply is received without an error. If the request was created with checked=false and
// reply=true, then the ping channel will be used to signal when a reply is received, and an error will be returned if a
// reply is received without an error. If the request was created with checked=false and reply=false, then neither
// channel will be used and an error will be returned if either is attempted to be read from.
func (r *Request[T]) Check() error {
	if r.replyChan != nil {
		return errors.New("expecting a reply")
	}
	if r.errorChan == nil {
		return errors.New("not expecting a possible error")
	}
	select {
	case err := <-r.errorChan:
		return err
	case <-r.pingChan:
		return nil
	default:
		r.conn.Sync()
		select {
		case err := <-r.errorChan:
			return err
		case <-r.pingChan:
			return nil
		case <-r.conn.doneRead:
			return io.EOF
		}
	}
}

func (r *Request[T]) processRequest(seq uint16, in *Reader, err error) bool {
	if r.sequence == seq {
		if err != nil {
			if r.errorChan != nil {
				r.errorChan <- err
			} else {
				r.conn.eventChan <- &errorEvent{err: err}
				if r.pingChan != nil {
					r.pingChan <- true
				}
			}
		} else {
			if r.replyChan == nil {
				slog.Warn("reply does not have a request with a valid reply channel", "sequence", seq)
				return false
			}
			r.replyChan <- in
		}
		return true
	}
	switch {
	case r.replyChan != nil && r.errorChan != nil:
		slog.Warn("found request that is expecting a reply but will never get it",
			"sequence", r.sequence,
			"current sequence", seq)
	case r.replyChan != nil && r.pingChan != nil:
		slog.Warn("found request that is expecting a reply and not an error, but will never get it",
			"sequence", r.sequence,
			"current sequence", seq)
	case r.pingChan != nil && r.errorChan != nil:
		r.pingChan <- true
	}
	return false
}

func (r *Request[T]) setSequenceID(seq uint16) {
	r.sequence = seq
}
