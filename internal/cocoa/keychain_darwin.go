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
	"slices"
	"strconv"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/errs"
)

// OSStatus is a Security framework result code. It implements error so a failed call can be tested with errors.Is.
type OSStatus int32

const (
	// ErrSecUserCanceled is errSecUserCanceled: the user canceled the operation.
	ErrSecUserCanceled OSStatus = -128
	// ErrSecNotAvailable is errSecNotAvailable: no keychain is available.
	ErrSecNotAvailable OSStatus = -25291
	// ErrSecAuthFailed is errSecAuthFailed: the user name or passphrase was incorrect.
	ErrSecAuthFailed OSStatus = -25293
	// ErrSecNoSuchKeychain is errSecNoSuchKeychain: the keychain does not exist.
	ErrSecNoSuchKeychain OSStatus = -25294
	// ErrSecDuplicateItem is errSecDuplicateItem: the item already exists.
	ErrSecDuplicateItem OSStatus = -25299
	// ErrSecItemNotFound is errSecItemNotFound: the item could not be found.
	ErrSecItemNotFound OSStatus = -25300
	// ErrSecNoDefaultKeychain is errSecNoDefaultKeychain: a default keychain does not exist.
	ErrSecNoDefaultKeychain OSStatus = -25307
	// ErrSecInteractionNotAllowed is errSecInteractionNotAllowed: user interaction was required but not allowed.
	ErrSecInteractionNotAllowed OSStatus = -25308
)

var (
	secOnce             sync.Once
	secItemAdd          func(attrs objc.ID, result *objc.ID) int32
	secItemCopyMatching func(query objc.ID, result *objc.ID) int32
	secItemUpdate       func(query, attrs objc.ID) int32
	secItemDelete       func(query objc.ID) int32
	secCopyErrorMessage func(status int32, reserved uintptr) objc.ID

	kSecClass                = secConstant("kSecClass")
	kSecClassGenericPassword = secConstant("kSecClassGenericPassword")
	kSecAttrService          = secConstant("kSecAttrService")
	kSecAttrAccount          = secConstant("kSecAttrAccount")
	kSecAttrLabel            = secConstant("kSecAttrLabel")
	kSecValueData            = secConstant("kSecValueData")
	kSecReturnData           = secConstant("kSecReturnData")
	kSecReturnAttributes     = secConstant("kSecReturnAttributes")
	kSecMatchLimit           = secConstant("kSecMatchLimit")
	kSecMatchLimitOne        = secConstant("kSecMatchLimitOne")
	kSecMatchLimitAll        = secConstant("kSecMatchLimitAll")
)

func ensureSecFuncs() {
	secOnce.Do(func() {
		lib := LoadFramework("Security")
		purego.RegisterLibFunc(&secItemAdd, lib, "SecItemAdd")
		purego.RegisterLibFunc(&secItemCopyMatching, lib, "SecItemCopyMatching")
		purego.RegisterLibFunc(&secItemUpdate, lib, "SecItemUpdate")
		purego.RegisterLibFunc(&secItemDelete, lib, "SecItemDelete")
		purego.RegisterLibFunc(&secCopyErrorMessage, lib, "SecCopyErrorMessageString")
	})
}

// secConstant returns a function that resolves the Security framework's CFStringRef constant named symbol on first use.
func secConstant(symbol string) func() objc.ID {
	return sync.OnceValue(func() objc.ID { return NSStringConstant("Security", symbol) })
}

// Error returns the Security framework's description of the status.
func (s OSStatus) Error() string {
	ensureSecFuncs()
	var msg string
	WithPool(func() {
		if str := secCopyErrorMessage(int32(s), 0); str != 0 {
			msg = GoStringFromNSString(str)
			Release(str)
		}
	})
	if msg == "" {
		return "OSStatus " + strconv.Itoa(int(s))
	}
	return msg
}

func keychainError(fn string, status OSStatus) error {
	return errs.NewWithCause(fn+" failed: "+status.Error(), status)
}

// keychainQuery returns an autoreleased dictionary that identifies the generic password for service and account,
// followed by the extra key/value pairs.
func keychainQuery(service, account string, extra ...objc.ID) objc.ID {
	return NSDictionaryFromPairs(append([]objc.ID{
		kSecClass(), kSecClassGenericPassword(),
		kSecAttrService(), NSStringFromGo(service),
		kSecAttrAccount(), NSStringFromGo(account),
	}, extra...)...)
}

// keychainSetAttempts bounds the update-then-add rounds of KeychainSetGenericPassword, which starts another when a
// writer adds the item between its SecItemUpdate and its SecItemAdd.
const keychainSetAttempts = 3

