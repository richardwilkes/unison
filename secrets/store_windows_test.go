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
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

func TestTargetName(t *testing.T) {
	c := check.New(t)
	c.Equal("svc:acct", targetName("svc", "acct"))
	c.Equal("svc:acct:work", targetName("svc", "acct:work"))
}

func TestAccountsFrom(t *testing.T) {
	c := check.New(t)
	generic := func(target, user string) *w32.Credential {
		return &w32.Credential{TargetName: target, UserName: user, Type: w32.CredTypeGeneric}
	}
	c.Nil(accountsFrom("svc", nil))
	c.Nil(accountsFrom("svc", []*w32.Credential{
		generic("svc2:bob", "bob"), generic("svcbob", "bob"),
		generic("svc:", "carol"),
	}))

	domain := generic("svc:dave", "dave")
	domain.Type = 2 // CRED_TYPE_DOMAIN_PASSWORD
	long := strings.Repeat("a", w32.CredMaxUsernameLength+1)
	c.Equal([]string{long, "alice", "bob", "carol:work", "erin", "grace"}, accountsFrom("svc", []*w32.Credential{
		generic("svc:carol:work", "carol:work"),
		generic("svc2:frank", "frank"), // another service sharing the prefix
		generic("svc:bob", "bob"),
		generic("svc:grace", "heidi"), // the account comes from the TargetName, not the UserName
		nil,
		domain,
		generic("svc:", ""),
		generic("svc:alice", "alice"),
		generic("SVC:erin", "erin"), // Windows ignores the case of a TargetName
		generic("svc:bob", "bob"),
		newCredential("svc", long, nil), // too long for a UserName
	}))
}

func TestAccountsFromIgnoresCase(t *testing.T) {
	c := check.New(t)
	// Windows looks up a TargetName without regard to case, so Get("svc", "bob") reads what Set("SVC", "bob") wrote.
	c.Equal([]string{"bob"}, accountsFrom("svc", []*w32.Credential{newCredential("SVC", "bob", nil)}))
	c.Equal([]string{"Bob"}, accountsFrom("SVC", []*w32.Credential{newCredential("svc", "Bob", nil)}))
}

func TestNewCredential(t *testing.T) {
	c := check.New(t)
	cred := newCredential("svc", "bob", []byte("x"))
	c.Equal("svc:bob", cred.TargetName)
	c.Equal("bob", cred.UserName)
	c.Equal([]byte("x"), cred.Blob)
	c.Equal(uint32(w32.CredTypeGeneric), cred.Type)
	c.Equal(uint32(w32.CredPersistLocalMachine), cred.Persist)

	// The UserName limit counts UTF-16 code units, not bytes or runes.
	for _, one := range []struct {
		account string
		kept    bool
	}{
		{strings.Repeat("a", w32.CredMaxUsernameLength), true},
		{strings.Repeat("a", w32.CredMaxUsernameLength+1), false},
		{strings.Repeat("\u00e9", w32.CredMaxUsernameLength), true},
		{strings.Repeat("\U0001F600", (w32.CredMaxUsernameLength+1)/2), false},
	} {
		cred = newCredential("svc", one.account, nil)
		c.Equal("svc:"+one.account, cred.TargetName)
		if one.kept {
			c.Equal(one.account, cred.UserName)
		} else {
			c.Equal("", cred.UserName)
		}
	}
}

func TestUTF16Len(t *testing.T) {
	c := check.New(t)
	c.Equal(0, utf16Len(""))
	c.Equal(3, utf16Len("abc"))
	c.Equal(1, utf16Len("\u00e9"))
	c.Equal(2, utf16Len("\U0001F600"))
	c.Equal(4, utf16Len("a\U0001F600\u00e9"))
}

func TestFitsTargetName(t *testing.T) {
	c := check.New(t)
	c.True(fitsTargetName("svc:acct"))
	c.True(fitsTargetName(strings.Repeat("a", w32.CredMaxGenericTargetNameLength)))
	c.False(fitsTargetName(strings.Repeat("a", w32.CredMaxGenericTargetNameLength+1)))
	// The limit counts UTF-16 code units, not bytes or runes.
	c.True(fitsTargetName(strings.Repeat("\u00e9", w32.CredMaxGenericTargetNameLength)))
	c.False(fitsTargetName(strings.Repeat("\U0001F600", (w32.CredMaxGenericTargetNameLength+1)/2)))
}

