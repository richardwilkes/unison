// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

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
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xruntime"
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

// PointFromNSPoint converts an NSPoint to a geom.Point.
func PointFromNSPoint(pt NSPoint) geom.Point {
	return geom.NewPoint(float32(pt.X), float32(pt.Y))
}

// NSPointFromPoint converts a geom.Point to an NSPoint.
func NSPointFromPoint(pt geom.Point) NSPoint {
	return NSPoint{X: float64(pt.X), Y: float64(pt.Y)}
}

// SizeFromNSSize converts an NSSize to a geom.Size.
func SizeFromNSSize(size NSSize) geom.Size {
	return geom.NewSize(float32(size.Width), float32(size.Height))
}

// RectFromNSRect converts an NSRect to a geom.Rect.
func RectFromNSRect(r NSRect) geom.Rect {
	return geom.NewRect(float32(r.Origin.X), float32(r.Origin.Y), float32(r.Size.Width), float32(r.Size.Height))
}

// NSRectFromRect converts a geom.Rect to an NSRect.
func NSRectFromRect(r geom.Rect) NSRect {
	return NSRect{
		Origin: NSPoint{X: float64(r.X), Y: float64(r.Y)},
		Size:   NSSize{Width: float64(r.Width), Height: float64(r.Height)},
	}
}

// NSUTF8StringEncoding is Foundation's constant for UTF-8 in the NSString*Encoding APIs.
const NSUTF8StringEncoding = 4

var (
	frameworkLock     sync.Mutex
	frameworkHandles  = make(map[string]uintptr)
	libObjCOnce       sync.Once
	libObjCHandle     uintptr
	poolOnce          sync.Once
	poolPushFunc      func() uintptr
	poolPopFunc       func(uintptr)
	superOnce         sync.Once
	msgSendSuper2Func func(super *objcSuper, cmd objc.SEL, args ...any) objc.ID
	selCache          sync.Map // map[string]objc.SEL
	clsCache          sync.Map // map[string]objc.Class
	appKitStringCache sync.Map // map[string]objc.ID
	axNotifyOnce      sync.Once
	axPostNotify      func(element, notification objc.ID)
	axPostNotifyInfo  func(element, notification, userInfo objc.ID)
)

// LoadFramework loads the named macOS system framework (e.g. "AppKit") into the process, making its symbols and
// Objective-C classes visible, and returns its dlopen handle for symbol lookup. It is safe to call from any
// goroutine and any number of times; each framework is loaded once. The frameworks this bridge asks for are present
// on every macOS installation, so a failure here is unrecoverable and panics.
func LoadFramework(name string) uintptr {
	frameworkLock.Lock()
	defer frameworkLock.Unlock()
	if handle, ok := frameworkHandles[name]; ok {
		return handle
	}
	handle, err := purego.Dlopen("/System/Library/Frameworks/"+name+".framework/"+name,
		purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		panic(fmt.Errorf("cocoa: unable to load %s: %w", name, err))
	}
	frameworkHandles[name] = handle
	return handle
}

// LoadAppKit loads the AppKit framework (and, transitively, Foundation and the other frameworks it links against)
// into the process, making its Objective-C classes visible to the runtime.
func LoadAppKit() {
	LoadFramework("AppKit")
}

// NSStringConstant returns the value of an exported NSString* constant (e.g. AppKit's NSCalibratedRGBColorSpace)
// from the given framework. The requested symbols are compile-time known and always present, so a resolution
// failure is a programming error and panics.
func NSStringConstant(framework, symbol string) objc.ID {
	ptr, err := purego.Dlsym(LoadFramework(framework), symbol)
	if err != nil || ptr == 0 {
		panic(fmt.Errorf("cocoa: unable to resolve %s in %s: %w", symbol, framework, err))
	}
	return *xruntime.PtrFromUintptr[objc.ID](ptr)
}

// AppKitString returns the value of an exported AppKit NSString* constant — a role, subrole or notification name, say —
// caching it on first use so that a symbol consulted on every query from an assistive technology is resolved once
// rather than dlsym'd over and over. The constants are immortal objects owned by AppKit, so the cached value stays
// valid for the life of the process. It panics if the symbol does not exist, since the names are compile-time known.
func AppKitString(symbol string) objc.ID {
	if v, ok := appKitStringCache.Load(symbol); ok {
		return v.(objc.ID) //nolint:errcheck // There is only one thing it can be
	}
	v := NSStringConstant("AppKit", symbol)
	appKitStringCache.Store(symbol, v)
	return v
}

