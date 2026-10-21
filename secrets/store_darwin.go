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

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/cocoa"
)

// keychainStore keeps each secret as a generic password whose service and account attributes are the service and
// account, labeled with the service. A new item goes into the user's default keychain, normally the login keychain,
// and lookups search the user's keychain search list. The data protection keychain is never used, since it needs an
// entitlement that unsigned programs lack. A failure because no keychain can be reached matches ErrUnavailable.
type keychainStore struct{}

func newPlatformStore() Store {
	return keychainStore{}
}

func (keychainStore) Set(service, account string, secret []byte) error {
	return mapKeychainUnavailable(cocoa.KeychainSetGenericPassword(service, account, secret))
}

func (keychainStore) Get(service, account string) ([]byte, error) {
	secret, err := cocoa.KeychainGenericPassword(service, account)
	if err != nil {
		return nil, mapKeychainError(err, service, account)
	}
	return secret, nil
}

func (keychainStore) Delete(service, account string) error {
	return mapKeychainError(cocoa.KeychainDeleteGenericPassword(service, account), service, account)
}

func (keychainStore) Accounts(service string) ([]string, error) {
	accounts, err := cocoa.KeychainGenericPasswordAccounts(service)
	if err != nil {
		return nil, mapKeychainUnavailable(err)
	}
	return accounts, nil
}

// mapKeychainError turns an error for a missing item into one that matches ErrNotFound and passes any other error to
// mapKeychainUnavailable.
func mapKeychainError(err error, service, account string) error {
	if errors.Is(err, cocoa.ErrSecItemNotFound) {
		return errs.NewWithCausef(ErrNotFound, "no secret is stored for service %q and account %q", service, account)
	}
	return mapKeychainUnavailable(err)
}

// mapKeychainUnavailable turns an error for a keychain that cannot be reached into one that matches ErrUnavailable and
// returns any other error, such as a canceled or refused access prompt, unchanged.
func mapKeychainUnavailable(err error) error {
	if status, ok := errors.AsType[cocoa.OSStatus](err); ok {
		switch status {
		case cocoa.ErrSecNotAvailable, cocoa.ErrSecNoSuchKeychain, cocoa.ErrSecNoDefaultKeychain:
			return errs.NewWithCausef(ErrUnavailable, "unable to reach a keychain (OSStatus %d): %s", status, err)
		}
	}
	return err
}
