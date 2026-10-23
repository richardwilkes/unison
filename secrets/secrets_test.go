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
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/secrets"
)

// rootPackage is the import path this package must never depend on.
const rootPackage = "github.com/richardwilkes/unison"

const (
	testService = "com.example.unison.secrets"
	testAccount = "alice"
)

// recordingStore records each call it receives and answers with the results it was given.
type recordingStore struct {
	err      error
	calls    []string
	data     []byte
	accounts []string
}

func (s *recordingStore) Set(service, account string, secret []byte) error {
	s.calls = append(s.calls, fmt.Sprintf("Set %s %s %q", service, account, secret))
	return s.err
}

func (s *recordingStore) Get(service, account string) ([]byte, error) {
	s.calls = append(s.calls, "Get "+service+" "+account)
	return s.data, s.err
}

func (s *recordingStore) Delete(service, account string) error {
	s.calls = append(s.calls, "Delete "+service+" "+account)
	return s.err
}

func (s *recordingStore) Accounts(service string) ([]string, error) {
	s.calls = append(s.calls, "Accounts "+service)
	return s.accounts, s.err
}

// TestNeverImportsTheRootPackage verifies the claim the package documentation makes about itself: command-line tools
// can use this package without the root unison package's initialization, which holds only while nothing here imports
// it.
func TestNeverImportsTheRootPackage(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	entries, err := os.ReadDir(".")
	c.NoError(err)
	fset := token.NewFileSet()
	examined := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		f, parseErr := parser.ParseFile(fset, name, nil, parser.ImportsOnly|parser.SkipObjectResolution)
		c.NoError(parseErr)
		if f == nil {
			continue
		}
		examined++
		for _, imported := range f.Imports {
			path, unquoteErr := strconv.Unquote(imported.Path.Value)
			c.NoError(unquoteErr)
			c.NotEqual(rootPackage, path, "%s must not import the root unison package",
				fset.Position(imported.Pos()))
		}
	}
	c.True(examined > 0, "no source files were examined")
}

func TestFunctionsForwardToTheInstalledStore(t *testing.T) {
	c := check.New(t)
	store := &recordingStore{data: []byte("hello"), accounts: []string{"bob", "carol:work"}}
	restore := secrets.UseStore(store)
	defer restore()

	c.NoError(secrets.Set(testService, testAccount, []byte("world")))
	got, err := secrets.Get(testService, testAccount)
	c.NoError(err)
	c.Equal([]byte("hello"), got)
	c.NoError(secrets.Delete(testService, testAccount))
	accounts, err := secrets.Accounts(testService)
	c.NoError(err)
	c.Equal([]string{"bob", "carol:work"}, accounts)
	c.NoError(secrets.Set(testService, "dave:home", nil))
	c.Equal([]string{
		"Set " + testService + " " + testAccount + ` "world"`,
		"Get " + testService + " " + testAccount,
		"Delete " + testService + " " + testAccount,
		"Accounts " + testService,
		"Set " + testService + ` dave:home ""`,
	}, store.calls)

	failure := errors.New("store failure")
	store.err = failure
	store.calls = nil
	c.True(errors.Is(secrets.Set(testService, testAccount, nil), failure))
	got, err = secrets.Get(testService, testAccount)
	c.True(errors.Is(err, failure))
	c.Nil(got)
	c.True(errors.Is(secrets.Delete(testService, testAccount), failure))
	accounts, err = secrets.Accounts(testService)
	c.True(errors.Is(err, failure))
	c.Equal([]string{"bob", "carol:work"}, accounts)
	c.Equal(4, len(store.calls))

	store.err = secrets.ErrNotFound
	_, err = secrets.Get(testService, testAccount)
	c.True(errors.Is(err, secrets.ErrNotFound))
}

func TestGetNeverReturnsNilWithoutAnError(t *testing.T) {
	c := check.New(t)
	restore := secrets.UseStore(&recordingStore{})
	defer restore()
	got, err := secrets.Get(testService, testAccount)
	c.NoError(err)
	c.NotNil(got)
	c.Equal([]byte{}, got)
}

func TestInvalidArgumentsNeverReachTheStore(t *testing.T) {
	c := check.New(t)
	store := &recordingStore{}
	restore := secrets.UseStore(store)
	defer restore()

	invalidServices := []string{"", "com.example\x00service", "com.example\xffservice", "com.example:service"}
	invalidAccounts := []string{"", "al\x00ice", "al\xffice"}
	checkRejected := func(err error, service, account string) {
		t.Helper()
		c.HasError(err, "service %q, account %q", service, account)
		c.False(errors.Is(err, secrets.ErrNotFound))
		c.False(errors.Is(err, secrets.ErrUnavailable))
	}
	try := func(service, account string) {
		t.Helper()
		checkRejected(secrets.Set(service, account, []byte("hello")), service, account)
		got, err := secrets.Get(service, account)
		checkRejected(err, service, account)
		c.Nil(got)
		checkRejected(secrets.Delete(service, account), service, account)
	}
	for _, service := range invalidServices {
		try(service, testAccount)
		accounts, err := secrets.Accounts(service)
		checkRejected(err, service, "")
		c.Nil(accounts)
	}
	for _, account := range invalidAccounts {
		try(testService, account)
	}
	c.Equal(0, len(store.calls))

	// An account may contain a colon.
	c.NoError(secrets.Set(testService, "alice:work", []byte("hello")))
	c.Equal(1, len(store.calls))
}

// TestUseStoreRestoresWhatWasInstalledBefore verifies that each restore function removes only the store it installed,
// so restores that run again or out of order neither put back a covered store nor take away the newest one.
func TestUseStoreRestoresWhatWasInstalledBefore(t *testing.T) {
	c := check.New(t)
	first := &recordingStore{}
	second := &recordingStore{}
	third := &recordingStore{}
	reach := func(want *recordingStore) {
		t.Helper()
		before := len(want.calls)
		_, err := secrets.Accounts(testService)
		c.NoError(err)
		c.Equal(before+1, len(want.calls))
	}

	restoreFirst := secrets.UseStore(first)
	defer restoreFirst()
	reach(first)

	restoreSecond := secrets.UseStore(second)
	reach(second)
	restoreSecond()
	reach(first)

	// Calling a restore function again must have no effect.
	restoreThird := secrets.UseStore(third)
	restoreSecond()
	reach(third)
	restoreThird()
	reach(first)

	// A nil store selects the platform store, which is never called here; restoring brings back the one before it.
	restoreNil := secrets.UseStore(nil)
	restoreNil()
	reach(first)

	// Restoring x while y covers it leaves y in place, and restoring y then brings back the store below x, not x.
	x := &recordingStore{}
	y := &recordingStore{}
	restoreX := secrets.UseStore(x)
	restoreY := secrets.UseStore(y)
	restoreX()
	reach(y)
	restoreX()
	reach(y)
	restoreY()
	reach(first)
	c.Equal(5, len(first.calls))
	c.Equal(1, len(second.calls))
	c.Equal(1, len(third.calls))
	c.Equal(0, len(x.calls))
	c.Equal(2, len(y.calls))
}
