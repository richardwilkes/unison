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
	"sync"
	"unicode/utf8"

	"github.com/richardwilkes/toolbox/v2/errs"
)

var (
	// ErrNotFound reports that no secret exists for the service and account.
	ErrNotFound = errors.New("secrets: not found")
	// ErrUnavailable reports that no secret store can be reached: on Linux there is no session bus or nothing provides
	// the Secret Service, on macOS no keychain is available, or on Windows the logon session has no credential set.
	ErrUnavailable = errors.New("secrets: no secret store is available")
)

var (
	storeLock sync.RWMutex
	// layers holds what UseStore installed, oldest first. The last one is in use, and a nil store selects the platform
	// store, as does an empty stack.
	layers        []*storeLayer
	platformStore = sync.OnceValue(newPlatformStore)
)

// storeLayer is one UseStore call's entry in layers, which its restore function finds by identity.
type storeLayer struct {
	store Store
}

// Store holds secrets, each identified by a service and an account. Implementations must be safe for concurrent use.
// Any method may block, and the platform store may put up a dialog, so call off the UI thread when a user is waiting.
type Store interface {
	// Set stores secret for the service and account, replacing any secret already there. A nil or empty secret is
	// stored as a zero-length one.
	Set(service, account string, secret []byte) error
	// Get returns a copy of the secret for the service and account, or an error matching ErrNotFound when there is
	// none.
	Get(service, account string) ([]byte, error)
	// Delete removes the secret for the service and account, or returns an error matching ErrNotFound when there is
	// none.
	Delete(service, account string) error
	// Accounts returns the sorted, distinct accounts that hold a secret for the service, or nil when there are none.
	Accounts(service string) ([]string, error)
}

// Set stores secret for the service and account in the current store, replacing any secret already there. A nil or
// empty secret is stored as a zero-length one. An invalid service or account (see the package documentation) is
// rejected before the store is consulted.
func Set(service, account string, secret []byte) error {
	if err := validate(service, account); err != nil {
		return err
	}
	return current().Set(service, account, secret)
}

// Get returns the secret for the service and account from the current store, or an error matching ErrNotFound when
// there is none. It never returns a nil secret without an error: an empty secret comes back as a zero-length slice. An
// invalid service or account is rejected before the store is consulted.
func Get(service, account string) ([]byte, error) {
	if err := validate(service, account); err != nil {
		return nil, err
	}
	secret, err := current().Get(service, account)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		secret = []byte{}
	}
	return secret, nil
}

// Delete removes the secret for the service and account from the current store, or returns an error matching
// ErrNotFound when there is none. An invalid service or account is rejected before the store is consulted.
func Delete(service, account string) error {
	if err := validate(service, account); err != nil {
		return err
	}
	return current().Delete(service, account)
}

// Accounts returns the sorted, distinct accounts that hold a secret for the service in the current store, or nil when
// there are none. An invalid service is rejected before the store is consulted.
func Accounts(service string) ([]string, error) {
	if err := validateService(service); err != nil {
		return nil, err
	}
	return current().Accounts(service)
}

// UseStore installs store on top of any installed before it and returns a function that removes it again. The package
// functions use the most recently installed store that has not been removed, or the platform store when there is none.
// A nil store selects the platform store. The returned function removes only its own store, wherever it sits, so
// restore functions may run in any order, and calling one again has no effect.
func UseStore(store Store) (restore func()) {
	layer := &storeLayer{store: store}
	storeLock.Lock()
	layers = append(layers, layer)
	storeLock.Unlock()
	return func() {
		storeLock.Lock()
		if i := slices.Index(layers, layer); i >= 0 {
			layers = slices.Delete(layers, i, i+1)
		}
		storeLock.Unlock()
	}
}

func current() Store {
	var store Store
	storeLock.RLock()
	if len(layers) != 0 {
		store = layers[len(layers)-1].store
	}
	storeLock.RUnlock()
	if store == nil {
		return platformStore()
	}
	return store
}

func validate(service, account string) error {
	if err := validateService(service); err != nil {
		return err
	}
	return validateName("account", account)
}

func validateService(service string) error {
	if err := validateName("service", service); err != nil {
		return err
	}
	if strings.ContainsRune(service, ':') {
		return errs.Newf("the service %q must not contain a colon", service)
	}
	return nil
}

func validateName(what, name string) error {
	switch {
	case name == "":
		return errs.Newf("the %s must not be empty", what)
	case !utf8.ValidString(name):
		return errs.Newf("the %s %q is not valid UTF-8", what, name)
	case strings.ContainsRune(name, 0):
		return errs.Newf("the %s %q must not contain a NUL character", what, name)
	default:
		return nil
	}
}
