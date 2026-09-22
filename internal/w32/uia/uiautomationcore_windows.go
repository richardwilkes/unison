// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package uia

import (
	"unsafe"

	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// Bindings for the UI Automation provider API. A (*LazyProc).Call panics when the export cannot be found, so anything
// that can be reached on a system without the API must check with Find first.
//
// RaiseNotificationEvent arrived in Windows 10 1709 and DisconnectProvider is newer than the rest of the provider
// API, so either may genuinely be missing even where the DLL is present.
// ClientsAreListening checks because it stands in for a presence test on the whole API: everything it gates is
// reached only after it has answered true. ReturnRawElementProvider checks because the adapter calls it on the
// teardown path — Window.Destroy withdraws the window's provider with it — and an adapter exists whenever
// accessibility was switched on, which the root package does from the environment at startup rather than in answer to a
// WM_GETOBJECT.
//
// The rest need no check: they are reachable only from inside a provider method, and the only way into a provider is a
// WM_GETOBJECT asking for RootObjectId, which nothing but UI Automation sends.
var (
	uiautomationcore                        = windows.NewLazySystemDLL("uiautomationcore.dll")
	clientsAreListeningProc                 = uiautomationcore.NewProc("UiaClientsAreListening")
	disconnectProviderProc                  = uiautomationcore.NewProc("UiaDisconnectProvider")
	getReservedMixedAttributeValueProc      = uiautomationcore.NewProc("UiaGetReservedMixedAttributeValue")
	getReservedNotSupportedValueProc        = uiautomationcore.NewProc("UiaGetReservedNotSupportedValue")
	hostProviderFromHwndProc                = uiautomationcore.NewProc("UiaHostProviderFromHwnd")
	raiseAutomationEventProc                = uiautomationcore.NewProc("UiaRaiseAutomationEvent")
	raiseAutomationPropertyChangedEventProc = uiautomationcore.NewProc("UiaRaiseAutomationPropertyChangedEvent")
	raiseNotificationEventProc              = uiautomationcore.NewProc("UiaRaiseNotificationEvent")
	raiseStructureChangedEventProc          = uiautomationcore.NewProc("UiaRaiseStructureChangedEvent")
	returnRawElementProviderProc            = uiautomationcore.NewProc("UiaReturnRawElementProvider")
)

// Rect is a rectangle in screen coordinates, given as an origin plus a size rather than as two corners. Unlike
// almost everything else in Win32 its members are doubles, since UI Automation works in device-independent units.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ns-uiautomationcore-uiarect
type Rect struct {
	// Left is the x coordinate of the rectangle's left edge.
	Left float64
	// Top is the y coordinate of the rectangle's top edge.
	Top float64
	// Width is how wide the rectangle is.
	Width float64
	// Height is how tall the rectangle is.
	Height float64
}

// Point is a point in screen coordinates. Like Rect its members are doubles, since UI Automation works in
// device-independent units, and it is the one argument in the whole provider API passed as a structure by value:
// ITextProvider::RangeFromPoint takes one. Which registers it arrives in differs between the two architectures, which
// is what text_point_windows_amd64.go and text_point_windows_arm64.go are for.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ns-uiautomationcore-uiapoint
type Point struct {
	// X is the horizontal coordinate.
	X float64
	// Y is the vertical coordinate.
	Y float64
}

// GetReservedNotSupportedValue returns the singleton UI Automation defines for "this provider will never supply this
// value", which is the only thing a text range may answer for an attribute it knows nothing about: an empty VARIANT
// means "ask the host provider instead" and a made-up value would be read as the truth about the text.
//
// The pointer is a process-wide singleton rather than an object with a lifetime, so it is stored into a VARIANT without
// adding a reference and must not be released — the one place in this package where that is right. UI Automation's
// documentation says so explicitly, and VariantClear on such a VARIANT is harmless because the singleton's Release does
// nothing.
//
// The export is looked up rather than called blind, because this is reached from inside a COM callback: Call panics
// when the DLL or the export is missing, and a panic crossing back into UI Automation is far worse than a refusal. A
// missing export answers w32.COM_E_NOTIMPL, which the caller reports as E_NOTSUPPORTED — the truth about the
// attribute either way. See storeReserved.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcoreapi/nf-uiautomationcoreapi-uiagetreservednotsupportedvalue
func GetReservedNotSupportedValue() (value *w32.Unknown, hr uintptr) {
	if getReservedNotSupportedValueProc.Find() != nil {
		return nil, uintptr(w32.COM_E_NOTIMPL)
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := getReservedNotSupportedValueProc.Call(uintptr(unsafe.Pointer(&value)))
	if !w32.HResultSucceeded(r) {
		return nil, r
	}
	return value, r
}

// GetReservedMixedAttributeValue returns the singleton UI Automation defines for "the text in this range does not
// agree about this attribute", which is how a client is told that a range covers more than one kind of text without
// having to be handed one of the values. The same reference rule applies as for GetReservedNotSupportedValue, and
// the export is looked up for the same reason.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcoreapi/nf-uiautomationcoreapi-uiagetreservedmixedattributevalue
func GetReservedMixedAttributeValue() (value *w32.Unknown, hr uintptr) {
	if getReservedMixedAttributeValueProc.Find() != nil {
		return nil, uintptr(w32.COM_E_NOTIMPL)
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := getReservedMixedAttributeValueProc.Call(uintptr(unsafe.Pointer(&value)))
	if !w32.HResultSucceeded(r) {
		return nil, r
	}
	return value, r
}

// ReturnRawElementProvider answers a WM_GETOBJECT message by handing UI Automation the fragment root for a window.
// The result is the w32.LRESULT the window procedure must return. Passing a nil provider with a zero wParam and
// lParam is the documented way to tell UI Automation that a window's provider is going away, which a window does as
// it is destroyed.
//
// That teardown call is why the export is looked up rather than called blind: it is made for any window whose adapter
// exists, and an adapter exists whenever accessibility was switched on, with no WM_GETOBJECT necessarily behind it. On
// a system without uiautomationcore.dll the result is zero, which is the w32.LRESULT for "not handled" and the right
// answer for a WM_GETOBJECT nothing can be provided for.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcoreapi/nf-uiautomationcoreapi-uiareturnrawelementprovider
func ReturnRawElementProvider(hwnd windows.HWND, wParam w32.WPARAM, lParam w32.LPARAM, provider unsafe.Pointer) w32.LRESULT {
	if returnRawElementProviderProc.Find() != nil {
		return 0
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := returnRawElementProviderProc.Call(uintptr(hwnd), uintptr(wParam), uintptr(lParam), uintptr(provider))
	return w32.LRESULT(r)
}

// HostProviderFromHwnd returns the system-supplied provider for a window, which a fragment root reports as its host
// provider so that UI Automation can fill in the window-level properties — the process id, the native window handle,
// the framework's own class name — without the fragment having to. The reference that comes back is the caller's to
// release.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcoreapi/nf-uiautomationcoreapi-uiahostproviderfromhwnd
func HostProviderFromHwnd(hwnd windows.HWND) (provider *w32.Unknown, hr uintptr) {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := hostProviderFromHwndProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&provider)))
	if !w32.HResultSucceeded(r) {
		return nil, r
	}
	return provider, r
}

// ClientsAreListening reports whether any UI Automation client has asked to be told about events. Every event this
// package raises is gated on it, so that a window whose provider was created by something other than a screen reader —
// the touch keyboard, or an inspection tool — costs nothing beyond building the snapshot.
//
// It also stands in for a presence check on the whole API: it returns false when uiautomationcore.dll or this export
// cannot be found, which keeps the raise functions below from being reached on a system that has neither.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcoreapi/nf-uiautomationcoreapi-uiaclientsarelistening
func ClientsAreListening() bool {
	if clientsAreListeningProc.Find() != nil {
		return false
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := clientsAreListeningProc.Call()
	return r != 0
}

// RaiseAutomationEvent tells listening clients that something happened to an element: it took the focus, it was
// invoked, it was selected, its window opened or closed. The provider must be the element the event is about.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcoreapi/nf-uiautomationcoreapi-uiaraiseautomationevent
func RaiseAutomationEvent(provider unsafe.Pointer, id EventID) uintptr {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := raiseAutomationEventProc.Call(uintptr(provider), uintptr(id))
	return r
}

// RaiseAutomationPropertyChangedEvent tells listening clients that one property of an element changed, and what it
// changed from and to. The property must be one the element actually supports, so a Toggle state change may only be
// raised on an element whose patterns include Toggle.
//
// The two VARIANTs are declared by value in the C signature. On both x64 and ARM64 a 24-byte structure is too large for
// a register pair, so the ABI passes it as a pointer to a copy the caller owns — which is exactly what the Go
// parameters are, hence the address of each is what goes across. The copies are never cleared here: a BSTR or interface
// reference inside one still belongs to whoever built the original VARIANT, which must clear it once this returns.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcoreapi/nf-uiautomationcoreapi-uiaraiseautomationpropertychangedevent
func RaiseAutomationPropertyChangedEvent(provider unsafe.Pointer, id PropertyID, oldValue, newValue VARIANT) uintptr {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := raiseAutomationPropertyChangedEventProc.Call(uintptr(provider), uintptr(id),
		uintptr(unsafe.Pointer(&oldValue)), uintptr(unsafe.Pointer(&newValue)))
	return r
}

// RaiseStructureChangedEvent tells listening clients that the tree beneath or around an element changed.
//
// Which element the provider must be depends on the change: for StructureChangeType_ChildAdded it is the child that
// appeared, reporting its own runtime identifier; for StructureChangeType_ChildRemoved it is the parent the child left,
// reporting the departed child's runtime identifier, since the child itself no longer exists to be asked; and for
// StructureChangeType_ChildrenInvalidated it is the parent whose children should all be read again, with no runtime
// identifier at all — pass a nil slice.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcoreapi/nf-uiautomationcoreapi-uiaraisestructurechangedevent
func RaiseStructureChangedEvent(provider unsafe.Pointer, changeType StructureChangeType, runtimeID []int32) uintptr {
	if len(runtimeID) == 0 {
		//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
		r, _, _ := raiseStructureChangedEventProc.Call(uintptr(provider), uintptr(changeType), 0, 0)
		return r
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := raiseStructureChangedEventProc.Call(uintptr(provider), uintptr(changeType),
		uintptr(unsafe.Pointer(&runtimeID[0])), uintptr(len(runtimeID)))
	return r
}

// RaiseNotificationEvent asks a client to announce a piece of text that does not correspond to any change in the
// tree, which is how an application says something out loud on its own initiative. The provider is normally the
// fragment root, since the announcement is about the window rather than about one element in it. Both BSTRs remain the
// caller's to free.
//
// The export arrived in Windows 10 1709, so it is looked up rather than called blind; on an older system the result is
// w32.COM_E_NOTIMPL and nothing is announced.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcoreapi/nf-uiautomationcoreapi-uiaraisenotificationevent
func RaiseNotificationEvent(provider unsafe.Pointer, kind NotificationKind, processing NotificationProcessing, displayString, activityID BSTR) uintptr {
	if raiseNotificationEventProc.Find() != nil {
		return uintptr(w32.COM_E_NOTIMPL)
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := raiseNotificationEventProc.Call(uintptr(provider), uintptr(kind), uintptr(processing),
		uintptr(displayString), uintptr(activityID))
	return r
}

// DisconnectProvider tells UI Automation to drop every reference it holds to a provider, so that a client still
// holding an element for something that no longer exists gets E_ELEMENTNOTAVAILABLE instead of a stale answer. It
// must be called before the provider's own references are released.
//
// The export is not present on every system this package supports, so it is looked up rather than called blind; when it
// is missing the result is w32.COM_E_NOTIMPL and the provider simply stays connected, answering
// E_ELEMENTNOTAVAILABLE from its own stale flag.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcoreapi/nf-uiautomationcoreapi-uiadisconnectprovider
func DisconnectProvider(provider unsafe.Pointer) uintptr {
	if disconnectProviderProc.Find() != nil {
		return uintptr(w32.COM_E_NOTIMPL)
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := disconnectProviderProc.Call(uintptr(provider))
	return r
}