// KeychainSetGenericPassword stores secret as the generic password for service and account, replacing the data of any
// existing item on the keychain search list in place, so the item keeps its other attributes and its access list. A
// new item goes into the user's default keychain, normally the login keychain, which must be on the search list. A
// failure wraps the OSStatus.
func KeychainSetGenericPassword(service, account string, secret []byte) error {
	ensureSecFuncs()
	var fn string
	var status OSStatus
	WithPool(func() {
		data := keychainData(secret)
		attrs := keychainQuery(service, account, kSecAttrLabel(), NSStringFromGo(service), kSecValueData(), data)
		query := keychainQuery(service, account)
		update := NSDictionaryFromPairs(kSecValueData(), data)
		// SecItemAdd checks for a duplicate only in the default keychain, so updating first finds an item elsewhere.
		for range keychainSetAttempts {
			fn = "SecItemUpdate"
			if status = OSStatus(secItemUpdate(query, update)); status != ErrSecItemNotFound {
				return
			}
			fn = "SecItemAdd"
			if status = OSStatus(secItemAdd(attrs, nil)); status != ErrSecDuplicateItem {
				return
			}
		}
	})
	if status != 0 {
		return keychainError(fn, status)
	}
	return nil
}

// keychainData returns an autoreleased NSData holding a copy of secret. SecItemUpdate leaves the stored data unchanged
// when the new data's bytes pointer is NULL, as it is for an empty NSData, so an empty secret yields an empty
// NSMutableData that keeps the buffer it was created with.
func keychainData(secret []byte) objc.ID {
	if len(secret) != 0 {
		return NSDataFromGo(secret)
	}
	data := objc.ID(Cls("NSMutableData")).Send(Sel("dataWithLength:"), uint64(1))
	data.Send(Sel("setLength:"), uint64(0))
	return data
}

// KeychainGenericPassword returns the generic password stored for service and account. A missing item yields an
// error that wraps ErrSecItemNotFound; other failures wrap their OSStatus.
func KeychainGenericPassword(service, account string) ([]byte, error) {
	ensureSecFuncs()
	var status OSStatus
	var secret []byte
	WithPool(func() {
		var result objc.ID
		status = OSStatus(secItemCopyMatching(keychainQuery(service, account, kSecReturnData(), NSNumberFromBool(true),
			kSecMatchLimit(), kSecMatchLimitOne()), &result))
		if result != 0 {
			secret = GoBytesFromNSData(result)
			Release(result)
		}
	})
	if status != 0 {
		return nil, keychainError("SecItemCopyMatching", status)
	}
	if secret == nil {
		secret = []byte{}
	}
	return secret, nil
}

// KeychainDeleteGenericPassword removes the generic password stored for service and account. A missing item yields
// an error that wraps ErrSecItemNotFound; other failures wrap their OSStatus.
func KeychainDeleteGenericPassword(service, account string) error {
	ensureSecFuncs()
	var status OSStatus
	WithPool(func() {
		status = OSStatus(secItemDelete(keychainQuery(service, account)))
	})
	if status != 0 {
		return keychainError("SecItemDelete", status)
	}
	return nil
}

// KeychainGenericPasswordAccounts returns the sorted, de-duplicated accounts that have a generic password stored for
// service. A service with none yields nil. A failure wraps the OSStatus.
func KeychainGenericPasswordAccounts(service string) ([]string, error) {
	ensureSecFuncs()
	var status OSStatus
	var accounts []string
	WithPool(func() {
		var result objc.ID
		status = OSStatus(secItemCopyMatching(NSDictionaryFromPairs(kSecClass(), kSecClassGenericPassword(),
			kSecAttrService(), NSStringFromGo(service), kSecReturnAttributes(), NSNumberFromBool(true),
			kSecMatchLimit(), kSecMatchLimitAll()), &result))
		if result != 0 {
			for _, attrs := range IDsFromNSArray(result) {
				// An item another tool created may lack an account or hold a non-string one.
				if acct := NSDictionaryObjectForKey(attrs, kSecAttrAccount()); acct != 0 &&
					objc.Send[bool](acct, Sel("isKindOfClass:"), Cls("NSString")) {
					accounts = append(accounts, GoStringFromNSString(acct))
				}
			}
			Release(result)
		}
	})
	switch status {
	case 0:
	case ErrSecItemNotFound:
		return nil, nil
	default:
		return nil, keychainError("SecItemCopyMatching", status)
	}
	slices.Sort(accounts)
	return slices.Compact(accounts), nil
}
