// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package demo

import (
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/secrets"
)

// TestNewDemoSecretsWindowReusesExistingWindow verifies that, with a secrets window recorded, the call returns it
// rather than creating a new one. A zero-value Window reports itself as invalid, so the ToFront on the reuse path is a
// safe no-op in this headless test. This test mutates the package-level secretsWindow and therefore must not call
// t.Parallel.
func TestNewDemoSecretsWindowReusesExistingWindow(t *testing.T) {
	c := check.New(t)
	sentinel := &unison.Window{}
	secretsWindow = sentinel
	t.Cleanup(func() { secretsWindow = nil })
	wnd, err := NewDemoSecretsWindow()
	c.NoError(err)
	c.True(wnd == sentinel, "an existing secrets window must be returned instead of creating a new one")
}

// TestTrackSecretsWindowClearsRecordOnClose verifies the wiring NewDemoSecretsWindow applies to a freshly created
// window: the window is recorded as the singleton and its WillCloseCallback (invoked by Window.Dispose) clears the
// record so a subsequent call can create a replacement. This test mutates the package-level secretsWindow and therefore
// must not call t.Parallel.
func TestTrackSecretsWindowClearsRecordOnClose(t *testing.T) {
	c := check.New(t)
	t.Cleanup(func() { secretsWindow = nil })
	wnd := &unison.Window{}
	trackSecretsWindow(wnd)
	c.True(secretsWindow == wnd, "the created window must be recorded as the singleton")
	c.NotNil(wnd.WillCloseCallback, "closing the window must be observable so the record can be cleared")
	wnd.WillCloseCallback()
	c.Nil(secretsWindow, "closing the window must clear the singleton record")
}

// TestSecretsOperationsReportOutcome checks the status text each button shows, against an in-memory store so the
// user's real store is never touched. It installs a store for the whole package and therefore must not call
// t.Parallel.
func TestSecretsOperationsReportOutcome(t *testing.T) {
	c := check.New(t)
	t.Cleanup(secrets.UseStore(secrets.NewMemoryStore()))
	const (
		service = "com.trollworks.unison.example"
		alice   = "alice"
		bob     = "bob"
		carol   = "carol"
	)

	c.Equal("No accounts", listAccounts(service, "", ""))
	c.True(strings.HasPrefix(fetchSecret(service, alice, ""), notFoundPrefix))
	c.Equal("Stored 7 bytes", storeSecret(service, alice, "hunter2"))
	c.Equal(`"hunter2" (7 bytes)`, fetchSecret(service, alice, ""))
	c.Equal("Stored 1 byte", storeSecret(service, bob, "x"))
	c.Equal("Stored 0 bytes", storeSecret(service, carol, ""))
	c.Equal(`"" (0 bytes)`, fetchSecret(service, carol, ""))
	c.Equal("alice, bob, carol", listAccounts(service, "", ""))
	c.Equal("Deleted", deleteSecret(service, alice, ""))
	c.True(strings.HasPrefix(deleteSecret(service, alice, ""), notFoundPrefix))
	c.Equal("bob, carol", listAccounts(service, "", ""))

	// A validation error carries no sentinel, and its text must not include the stack trace errs appends.
	text := storeSecret(service, "", "x")
	c.False(strings.HasPrefix(text, notFoundPrefix) || strings.HasPrefix(text, unavailablePrefix), text)
	c.False(strings.Contains(text, "\n"), text)
}

// TestSecretsErrorTextNamesSentinels checks the prefix and the stack-free message for each kind of failure.
func TestSecretsErrorTextNamesSentinels(t *testing.T) {
	c := check.New(t)
	c.Equal(notFoundPrefix+"no such item", secretsErrorText(errs.NewWithCause("no such item", secrets.ErrNotFound)))
	c.Equal(unavailablePrefix+"no bus", secretsErrorText(errs.NewWithCause("no bus", secrets.ErrUnavailable)))
	c.Equal(notFoundPrefix+secrets.ErrNotFound.Error(), secretsErrorText(secrets.ErrNotFound))
	c.Equal("access denied", secretsErrorText(errs.New("access denied")))
}