// Sel returns the selector for name, caching the result since objc.RegisterName takes the global Objective-C
// runtime lock.
func Sel(name string) objc.SEL {
	if s, ok := selCache.Load(name); ok {
		return s.(objc.SEL) //nolint:errcheck // There is only one thing it can be
	}
	s := objc.RegisterName(name)
	selCache.Store(name, s)
	return s
}

// Cls returns the class object for name, loading AppKit first if needed and caching the result. It panics if the
// class does not exist, since sending messages to a nil class silently returns nil and hides the typo.
func Cls(name string) objc.Class {
	if c, ok := clsCache.Load(name); ok {
		return c.(objc.Class) //nolint:errcheck // There is only one thing it can be
	}
	LoadAppKit()
	c := objc.GetClass(name)
	if c == 0 {
		panic("cocoa: no Objective-C class named " + name)
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

// libObjC returns the dlopen handle for the Objective-C runtime library, loading it on first use. libobjc is part of
// every macOS installation, so a failure here is unrecoverable and panics.
func libObjC() uintptr {
	libObjCOnce.Do(func() {
		lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			panic(fmt.Errorf("cocoa: unable to load libobjc: %w", err))
		}
		libObjCHandle = lib
	})
	return libObjCHandle
}

func ensurePoolFuncs() {
	poolOnce.Do(func() {
		lib := libObjC()
		purego.RegisterLibFunc(&poolPushFunc, lib, "objc_autoreleasePoolPush")
		purego.RegisterLibFunc(&poolPopFunc, lib, "objc_autoreleasePoolPop")
	})
}

// objcSuper mirrors the Objective-C runtime's struct objc_super. objc_msgSendSuper2 (the variant the compiler emits
// for [super sel] on the modern runtime) expects superClass to be the class whose method is making the call; it looks
// up that class's superclass itself.
type objcSuper struct {
	receiver   objc.ID
	superClass objc.Class
}

func ensureSuperFunc() {
	superOnce.Do(func() {
		purego.RegisterLibFunc(&msgSendSuper2Func, libObjC(), "objc_msgSendSuper2")
	})
}

// SendSuper sends sel to the superclass implementation of a method defined on cls, with obj as the receiver. It is
// the equivalent of a compiled [super sel] inside one of cls's methods, and must be used instead of objc.ID.SendSuper
// from any method implementation registered with objc.RegisterClass.
//
// objc.ID.SendSuper resolves "super" from the receiver's runtime class (object_getClass). That is only correct while
// the receiver's runtime class is the class that defined the method. AppKit and Foundation routinely replace an
// object's class with a dynamically created subclass (KVO's NSKVONotifying_* classes are the common case), and once
// that has happened the receiver's "superclass" is the class that defined the method, so the "super" call dispatches
// straight back into the same Go implementation and recurses until the stack overflows. Passing the defining class
// explicitly, as the compiler does, avoids that.
func SendSuper(obj objc.ID, cls objc.Class, sel objc.SEL, args ...any) objc.ID {
	ensureSuperFunc()
	return msgSendSuper2Func(&objcSuper{receiver: obj, superClass: cls}, sel, args...)
}

// PoolPush pushes a new autorelease pool and returns its token. Every call must be balanced by a PoolPop of that
// token. Unlike Objective-C, Go code gets no implicit pools, so any code that runs Objective-C calls outside an
// event-loop turn should bracket them with PoolPush/PoolPop (or use WithPool). Autorelease pools are per-thread:
// the pop must happen on the same OS thread as the push, so a goroutine using this pair directly must be locked to
// its OS thread for the pool's whole lifetime. Prefer WithPool, which handles that.
func PoolPush() uintptr {
	ensurePoolFuncs()
	return poolPushFunc()
}

// PoolPop pops the autorelease pool identified by the token returned from the matching PoolPush.
func PoolPop(pool uintptr) {
	poolPopFunc(pool)
}

// WithPool runs f inside its own autorelease pool. The calling goroutine is locked to its OS thread for the
// duration, since a pool must be pushed and popped on the same thread and an unlocked goroutine can migrate between
// OS threads at any preemption point. (The cgo bridge never had this hazard: its @autoreleasepool blocks lived
// entirely inside single C calls.)
func WithPool(f func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := PoolPush()
	defer PoolPop(pool)
	f()
}

// NSStringFromGo returns an autoreleased NSString with the contents of s. It never returns nil: invalid UTF-8 byte
// sequences are replaced with the Unicode replacement character rather than being allowed to turn the whole string
// into a nil that would raise NSInvalidArgumentException when passed to AppKit (e.g. a clipboard write of a text
// datum containing invalid UTF-8).
func NSStringFromGo(s string) objc.ID {
	return Autorelease(NewNSString(s))
}

// NewNSString returns an owned (+1) NSString with the contents of s. Unlike NSStringFromGo, the result is not
// autoreleased, so it is safe to create from code that may run outside any autorelease pool; balance it with Release.
// Like NSStringFromGo, it never returns nil: invalid UTF-8 byte sequences are replaced with the Unicode replacement
// character.
func NewNSString(s string) objc.ID {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "�")
	}
	return objc.ID(Cls("NSString")).Send(Sel("alloc")).Send(Sel("initWithBytes:length:encoding:"),
		s, uint64(len(s)), uint64(NSUTF8StringEncoding))
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

