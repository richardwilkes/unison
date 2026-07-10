// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package mac

// This file is the shared Objective-C helper layer for the purego-based (cgo-free) macOS bridge. It supplements
// github.com/ebitengine/purego/objc, which is used directly for class registration, selectors, and message sends.
//
// Requires purego v0.11.0-alpha.6 or later: earlier releases cannot pass or return structs through callbacks, which
// makes Objective-C method implementations like drawRect: (NSRect argument) or markedRange (NSRange return)
// impossible to write in Go. The feasibility of the struct paths used here was verified on both darwin/arm64 and
// darwin/amd64 by the Phase 0 spike described in plan.md/progress.md.
//
// Known constraint (verified empirically): purego v0.11.0-alpha.6 has a call-side bug on amd64 where a 16-byte
// struct argument that no longer fits entirely in the remaining integer registers is split across a register and
// the stack instead of being placed wholly on the stack as the System V ABI requires. Receiving such arguments in
// Go method implementations works correctly (AppKit is the caller there); only purego-initiated calls are affected.
// Avoid objc.ID.Send calls that place a struct argument after four or more preceding integer-register arguments
// (self and _cmd count), or pass such arguments by pointer through an NSInvocation instead.

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// Struct types matching the Apple 64-bit ABI. CGFloat and NSUInteger are both 8 bytes on every supported macOS
// architecture. The Go type names are chosen so that the Objective-C type encodings purego derives from them
// resemble the real Foundation encodings (e.g. {NSRect={NSPoint=dd}{NSSize=dd}}).
type (
	// NSPoint mirrors Foundation's NSPoint/CGPoint.
	NSPoint struct {
		X float64
		Y float64
	}
	// NSSize mirrors Foundation's NSSize/CGSize.
	NSSize struct {
		Width  float64
		Height float64
	}
	// NSRect mirrors Foundation's NSRect/CGRect.
	NSRect struct {
		Origin NSPoint
		Size   NSSize
	}
	// NSRange mirrors Foundation's NSRange.
	NSRange struct {
		Location uint64
		Length   uint64
	}
)

// NSUTF8StringEncoding is Foundation's constant for UTF-8 in the NSString*Encoding APIs.
const NSUTF8StringEncoding = 4

var (
	appKitOnce   sync.Once
	poolOnce     sync.Once
	poolPushFunc func() uintptr
	poolPopFunc  func(uintptr)
	selCache     sync.Map // map[string]objc.SEL
	clsCache     sync.Map // map[string]objc.Class
)

// LoadAppKit loads the AppKit framework (and, transitively, Foundation and the other frameworks it links against)
// into the process, making its Objective-C classes visible to the runtime. It is safe to call from any goroutine
// and any number of times; the load happens once. AppKit is present on every macOS installation, so a failure here
// is unrecoverable and panics.
func LoadAppKit() {
	appKitOnce.Do(func() {
		if _, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit",
			purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			panic(fmt.Errorf("mac: unable to load AppKit: %w", err))
		}
	})
}

// Sel returns the selector for name, caching the result since objc.RegisterName takes the global Objective-C
// runtime lock.
func Sel(name string) objc.SEL {
	if s, ok := selCache.Load(name); ok {
		return s.(objc.SEL)
	}
	s := objc.RegisterName(name)
	selCache.Store(name, s)
	return s
}

// Cls returns the class object for name, loading AppKit first if needed and caching the result. It panics if the
// class does not exist, since sending messages to a nil class silently returns nil and hides the typo.
func Cls(name string) objc.Class {
	if c, ok := clsCache.Load(name); ok {
		return c.(objc.Class)
	}
	LoadAppKit()
	c := objc.GetClass(name)
	if c == 0 {
		panic("mac: no Objective-C class named " + name)
	}
	clsCache.Store(name, c)
	return c
}

// Retain sends retain to obj and returns it. Sending to a nil object is a harmless no-op, matching Objective-C
// semantics.
func Retain(obj objc.ID) objc.ID {
	return obj.Send(Sel("retain"))
}

// Release sends release to obj. Sending to a nil object is a harmless no-op.
func Release(obj objc.ID) {
	obj.Send(Sel("release"))
}

// Autorelease sends autorelease to obj and returns it.
func Autorelease(obj objc.ID) objc.ID {
	return obj.Send(Sel("autorelease"))
}

func ensurePoolFuncs() {
	poolOnce.Do(func() {
		lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			panic(fmt.Errorf("mac: unable to load libobjc: %w", err))
		}
		purego.RegisterLibFunc(&poolPushFunc, lib, "objc_autoreleasePoolPush")
		purego.RegisterLibFunc(&poolPopFunc, lib, "objc_autoreleasePoolPop")
	})
}

