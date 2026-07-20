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
	"os"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/drag"
)

// TestHResultSucceeded verifies that HRESULT success is judged by the SUCCEEDED() rule (high bit clear) rather than the
// BOOL idiom (ret&0xff != 0), which inverted the meaning: S_OK (0) read as failure and most failure codes read as
// success.
func TestHResultSucceeded(t *testing.T) {
	c := check.New(t)
	c.True(hresultSucceeded(0))                            // S_OK
	c.True(hresultSucceeded(1))                            // S_FALSE
	c.True(hresultSucceeded(0x00040100))                   // DRAGDROP_S_DROP
	c.False(hresultSucceeded(uintptr(uint32(0x80004005)))) // E_FAIL
	c.False(hresultSucceeded(uintptr(uint32(0x80070057)))) // E_INVALIDARG
	c.False(hresultSucceeded(uintptr(uint32(0x80263001)))) // DWM_E_COMPOSITIONDISABLED
}

// TestWglProcAddressValid verifies that all of wglGetProcAddress's documented failure sentinels (NULL, 1, 2, 3, and -1)
// are rejected, while a plausible function pointer is accepted.
func TestWglProcAddressValid(t *testing.T) {
	c := check.New(t)
	for _, sentinel := range []uintptr{0, 1, 2, 3, ^uintptr(0)} {
		c.False(wglProcAddressValid(sentinel))
	}
	c.True(wglProcAddressValid(0x7FF6D3C41000))
}

// TestDropEffectOpConversions exercises the mappings between drag.Op values and Windows DROPEFFECT values.
func TestDropEffectOpConversions(t *testing.T) {
	c := check.New(t)
	c.Equal(drag.Copy, dropEffectToOp(DropEffectCopy))
	c.Equal(drag.Move, dropEffectToOp(DropEffectMove))
	c.Equal(drag.Copy|drag.Move, dropEffectToOp(DropEffectCopy|DropEffectMove))
	c.Equal(drag.Op(0), dropEffectToOp(DropEffectNone))
	c.Equal(DropEffectCopy, opToDropEffect(drag.Copy))
	c.Equal(DropEffectMove, opToDropEffect(drag.Move))
	c.Equal(DropEffectNone, opToDropEffect(0))
	c.Equal(DropEffectCopy|DropEffectMove, OpMaskToDropEffect(drag.Copy|drag.Move))
	c.Equal(DropEffect(0), OpMaskToDropEffect(0))
}

// TestDropResultEffect verifies that an accepted drop reports the operation that was in force from the last
// DragEnter/DragOver rather than DROPEFFECT_NONE, which told a source performing a Move that nothing happened, so it
// never deleted the original. A refused drop must still report DROPEFFECT_NONE.
func TestDropResultEffect(t *testing.T) {
	c := check.New(t)
	c.Equal(DropEffectMove, dropResultEffect(true, drag.Move))
	c.Equal(DropEffectCopy, dropResultEffect(true, drag.Copy))
	c.Equal(DropEffectNone, dropResultEffect(true, 0))
	c.Equal(DropEffectNone, dropResultEffect(false, drag.Move))
	c.Equal(DropEffectNone, dropResultEffect(false, drag.Copy))
}

// TestSourceHygiene guards against reintroducing patterns this package must not contain: syscall.NewLazyDLL searches
// the application directory before the system directory (a DLL-planting vector — opengl32.dll is not a KnownDLL);
// SysAllocString created BSTRs that were never freed and are unnecessary for PCWSTR parameters; and CoInitializeEx
// with COINIT_MULTITHREADED on the STA UI thread only ever "worked" because it failed with RPC_E_CHANGED_MODE.
func TestSourceHygiene(t *testing.T) {
	c := check.New(t)
	entries, err := os.ReadDir(".")
	c.NoError(err)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, readErr := os.ReadFile(name)
		c.NoError(readErr)
		content := string(data)
		for _, forbidden := range []string{"syscall.NewLazyDLL(", "SysAllocString(", "CoInitializeEx("} {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s contains forbidden call %s", name, forbidden)
			}
		}
	}
}
