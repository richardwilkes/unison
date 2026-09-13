// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package atspi

import (
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/dbus"
)

// cacheObject is the object at [CachePath] that hands an assistive technology the whole accessibility tree in one call.
// libatspi asks for it as soon as the application appears, with a two second timeout, and fills its own cache from the
// answer rather than making a call per node.
type cacheObject struct {
	a *Adapter
}

// Interfaces implements [dbus.Object]. The two signals are declared so that they show up in the object's introspection
// and so that the shape of the item they carry is written down in one place; [Adapter.Publish] and
// [Adapter.RemoveWindow] send them as nodes come and go.
func (o *cacheObject) Interfaces() []*dbus.Interface {
	return []*dbus.Interface{
		{
			Name: InterfaceCache,
			Methods: []*dbus.Method{
				{Name: "GetItems", Out: cacheItemsSignature, Handle: o.getItems},
			},
			Signals: []*dbus.Signal{
				{Name: "AddAccessible", Sig: cacheItemSignature},
				{Name: "RemoveAccessible", Sig: objectRefSignature},
			},
		},
	}
}

// getItems implements org.a11y.atspi.Cache.GetItems.
func (o *cacheObject) getItems(call *dbus.Call) {
	call.Reply(o.a.cacheItems())
}

// cacheItems returns one cache item for the application root and one for every reported node of every published window.
// An ignored node is left out, as it is everywhere else: it has no object of its own.
func (a *Adapter) cacheItems() []any {
	windows := a.windowOrder()
	items := make([]any, 0, 1+len(windows)*32)
	items = append(items, a.rootCacheItem(windows))
	for i, ws := range windows {
		data := ws.data.Load()
		if data == nil {
			continue
		}
		data.tree.Walk(func(n *accessibility.Node) bool {
			if n.Ignored {
				return true
			}
			o := &nodeObject{a: a, data: data, node: n}
			index := data.indexInParent(n.ID)
			if n.ID == data.tree.Root {
				index = i
			}
			items = append(items, o.cacheItem(index))
			return true
		})
	}
	return items
}

// rootCacheItem returns the cache item for the application root.
func (a *Adapter) rootCacheItem(windows []*windowState) dbus.Struct {
	return dbus.Struct{
		a.rootReference(),
		a.rootReference(),
		a.desktopReference(),
		int32(-1),
		int32(len(windows)),
		rootInterfaces(),
		applicationName(),
		uint32(RoleApplication),
		"",
		rootStates().Words(),
	}
}

// cacheItem returns the cache item for a node: everything an assistive technology would otherwise have to make ten calls
// to find out. index is where the node sits among its parent's children, which the caller already knows.
func (o *nodeObject) cacheItem(index int) dbus.Struct {
	return dbus.Struct{
		o.reference(),
		o.a.rootReference(),
		o.parentReference(),
		int32(index),
		int32(len(o.children())),
		Interfaces(o.node),
		o.node.Name,
		uint32(MapRole(o.node)),
		o.node.Description,
		o.states().Words(),
	}
}
