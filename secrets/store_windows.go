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
	"errors"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// credentialStore keeps each secret in the Credential Manager as a generic credential whose TargetName is
// "service:account". Since a service never contains a colon, the TargetName identifies exactly one pair. The UserName
// repeats the account so the Credential Manager's window shows it, unless the account is too long for one. Windows
// compares TargetNames without regard to case, so a service or account that differs from another only in letter case
// names the same secret. Credentials persist on the local machine, surviving logoff and reboot without roaming to the
// user's other computers. A network logon has no credential set, so every call into the Credential Manager then fails
// with an error matching ErrUnavailable.
type credentialStore struct{}

func newPlatformStore() Store {
	return credentialStore{}
}

func (credentialStore) Set(service, account string, secret []byte) error {
	if len(secret) > w32.CredMaxCredentialBlobSize {
		return errs.Newf("the secret is %d bytes, but the Windows Credential Manager holds at most %d", len(secret),
			w32.CredMaxCredentialBlobSize)
	}
	cred := newCredential(service, account, secret)
	if !fitsTargetName(cred.TargetName) {
		return errs.Newf("the service and account form a credential name %d UTF-16 code units long, but the Windows "+
			"Credential Manager allows at most %d", utf16Len(cred.TargetName), w32.CredMaxGenericTargetNameLength)
	}
	return mapUnavailableError(w32.CredWriteW(cred))
}

func (credentialStore) Get(service, account string) ([]byte, error) {
	name := targetName(service, account)
	if !fitsTargetName(name) {
		return nil, notFoundError(service, account)
	}
	cred, err := w32.CredReadW(name, w32.CredTypeGeneric)
	if err != nil {
		return nil, mapCredentialError(err, service, account)
	}
	if cred.Blob == nil {
		return []byte{}, nil
	}
	return cred.Blob, nil
}

func (credentialStore) Delete(service, account string) error {
	name := targetName(service, account)
	if !fitsTargetName(name) {
		return notFoundError(service, account)
	}
	return mapCredentialError(w32.CredDeleteW(name, w32.CredTypeGeneric), service, account)
}

func (credentialStore) Accounts(service string) ([]string, error) {
	filter := service + ":*"
	if !fitsTargetName(filter) {
		// With a one-code-unit account, the service's shortest TargetName is as long as the filter, so Set refused it.
		return nil, nil
	}
	if strings.ContainsRune(service, '*') {
		// Windows may treat an asterisk inside the filter as a wildcard or reject the filter, so list every
		// credential instead and let accountsFrom pick out the service's.
		filter = ""
	}
	creds, err := w32.CredEnumerateW(filter)
	if err != nil {
		return nil, mapUnavailableError(err)
	}
	return accountsFrom(service, creds), nil
}

// targetName returns the TargetName of the credential for the service and account.
func targetName(service, account string) string {
	return service + ":" + account
}

// fitsTargetName reports whether name is short enough to be the TargetName of a generic credential. Set refuses a name
// that is not, so no secret has one.
func fitsTargetName(name string) bool {
	return utf16Len(name) <= w32.CredMaxGenericTargetNameLength
}

// newCredential returns the credential Set writes. Its UserName is empty when the account is too long for one, which
// is harmless, since the Credential Manager ignores the UserName of a generic credential.
func newCredential(service, account string, secret []byte) *w32.Credential {
	cred := &w32.Credential{
		TargetName: targetName(service, account),
		Blob:       secret,
		Type:       w32.CredTypeGeneric,
		Persist:    w32.CredPersistLocalMachine,
	}
	if utf16Len(account) <= w32.CredMaxUsernameLength {
		cred.UserName = account
	}
	return cred
}

// utf16Len returns the length of s in UTF-16 code units, the characters Windows counts in its length limits.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// accountsFrom returns the sorted, distinct accounts of the generic credentials in creds that belong to the service,
// or nil when there are none. It takes each account from the TargetName, after the first colon, and keeps it only
// when the text before that colon is the service, which drops those of other services, such as "svc2:x" for the
// service "svc". The comparison ignores case, as Windows does when it looks up a TargetName.
func accountsFrom(service string, creds []*w32.Credential) []string {
	var accounts []string
	for _, cred := range creds {
		if cred == nil || cred.Type != w32.CredTypeGeneric {
			continue
		}
		if prefix, account, found := strings.Cut(cred.TargetName, ":"); found && account != "" &&
			strings.EqualFold(prefix, service) {
			accounts = append(accounts, account)
		}
	}
	slices.Sort(accounts)
	return slices.Compact(accounts)
}

// mapCredentialError turns an error for a missing credential into one that matches ErrNotFound and passes any other
// error through mapUnavailableError.
func mapCredentialError(err error, service, account string) error {
	if errors.Is(err, windows.ERROR_NOT_FOUND) {
		return notFoundError(service, account)
	}
	return mapUnavailableError(err)
}

// mapUnavailableError turns the error for a logon session that has no credential set, as no network logon does, into
// one that matches ErrUnavailable and returns any other error unchanged.
func mapUnavailableError(err error) error {
	if errors.Is(err, windows.ERROR_NO_SUCH_LOGON_SESSION) {
		return errs.NewWithCause("the Windows Credential Manager has no credential set for this logon session, as "+
			"with a network logon", ErrUnavailable)
	}
	return err
}

// notFoundError returns an error matching ErrNotFound for the service and account.
func notFoundError(service, account string) error {
	return errs.NewWithCausef(ErrNotFound, "no secret is stored for service %q and account %q", service, account)
}
