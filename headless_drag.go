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
	"bytes"
	"net/url"
	"slices"
	"strings"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
)

// This file is the headless stand-in for the drag & drop machinery of a window server. It has the same two halves the
// real ones do. The source side is a session that owns the pointer until the drag ends: for a drag this application
// started that means a nested event loop inside startDrag, exactly as the Linux and Windows backends run one inside
// nativeStartDrag, and for data arriving from outside the application it means no loop at all, since there is no
// source in this process to hold. The target side is the very same Window.dragEntered/dragUpdate/drop/dragExit the
// platform backends call, fed a drag.Info of this backend's own — each platform has one of those too.
//
// Everything here runs on the UI thread, reached either from the input router (headless_input.go), which hands the
// pointer, button and wheel events to the session for as long as one is active, or from the external-drag driver
// methods in headless_screen.go.

// HeadlessDragResult describes how a drag & drop session ended.
type HeadlessDragResult struct {
	// Source is the window that started the drag, or nil for a drag that came from outside the application.
	Source *Window
	// Target is the window the drag ended over, or nil if it ended over nothing that could receive it.
	Target *Window
	// Image is the drag image the source supplied to StartDrag, or nil if it supplied none or the drag came from
	// outside the application. Nothing draws it — the drag image is deliberately left out of Capture() — so this is
	// how a test checks what a source chose to show.
	Image *Image
	// Data is what the drag was carrying. It is a private copy: it shares nothing with the drag.Info the target
	// callbacks were handed, nor with what the session is still holding, so neither side can alter what the other
	// sees. Each LastDrag call makes another one, so writing to these bytes cannot be seen anywhere else at all.
	Data []drag.Data
	// Origin is the point, in the source window's root coordinate space, at which the source picked the drag image
	// up. It is the zero point for a drag from outside the application.
	Origin geom.Point
	// Op is the operation the target last reported it would perform, and drag.None when there was no target or it
	// wanted nothing to do with the drag where it ended.
	Op drag.Op
	// Dropped is true when the drop was offered to the target, which is the case whenever the drag was released over
	// one that had reported an operation other than drag.None.
	Dropped bool
	// Handled is true when the target's DropCallback reported that it dealt with the data.
	Handled bool
	// Canceled is true when the drag was abandoned rather than dropped: the Escape key was pressed, the source
	// window was destroyed, or the drag was canceled through its handle.
	Canceled bool
}

// headlessDragInfo is the drag.Info a headless session hands to drop targets. It is a plain list of the data the drag
// was started with, so what a target sees is exactly what the source, or the test standing in for one, supplied.
//
// The composite types have an encoding of their own, and it is the one to use when supplying data to an external drag:
// the file paths of a uti.FileURL entry and the URLs of a uti.URL one are one per line, separated by newlines, with
// blank lines ignored. The platforms each have their own wire format for these — a CF_HDROP structure on Windows, a
// text/uri-list on X11 — and this is the headless equivalent, chosen so that a test can write the data out by hand.
type headlessDragInfo struct {
	data   []drag.Data
	opMask drag.Op
}

// SourceDragOpMask returns the operations the source is willing to have performed. A source that named none of them is
// taken to mean a copy, which is the answer X11 gives when a drag arrives with no action list.
func (d *headlessDragInfo) SourceDragOpMask() drag.Op {
	if d.opMask == drag.None {
		return drag.Copy
	}
	return d.opMask
}

// DataTypes returns the UTIs present in the drag, in the order they were supplied and with any duplicates dropped.
func (d *headlessDragInfo) DataTypes() []string {
	result := make([]string, 0, len(d.data))
	for _, one := range d.data {
		if one.Type != nil && one.Type.UTI != "" && !slices.Contains(result, one.Type.UTI) {
			result = append(result, one.Type.UTI)
		}
	}
	return result
}

