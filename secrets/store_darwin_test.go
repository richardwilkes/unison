// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package secrets

import (
	"crypto/rand"
	"errors"
	"os"
	"slices"
	"strconv"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/cocoa"
)

func TestMapKeychainError(t *testing.T) {
	c := check.New(t)
	c.NoError(mapKeychainError(nil, "svc", "acct"))

	err := mapKeychainError(errs.NewWithCause("SecItemCopyMatching failed", cocoa.ErrSecItemNotFound), "svc", "acct")
	c.True(errors.Is(err, ErrNotFound))
	c.Contains(err.Error(), `"svc"`)
	c.Contains(err.Error(), `"acct"`)

	for _, status := range []cocoa.OSStatus{
		cocoa.ErrSecAuthFailed,
		cocoa.ErrSecInteractionNotAllowed,
	} {
		original := errs.NewWithCause("SecItemCopyMatching failed", status)
		err = mapKeychainError(original, "svc", "acct")
		c.Equal(error(original), err)
		c.True(errors.Is(err, status))
		c.False(errors.Is(err, ErrNotFound))
		c.False(errors.Is(err, ErrUnavailable))
		c.False(errors.Is(err, ErrPromptDismissed))
	}

	err = mapKeychainError(errs.NewWithCause("SecItemCopyMatching failed", cocoa.ErrSecUserCanceled), "svc", "acct")
	c.True(errors.Is(err, ErrPromptDismissed), "status %d: %v", cocoa.ErrSecUserCanceled, err)
	c.False(errors.Is(err, ErrNotFound))
	c.False(errors.Is(err, ErrUnavailable))

	for _, status := range []cocoa.OSStatus{
		cocoa.ErrSecNotAvailable,
		cocoa.ErrSecNoSuchKeychain,
		cocoa.ErrSecNoDefaultKeychain,
	} {
		err = mapKeychainError(errs.NewWithCause("SecItemCopyMatching failed", status), "svc", "acct")
		c.True(errors.Is(err, ErrUnavailable), "status %d: %v", status, err)
		c.False(errors.Is(err, ErrNotFound))
		c.Contains(err.Error(), strconv.Itoa(int(status)))
	}
}

// TestMapKeychainUnavailable covers the mapping Set and Accounts apply, which leaves a missing item alone.
func TestMapKeychainUnavailable(t *testing.T) {
	c := check.New(t)
	c.NoError(mapKeychainUnavailable(nil))

	for _, status := range []cocoa.OSStatus{
		cocoa.ErrSecItemNotFound,
		cocoa.ErrSecDuplicateItem,
		cocoa.ErrSecAuthFailed,
		cocoa.ErrSecInteractionNotAllowed,
	} {
		original := errs.NewWithCause("SecItemAdd failed", status)
		c.Equal(error(original), mapKeychainUnavailable(original))
	}

	canceled := errs.NewWithCause("SecItemAdd failed", cocoa.ErrSecUserCanceled)
	canceledErr := mapKeychainUnavailable(canceled)
	c.True(errors.Is(canceledErr, ErrPromptDismissed), "status %d: %v", cocoa.ErrSecUserCanceled, canceledErr)
	c.False(errors.Is(canceledErr, ErrUnavailable))

	for _, status := range []cocoa.OSStatus{
		cocoa.ErrSecNotAvailable,
		cocoa.ErrSecNoSuchKeychain,
		cocoa.ErrSecNoDefaultKeychain,
	} {
		err := mapKeychainUnavailable(errs.NewWithCause("SecItemAdd failed", status))
		c.True(errors.Is(err, ErrUnavailable), "status %d: %v", status, err)
		c.False(errors.Is(err, ErrNotFound))
		c.Contains(err.Error(), "SecItemAdd failed")
		c.Contains(err.Error(), strconv.Itoa(int(status)))
	}
}

// TestKeychainStoreLiveRoundTrip exercises the package functions against the user's keychain. Since it writes to the
// user's keychain, it only runs when UNISON_SECRETS_LIVE is set.
func TestKeychainStoreLiveRoundTrip(t *testing.T) {
	if os.Getenv("UNISON_SECRETS_LIVE") == "" {
		t.Skip("set UNISON_SECRETS_LIVE to run this test against the user's keychain")
	}
	c := check.New(t)
	t.Cleanup(UseStore(nil))
	const service = "com.trollworks.unison.secrets.test"
	account := "account-" + rand.Text()
	t.Cleanup(func() {
		_ = keychainStore{}.Delete(service, account) //nolint:errcheck // The item is usually gone already.
	})

	for _, payload := range [][]byte{[]byte("first value"), {0, 1, 2, 253, 254, 255}, nil} {
		c.NoError(Set(service, account, payload))
		got, err := Get(service, account)
		c.NoError(err)
		c.NotNil(got)
		c.Equal(len(payload), len(got))
		c.True(slices.Equal(payload, got), "got %v, want %v", got, payload)
	}

	accounts, err := Accounts(service)
	c.NoError(err)
	c.True(slices.Contains(accounts, account), "accounts %v lack %q", accounts, account)

	c.NoError(Delete(service, account))
	got, err := Get(service, account)
	c.True(errors.Is(err, ErrNotFound), "Get after Delete: %v", err)
	c.Nil(got)
	err = Delete(service, account)
	c.True(errors.Is(err, ErrNotFound), "second Delete: %v", err)
	accounts, err = Accounts(service)
	c.NoError(err)
	c.False(slices.Contains(accounts, account), "accounts %v still hold %q", accounts, account)
}
