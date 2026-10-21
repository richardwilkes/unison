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
	"slices"
	"strings"
	"sync"
)

var _ Store = &MemoryStore{}

// MemoryStore is a Store that keeps secrets in the process's memory and never touches the operating system. A headless
// session installs a fresh one. The zero value is ready for use. Like the Windows Credential Manager, it ignores letter
// case, so a service or account that differs from another only in case names the same secret, and Accounts lists the
// spelling of the account that Set last used.
type MemoryStore struct {
	// services maps a lowercased service to its secrets, which are keyed by lowercased account.
	services map[string]map[string]memorySecret
	lock     sync.Mutex
}

// memorySecret is a secret and the spelling of its account that Set last used.
type memorySecret struct {
	account string
	data    []byte
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Set stores a copy of secret for the service and account, replacing any secret already there.
func (s *MemoryStore) Set(service, account string, secret []byte) error {
	data := clone(secret)
	service = strings.ToLower(service)
	key := strings.ToLower(account)
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.services == nil {
		s.services = make(map[string]map[string]memorySecret)
	}
	accounts := s.services[service]
	if accounts == nil {
		accounts = make(map[string]memorySecret)
		s.services[service] = accounts
	}
	accounts[key] = memorySecret{account: account, data: data}
	return nil
}

// Get returns a copy of the secret for the service and account, or ErrNotFound when there is none.
func (s *MemoryStore) Get(service, account string) ([]byte, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	one, ok := s.services[strings.ToLower(service)][strings.ToLower(account)]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(one.data), nil
}

// Delete removes the secret for the service and account, or returns ErrNotFound when there is none.
func (s *MemoryStore) Delete(service, account string) error {
	service = strings.ToLower(service)
	account = strings.ToLower(account)
	s.lock.Lock()
	defer s.lock.Unlock()
	accounts := s.services[service]
	if _, ok := accounts[account]; !ok {
		return ErrNotFound
	}
	delete(accounts, account)
	if len(accounts) == 0 {
		delete(s.services, service)
	}
	return nil
}

// Accounts returns the sorted accounts that hold a secret for the service, or nil when there are none.
func (s *MemoryStore) Accounts(service string) ([]string, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	stored := s.services[strings.ToLower(service)]
	if len(stored) == 0 {
		return nil, nil
	}
	accounts := make([]string, 0, len(stored))
	for _, one := range stored {
		accounts = append(accounts, one.account)
	}
	slices.Sort(accounts)
	return accounts, nil
}

// clone returns a copy of data that is never nil, even when data is.
func clone(data []byte) []byte {
	return append(make([]byte, 0, len(data)), data...)
}
