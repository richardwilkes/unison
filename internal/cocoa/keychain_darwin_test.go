// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/ebitengine/purego/objc"
)

func TestKeychainOSStatusError(t *testing.T) {
	msg := ErrSecItemNotFound.Error()
	if msg == "" || msg == "OSStatus -25300" {
		t.Errorf("ErrSecItemNotFound.Error() = %q, want the Security framework's description", msg)
	}
	err := keychainError("x", ErrSecItemNotFound)
	if !errors.Is(err, ErrSecItemNotFound) {
		t.Errorf("errors.Is(%v, ErrSecItemNotFound) = false, want true", err)
	}
	if errors.Is(err, ErrSecDuplicateItem) {
		t.Errorf("errors.Is(%v, ErrSecDuplicateItem) = true, want false", err)
	}
}

// fakeKeychains replaces SecItemAdd and SecItemUpdate with a model of the file-based keychains, each of which maps a
// service and account to a secret. As with the real ones, SecItemAdd checks for a duplicate in and writes to only the
// default keychain, here the first, while SecItemUpdate changes every match on the search list, here all of them.
type fakeKeychains struct {
	keychains []map[[2]string][]byte
	// beforeAdd and beforeUpdate, when set, run ahead of each call, as a concurrent writer would.
	beforeAdd    func()
	beforeUpdate func()
	calls        []string
}

func useFakeKeychains(t *testing.T, f *fakeKeychains) {
	t.Helper()
	ensureSecFuncs()
	add, update := secItemAdd, secItemUpdate
	t.Cleanup(func() { secItemAdd, secItemUpdate = add, update })
	key := func(dict objc.ID) [2]string {
		return [2]string{
			GoStringFromNSString(NSDictionaryObjectForKey(dict, kSecAttrService())),
			GoStringFromNSString(NSDictionaryObjectForKey(dict, kSecAttrAccount())),
		}
	}
	secItemAdd = func(attrs objc.ID, _ *objc.ID) int32 {
		f.calls = append(f.calls, "add")
		if f.beforeAdd != nil {
			f.beforeAdd()
		}
		k := key(attrs)
		if _, found := f.keychains[0][k]; found {
			return int32(ErrSecDuplicateItem)
		}
		f.keychains[0][k] = GoBytesFromNSData(NSDictionaryObjectForKey(attrs, kSecValueData()))
		return 0
	}
	secItemUpdate = func(query, attrs objc.ID) int32 {
		f.calls = append(f.calls, "update")
		if f.beforeUpdate != nil {
			f.beforeUpdate()
		}
		k := key(query)
		status := int32(ErrSecItemNotFound)
		for _, kc := range f.keychains {
			if _, found := kc[k]; found {
				kc[k] = GoBytesFromNSData(NSDictionaryObjectForKey(attrs, kSecValueData()))
				status = 0
			}
		}
		return status
	}
}

// TestKeychainSetReplacesItemOutsideDefaultKeychain checks that a Set replaces an item held by a keychain on the
// search list other than the default one, rather than adding a second item to the default keychain.
func TestKeychainSetReplacesItemOutsideDefaultKeychain(t *testing.T) {
	k := [2]string{"svc", "acct"}
	f := &fakeKeychains{keychains: []map[[2]string][]byte{{}, {k: []byte("old")}}}
	useFakeKeychains(t, f)
	if err := KeychainSetGenericPassword("svc", "acct", []byte("new")); err != nil {
		t.Fatalf("KeychainSetGenericPassword failed: %v", err)
	}
	if got, found := f.keychains[0][k]; found {
		t.Errorf("KeychainSetGenericPassword added %q to the default keychain", got)
	}
	if got := f.keychains[1][k]; string(got) != "new" {
		t.Errorf("item outside the default keychain = %q, want %q", got, "new")
	}
}

// TestKeychainSetRetriesRacingAdd checks that a Set whose SecItemAdd loses a race with another writer's add replaces
// that writer's item, and that it gives up after keychainSetAttempts rounds when every round loses.
func TestKeychainSetRetriesRacingAdd(t *testing.T) {
	k := [2]string{"svc", "acct"}
	f := &fakeKeychains{keychains: []map[[2]string][]byte{{}}}
	f.beforeAdd = func() {
		f.keychains[0][k] = []byte("racer")
		f.beforeAdd = nil
	}
	useFakeKeychains(t, f)
	if err := KeychainSetGenericPassword("svc", "acct", []byte("new")); err != nil {
		t.Fatalf("KeychainSetGenericPassword failed: %v", err)
	}
	if got := f.keychains[0][k]; string(got) != "new" {
		t.Errorf("item = %q, want %q", got, "new")
	}

	f.calls = nil
	f.beforeUpdate = func() { delete(f.keychains[0], k) }
	f.beforeAdd = func() { f.keychains[0][k] = []byte("racer") }
	err := KeychainSetGenericPassword("svc", "acct", []byte("new"))
	if !errors.Is(err, ErrSecDuplicateItem) {
		t.Errorf("KeychainSetGenericPassword = %v, want ErrSecDuplicateItem", err)
	}
	if len(f.calls) != 2*keychainSetAttempts {
		t.Errorf("KeychainSetGenericPassword made calls %v, want %d rounds", f.calls, keychainSetAttempts)
	}
}

const liveKeychainService = "com.trollworks.unison.secrets.test"

