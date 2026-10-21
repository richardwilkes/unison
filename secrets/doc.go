// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// Package secrets keeps small secrets, such as passwords, tokens and keys, in the operating system's credential store,
// where each one is identified by a service and an account.
//
// The store behind each platform is:
//   - macOS: the Keychain, which holds each secret as a generic password. A new secret goes into the user's default
//     keychain, normally the login keychain. A call that finds no keychain to use, as for an account without a default
//     keychain, returns an error matching [ErrUnavailable].
//   - Windows: the Credential Manager, which holds each secret as a generic credential named "service:account". A
//     logon session without a credential set, as a network logon is, makes every call return an error matching
//     [ErrUnavailable].
//   - Linux: the freedesktop Secret Service (GNOME Keyring, KWallet and others), reached over the session D-Bus. When
//     there is no session bus or nothing provides the Secret Service, every call returns an error matching
//     [ErrUnavailable].
//
// A service or account must be non-empty, valid UTF-8 and free of NUL characters, and a service must not contain a
// colon, since Windows joins the two with one. An account may contain colons. Windows refuses a secret larger than 2560
// bytes, so portable code stays within that. On Windows, and in a [MemoryStore], a service or account that differs from
// another only in letter case names the same secret, so portable code must not rely on case alone to tell secrets
// apart. Windows also refuses a service and account whose joined "service:account" name is longer than 32767 UTF-16
// code units.
//
// Every call may block, and the platform store may put up a dialog, such as a Keychain access prompt or a request to
// unlock the keyring, so call off the UI thread whenever a user is waiting.
//
// On macOS, a Keychain item trusts the code identity of the program that created it. A binary that is unsigned or
// ad-hoc signed, as those from go build and go test are, gets a new identity each time it is rebuilt, so a rebuilt
// program is prompted for the items an earlier build created, or refused when no prompt can be shown. A program that
// ships needs a stable signing identity.
//
// A headless unison session installs a fresh [MemoryStore], which ignores letter case as Windows does, and leaves it
// installed after the session ends, until the next session replaces it, so a session never touches the user's real
// store, not even from a goroutine that outlives it. [UseStore] installs any other [Store]. The package functions use
// the newest store still installed, so a store a test installs before a session starts goes unused from then on, while
// one installed after the session started is used until it is removed or the next session starts.
//
// This package never imports the root unison package, so command-line tools can use it without that package's
// initialization, and a test enforces that.
package secrets
