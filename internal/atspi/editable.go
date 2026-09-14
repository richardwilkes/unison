// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package atspi

import (
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/dbus"
)

// editableTextInterface returns the org.a11y.atspi.EditableText interface of a node whose text can be changed, which is
// the only way AT-SPI has of putting characters into a control: org.a11y.atspi.Text moves the caret and the selection
// but never alters a character, and [States] claims ATSPI_STATE_EDITABLE for exactly the nodes this is given to.
// Without it an assistive technology can read a Unison field and move about inside it but can never edit it, while the
// same field is writable through VoiceOver and through UI Automation's value and text patterns.
//
// Every offset is a rune index, which is what AT-SPI calls a character, and every answer is optimistic in the same way
// the rest of the package's requests are: the change has been handed to the user interface thread, and the control
// still holds what it held until the next snapshot says otherwise.
//
// The three clipboard methods report that they did nothing. AT-SPI defines them as acting on the desktop's clipboard
// rather than on the text, which is not something the schema carries a request for, and a cut that deleted the
// selection without putting it anywhere would lose what the user asked to move.
func (o *nodeObject) editableTextInterface() *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceEditableText,
		Methods: []*dbus.Method{
			{Name: "SetTextContents", In: "s", Out: "b", Handle: o.setTextContents},
			{Name: "InsertText", In: "isi", Out: "b", Handle: o.insertText},
			{Name: "CopyText", In: "ii", Handle: o.copyText},
			{Name: "CutText", In: "ii", Out: "b", Handle: replyFalse},
			{Name: "DeleteText", In: "ii", Out: "b", Handle: o.deleteText},
			{Name: "PasteText", In: "i", Out: "b", Handle: replyFalse},
		},
	}
}

// setTextContents implements org.a11y.atspi.EditableText.SetTextContents, which replaces the whole content. It is the
// one editing method that is a value change rather than an edit of a range, which is how the widgets themselves take
// it: a control with no notion of ranges, such as a spin button, still knows what to do with a new value.
func (o *nodeObject) setTextContents(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	if !o.node.Actions.Has(accessibility.SetValue) {
		call.Reply(o.replaceRunes(0, len(o.textRunes()), stringArg(args, 0)))
		return
	}
	call.Reply(o.a.dispatch(accessibility.ActionRequest{
		Node:   o.node.ID,
		Action: accessibility.SetValue,
		Value:  stringArg(args, 0),
	}))
}

// insertText implements org.a11y.atspi.EditableText.InsertText, which is an empty range replaced by the text. length is
// how many characters of the string the caller means, which libatspi fills in with the whole of it; a shorter count
// takes that many runes.
func (o *nodeObject) insertText(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	at := int(int32Arg(args, 0))
	runes := []rune(stringArg(args, 1))
	if length := int(int32Arg(args, 2)); length >= 0 && length < len(runes) {
		runes = runes[:length]
	}
	call.Reply(o.replaceRunes(at, at, string(runes)))
}

// deleteText implements org.a11y.atspi.EditableText.DeleteText, which is a range replaced by nothing.
func (o *nodeObject) deleteText(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	call.Reply(o.replaceRunes(int(int32Arg(args, 0)), int(int32Arg(args, 1)), ""))
}

// copyText implements org.a11y.atspi.EditableText.CopyText, which is the one method of the interface that answers with
// nothing at all, so there is no way to say that it did nothing either.
func (o *nodeObject) copyText(call *dbus.Call) {
	if _, ok := callArgs(call); !ok {
		return
	}
	call.Reply()
}

// replaceRunes asks the widget to put text in place of a range of the runes it holds, reporting whether the request was
// accepted rather than whether it has happened. A control that does not offer to have its text replaced says no. The
// range is brought within the content, and an end before its start is read as the empty range at that point, which is
// what an insertion is.
func (o *nodeObject) replaceRunes(start, end int, text string) bool {
	if !o.node.Actions.Has(accessibility.ReplaceText) {
		return false
	}
	count := len(o.textRunes())
	start = clamp(start, 0, count)
	return o.a.dispatch(accessibility.ActionRequest{
		Node:   o.node.ID,
		Action: accessibility.ReplaceText,
		Value:  text,
		Start:  start,
		End:    clamp(end, start, count),
	})
}

// stringArg returns one argument of a call as a string. An empty string stands in for an argument that is not there or
// is not the type the method's signature promised, neither of which the connection lets through.
func stringArg(args []any, i int) string {
	if i >= len(args) {
		return ""
	}
	value, ok := args[i].(string)
	if !ok {
		return ""
	}
	return value
}