// liveKeychainAccount skips the test unless UNISON_SECRETS_LIVE is set, since the test writes to the user's keychain,
// and otherwise returns a fresh account whose item for liveKeychainService is deleted when the test ends.
func liveKeychainAccount(t *testing.T) string {
	t.Helper()
	if os.Getenv("UNISON_SECRETS_LIVE") == "" {
		t.Skip("set UNISON_SECRETS_LIVE to run this test against the user's keychain")
	}
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		t.Fatal(err)
	}
	account := "account-" + hex.EncodeToString(buf[:])
	t.Cleanup(func() {
		_ = KeychainDeleteGenericPassword(liveKeychainService, account) //nolint:errcheck // Best-effort cleanup
	})
	return account
}

// TestKeychainLiveRoundTrip exercises the generic password functions against the user's keychain.
func TestKeychainLiveRoundTrip(t *testing.T) {
	account := liveKeychainAccount(t)
	const service = liveKeychainService

	// The final empty payload covers emptying an existing item, which SecItemUpdate ignores when given an empty NSData.
	for _, payload := range [][]byte{{}, []byte("first value"), {0, 1, 2, 253, 254, 255}, {}} {
		if err := KeychainSetGenericPassword(service, account, payload); err != nil {
			t.Fatalf("KeychainSetGenericPassword(%v) failed: %v", payload, err)
		}
		got, err := KeychainGenericPassword(service, account)
		if err != nil {
			t.Fatalf("KeychainGenericPassword failed: %v", err)
		}
		if got == nil || !bytes.Equal(got, payload) {
			t.Fatalf("KeychainGenericPassword = %#v, want %#v", got, payload)
		}
	}

	accounts, err := KeychainGenericPasswordAccounts(service)
	if err != nil {
		t.Fatalf("KeychainGenericPasswordAccounts failed: %v", err)
	}
	if !slices.Contains(accounts, account) {
		t.Errorf("KeychainGenericPasswordAccounts = %v, want it to contain %q", accounts, account)
	}

	if err = KeychainDeleteGenericPassword(service, account); err != nil {
		t.Fatalf("KeychainDeleteGenericPassword failed: %v", err)
	}
	if _, err = KeychainGenericPassword(service, account); !errors.Is(err, ErrSecItemNotFound) {
		t.Errorf("KeychainGenericPassword after delete = %v, want ErrSecItemNotFound", err)
	}
	if err = KeychainDeleteGenericPassword(service, account); !errors.Is(err, ErrSecItemNotFound) {
		t.Errorf("second KeychainDeleteGenericPassword = %v, want ErrSecItemNotFound", err)
	}
}

// TestKeychainLiveSetEmptyKeepsItem checks that replacing a secret with an empty one updates the item in place rather
// than deleting and re-adding it, which would drop its other attributes and its access list.
func TestKeychainLiveSetEmptyKeepsItem(t *testing.T) {
	account := liveKeychainAccount(t)
	if err := KeychainSetGenericPassword(liveKeychainService, account, []byte("value")); err != nil {
		t.Fatalf("KeychainSetGenericPassword failed: %v", err)
	}
	kSecAttrComment := secConstant("kSecAttrComment")
	const comment = "kept across an empty Set"
	var status OSStatus
	WithPool(func() {
		status = OSStatus(secItemUpdate(keychainQuery(liveKeychainService, account),
			NSDictionaryFromPairs(kSecAttrComment(), NSStringFromGo(comment))))
	})
	if status != 0 {
		t.Fatalf("SecItemUpdate of the comment failed: %v", status)
	}

	if err := KeychainSetGenericPassword(liveKeychainService, account, nil); err != nil {
		t.Fatalf("KeychainSetGenericPassword(nil) failed: %v", err)
	}
	var got string
	WithPool(func() {
		var result objc.ID
		status = OSStatus(secItemCopyMatching(keychainQuery(liveKeychainService, account, kSecReturnAttributes(),
			NSNumberFromBool(true), kSecMatchLimit(), kSecMatchLimitOne()), &result))
		if result != 0 {
			got = GoStringFromNSString(NSDictionaryObjectForKey(result, kSecAttrComment()))
			Release(result)
		}
	})
	if status != 0 {
		t.Fatalf("SecItemCopyMatching of the attributes failed: %v", status)
	}
	if got != comment {
		t.Errorf("comment after KeychainSetGenericPassword(nil) = %q, want %q", got, comment)
	}
	data, err := KeychainGenericPassword(liveKeychainService, account)
	if err != nil {
		t.Fatalf("KeychainGenericPassword failed: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("KeychainGenericPassword = %q, want empty", data)
	}
}

// TestKeychainLiveSetRacesDelete checks that a Set never fails when a Delete of the same item races it.
func TestKeychainLiveSetRacesDelete(t *testing.T) {
	account := liveKeychainAccount(t)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if err := KeychainDeleteGenericPassword(liveKeychainService, account); err != nil &&
				!errors.Is(err, ErrSecItemNotFound) {
				t.Errorf("KeychainDeleteGenericPassword failed: %v", err)
				return
			}
		}
	}()
	payloads := [][]byte{[]byte("value"), nil}
	for i := range 100 {
		if err := KeychainSetGenericPassword(liveKeychainService, account, payloads[i%2]); err != nil {
			t.Errorf("KeychainSetGenericPassword on round %d failed: %v", i, err)
			break
		}
	}
	close(stop)
	<-done
}