// HasDataType reports whether the drag carries data that satisfies a request for this UTI: the very type, or one that
// conforms to it in either direction. That is the same rule eligibleWindowAt applies when deciding whether a window's
// registration is satisfied by what the drag carries, and it has to be the same rule here, since the panel-level checks
// — HasString, Well.DefaultCanAcceptDrop and the like — ask for the type they know rather than the one the drag happens
// to carry: a drag carrying public.plain-text is admitted to a window registered for public.utf8-plain-text, and
// HasString must then find it. x11DragInfo resolves by target lookup for the same reason.
func (d *headlessDragInfo) HasDataType(dataType string) bool {
	return d.index(dataType) >= 0
}

// Data returns the bytes carried for this UTI, or nil if the drag carries none. See HasDataType for how the match is
// made.
func (d *headlessDragInfo) Data(dataType string) []byte {
	if i := d.index(dataType); i >= 0 {
		return d.data[i].Data
	}
	return nil
}

// index returns the position of the entry that answers for dataType, or -1 if none does. An exact match is preferred
// over a conforming one, so a drag carrying both a type and one of its descendants hands out the one that was asked
// for.
func (d *headlessDragInfo) index(dataType string) int {
	if i := slices.IndexFunc(d.data, func(one drag.Data) bool {
		return one.Type != nil && one.Type.UTI == dataType
	}); i >= 0 {
		return i
	}
	return slices.IndexFunc(d.data, func(one drag.Data) bool { return headlessTypeSatisfies(one.Type, dataType) })
}

// headlessTypeSatisfies reports whether data of the offered type satisfies a request for the wanted UTI: it is the very
// type, or the two conform to one another in either direction, which is the same test the clipboard applies in
// selectDataType. A wanted UTI that is not registered with the uti package can only be matched exactly, since there is
// nothing to walk its conformance from.
func headlessTypeSatisfies(offered *uti.DataType, wanted string) bool {
	if offered == nil {
		return false
	}
	if offered.UTI == wanted {
		return true
	}
	lookup := uti.ByUTI(wanted)
	return lookup != nil && (offered.ConformsTo(lookup) || lookup.ConformsTo(offered))
}

func (d *headlessDragInfo) HasString() bool {
	return d.HasDataType(uti.UTF8PlainText.UTI)
}

func (d *headlessDragInfo) Text() string {
	return string(d.Data(uti.UTF8PlainText.UTI))
}

// HasFilePaths reports whether the drag carries file paths, which travel as uti.FileURL — the type Well and the other
// file-accepting widgets in this package register for.
func (d *headlessDragInfo) HasFilePaths() bool {
	return d.HasDataType(uti.FileURL.UTI)
}

// FilePaths returns the file paths the drag carries, one per line of the uti.FileURL data. The platforms hand back
// plain paths here rather than file: URLs, so these are the lines as they were supplied.
func (d *headlessDragInfo) FilePaths() []string {
	return headlessDragLines(d.Data(uti.FileURL.UTI))
}

func (d *headlessDragInfo) HasURLs() bool {
	return d.HasDataType(uti.URL.UTI)
}

func (d *headlessDragInfo) URLs() []*url.URL {
	lines := headlessDragLines(d.Data(uti.URL.UTI))
	result := make([]*url.URL, 0, len(lines))
	for _, line := range lines {
		// A line that is not a URL is skipped rather than reported: this is the same tolerance the platforms show
		// towards a malformed entry in a URI list, since the rest of the drag is still perfectly usable.
		if u, err := url.Parse(line); err == nil {
			result = append(result, u)
		}
	}
	return result
}

// headlessDragLines splits the newline-separated encoding the composite types use, discarding blank lines so that a
// trailing newline, or the blank line a hand-written test datum tends to end up with, does not become an entry.
func headlessDragLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	var result []string
	for line := range strings.Lines(string(data)) {
		if line = strings.TrimRight(line, "\r\n"); line != "" {
			result = append(result, line)
		}
	}
	return result
}

