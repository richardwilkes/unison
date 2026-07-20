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
	"sync"
	"testing"

	"github.com/ebitengine/purego/objc"
)

var (
	deallocSpyClassOnce sync.Once
	deallocSpyClass     objc.Class
	deallocSpyClassErr  error
	// deallocSpyFlags maps live spy instances to the flag their dealloc sets. Only touched from the main thread.
	deallocSpyFlags = make(map[objc.ID]*bool)
)

// newDeallocSpy returns a new instance of a class whose dealloc sets *flag, so tests can prove that an object graph
// holding the spy was actually deallocated rather than merely released once.
func newDeallocSpy(t *testing.T, flag *bool) objc.ID {
	t.Helper()
	deallocSpyClassOnce.Do(func() {
		deallocSpyClass, deallocSpyClassErr = objc.RegisterClass("TestDeallocSpy", Cls("NSObject"), nil, nil,
			[]objc.MethodDef{{
				Cmd: Sel("dealloc"),
				Fn: func(self objc.ID, _ objc.SEL) {
					if f, ok := deallocSpyFlags[self]; ok {
						*f = true
						delete(deallocSpyFlags, self)
					}
					objc.SendSuper[objc.ID](self, Sel("dealloc"))
				},
			}})
	})
	if deallocSpyClassErr != nil {
		t.Fatal(deallocSpyClassErr)
	}
	spy := objc.ID(deallocSpyClass).Send(Sel("new"))
	deallocSpyFlags[spy] = flag
	return spy
}

// attachDeallocSpy ties a dealloc spy to the menu item's lifetime via its representedObject (a strong property), so
// *flag flips exactly when the item is deallocated.
func attachDeallocSpy(t *testing.T, item MenuItem, flag *bool) {
	t.Helper()
	spy := newDeallocSpy(t, flag)
	objc.ID(item).Send(Sel("setRepresentedObject:"), spy)
	Release(spy) // the item now holds the only reference
}

func TestNewMenuBasics(t *testing.T) {
	runOnMain(func() {
		sharedApp()
		// Normalization-stable characters only (ASCII, CJK); AppKit may hand strings back in NFD form.
		for _, title := range []string{"Test Menu", "漢字 メニュー", ""} {
			m := NewMenu(title, nil)
			if m == 0 {
				t.Fatalf("NewMenu(%q) returned 0", title)
			}
			if got := m.Title(); got != title {
				t.Errorf("Title() = %q, want %q", got, title)
			}
			if got := m.NumberOfItems(); got != 0 {
				t.Errorf("NumberOfItems() = %d for a fresh menu, want 0", got)
			}
			// Every menu gets the shared MenuDelegate so AppKit sends menuNeedsUpdate: at tracking start, whether or
			// not an updater was registered.
			delegate := objc.ID(m).Send(Sel("delegate"))
			if delegate == 0 {
				t.Fatal("menu has no delegate")
			}
			if delegate != menuDelegate() {
				t.Errorf("menu delegate = %#x, want the shared MenuDelegate %#x", delegate, menuDelegate())
			}
			if !objc.Send[bool](delegate, Sel("respondsToSelector:"), Sel("menuNeedsUpdate:")) {
				t.Error("MenuDelegate does not respond to menuNeedsUpdate:")
			}
			m.Release()
		}
	})
}

