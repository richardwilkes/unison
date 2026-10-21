// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"crypto/rand"
	"errors"
	"os"
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
	"golang.org/x/sys/windows"
)

const liveTargetPrefix = "com.trollworks.unison.secrets.test:"

// TestCredentialWLayout verifies that credentialW matches the SDK's CREDENTIALW layout on 64-bit Windows.
func TestCredentialWLayout(t *testing.T) {
	c := check.New(t)
	var cw credentialW
	c.Equal(uintptr(80), unsafe.Sizeof(cw))
	c.Equal(uintptr(0), unsafe.Offsetof(cw.Flags))
	c.Equal(uintptr(4), unsafe.Offsetof(cw.Type))
	c.Equal(uintptr(8), unsafe.Offsetof(cw.TargetName))
	c.Equal(uintptr(16), unsafe.Offsetof(cw.Comment))
	c.Equal(uintptr(24), unsafe.Offsetof(cw.LastWritten))
	c.Equal(uintptr(32), unsafe.Offsetof(cw.CredentialBlobSize))
	c.Equal(uintptr(40), unsafe.Offsetof(cw.CredentialBlob))
	c.Equal(uintptr(48), unsafe.Offsetof(cw.Persist))
	c.Equal(uintptr(52), unsafe.Offsetof(cw.AttributeCount))
	c.Equal(uintptr(56), unsafe.Offsetof(cw.Attributes))
	c.Equal(uintptr(64), unsafe.Offsetof(cw.TargetAlias))
	c.Equal(uintptr(72), unsafe.Offsetof(cw.UserName))
}

// TestCredResult verifies that ERROR_NOT_FOUND and ERROR_NO_SUCH_LOGON_SESSION survive as themselves for errors.Is,
// while other failures do not.
func TestCredResult(t *testing.T) {
	c := check.New(t)
	c.NoError(credResult("CredReadW", 1, windows.ERROR_NOT_FOUND))
	err := credResult("CredReadW", 0, windows.ERROR_NOT_FOUND)
	c.True(errors.Is(err, windows.ERROR_NOT_FOUND))
	c.False(errors.Is(err, windows.ERROR_NO_SUCH_LOGON_SESSION))
	err = credResult("CredWriteW", 0, windows.ERROR_NO_SUCH_LOGON_SESSION)
	c.True(errors.Is(err, windows.ERROR_NO_SUCH_LOGON_SESSION))
	c.False(errors.Is(err, windows.ERROR_NOT_FOUND))
	err = credResult("CredReadW", 0, windows.ERROR_ACCESS_DENIED)
	c.HasError(err)
	c.False(errors.Is(err, windows.ERROR_NOT_FOUND))
	c.False(errors.Is(err, windows.ERROR_NO_SUCH_LOGON_SESSION))
}

// TestCredentialLiveRoundTrip exercises the real Credential Manager, so it only runs when UNISON_SECRETS_LIVE is set.
func TestCredentialLiveRoundTrip(t *testing.T) {
	if os.Getenv("UNISON_SECRETS_LIVE") == "" {
		t.Skip("set UNISON_SECRETS_LIVE to run this test against the Windows Credential Manager")
	}
	c := check.New(t)
	account := rand.Text()
	target := liveTargetPrefix + account
	t.Cleanup(func() {
		_ = CredDeleteW(target, CredTypeGeneric) //nolint:errcheck // The credential is usually gone already.
	})
	payload := []byte("unison credential round trip")
	c.NoError(CredWriteW(&Credential{
		TargetName: target,
		UserName:   account,
		Blob:       payload,
		Type:       CredTypeGeneric,
		Persist:    CredPersistLocalMachine,
	}))

	got, err := CredReadW(target, CredTypeGeneric)
	c.NoError(err)
	if got == nil {
		t.FailNow()
	}
	c.Equal(target, got.TargetName)
	c.Equal(account, got.UserName)
	c.Equal(payload, got.Blob)
	c.Equal(uint32(CredTypeGeneric), got.Type)
	c.Equal(uint32(CredPersistLocalMachine), got.Persist)

	list, err := CredEnumerateW(liveTargetPrefix + "*")
	c.NoError(err)
	found := false
	for _, one := range list {
		if one.TargetName == target {
			found = true
			c.Equal(account, one.UserName)
			c.Equal(payload, one.Blob)
		}
	}
	c.True(found)

	c.NoError(CredDeleteW(target, CredTypeGeneric))
	_, err = CredReadW(target, CredTypeGeneric)
	c.True(errors.Is(err, windows.ERROR_NOT_FOUND))
	c.True(errors.Is(CredDeleteW(target, CredTypeGeneric), windows.ERROR_NOT_FOUND))
}