func TestCredentialStoreAnswersOverlongNamesItself(t *testing.T) {
	c := check.New(t)
	// Set refuses these names, so no secret can have them, and the store answers without calling into the Credential
	// Manager.
	const service = "com.trollworks.unison.secrets.test"
	account := strings.Repeat("a", w32.CredMaxGenericTargetNameLength)
	got, err := credentialStore{}.Get(service, account)
	c.True(errors.Is(err, ErrNotFound), "Get: %v", err)
	c.Contains(err.Error(), strconv.Quote(service))
	c.Nil(got)
	err = credentialStore{}.Delete(service, account)
	c.True(errors.Is(err, ErrNotFound), "Delete: %v", err)

	// With a one-code-unit account, the shortest credential name of this service is one code unit too long.
	for _, longService := range []string{
		strings.Repeat("s", w32.CredMaxGenericTargetNameLength-1),
		strings.Repeat("*", w32.CredMaxGenericTargetNameLength-1),
	} {
		var accounts []string
		accounts, err = credentialStore{}.Accounts(longService)
		c.NoError(err)
		c.Nil(accounts)
	}
}

func TestMapCredentialError(t *testing.T) {
	c := check.New(t)
	c.NoError(mapCredentialError(nil, "svc", "acct"))

	err := mapCredentialError(windows.ERROR_NOT_FOUND, "svc", "acct")
	c.True(errors.Is(err, ErrNotFound))
	c.Contains(err.Error(), `"svc"`)
	c.Contains(err.Error(), `"acct"`)
	c.False(errors.Is(err, ErrUnavailable))

	err = mapCredentialError(windows.ERROR_NO_SUCH_LOGON_SESSION, "svc", "acct")
	c.True(errors.Is(err, ErrUnavailable))
	c.False(errors.Is(err, ErrNotFound))
	c.Contains(err.Error(), "logon session")

	original := errs.New("CredReadW failed: Access is denied. (error code 0x5)")
	err = mapCredentialError(original, "svc", "acct")
	c.Equal(error(original), err)
	c.False(errors.Is(err, ErrNotFound))
	c.False(errors.Is(err, ErrUnavailable))
}

func TestMapUnavailableError(t *testing.T) {
	c := check.New(t)
	c.NoError(mapUnavailableError(nil))

	err := mapUnavailableError(windows.ERROR_NO_SUCH_LOGON_SESSION)
	c.True(errors.Is(err, ErrUnavailable))
	c.False(errors.Is(err, ErrNotFound))
	c.Contains(err.Error(), "logon session")

	// Only mapCredentialError, which Get and Delete use, turns a missing credential into ErrNotFound.
	c.Equal(error(windows.ERROR_NOT_FOUND), mapUnavailableError(windows.ERROR_NOT_FOUND))

	original := errs.New("CredWriteW failed: Access is denied. (error code 0x5)")
	err = mapUnavailableError(original)
	c.Equal(error(original), err)
	c.False(errors.Is(err, ErrUnavailable))
}

func TestCredentialStoreRefusesOversizedSecret(t *testing.T) {
	c := check.New(t)
	// The size check runs before any call into the Credential Manager, so this never reaches the OS.
	err := credentialStore{}.Set("com.trollworks.unison.secrets.test", "oversized",
		make([]byte, w32.CredMaxCredentialBlobSize+1))
	c.HasError(err)
	c.Contains(err.Error(), strconv.Itoa(w32.CredMaxCredentialBlobSize+1))
	c.Contains(err.Error(), strconv.Itoa(w32.CredMaxCredentialBlobSize))
	c.False(errors.Is(err, ErrNotFound))
	c.False(errors.Is(err, ErrUnavailable))
}

func TestCredentialStoreRefusesOverlongTargetName(t *testing.T) {
	c := check.New(t)
	// The length check runs before any call into the Credential Manager, so this never reaches the OS.
	err := credentialStore{}.Set("svc", strings.Repeat("a", w32.CredMaxGenericTargetNameLength), []byte("x"))
	c.HasError(err)
	c.Contains(err.Error(), strconv.Itoa(w32.CredMaxGenericTargetNameLength+len("svc:")))
	c.Contains(err.Error(), strconv.Itoa(w32.CredMaxGenericTargetNameLength))
	c.False(errors.Is(err, ErrNotFound))
	c.False(errors.Is(err, ErrUnavailable))
}

