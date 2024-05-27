// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/errs"
)

// Possible values for the DockState Type field.
const (
	LayoutType    = "layout"
	ContainerType = "container"
	DockableType  = "dockable"
)

// DockState holds a snapshot of the arrangement of Dockables within a Dock.
type DockState struct {
	Type         string       `json:"type"`                    // One of LayoutType, ContainerType or DockableType
	Key          string       `json:"key,omitempty"`           // Only valid when Type == DockableType
	Children     []*DockState `json:"children,omitempty"`      // Only valid when Type != DockableType
	CurrentIndex int          `json:"current_index,omitempty"` // Only valid when Type == ContainerType
	Divider      float32      `json:"divider,omitempty"`       // Only valid when Type == LayoutType
	Horizontal   bool         `json:"horizontal,omitempty"`    // Only valid when Type == LayoutType
}

// NewDockState creates a new DockState for the given Dock. keyFromDockable will be passed each Dockable within the Dock
// and is expected to return a unique string that will be used to locate the Dockable when the Apply() method is called.
func NewDockState(dock *Dock, keyFromDockable func(Dockable) string) *DockState {
	return collectDockState(dock.RootDockLayout(), keyFromDockable)
}

func collectDockState(node DockLayoutNode, keyFromDockable func(Dockable) string) *DockState {
	switch t := node.(type) {
	case *DockContainer:
		children := make([]*DockState, 0, len(t.Dockables()))
		for _, d := range t.Dockables() {
			children = append(children, &DockState{
				Type: DockableType,
				Key:  keyFromDockable(d),
			})
		}
		return &DockState{
			Type:         ContainerType,
			Children:     children,
			CurrentIndex: 1 + t.CurrentDockableIndex(),
		}
	case *DockLayout:
		children := make([]*DockState, 0, 2)
		for _, n := range t.nodes {
			if !toolbox.IsNil(n) {
				children = append(children, collectDockState(n, keyFromDockable))
			}
		}
		return &DockState{
			Type:       LayoutType,
			Children:   children,
			Divider:    t.divider,
			Horizontal: t.Horizontal,
		}
	default:
		errs.Log(errs.New("invalid dock data"))
		return nil
	}
}

// Apply the saved DockState to the specified Dock. keyToDockable is called for each Dockable that was in the Dock when
// the state was captured, passing in the unique key that was used when keyFromDockable in NewDockState() was called.
func (d *DockState) Apply(dock *Dock, keyToDockable func(string) Dockable) {
	dock.RemoveAllChildren()
	d.apply(dock.RootDockLayout(), keyToDockable)
	dock.MarkForLayoutRecursively()
	dock.Layout()
	dock.MarkForRedraw()
}

func (d *DockState) apply(node DockLayoutNode, keyToDockable func(string) Dockable) {
	switch t := node.(type) {
	case *DockContainer:
		for _, child := range d.Children {
			if dockable := resolveDockable(keyToDockable(child.Key)); !toolbox.IsNil(dockable) {
				t.content.AddChild(dockable)
				t.header.AddChild(newDockTab(dockable))
			}
		}
		t.content.SetCurrentIndex(d.CurrentIndex - 1)
	case *DockLayout:
		t.divider = d.Divider
		t.Horizontal = d.Horizontal
		for i, child := range d.Children {
			if child.Type == LayoutType {
				t.nodes[i] = &DockLayout{
					dock:   t.dock,
					parent: t,
				}
			} else {
				dc := NewDockContainer(t.dock, nil)
				t.nodes[i] = dc
				t.dock.AddChild(dc)
			}
			child.apply(t.nodes[i], keyToDockable)
		}
		for i := len(d.Children); i < 2; i++ {
			t.nodes[i] = nil
		}
	}
}
