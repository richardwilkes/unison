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
	"bytes"
	"errors"
	"time"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"golang.org/x/sys/windows"
)

// CredReadW and CredEnumerateW return CREDENTIALW structures that the system allocated and that must be released with
// CredFree. Their wrappers copy each one into a Go-owned Credential and free the original before returning, so no
// caller holds a pointer into memory the system may reclaim, or has to remember to free it. The copy leaves out the
// attributes and TargetAlias, so a credential written back with CredWriteW loses them.
//
// Every wrapper returns windows.ERROR_NO_SUCH_LOGON_SESSION, which errors.Is recognizes, when the logon session has no
// credential set, as no network logon does.

const (
	// CredTypeGeneric is CRED_TYPE_GENERIC: a credential no authentication package uses, read only by the application
	// that stores it.
	CredTypeGeneric = 1
	// CredPersistLocalMachine is CRED_PERSIST_LOCAL_MACHINE: the credential survives logoff and reboot on this computer
	// but does not roam to the user's other computers.
	CredPersistLocalMachine = 2
	// CredMaxCredentialBlobSize is CRED_MAX_CREDENTIAL_BLOB_SIZE, the largest blob CredWriteW accepts, in bytes.
	CredMaxCredentialBlobSize = 5 * 512
	// CredMaxUsernameLength is CRED_MAX_USERNAME_LENGTH, the longest UserName CredWriteW accepts, in UTF-16 code units.
	CredMaxUsernameLength = 256 + 1 + 256
	// CredMaxGenericTargetNameLength is CRED_MAX_GENERIC_TARGET_NAME_LENGTH, the longest TargetName CredWriteW accepts
	// for a generic credential, in UTF-16 code units.
	CredMaxGenericTargetNameLength = 32767
)

var (
	advapi32           = windows.NewLazySystemDLL("advapi32.dll")
	credWriteWProc     = advapi32.NewProc("CredWriteW")
	credReadWProc      = advapi32.NewProc("CredReadW")
	credDeleteWProc    = advapi32.NewProc("CredDeleteW")
	credEnumerateWProc = advapi32.NewProc("CredEnumerateW")
	credFreeProc       = advapi32.NewProc("CredFree")
)

// credentialW mirrors CREDENTIALW (80 bytes on 64-bit Windows).
type credentialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// Credential is a Go-owned copy of a CREDENTIALW; nothing in it points at OS memory. Blob is nil when the credential
// has none. CredWriteW ignores LastWritten, which the system sets.
type Credential struct {
	TargetName  string
	UserName    string
	Comment     string
	Blob        []byte
	LastWritten time.Time
	Type        uint32
	Persist     uint32
	Flags       uint32
}

// CredWriteW https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-credwritew
//
// Creates the credential, or replaces the one with the same TargetName and Type. An empty Comment or UserName is
// passed as NULL.
func CredWriteW(cred *Credential) error {
	target, err := windows.UTF16PtrFromString(cred.TargetName)
	if err != nil {
		return errs.Wrap(err)
	}
	var cw credentialW
	cw.Flags = cred.Flags
	cw.Type = cred.Type
	cw.TargetName = target
	cw.Persist = cred.Persist
	if cw.Comment, err = optionalUTF16Ptr(cred.Comment); err != nil {
		return err
	}
	if cw.UserName, err = optionalUTF16Ptr(cred.UserName); err != nil {
		return err
	}
	if len(cred.Blob) != 0 {
		cw.CredentialBlob = &cred.Blob[0]
		cw.CredentialBlobSize = uint32(len(cred.Blob))
	}
	r1, _, e := credWriteWProc.Call(uintptr(unsafe.Pointer(&cw)), 0)
	return credResult("CredWriteW", r1, e)
}

// CredReadW https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-credreadw
//
// A missing credential is reported as windows.ERROR_NOT_FOUND, which errors.Is recognizes.
func CredReadW(targetName string, credType uint32) (*Credential, error) {
	target, err := windows.UTF16PtrFromString(targetName)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	var p *credentialW
	r1, _, e := credReadWProc.Call(uintptr(unsafe.Pointer(target)), uintptr(credType), 0, uintptr(unsafe.Pointer(&p)))
	if err = credResult("CredReadW", r1, e); err != nil {
		return nil, err
	}
	defer credFree(unsafe.Pointer(p))
	return copyCredential(p), nil
}

// CredDeleteW https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-creddeletew
//
// A missing credential is reported as windows.ERROR_NOT_FOUND, which errors.Is recognizes.
func CredDeleteW(targetName string, credType uint32) error {
	target, err := windows.UTF16PtrFromString(targetName)
	if err != nil {
		return errs.Wrap(err)
	}
	r1, _, e := credDeleteWProc.Call(uintptr(unsafe.Pointer(target)), uintptr(credType), 0)
	return credResult("CredDeleteW", r1, e)
}

// CredEnumerateW https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-credenumeratew
//
// The filter matches target names, with a trailing '*' matching any suffix; an empty filter matches every credential.
// Finding no match returns nil and no error.
func CredEnumerateW(filter string) ([]*Credential, error) {
	var pattern *uint16
	if filter != "" {
		var err error
		if pattern, err = windows.UTF16PtrFromString(filter); err != nil {
			return nil, errs.Wrap(err)
		}
	}
	var count uint32
	var arr **credentialW
	r1, _, e := credEnumerateWProc.Call(uintptr(unsafe.Pointer(pattern)), 0, uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&arr)))
	if err := credResult("CredEnumerateW", r1, e); err != nil {
		if errors.Is(err, windows.ERROR_NOT_FOUND) {
			return nil, nil
		}
		return nil, err
	}
	defer credFree(unsafe.Pointer(arr))
	list := make([]*Credential, 0, count)
	for _, one := range unsafe.Slice(arr, count) {
		list = append(list, copyCredential(one))
	}
	return list, nil
}

// optionalUTF16Ptr returns nil for an empty string, which the credential functions read as an absent value.
func optionalUTF16Ptr(s string) (*uint16, error) {
	if s == "" {
		return nil, nil
	}
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return p, nil
}

// copyCredential copies c, which the system owns, into Go memory.
func copyCredential(c *credentialW) *Credential {
	cred := &Credential{
		TargetName:  windows.UTF16PtrToString(c.TargetName),
		UserName:    windows.UTF16PtrToString(c.UserName),
		Comment:     windows.UTF16PtrToString(c.Comment),
		LastWritten: time.Unix(0, c.LastWritten.Nanoseconds()),
		Type:        c.Type,
		Persist:     c.Persist,
		Flags:       c.Flags,
	}
	if c.CredentialBlobSize != 0 && c.CredentialBlob != nil {
		cred.Blob = bytes.Clone(unsafe.Slice(c.CredentialBlob, c.CredentialBlobSize))
	}
	return cred
}

// credFree releases a buffer CredReadW or CredEnumerateW allocated.
func credFree(p unsafe.Pointer) {
	//nolint:errcheck // CredFree returns nothing.
	credFreeProc.Call(uintptr(p))
}

// credResult turns the BOOL result and last error of a credential function into an error. ERROR_NOT_FOUND and
// ERROR_NO_SUCH_LOGON_SESSION are returned as themselves, so callers can test for them with errors.Is; lastError would
// discard the code.
func credResult(fn string, r1 uintptr, e error) error {
	if r1&0xff != 0 {
		return nil
	}
	var errno windows.Errno
	if errors.As(e, &errno) && (errno == windows.ERROR_NOT_FOUND || errno == windows.ERROR_NO_SUCH_LOGON_SESSION) {
		return errno
	}
	return lastError(fn, e, nil)
}