// TestCredentialStoreLiveRoundTrip exercises the package functions against the Credential Manager. Since it writes to
// the user's credentials, it only runs when UNISON_SECRETS_LIVE is set.
func TestCredentialStoreLiveRoundTrip(t *testing.T) {
	if os.Getenv("UNISON_SECRETS_LIVE") == "" {
		t.Skip("set UNISON_SECRETS_LIVE to run this test against the Windows Credential Manager")
	}
	c := check.New(t)
	t.Cleanup(UseStore(nil))
	const service = "com.trollworks.unison.secrets.test"
	account := "account-" + rand.Text()
	t.Cleanup(func() {
		_ = credentialStore{}.Delete(service, account) //nolint:errcheck // The credential is usually gone already.
	})

	largest := make([]byte, w32.CredMaxCredentialBlobSize)
	for i := range largest {
		largest[i] = byte(i)
	}
	for _, payload := range [][]byte{[]byte("first value"), {0, 1, 2, 253, 254, 255}, largest, nil} {
		c.NoError(Set(service, account, payload))
		got, err := Get(service, account)
		c.NoError(err)
		c.NotNil(got)
		c.Equal(len(payload), len(got))
		c.True(slices.Equal(payload, got), "got %v, want %v", got, payload)
	}

	cred, err := w32.CredReadW(targetName(service, account), w32.CredTypeGeneric)
	c.NoError(err)
	if cred != nil {
		c.Equal(account, cred.UserName)
		c.Equal(uint32(w32.CredPersistLocalMachine), cred.Persist)
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

// TestCredentialStoreLiveAccountNames checks, against the Credential Manager, that an account too long for a UserName
// still round-trips and is listed, and that names differing only in case reach the same secret. Since it writes to the
// user's credentials, it only runs when UNISON_SECRETS_LIVE is set.
func TestCredentialStoreLiveAccountNames(t *testing.T) {
	if os.Getenv("UNISON_SECRETS_LIVE") == "" {
		t.Skip("set UNISON_SECRETS_LIVE to run this test against the Windows Credential Manager")
	}
	c := check.New(t)
	t.Cleanup(UseStore(nil))
	const service = "com.trollworks.unison.secrets.test"
	prefix := "account-" + rand.Text() + "-"
	fits := prefix + strings.Repeat("a", w32.CredMaxUsernameLength-len(prefix))
	long := prefix + strings.Repeat("b", w32.CredMaxUsernameLength)
	mixed := prefix + "MiXeD"
	t.Cleanup(func() {
		for _, account := range []string{fits, long, mixed} {
			_ = credentialStore{}.Delete(service, account) //nolint:errcheck // The credential is usually gone already.
		}
	})

	for _, one := range []struct {
		account  string
		userName string
	}{{fits, fits}, {long, ""}} {
		c.NoError(Set(service, one.account, []byte(one.account)))
		got, err := Get(service, one.account)
		c.NoError(err)
		c.Equal([]byte(one.account), got)
		var cred *w32.Credential
		cred, err = w32.CredReadW(targetName(service, one.account), w32.CredTypeGeneric)
		c.NoError(err)
		if cred != nil {
			c.Equal(one.userName, cred.UserName)
		}
	}
	accounts, err := Accounts(service)
	c.NoError(err)
	c.True(slices.Contains(accounts, fits), "accounts lack the account that fits a UserName")
	c.True(slices.Contains(accounts, long), "accounts lack the account too long for a UserName")

	c.NoError(Set(service, mixed, []byte("first")))
	c.NoError(Set(service, strings.ToLower(mixed), []byte("second")))
	got, err := Get(service, strings.ToUpper(mixed))
	c.NoError(err)
	c.Equal([]byte("second"), got)
	c.NoError(Delete(service, mixed))
	_, err = Get(service, strings.ToLower(mixed))
	c.True(errors.Is(err, ErrNotFound), "Get after Delete: %v", err)
}