func TestMenuStructure(t *testing.T) {
	runOnMain(func() {
		sharedApp()
		m := NewMenu("Structure", nil)
		defer m.Release()
		mi1 := NewMenuItem(101, "First", "", 0, nil, nil)
		mi2 := NewMenuItem(102, "Second", "", 0, nil, nil)
		if got := mi1.Menu(); got != 0 {
			t.Errorf("Menu() = %#x before insertion, want 0", got)
		}
		m.InsertItemAtIndex(mi1, 0)
		m.InsertItemAtIndex(mi2, 1)
		sep := NewSeparatorMenuItem()
		m.InsertItemAtIndex(sep, 1) // between the two items
		if got := m.NumberOfItems(); got != 3 {
			t.Fatalf("NumberOfItems() = %d, want 3", got)
		}
		if got := m.ItemAtIndex(0); got != mi1 {
			t.Errorf("ItemAtIndex(0) = %#x, want %#x", got, mi1)
		}
		if got := m.ItemAtIndex(2); got != mi2 {
			t.Errorf("ItemAtIndex(2) = %#x, want %#x", got, mi2)
		}
		if !m.ItemAtIndex(1).IsSeparatorItem() {
			t.Error("ItemAtIndex(1).IsSeparatorItem() = false, want true")
		}
		if mi1.IsSeparatorItem() {
			t.Error("IsSeparatorItem() = true for a regular item")
		}
		if got := mi1.Menu(); got != m {
			t.Errorf("Menu() = %#x after insertion, want %#x", got, m)
		}

		// Submenu wiring, the shape root uses for nested menus. SetSubMenu transfers ownership of the submenu to the
		// item, so no Release of sub is needed (or permitted) here.
		if got := mi1.SubMenu(); got != 0 {
			t.Errorf("SubMenu() = %#x before SetSubMenu, want 0", got)
		}
		sub := NewMenu("Sub", nil)
		mi1.SetSubMenu(sub)
		if got := mi1.SubMenu(); got != sub {
			t.Errorf("SubMenu() = %#x, want %#x", got, sub)
		}

		m.RemoveItemAtIndex(1)
		if got := m.NumberOfItems(); got != 2 {
			t.Errorf("NumberOfItems() = %d after RemoveItemAtIndex, want 2", got)
		}
		if got := m.ItemAtIndex(1); got != mi2 {
			t.Errorf("ItemAtIndex(1) = %#x after removal, want %#x", got, mi2)
		}
		// The menu owns its items, so RemoveAll destroys them; keep a reference to mi2 across the removal so its
		// menu back-pointer can still be checked afterward.
		Retain(objc.ID(mi2))
		m.RemoveAll()
		if got := m.NumberOfItems(); got != 0 {
			t.Errorf("NumberOfItems() = %d after RemoveAll, want 0", got)
		}
		if got := mi2.Menu(); got != 0 {
			t.Errorf("Menu() = %#x after RemoveAll, want 0", got)
		}
		Release(objc.ID(mi2))
	})
}

// TestMenuNeedsUpdateRouting proves the Go-registered MenuDelegate routes menuNeedsUpdate: to the updater registered
// for exactly the menu being updated. AppKit only sends the message at the start of a real (user-interactive)
// tracking session, so the test drives the delegate through objc_msgSend the way AppKit would.
func TestMenuNeedsUpdateRouting(t *testing.T) {
	runOnMain(func() {
		sharedApp()
		var updated []Menu
		m1 := NewMenu("Updating", func(m Menu) { updated = append(updated, m) })
		defer m1.Release()
		m2 := NewMenu("NoUpdater", nil)
		defer m2.Release()
		delegate := objc.ID(m1).Send(Sel("delegate"))
		if delegate == 0 {
			t.Fatal("menu has no delegate")
		}
		delegate.Send(Sel("menuNeedsUpdate:"), objc.ID(m1))
		if len(updated) != 1 || updated[0] != m1 {
			t.Errorf("updater calls after m1 update = %v, want [%#x]", updated, m1)
		}
		// A menu without an updater must be a safe no-op and must not reach some other menu's updater.
		delegate.Send(Sel("menuNeedsUpdate:"), objc.ID(m2))
		if len(updated) != 1 {
			t.Errorf("updater calls after m2 update = %v, want just [%#x]", updated, m1)
		}
	})
}

