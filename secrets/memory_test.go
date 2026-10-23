// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package secrets_test

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/secrets"
)

func TestMemoryStoreZeroValue(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	var store secrets.MemoryStore
	_, err := store.Get(testService, testAccount)
	c.True(errors.Is(err, secrets.ErrNotFound))
	c.True(errors.Is(store.Delete(testService, testAccount), secrets.ErrNotFound))
	c.NoError(store.Set(testService, testAccount, []byte("first")))
	got, err := store.Get(testService, testAccount)
	c.NoError(err)
	c.Equal([]byte("first"), got)
}

func TestMemoryStoreIsolatesServices(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	store := secrets.NewMemoryStore()
	const other = testService + ".other"
	c.NoError(store.Set(testService, testAccount, []byte("first")))
	c.NoError(store.Set(other, testAccount, []byte("second")))
	got, err := store.Get(other, testAccount)
	c.NoError(err)
	c.Equal([]byte("second"), got)

	c.NoError(store.Delete(testService, testAccount))
	_, err = store.Get(testService, testAccount)
	c.True(errors.Is(err, secrets.ErrNotFound))
	got, err = store.Get(other, testAccount)
	c.NoError(err)
	c.Equal([]byte("second"), got)
	accounts, err := store.Accounts(testService)
	c.NoError(err)
	c.Nil(accounts)
	accounts, err = store.Accounts(other)
	c.NoError(err)
	c.Equal([]string{testAccount}, accounts)
}

func TestMemoryStoreAccounts(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	store := secrets.NewMemoryStore()
	accounts, err := store.Accounts(testService)
	c.NoError(err)
	c.Nil(accounts)

	for _, account := range []string{"carol", testAccount, "erin:work", testAccount, "erin"} {
		c.NoError(store.Set(testService, account, []byte(account)))
	}
	c.NoError(store.Set(testService+".other", "zed", []byte("zed")))
	accounts, err = store.Accounts(testService)
	c.NoError(err)
	c.Equal([]string{testAccount, "carol", "erin", "erin:work"}, accounts)
}

func TestMemoryStoreIgnoresCase(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	store := secrets.NewMemoryStore()
	upperService := strings.ToUpper(testService)
	c.NoError(store.Set(testService, "Alice", []byte("first")))
	c.NoError(store.Set(testService, "alice", []byte("second")))
	got, err := store.Get(testService, "ALICE")
	c.NoError(err)
	c.Equal([]byte("second"), got)
	got, err = store.Get(upperService, "Alice")
	c.NoError(err)
	c.Equal([]byte("second"), got)

	// Set replaces the whole secret, so Accounts lists the spelling it last used.
	accounts, err := store.Accounts(upperService)
	c.NoError(err)
	c.Equal([]string{"alice"}, accounts)
	c.NoError(store.Set(upperService, "ALICE", []byte("third")))
	c.NoError(store.Set(testService, "Bob", []byte("fourth")))
	accounts, err = store.Accounts(testService)
	c.NoError(err)
	c.Equal([]string{"ALICE", "Bob"}, accounts)

	c.NoError(store.Delete(testService, "aLiCe"))
	got, err = store.Get(testService, "Alice")
	c.True(errors.Is(err, secrets.ErrNotFound))
	c.Nil(got)
	c.True(errors.Is(store.Delete(upperService, "alice"), secrets.ErrNotFound))
	c.NoError(store.Delete(upperService, "bob"))
	accounts, err = store.Accounts(testService)
	c.NoError(err)
	c.Nil(accounts)
}

func TestMemoryStoreEmptySecret(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	store := secrets.NewMemoryStore()
	for _, empty := range [][]byte{nil, {}} {
		c.NoError(store.Set(testService, testAccount, empty))
		got, err := store.Get(testService, testAccount)
		c.NoError(err)
		c.NotNil(got)
		c.Equal([]byte{}, got)
	}
}

func TestMemoryStoreCopiesSecrets(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	store := secrets.NewMemoryStore()
	payload := []byte("first")
	c.NoError(store.Set(testService, testAccount, payload))
	payload[0] = 'F'
	got, err := store.Get(testService, testAccount)
	c.NoError(err)
	c.Equal([]byte("first"), got)

	got[0] = 'F'
	got, err = store.Get(testService, testAccount)
	c.NoError(err)
	c.Equal([]byte("first"), got)
}

func TestMemoryStoreConcurrentUse(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	store := secrets.NewMemoryStore()
	const workers = 8
	const rounds = 100
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Go(func() {
			account := "worker" + strconv.Itoa(worker)
			for round := range rounds {
				payload := []byte(strconv.Itoa(round))
				c.NoError(store.Set(testService, account, payload))
				got, err := store.Get(testService, account)
				c.NoError(err)
				c.Equal(payload, got)
				_, err = store.Accounts(testService)
				c.NoError(err)
				if round%2 == 0 {
					c.NoError(store.Delete(testService, account))
				}
			}
		})
	}
	wg.Wait()
	accounts, err := store.Accounts(testService)
	c.NoError(err)
	c.Equal(workers, len(accounts))
}
