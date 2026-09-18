// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package dbus

import "fmt"

// The standard D-Bus error names that Unison either returns from its own objects or recognizes in replies.
const (
	// UnknownObject is returned when no object exists at the requested path.
	UnknownObject = "org.freedesktop.DBus.Error.UnknownObject"
	// UnknownInterface is returned when an object does not implement the requested interface.
	UnknownInterface = "org.freedesktop.DBus.Error.UnknownInterface"
	// UnknownMethod is returned when an interface does not have the requested method.
	UnknownMethod = "org.freedesktop.DBus.Error.UnknownMethod"
	// UnknownProperty is returned when an interface does not have the requested property.
	UnknownProperty = "org.freedesktop.DBus.Error.UnknownProperty"
	// InvalidArgs is returned when the arguments of a call are the wrong number, type or value.
	InvalidArgs = "org.freedesktop.DBus.Error.InvalidArgs"
	// PropertyReadOnly is returned when an attempt is made to set a property that cannot be written.
	PropertyReadOnly = "org.freedesktop.DBus.Error.PropertyReadOnly"
	// NotSupported is returned when a request is understood but deliberately not implemented.
	NotSupported = "org.freedesktop.DBus.Error.NotSupported"
	// Failed is the generic error name, used when nothing more specific applies.
	Failed = "org.freedesktop.DBus.Error.Failed"
	// ServiceUnknown is returned by the bus when the destination of a call is not available.
	ServiceUnknown = "org.freedesktop.DBus.Error.ServiceUnknown"
)

// Error is a D-Bus error: a name, which is the machine-readable part that callers should test against, plus an optional
// human-readable message.
type Error struct {
	Name    string
	Message string
}

// Errorf creates a new [Error] with the given name and a message built from the format and args.
func Errorf(name, format string, args ...any) *Error {
	return &Error{Name: name, Message: fmt.Sprintf(format, args...)}
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Message == "" {
		return e.Name
	}
	return e.Name + ": " + e.Message
}