func TestMenuItemAccessors(t *testing.T) {
	runOnMain(func() {
		sharedApp()
		mods := EventModifierFlagCommand | EventModifierFlagShift
		mi := NewMenuItem(42, "Item Title", "a", mods, nil, nil)
		if mi == 0 {
			t.Fatal("NewMenuItem returned 0")
		}
		defer mi.Release() // never inserted into a menu, so the creation reference must be balanced here
		if got := mi.Tag(); got != 42 {
			t.Errorf("Tag() = %d, want 42", got)
		}
		if got := mi.Title(); got != "Item Title" {
			t.Errorf("Title() = %q, want %q", got, "Item Title")
		}
		mi.SetTitle("漢字 タイトル")
		if got := mi.Title(); got != "漢字 タイトル" {
			t.Errorf("Title() = %q after SetTitle, want %q", got, "漢字 タイトル")
		}
		key, gotMods := mi.KeyBinding()
		if key != "a" || gotMods != mods {
			t.Errorf("KeyBinding() = %q/%#x, want %q/%#x", key, gotMods, "a", mods)
		}
		// Function-key equivalents use the Unicode private-use code points root maps key codes to.
		mi.SetKeyBinding("", EventModifierFlagOption)
		key, gotMods = mi.KeyBinding()
		if key != "" || gotMods != EventModifierFlagOption {
			t.Errorf("KeyBinding() = %q/%#x after SetKeyBinding, want %q/%#x", key, gotMods, "",
				EventModifierFlagOption)
		}
		// The action/target wiring NewMenuItem promises: action handleMenuItem:, target the shared MenuItemDelegate.
		if got := objc.SEL(objc.ID(mi).Send(Sel("action"))); got != Sel("handleMenuItem:") {
			t.Errorf("action = %v, want handleMenuItem:", got)
		}
		if got := objc.ID(mi).Send(Sel("target")); got != menuItemDelegate() {
			t.Errorf("target = %#x, want the shared MenuItemDelegate %#x", got, menuItemDelegate())
		}
		for _, state := range []ControlStateValue{ControlStateValueOn, ControlStateValueMixed, ControlStateValueOff} {
			mi.SetState(state)
			if got := mi.State(); got != state {
				t.Errorf("State() = %d, want %d", got, state)
			}
		}
	})
}

// TestMenuItemValidation proves AppKit's menu auto-enablement reaches the Go-implemented validateMenuItem: through
// real dispatch: [menu update] walks the items, asks each item's target to validate it, and enables or disables the
// item based on the answer. Items without a registered validator must default to enabled.
func TestMenuItemValidation(t *testing.T) {
	runOnMain(func() {
		sharedApp()
		m := NewMenu("Validation", nil)
		defer m.Release()
		var validated []int
		miEnabled := NewMenuItem(201, "Enabled", "", 0, func(item MenuItem) bool {
			validated = append(validated, item.Tag())
			return true
		}, nil)
		miDisabled := NewMenuItem(202, "Disabled", "", 0, func(item MenuItem) bool {
			validated = append(validated, item.Tag())
			return false
		}, nil)
		miDefault := NewMenuItem(203, "Default", "", 0, nil, nil)
		m.InsertItemAtIndex(miEnabled, 0)
		m.InsertItemAtIndex(miDisabled, 1)
		m.InsertItemAtIndex(miDefault, 2)
		if !objc.Send[bool](objc.ID(m), Sel("autoenablesItems")) {
			t.Fatal("autoenablesItems = false; the validation path under test would never run")
		}
		WithPool(func() {
			objc.ID(m).Send(Sel("update"))
		})
		if len(validated) != 2 {
			t.Fatalf("validators ran for tags %v, want [201 202] in some order", validated)
		}
		if got := objc.Send[bool](objc.ID(miEnabled), Sel("isEnabled")); !got {
			t.Error("item with true-returning validator is disabled after [menu update]")
		}
		if got := objc.Send[bool](objc.ID(miDisabled), Sel("isEnabled")); got {
			t.Error("item with false-returning validator is enabled after [menu update]")
		}
		if got := objc.Send[bool](objc.ID(miDefault), Sel("isEnabled")); !got {
			t.Error("item without a validator is disabled after [menu update], want the default of enabled")
		}
	})
}

