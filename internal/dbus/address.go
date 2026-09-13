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
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// transport identifies one endpoint that a client may connect to. Only Unix domain sockets are supported, so network is
// always "unix". address is either a filesystem path or, when it begins with "@", a name in the abstract socket
// namespace, which only Linux implements: dialing one elsewhere fails at runtime rather than being rejected here, since
// address strings are parsed on every platform but only ever come from a Linux desktop session.
type transport struct {
	network string
	address string
}

// parseAddress parses a D-Bus server address, which is a ";" separated list of alternatives that should be tried in
// order, each of the form "transport:key=value,key=value". Only the "unix" transport is supported, via its "path",
// "abstract" and "runtime" keys; unsupported alternatives are skipped, and an error that explains why is returned only
// if nothing usable is left. Unknown keys are ignored, as the specification requires. Surrounding whitespace is
// stripped from each alternative, since an address that reaches us from the environment often has a space after a
// separator or a newline at the end, and carrying either into the parse turns a usable alternative into an unknown
// transport or a socket path into one that no socket could have.
func parseAddress(addr string) ([]transport, error) {
	var (
		list    []transport
		skipped []error
	)
	for entry := range strings.SplitSeq(addr, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		t, err := parseAddressEntry(entry)
		if err != nil {
			skipped = append(skipped, err)
			continue
		}
		list = append(list, t)
	}
	if len(list) == 0 {
		if len(skipped) != 0 {
			return nil, errors.Join(skipped...)
		}
		return nil, fmt.Errorf("dbus: %q contains no address", addr)
	}
	return list, nil
}

// parseAddressEntry parses a single alternative from within a D-Bus server address.
func parseAddressEntry(entry string) (transport, error) {
	kind, rest, found := strings.Cut(entry, ":")
	if !found {
		return transport{}, fmt.Errorf("dbus: address %q does not name a transport", entry)
	}
	if kind != "unix" {
		return transport{}, fmt.Errorf("dbus: the %q transport in address %q is not supported; only unix sockets are",
			kind, entry)
	}
	var t transport
	for pair := range strings.SplitSeq(rest, ",") {
		if pair == "" {
			continue
		}
		key, escaped, ok := strings.Cut(pair, "=")
		if !ok {
			return transport{}, fmt.Errorf("dbus: address %q has the malformed key/value pair %q", entry, pair)
		}
		value, err := unescapeAddressValue(escaped)
		if err != nil {
			return transport{}, err
		}
		var resolved string
		switch key {
		case "path":
			// An empty value is rejected rather than ignored, since leaving it out of the socket count would both
			// accept "unix:path=,abstract=x" as an abstract socket and blame "unix:path=" for having no key at all.
			if value == "" {
				return transport{}, fmt.Errorf("dbus: address %q has an empty path", entry)
			}
			resolved = value
		case "abstract":
			if value == "" {
				return transport{}, fmt.Errorf("dbus: address %q has an empty abstract name", entry)
			}
			resolved = "@" + value // The leading "@" selects the Linux abstract socket namespace
		case "runtime":
			if value != "yes" {
				return transport{}, fmt.Errorf("dbus: address %q has an unsupported runtime value %q", entry, value)
			}
			resolved = runtimeBusPath()
		default: // Unknown keys, such as "guid", are ignored
			continue
		}
		if t.address != "" {
			return transport{}, fmt.Errorf("dbus: address %q specifies more than one socket", entry)
		}
		t.network = "unix"
		t.address = resolved
	}
	if t.address == "" {
		return transport{}, fmt.Errorf("dbus: address %q has no path, abstract or runtime key", entry)
	}
	return t, nil
}

// addressSafeBytes are the punctuation bytes that a D-Bus address value may contain without being escaped, in addition
// to the alphanumerics and the underscore that [isNameChar] accepts. Every other byte is written as "%" followed by two
// hexadecimal digits.
const addressSafeBytes = "-/.\\*"

// unescapeAddressValue undoes the escaping that [escapeAddressValue] applies. Bytes that did not need to be escaped are
// accepted as they are, since that is what other implementations produce.
func unescapeAddressValue(value string) (string, error) {
	if !strings.Contains(value, "%") {
		return value, nil
	}
	var sb strings.Builder
	sb.Grow(len(value))
	for i := 0; i < len(value); i++ {
		if value[i] != '%' {
			sb.WriteByte(value[i])
			continue
		}
		if i+2 >= len(value) {
			return "", fmt.Errorf("dbus: address value %q ends with an incomplete escape", value)
		}
		b, err := strconv.ParseUint(value[i+1:i+3], 16, 8)
		if err != nil {
			return "", fmt.Errorf("dbus: address value %q contains the invalid escape %q", value, value[i:i+3])
		}
		sb.WriteByte(byte(b))
		i += 2
	}
	return sb.String(), nil
}

// escapeAddressValue escapes a value so that it may be used in an address string.
func escapeAddressValue(value string) string {
	var sb strings.Builder
	sb.Grow(len(value))
	for i := 0; i < len(value); i++ {
		if ch := value[i]; isNameChar(ch) || strings.IndexByte(addressSafeBytes, ch) != -1 {
			sb.WriteByte(ch)
		} else {
			fmt.Fprintf(&sb, "%%%02x", ch)
		}
	}
	return sb.String()
}
