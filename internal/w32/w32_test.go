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
	"go/ast"
	"go/parser"
	"go/token"
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

// TestPointerLifetimeAndDeadCodeHygiene guards against reintroducing patterns this package has deliberately removed.
// runtime.KeepAlive: every native call goes through syscall.SyscallN or x/sys/windows (*LazyProc).Call/(*Proc).Call,
// all marked //go:uintptrescapes, so a uintptr(unsafe.Pointer(p)) conversion written in the call's argument list is
// already kept alive for the call (see the package documentation); a KeepAlive therefore signals a conversion hoisted
// out of the call expression, the one pattern that genuinely is unsafe. WM_CAP_/WM_DM_/WM_PLAYBACK_: video-capture
// and DirectShow macro sets that once padded the window-message constants despite not being window messages at all.
// maxUint16Array: DisplayName built an unsafe.Slice of ~2^30 elements over a small CoTaskMem allocation, violating
// the unsafe.Slice contract; windows.UTF16PtrToString is the correct tool.
func TestPointerLifetimeAndDeadCodeHygiene(t *testing.T) {
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
		for _, forbidden := range []string{"runtime.KeepAlive(", "WM_CAP_", "WM_DM_", "WM_PLAYBACK_", "maxUint16Array"} {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s contains forbidden pattern %s", name, forbidden)
			}
		}
	}
}

// parsePackageSources parses every non-test .go file in this directory, regardless of build constraints, so hygiene
// checks cover the platform-specific files no matter where the tests run.
func parsePackageSources(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	c := check.New(t)
	entries, err := os.ReadDir(".")
	c.NoError(err)
	fset := token.NewFileSet()
	files := make(map[string]*ast.File)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, name, nil, 0)
		c.NoError(parseErr)
		files[name] = file
	}
	return fset, files
}

// isUintptrUnsafePointerConversion reports whether e has the form uintptr(unsafe.Pointer(...)).
func isUintptrUnsafePointerConversion(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	fn, ok := call.Fun.(*ast.Ident)
	if !ok || fn.Name != "uintptr" {
		return false
	}
	inner, ok := call.Args[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := inner.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Pointer" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "unsafe"
}

// TestNoHoistedUnsafePointerConversions guards against storing uintptr(unsafe.Pointer(p)) in a variable before a
// native call. The keep-alive exemption only applies to conversions written directly in a //go:uintptrescapes call's
// argument list (see the package documentation); once the result sits in a local, p is dead to the GC and its memory
// can be collected while the kernel call is still using it. Assignments to struct fields (e.g. the lpVtbl slots,
// which point at package-level vtables) and through pointer dereferences (COM out-parameters) are the deliberate,
// pinned-or-global exceptions and remain allowed.
func TestNoHoistedUnsafePointerConversions(t *testing.T) {
	fset, files := parsePackageSources(t)
	for name, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch stmt := n.(type) {
			case *ast.AssignStmt:
				if len(stmt.Lhs) != len(stmt.Rhs) {
					return true
				}
				for i, rhs := range stmt.Rhs {
					if _, isIdent := stmt.Lhs[i].(*ast.Ident); isIdent && isUintptrUnsafePointerConversion(rhs) {
						t.Errorf("%s: %s: uintptr(unsafe.Pointer(...)) hoisted into a variable; write the conversion inline in the call's argument list",
							name, fset.Position(rhs.Pos()))
					}
				}
			case *ast.ValueSpec:
				for _, value := range stmt.Values {
					if isUintptrUnsafePointerConversion(value) {
						t.Errorf("%s: %s: uintptr(unsafe.Pointer(...)) hoisted into a variable; write the conversion inline in the call's argument list",
							name, fset.Position(value.Pos()))
					}
				}
			}
			return true
		})
	}
}

// TestUnpinRequiresFinalComRelease guards against unpinning a Go-implemented COM object anywhere but the spot where
// comRelease reports the final reference dropped. An unconditional Unpin (as DropTarget.Revoke once did) removes the
// sole thing keeping the object alive while OLE may still hold AddRef'd pointers to it, so a later callback
// dereferences freed memory.
func TestUnpinRequiresFinalComRelease(t *testing.T) {
	fset, files := parsePackageSources(t)
	for name, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			var unpins []token.Pos
			releasesCount := false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, isCall := n.(*ast.CallExpr)
				if !isCall {
					return true
				}
				switch fun := call.Fun.(type) {
				case *ast.SelectorExpr:
					if fun.Sel.Name == "Unpin" {
						unpins = append(unpins, call.Pos())
					}
				case *ast.Ident:
					if fun.Name == "comRelease" {
						releasesCount = true
					}
				}
				return true
			})
			if len(unpins) != 0 && !releasesCount {
				for _, pos := range unpins {
					t.Errorf("%s: %s: Unpin called in %s, which never consults comRelease; unpin only when the final COM reference is released",
						name, fset.Position(pos), fn.Name.Name)
				}
			}
		}
	}
}
