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
	"sync/atomic"
	"testing"

	"github.com/ebitengine/purego/objc"
)

var superTestCalls atomic.Int32

// TestSendSuperFromDynamicSubclass proves that SendSuper dispatches to the superclass of the class that defined the
// method even when the receiver's runtime class is a subclass of it. That is the situation created when AppKit or
// Foundation isa-swizzles an object (KVO's NSKVONotifying_* classes); resolving "super" from the receiver's runtime
// class instead, as objc.ID.SendSuper does, would call the same implementation again and recurse until the stack
// overflowed.
func TestSendSuperFromDynamicSubclass(t *testing.T) {
	var base objc.Class
	var err error
	base, err = objc.RegisterClass("cocoaSuperTestBase", Cls("NSObject"), nil, nil, []objc.MethodDef{
		{
			Cmd: Sel("description"),
			Fn: func(self objc.ID, _ objc.SEL) objc.ID {
				if superTestCalls.Add(1) > 1 {
					// Bail out rather than let a regression take the test process down with a stack overflow.
					return 0
				}
				return SendSuper(self, base, Sel("description"))
			},
		},
	})
	if err != nil {
		t.Fatalf("unable to register base class: %v", err)
	}
	sub, err := objc.RegisterClass("cocoaSuperTestSub", base, nil, nil, nil)
	if err != nil {
		t.Fatalf("unable to register subclass: %v", err)
	}
	WithPool(func() {
		obj := objc.ID(sub).Send(Sel("alloc")).Send(Sel("init"))
		defer Release(obj)
		desc := obj.Send(Sel("description"))
		if got := superTestCalls.Load(); got != 1 {
			t.Errorf("description implementation ran %d times, want 1", got)
		}
		if desc == 0 {
			t.Fatal("super description returned nil")
		}
		if s := GoStringFromNSString(desc); s == "" {
			t.Error("super description returned an empty string")
		}
	})
}
