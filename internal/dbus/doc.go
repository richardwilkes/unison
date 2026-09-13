// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// Package dbus implements the subset of the D-Bus protocol
// (https://dbus.freedesktop.org/doc/dbus-specification.html) that Unison needs in order to speak to the desktop
// session bus and to the AT-SPI2 accessibility bus. It is pure Go, has no third-party dependencies, and is portable:
// the wire format has no operating system dependencies at all, so it is developed and tested everywhere even though
// only Linux makes use of it.
//
// # Wire format
//
// Only the little-endian encoding is produced, since every platform Unison runs on is little-endian, but either
// encoding is accepted: the specification lets each peer choose, so a big-endian peer may turn up on any bus. A
// big-endian message is converted as it is decoded, which means a [Message] always holds the little-endian encoding of
// its body no matter where it came from. Unix file descriptor passing (type code 'h') is not supported and is rejected
// as an error wherever it appears.
//
// The mapping between D-Bus types and Go types is:
//
//	y  byte                      → byte
//	b  boolean                   → bool
//	n  int16                     → int16
//	q  uint16                    → uint16
//	i  int32                     → int32
//	u  uint32                    → uint32
//	x  int64                     → int64
//	t  uint64                    → uint64
//	d  double                    → float64
//	s  string                    → string
//	o  object path               → [ObjectPath]
//	g  signature                 → [Signature]
//	v  variant                   → [Variant]
//	a… array                     → []any, with fast paths for []byte, []string, []int32, []uint32, []ObjectPath and
//	                               []ObjectRef
//	a{…} dictionary              → [Dict] (an ordered slice of [DictEntry])
//	(…)  structure               → [Struct], except that (so) becomes [ObjectRef]
//
// Marshaling is driven by a [Signature] rather than by the Go types, so it is more lenient than the list above: any
// Go integer type is accepted for any integer type code as long as the value fits, any slice type is accepted for an
// array, a Go map is accepted for a dictionary (its entries are sorted by key so that the encoding is deterministic),
// [Struct] and []any are interchangeable, and a bare value is accepted for a variant if its signature can be derived
// from it (see [SignatureOf]). Unmarshaling always produces the canonical Go types listed above.
//
// # Limits
//
// The specification's limits are enforced: a message may not exceed [MaxMessageSize] bytes, the marshaled body of an
// array may not exceed [MaxArraySize] bytes, a signature may not exceed [MaxSignatureLength] bytes, and arrays,
// structures and variants may not each nest more than [MaxDepth] deep. Decoding is strict about the rest of the format
// as well: alignment padding must be NUL, strings must be valid UTF-8 and NUL terminated, an array's elements must end
// exactly where its length says they do, and a header field may not appear twice.
//
// # Connections
//
// [Dial] connects to a bus over a Unix domain socket, authenticates with the SASL EXTERNAL mechanism and performs the
// Hello handshake, while [Session] does the same for the desktop session bus and hands out the one connection that the
// whole process shares. A [Conn] makes calls, emits signals, routes the signals it receives to the handlers registered
// with [Conn.Subscribe], and answers the calls made to the objects published with [Conn.Export] and
// [Conn.ExportSubtree]; see [Conn] for the goroutines involved and what that requires of a handler, and [Object] for
// what an exported object looks like.
//
// Only Unix domain sockets are supported, since that is all any desktop bus uses. Addresses in the abstract socket
// namespace (unix:abstract=...) parse everywhere but can only be connected to on Linux, which is the only system that
// implements that namespace.
package dbus