// headlessDragSession is a drag & drop in progress. There is at most one at a time, held on the session as hs.drag,
// and its presence is what diverts the pointer, button and wheel events away from the ordinary routing.
type headlessDragSession struct {
	info   *headlessDragInfo
	source *Window
	// img and origin are the drag image and where it was picked up from, as the source supplied them. Nothing draws
	// them — the drag image is deliberately left out of Capture() — so they are held only to be reported through
	// HeadlessDragResult when the drag ends.
	img *Image
	// target is the window the drag is currently over, which is nil whenever that window cannot receive this drag.
	target *Window
	origin geom.Point
	// lastOp is what the target reported the last time it was asked, and is therefore what a release at this moment
	// would drop as. drag.None means the target has nothing to do with the drag where it currently is.
	lastOp drag.Op
}

// beginDrag starts a session carrying the given data. source is the window that started it, or nil for data arriving
// from outside the application. The initial motion is delivered before this returns, so a target that is already under
// the pointer is entered rather than having to wait for the pointer to move.
func (s *headlessState) beginDrag(source *Window, img *Image, origin geom.Point, opMask drag.Op, data []drag.Data) {
	s.drag = &headlessDragSession{
		info: &headlessDragInfo{
			// Copied, both the list and the bytes in it, so that a source reusing its buffers cannot alter what the
			// drag is carrying. The platforms give the same guarantee by handing the data to the window server.
			data:   cloneDragData(data),
			opMask: opMask,
		},
		source: source,
		img:    img,
		origin: origin,
	}
	// Nothing may still be holding the pointer: StartDrag has already synthesized the source window's mouse up, and
	// leaving the grab in place would send the motions that make up the drag to the source as further drags instead of
	// to the session.
	s.capture = nil
	clear(s.buttons)
	s.dragMoved(s.pointer, s.lastMods)
}

// cloneDragData returns a private copy of the data a drag is carrying. Like bytes.Clone, it returns nil for nil, so
// that a result which never carried anything — the zero value LastDrag() promises before any drag has happened — stays
// the zero value after it has been copied.
func cloneDragData(data []drag.Data) []drag.Data {
	if data == nil {
		return nil
	}
	result := make([]drag.Data, 0, len(data))
	for _, one := range data {
		result = append(result, drag.Data{Type: one.Type, Data: bytes.Clone(one.Data)})
	}
	return result
}

// dragMoved routes a pointer motion to the active drag session, entering and leaving windows as the pointer crosses
// them and asking whichever one it is over what it would do with a drop here.
func (s *headlessState) dragMoved(pt geom.Point, mods mod.Modifiers) {
	s.pointer = pt
	s.lastMods = mods
	session := s.drag
	target := s.eligibleWindowAt(pt)
	if target == session.target {
		if target != nil {
			session.lastOp = target.dragUpdate(session.info, windowLocal(target, pt), mods)
		}
		return
	}
	if previous := session.target; previous != nil && previous.IsValid() {
		previous.dragExit()
	}
	session.target = target
	session.lastOp = drag.None
	if target != nil {
		session.lastOp = target.dragEntered(session.info, windowLocal(target, pt), mods)
	}
}

// dragWheel forwards a wheel rotation to the window of this application under the pointer, so that a window which
// scrolls its content keeps working while the pointer sits over it holding a drag — whether or not that window can
// receive the drag, since scrolling only moves a view. That is what the Linux backend does when a wheel button arrives
// during its drag loop, delivering it to whichever of the application's windows is under the pointer, and what the
// Windows one does from its drag scroll hook; a wheel over no window of this application at all is dropped, as it is
// there. When the window is the drop target, the drop feedback is then recomputed at the very same point, since the
// content the pointer is over has moved out from under it.
func (s *headlessState) dragWheel(pt, delta geom.Point, mods mod.Modifiers) {
	s.pointer = pt
	s.lastMods = mods
	w := s.windowAt(pt)
	if w == nil {
		return
	}
	local := windowLocal(w, pt)
	w.mouseWheel(local, delta, mods)
	if session := s.drag; session != nil && session.target == w && w.IsValid() {
		session.lastOp = w.dragUpdate(session.info, local, mods)
	}
}