// GoStringFromCString returns a Go string copied from a NUL-terminated C string. A nil pointer yields "".
func GoStringFromCString(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}

// GoBytesFromNSData returns a copy of an NSData's contents. A nil or empty NSData yields nil.
func GoBytesFromNSData(data objc.ID) []byte {
	if data == 0 {
		return nil
	}
	n := objc.Send[uint64](data, Sel("length"))
	if n == 0 {
		return nil
	}
	p := objc.Send[*byte](data, Sel("bytes"))
	if p == nil {
		return nil
	}
	out := make([]byte, n)
	copy(out, unsafe.Slice(p, n))
	return out
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

// NSDictionaryFromPairs returns an autoreleased NSDictionary built from alternating keys and values, as an
// Objective-C @{key: value} literal would be. A trailing key with no value is dropped, and no pairs at all yields an
// empty dictionary rather than nil, since AppKit's userInfo parameters reject nil.
func NSDictionaryFromPairs(pairs ...objc.ID) objc.ID {
	count := len(pairs) / 2
	if count == 0 {
		return objc.ID(Cls("NSDictionary")).Send(Sel("dictionary"))
	}
	keys := make([]objc.ID, count)
	objects := make([]objc.ID, count)
	for i := range count {
		keys[i] = pairs[i*2]
		objects[i] = pairs[i*2+1]
	}
	return objc.ID(Cls("NSDictionary")).Send(Sel("dictionaryWithObjects:forKeys:count:"),
		unsafe.Pointer(&objects[0]), unsafe.Pointer(&keys[0]), uint64(count))
}

// ensureAXNotifyFuncs binds the two NSAccessibility notification functions on first use, so a process that never
// serves an assistive technology never resolves them.
func ensureAXNotifyFuncs() {
	axNotifyOnce.Do(func() {
		lib := LoadFramework("AppKit")
		purego.RegisterLibFunc(&axPostNotify, lib, "NSAccessibilityPostNotification")
		purego.RegisterLibFunc(&axPostNotifyInfo, lib, "NSAccessibilityPostNotificationWithUserInfo")
	})
}

// NSAccessibilityPostNotification tells the accessibility system that something about element has changed. A nil
// element or notification is ignored, since AppKit raises on those. Must be called on the main thread.
func NSAccessibilityPostNotification(element, notification objc.ID) {
	if element == 0 || notification == 0 {
		return
	}
	ensureAXNotifyFuncs()
	axPostNotify(element, notification)
}

// NSAccessibilityPostNotificationWithUserInfo is NSAccessibilityPostNotification for the notifications that carry
// additional information, such as the text of an announcement. A nil element or notification is ignored. Must be
// called on the main thread.
func NSAccessibilityPostNotificationWithUserInfo(element, notification, userInfo objc.ID) {
	if element == 0 || notification == 0 {
		return
	}
	ensureAXNotifyFuncs()
	axPostNotifyInfo(element, notification, userInfo)
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

// NSNumberFromBool returns an autoreleased NSNumber holding the given boolean. It is not the same thing as
// NSNumberFromInt64 of 0 or 1: an NSNumber made this way is a boolean rather than an integer, and bridges to the
// CFBoolean that the accessibility attributes documented as holding one expect.
func NSNumberFromBool(value bool) objc.ID {
	return objc.ID(Cls("NSNumber")).Send(Sel("numberWithBool:"), value)
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

// NSDictionaryObjectForKey returns the object an NSDictionary holds for a key, or 0 when it holds none. A nil
// dictionary or key yields 0, so a caller reading an optional entry out of a dictionary an assistive technology handed
// it does not have to check either first.
func NSDictionaryObjectForKey(dict, key objc.ID) objc.ID {
	if dict == 0 || key == 0 {
		return 0
	}
	return dict.Send(Sel("objectForKey:"), key)
}

// GoStringsFromNSArray returns the elements of an NSArray of NSStrings as Go strings. A nil or empty array yields nil,
// and an element that is not a string is skipped rather than turned into an empty entry, since an array handed over by
// another process may hold anything at all.
func GoStringsFromNSArray(array objc.ID) []string {
	count := NSArrayCount(array)
	if count == 0 {
		return nil
	}
	strs := make([]string, 0, count)
	for i := range count {
		obj := NSArrayObjectAt(array, i)
		if obj == 0 || !objc.Send[bool](obj, Sel("isKindOfClass:"), Cls("NSString")) {
			continue
		}
		strs = append(strs, GoStringFromNSString(obj))
	}
	return strs
}

// NewNSMutableAttributedString returns an autoreleased NSMutableAttributedString holding the contents of s and no
// attributes.
func NewNSMutableAttributedString(s string) objc.ID {
	return Autorelease(objc.ID(Cls("NSMutableAttributedString")).Send(Sel("alloc")).Send(Sel("initWithString:"),
		NSStringFromGo(s)))
}

// NSMutableAttributedStringSetAttributes gives a range of an NSMutableAttributedString the attributes of a dictionary,
// replacing whatever attributes that range carried. The range is clamped to the string, and one beginning past its end
// sets nothing at all: setAttributes:range: raises NSRangeException for a range reaching past the content, and an
// Objective-C exception raised inside a Go callback cannot be caught and takes the process with it.
//
// It is deliberately this whole-dictionary setter rather than addAttribute:value:range:, which would place the 16-byte
// NSRange after four integer-register arguments and run into the purego amd64 struct-straddle bug documented at the top
// of this file. Here the range follows only three — self, the selector and the dictionary — so it is passed in
// registers the ABI agrees on. A caller wanting several attributes over one range builds the dictionary and sets them
// together, which is a single message rather than one per attribute anyway.
func NSMutableAttributedStringSetAttributes(str, attributes objc.ID, r NSRange) {
	if str == 0 || attributes == 0 {
		return
	}
	length := objc.Send[uint64](str, Sel("length"))
	if r.Location > length {
		return
	}
	if r.Length > length-r.Location {
		r.Length = length - r.Location
	}
	str.Send(Sel("setAttributes:range:"), attributes, r)
}

// NSAttributedStringAttributesAtIndex returns the attributes in force at a UTF-16 index of an NSAttributedString, or 0
// for a nil string or an index at or past its end — attributesAtIndex:effectiveRange: raises NSRangeException for one,
// and an exception raised inside a Go callback cannot be caught. It exists for the tests, which read back what the
// adapter wrote.
func NSAttributedStringAttributesAtIndex(str objc.ID, index uint64) objc.ID {
	if str == 0 || index >= objc.Send[uint64](str, Sel("length")) {
		return 0
	}
	var effective NSRange
	return str.Send(Sel("attributesAtIndex:effectiveRange:"), index, unsafe.Pointer(&effective))
}

// nsRangeEncoding is the Objective-C type encoding an NSValue holding an NSRange answers with, which is how one is told
// apart from an NSValue holding something else — an NSNumber, notably, which is an NSValue too and whose rangeValue
// raises. Only the prefix is compared, since the two fields are spelled by whichever integer type the range was made
// from.
const nsRangeEncoding = "{_NSRange="

// NSRangeFromNSValue returns the range an NSValue holds, and false for a nil object, an object that is not an NSValue,
// or an NSValue holding anything but a range. The type is checked through objCType, which every NSValue answers with
// the encoding of what it holds: asking one of another type for its range raises an Objective-C exception, which inside
// a Go callback cannot be caught and takes the process with it, and a value that arrived from another process cannot be
// taken on trust.
func NSRangeFromNSValue(value objc.ID) (r NSRange, ok bool) {
	if value == 0 || !objc.Send[bool](value, Sel("isKindOfClass:"), Cls("NSValue")) {
		return NSRange{}, false
	}
	if !strings.HasPrefix(GoStringFromCString(objc.Send[*byte](value, Sel("objCType"))), nsRangeEncoding) {
		return NSRange{}, false
	}
	return objc.Send[NSRange](value, Sel("rangeValue")), true
}

// BoolFromNSNumber returns the value of an NSNumber as a bool. A nil NSNumber yields false, which is what an absent
// flag in a dictionary an assistive technology handed over means.
func BoolFromNSNumber(num objc.ID) bool {
	if num == 0 {
		return false
	}
	return objc.Send[bool](num, Sel("boolValue"))
}

// NSURLFromString returns an autoreleased NSURL for the given string, or 0 when NSURL cannot parse it as one — which is
// the answer for an empty string and for anything else that is not a URL.
func NSURLFromString(url string) objc.ID {
	if url == "" {
		return 0
	}
	return objc.ID(Cls("NSURL")).Send(Sel("URLWithString:"), NSStringFromGo(url))
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