// TestMenuItemAction proves a menu item's action fires the registered handler through AppKit's own dispatch:
// performActionForItemAtIndex: sends the item's action (handleMenuItem:) to its target (the shared
// MenuItemDelegate) via NSApplication's action routing, exactly as choosing the item from a menu would.
func TestMenuItemAction(t *testing.T) {
	runOnMain(func() {
		sharedApp()
		m := NewMenu("Action", nil)
		defer m.Release()
		var handled []int
		mi := NewMenuItem(301, "Do It", "", 0, nil, func(item MenuItem) { handled = append(handled, item.Tag()) })
		miNoHandler := NewMenuItem(302, "Inert", "", 0, nil, nil)
		m.InsertItemAtIndex(mi, 0)
		m.InsertItemAtIndex(miNoHandler, 1)
		WithPool(func() {
			objc.ID(m).Send(Sel("performActionForItemAtIndex:"), int64(0))
			// An item without a registered handler must be a safe no-op.
			objc.ID(m).Send(Sel("performActionForItemAtIndex:"), int64(1))
		})
		if len(handled) != 1 || handled[0] != 301 {
			t.Errorf("handler calls = %v, want [301]", handled)
		}
	})
}

// TestMenuReleaseCleansUpRegistrations proves Menu.Release removes the menu's updater and every contained item's
// validator and handler from the registration maps.
func TestMenuReleaseCleansUpRegistrations(t *testing.T) {
	runOnMain(func() {
		sharedApp()
		m := NewMenu("Cleanup", func(Menu) {})
		mi1 := NewMenuItem(401, "One", "", 0, func(MenuItem) bool { return true }, func(MenuItem) {})
		mi2 := NewMenuItem(402, "Two", "", 0, func(MenuItem) bool { return true }, func(MenuItem) {})
		m.InsertItemAtIndex(mi1, 0)
		m.InsertItemAtIndex(mi2, 1)
		if _, ok := menuUpdaters[m]; !ok {
			t.Error("menuUpdaters is missing the menu before Release")
		}
		for _, mi := range []MenuItem{mi1, mi2} {
			if _, ok := menuItemValidators[mi]; !ok {
				t.Errorf("menuItemValidators is missing item %d before Release", mi.Tag())
			}
			if _, ok := menuItemHandlers[mi]; !ok {
				t.Errorf("menuItemHandlers is missing item %d before Release", mi.Tag())
			}
		}
		m.Release()
		if _, ok := menuUpdaters[m]; ok {
			t.Error("menuUpdaters still contains the menu after Release")
		}
		for _, mi := range []MenuItem{mi1, mi2} {
			if _, ok := menuItemValidators[mi]; ok {
				t.Error("menuItemValidators still contains an item after Release")
			}
			if _, ok := menuItemHandlers[mi]; ok {
				t.Error("menuItemHandlers still contains an item after Release")
			}
		}
	})
}

