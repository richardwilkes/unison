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
	"testing"

	"github.com/ebitengine/purego/objc"
)

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

		// Submenu wiring, the shape root uses for nested menus.
		if got := mi1.SubMenu(); got != 0 {
			t.Errorf("SubMenu() = %#x before SetSubMenu, want 0", got)
		}
		sub := NewMenu("Sub", nil)
		defer sub.Release()
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
		m.RemoveAll()
		if got := m.NumberOfItems(); got != 0 {
			t.Errorf("NumberOfItems() = %d after RemoveAll, want 0", got)
		}
		if got := mi2.Menu(); got != 0 {
			t.Errorf("Menu() = %#x after RemoveAll, want 0", got)
		}
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
// validator and handler from the registration maps, matching the cgo bridge's cleanup contract.
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
