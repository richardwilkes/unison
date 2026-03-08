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

var _ cookieProcessor = &cookie[*GetInputFocusReply]{}

type cookieProcessor interface {
	processCookie(seq uint16, r *protoBufferReader, err error) bool
	setSequenceID(seq uint16)
}

type cookie[T protoReader] struct {
	conn      *Conn
	replyChan chan *protoBufferReader
	errorChan chan error
	pingChan  chan bool
	replyData T
	sequence  uint16
}

func newCookie[T protoReader](conn *Conn, checked, reply bool, replyData T) *cookie[T] {
	c := cookie[T]{
		conn:      conn,
		replyData: replyData,
	}
	if checked {
		c.errorChan = make(chan error, 1)
		if !reply {
			c.pingChan = make(chan bool, 1)
		}
	}
	if reply {
		c.replyChan = make(chan *protoBufferReader, 1)
		if !checked {
			c.pingChan = make(chan bool, 1)
		}
	}
	return &c
}

func (c *cookie[T]) Reply() (T, error) {
	if c.errorChan != nil {
		return c.replyChecked()
	}
	return c.replyUnchecked()
}

func (c *cookie[T]) replyChecked() (T, error) {
	if c.replyChan == nil || c.errorChan == nil {
		return c.replyData, errs.New("not expecting a reply or an error")
	}
	select {
	case r := <-c.replyChan:
		c.replyData.protoRead(r)
		return c.replyData, nil
	case err := <-c.errorChan:
		return c.replyData, err
	case <-c.conn.doneRead:
		return c.replyData, io.EOF
	}
}

func (c *cookie[T]) replyUnchecked() (T, error) {
	if c.replyChan == nil {
		return c.replyData, errs.New("not expecting a reply")
	}
	select {
	case r := <-c.replyChan:
		c.replyData.protoRead(r)
		return c.replyData, nil
	case <-c.pingChan:
		return c.replyData, nil
	case <-c.conn.doneRead:
		return c.replyData, io.EOF
	}
}

func (c *cookie[T]) Check() error {
	if c.replyChan != nil {
		return errors.New("expecting a reply")
	}
	if c.errorChan == nil {
		return errors.New("not expecting a possible error")
	}
	select {
	case err := <-c.errorChan:
		return err
	case <-c.pingChan:
		return nil
	default:
		c.conn.Sync()
		select {
		case err := <-c.errorChan:
			return err
		case <-c.pingChan:
			return nil
		case <-c.conn.doneRead:
			return io.EOF
		}
	}
}

func (c *cookie[T]) processCookie(seq uint16, r *protoBufferReader, err error) bool {
	if c.sequence == seq {
		if err != nil {
			if c.errorChan != nil {
				c.errorChan <- err
			} else {
				c.conn.eventChan <- err
				if c.pingChan != nil {
					c.pingChan <- true
				}
			}
		} else {
			if c.replyChan == nil {
				slog.Warn("reply does not have a cookie with a valid reply channel", "sequence", seq)
				return false
			}
			c.replyChan <- r
		}
		return true
	}
	switch {
	case c.replyChan != nil && c.errorChan != nil:
		slog.Warn("found cookie that is expecting a reply but will never get it",
			"sequence", c.sequence,
			"current sequence", seq)
	case c.replyChan != nil && c.pingChan != nil:
		slog.Warn("found cookie that is expecting a reply and not an error, but will never get it",
			"sequence", c.sequence,
			"current sequence", seq)
	case c.pingChan != nil && c.errorChan != nil:
		c.pingChan <- true
	}
	return false
}

func (c *cookie[T]) setSequenceID(seq uint16) {
	c.sequence = seq
}