// PoolPush pushes a new autorelease pool and returns its token. Every call must be balanced by a PoolPop of that
// token. Unlike Objective-C, Go code gets no implicit pools, so any code that runs Objective-C calls outside an
// event-loop turn should bracket them with PoolPush/PoolPop (or use WithPool).
func PoolPush() uintptr {
	ensurePoolFuncs()
	return poolPushFunc()
}

// PoolPop pops the autorelease pool identified by the token returned from the matching PoolPush.
func PoolPop(pool uintptr) {
	poolPopFunc(pool)
}

// WithPool runs f inside its own autorelease pool.
func WithPool(f func()) {
	pool := PoolPush()
	defer PoolPop(pool)
	f()
}

// NSStringFromGo returns an autoreleased NSString with the contents of s.
func NSStringFromGo(s string) objc.ID {
	return objc.ID(Cls("NSString")).Send(Sel("stringWithUTF8String:"), s)
}

// GoStringFromNSString returns the contents of the given NSString as a Go string. A nil NSString yields "".
func GoStringFromNSString(str objc.ID) string {
	if str == 0 {
		return ""
	}
	n := objc.Send[uint64](str, Sel("lengthOfBytesUsingEncoding:"), uint64(NSUTF8StringEncoding))
	if n == 0 {
		return ""
	}
	p := objc.Send[*byte](str, Sel("UTF8String"))
	if p == nil {
		return ""
	}
	return string(unsafe.Slice(p, n))
}

// NSArrayFromIDs returns an autoreleased NSArray containing the given objects.
func NSArrayFromIDs(ids ...objc.ID) objc.ID {
	if len(ids) == 0 {
		return objc.ID(Cls("NSArray")).Send(Sel("array"))
	}
	return objc.ID(Cls("NSArray")).Send(Sel("arrayWithObjects:count:"), unsafe.Pointer(&ids[0]), uint64(len(ids)))
}

// NSArrayCount returns the number of elements in an NSArray. A nil array yields 0.
func NSArrayCount(array objc.ID) uint64 {
	if array == 0 {
		return 0
	}
	return objc.Send[uint64](array, Sel("count"))
}

// NSArrayObjectAt returns the element of an NSArray at the given index.
func NSArrayObjectAt(array objc.ID, index uint64) objc.ID {
	return array.Send(Sel("objectAtIndex:"), index)
}

// IDsFromNSArray returns the elements of an NSArray as a Go slice. A nil array yields nil.
func IDsFromNSArray(array objc.ID) []objc.ID {
	count := NSArrayCount(array)
	if count == 0 {
		return nil
	}
	ids := make([]objc.ID, count)
	for i := range count {
		ids[i] = NSArrayObjectAt(array, i)
	}
	return ids
}

// NSNumberFromInt64 returns an autoreleased NSNumber holding the given value.
func NSNumberFromInt64(value int64) objc.ID {
	return objc.ID(Cls("NSNumber")).Send(Sel("numberWithLongLong:"), value)
}

// Int64FromNSNumber returns the value of an NSNumber as an int64. A nil NSNumber yields 0.
func Int64FromNSNumber(num objc.ID) int64 {
	if num == 0 {
		return 0
	}
	return objc.Send[int64](num, Sel("longLongValue"))
}

// NSNumberFromFloat64 returns an autoreleased NSNumber holding the given value.
func NSNumberFromFloat64(value float64) objc.ID {
	return objc.ID(Cls("NSNumber")).Send(Sel("numberWithDouble:"), value)
}

// Float64FromNSNumber returns the value of an NSNumber as a float64. A nil NSNumber yields 0.
func Float64FromNSNumber(num objc.ID) float64 {
	if num == 0 {
		return 0
	}
	return objc.Send[float64](num, Sel("doubleValue"))
}

// NSURLFromFilePath returns an autoreleased file NSURL for the given path.
func NSURLFromFilePath(path string) objc.ID {
	return objc.ID(Cls("NSURL")).Send(Sel("fileURLWithPath:"), NSStringFromGo(path))
}

// FilePathFromNSURL returns the path component of an NSURL (for file URLs, the file-system path). A nil NSURL
// yields "". Note that macOS reports paths in decomposed Unicode form (NFD), so the returned string may not be
// byte-identical to the NFC form the path was created with, even though both name the same file.
func FilePathFromNSURL(url objc.ID) string {
	if url == 0 {
		return ""
	}
	return GoStringFromNSString(url.Send(Sel("path")))
}