// dropDrag ends the session with the release of the mouse button. The target is offered the drop only if it said it
// would do something with one here; otherwise the release is the end of a drag it had already declined, and it gets
// the exit it would have received had the pointer left it.
func (s *headlessState) dropDrag(pt geom.Point, mods mod.Modifiers) {
	session := s.drag
	s.pointer = pt
	s.lastMods = mods
	result := HeadlessDragResult{
		Source: session.source,
		Target: session.target,
		Data:   session.info.data,
		Image:  session.img,
		Origin: session.origin,
		Op:     session.lastOp,
	}
	if target := session.target; target != nil && target.IsValid() {
		if session.lastOp == drag.None {
			target.dragExit()
		} else {
			result.Dropped = true
			result.Handled = target.drop(session.info, windowLocal(target, pt), mods)
		}
	}
	s.finishDrag(result)
}

// cancelDrag abandons the session, which is what the Escape key does on every platform.
func (s *headlessState) cancelDrag() {
	session := s.drag
	if target := session.target; target != nil && target.IsValid() {
		target.dragExit()
	}
	s.finishDrag(HeadlessDragResult{
		Source:   session.source,
		Target:   session.target,
		Data:     session.info.data,
		Image:    session.img,
		Origin:   session.origin,
		Canceled: true,
	})
}

// finishDrag retires the session and records how it ended.
func (s *headlessState) finishDrag(result HeadlessDragResult) {
	// Copied, both the list and the bytes in it, for the same reason beginDrag copies what it was given: the data a
	// caller takes away must not be the very storage the session handed to the target callbacks, since a target that
	// kept its drag.Info would otherwise see every change made to the result, and the result would see every change
	// made through the info.
	result.Data = cloneDragData(result.Data)
	s.drag = nil
	s.lastDrag = result
	// The release that ends a drag is consumed by the drag itself on every platform, and StartDrag synthesized the
	// source window's mouse up before the drag even began, so there is no press left anywhere to account for.
	s.capture = nil
	clear(s.buttons)
	// With the pointer no longer owned by the drag, it is once again simply somewhere, and the window it is over gets
	// the entry it would have had all along.
	s.updateHover(s.pointer, s.lastMods)
}

// dragWindowDestroyed keeps a session from holding on to a window that is being destroyed.
func (s *headlessState) dragWindowDestroyed(w *Window) {
	session := s.drag
	if session == nil {
		return
	}
	if session.target == w {
		// No exit callback: the panels that would receive it are going away with the window.
		session.target = nil
		session.lastOp = drag.None
	}
	if session.source == w {
		// Losing the source ends the drag. For a drag this application started, that is also what lets the nested loop
		// in startDrag unwind, since it spins for as long as the session is there.
		s.cancelDrag()
	}
}

// eligibleWindowAt returns the window at pt that may receive the active drag, or nil if the one there may not.
//
// Only the window on top at that point is ever considered: a window that is not a target for this drag does not let it
// through to whatever is behind it, any more than an unrelated application's window would. Whether it is a target is
// decided by what it registered with RegisterForDragTypes, mirroring AppKit's registerForDraggedTypes — a window that
// registered nothing never sees a drag event. The types are matched by conformance in either direction rather than
// exactly, which is the same test the clipboard applies in selectDataType and the same one headlessDragInfo applies
// when the target's panels ask what the drag carries.
func (s *headlessState) eligibleWindowAt(pt geom.Point) *Window {
	w := s.windowAt(pt)
	if w == nil {
		return nil
	}
	hw := headlessWindowFor(w)
	if hw == nil {
		return nil
	}
	for _, registered := range hw.dragTypes {
		if registered != nil && s.drag.info.HasDataType(registered.UTI) {
			return w
		}
	}
	return nil
}