// TestMenuTreeReleaseDeallocatesAndCleansUp proves the ownership contract end to end: creation hands back the only
// reference, InsertItemAtIndex and SetSubMenu transfer references into the tree, and releasing the root deallocates
// every item and submenu (verified by dealloc spies) while emptying the registration maps for the whole tree,
// including entries reachable only through a submenu — the case whose omission let the maps grow without bound.
func TestMenuTreeReleaseDeallocatesAndCleansUp(t *testing.T) {
	runOnMain(func() {
		sharedApp()
		var parentGone, subItemGone bool
		var m, sub Menu
		var parent, subItem MenuItem
		// AppKit autoreleases internal references during menu assembly (e.g. the NSMenuDidAddItemNotification it
		// posts carries the item), so the deallocations only complete once the pool pops; assert after WithPool.
		WithPool(func() {
			m = NewMenu("Owner", func(Menu) {})
			sub = NewMenu("Sub", func(Menu) {})
			subItem = NewMenuItem(502, "SubChild", "", 0, func(MenuItem) bool { return true }, func(MenuItem) {})
			attachDeallocSpy(t, subItem, &subItemGone)
			sub.InsertItemAtIndex(subItem, 0)
			parent = NewMenuItem(501, "Parent", "", 0, nil, func(MenuItem) {})
			attachDeallocSpy(t, parent, &parentGone)
			parent.SetSubMenu(sub)
			m.InsertItemAtIndex(parent, 0)
			for name, ok := range map[string]bool{
				"menuUpdaters[m]":             menuUpdaters[m] != nil,
				"menuUpdaters[sub]":           menuUpdaters[sub] != nil,
				"menuItemHandlers[parent]":    menuItemHandlers[parent] != nil,
				"menuItemValidators[subItem]": menuItemValidators[subItem] != nil,
				"menuItemHandlers[subItem]":   menuItemHandlers[subItem] != nil,
			} {
				if !ok {
					t.Errorf("%s missing before Release", name)
				}
			}
			m.Release()
		})
		if !parentGone {
			t.Error("the parent item was not deallocated by releasing the root menu")
		}
		if !subItemGone {
			t.Error("the submenu's item was not deallocated by releasing the root menu")
		}
		if _, ok := menuUpdaters[m]; ok {
			t.Error("menuUpdaters still contains the root menu after Release")
		}
		if _, ok := menuUpdaters[sub]; ok {
			t.Error("menuUpdaters still contains the submenu after Release")
		}
		if _, ok := menuItemHandlers[parent]; ok {
			t.Error("menuItemHandlers still contains the parent item after Release")
		}
		if _, ok := menuItemValidators[subItem]; ok {
			t.Error("menuItemValidators still contains the submenu's item after Release")
		}
		if _, ok := menuItemHandlers[subItem]; ok {
			t.Error("menuItemHandlers still contains the submenu's item after Release")
		}
	})
}

// TestMenuRemovalDestroysItems proves RemoveItemAtIndex and RemoveAll destroy the removed items: the items (and any
// submenu tree hanging off them) are deallocated and their registrations dropped, so repeated remove-and-recreate
// cycles — the pattern a menu updater that rebuilds its items on every open produces — cannot accumulate anything.
func TestMenuRemovalDestroysItems(t *testing.T) {
	runOnMain(func() {
		sharedApp()
		m := NewMenu("Removal", nil)
		var plainGone, subOwnerGone, subItemGone bool
		var plain, subOwner, subItem MenuItem
		var sub Menu
		WithPool(func() {
			plain = NewMenuItem(601, "Plain", "", 0, func(MenuItem) bool { return true }, func(MenuItem) {})
			attachDeallocSpy(t, plain, &plainGone)
			m.InsertItemAtIndex(plain, 0)
			sub = NewMenu("Sub", func(Menu) {})
			subItem = NewMenuItem(603, "SubChild", "", 0, nil, func(MenuItem) {})
			attachDeallocSpy(t, subItem, &subItemGone)
			sub.InsertItemAtIndex(subItem, 0)
			subOwner = NewMenuItem(602, "SubOwner", "", 0, nil, nil)
			attachDeallocSpy(t, subOwner, &subOwnerGone)
			subOwner.SetSubMenu(sub)
			m.InsertItemAtIndex(subOwner, 1)
			m.RemoveItemAtIndex(0)
		})
		if !plainGone {
			t.Error("RemoveItemAtIndex did not deallocate the removed item")
		}
		if _, ok := menuItemValidators[plain]; ok {
			t.Error("menuItemValidators still contains the removed item")
		}
		if _, ok := menuItemHandlers[plain]; ok {
			t.Error("menuItemHandlers still contains the removed item")
		}
		WithPool(func() {
			m.RemoveAll()
		})
		if !subOwnerGone {
			t.Error("RemoveAll did not deallocate the submenu-owning item")
		}
		if !subItemGone {
			t.Error("RemoveAll did not deallocate the submenu's item")
		}
		if _, ok := menuUpdaters[sub]; ok {
			t.Error("menuUpdaters still contains the removed item's submenu")
		}
		if _, ok := menuItemHandlers[subItem]; ok {
			t.Error("menuItemHandlers still contains the removed submenu's item")
		}
		m.Release()
	})
}
