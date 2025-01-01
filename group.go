// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"slices"

	"github.com/richardwilkes/toolbox"
)

// Grouper is the interface that a panel must implement to be part of a group.
type Grouper interface {
	Paneler
	Group() *Group
	SetGroup(group *Group)
}

// Group is used to ensure only one panel in a group is selected at a time.
type Group struct {
	selected Grouper
	panel    []Grouper
}

// NewGroup creates a new group for the specified set of panels. Each panel is removed from any other group it may be in
// and placed in the newly created one.
func NewGroup(panel ...Grouper) *Group {
	sg := &Group{panel: panel}
	for _, one := range panel {
		sg.Add(one)
	}
	return sg
}

// Add a panel to the group, removing it from any group it may have previously been associated with.
func (sg *Group) Add(panel Grouper) {
	if sg == nil {
		return
	}
	group := panel.Group()
	if group != nil {
		group.Remove(panel)
	}
	panel.SetGroup(sg)
	sg.panel = append(sg.panel, panel)
}

// Remove a panel from the group.
func (sg *Group) Remove(panel Grouper) {
	if sg == nil {
		return
	}
	if panel.Group() == sg {
		for i, one := range sg.panel {
			if !one.AsPanel().Is(panel) {
				continue
			}
			if sg.Selected(one) {
				sg.Select(nil)
			}
			sg.panel = slices.Delete(sg.panel, i, i+1)
			panel.SetGroup(nil)
			break
		}
	}
}

// Selected returns true if the panel is currently selected.
func (sg *Group) Selected(panel Grouper) bool {
	if sg == nil || toolbox.IsNil(panel) {
		return false
	}
	return panel.AsPanel().Is(sg.selected)
}

// Select a panel, deselecting all others in the group.
func (sg *Group) Select(panel Grouper) {
	if sg == nil {
		return
	}
	panelIsNil := toolbox.IsNil(panel)
	if (panelIsNil || panel.Group() == sg) && sg.selected != panel {
		if !toolbox.IsNil(sg.selected) {
			sg.selected.AsPanel().MarkForRedraw()
		}
		if panelIsNil {
			sg.selected = nil
		} else {
			sg.selected = panel
			panel.AsPanel().MarkForRedraw()
		}
	}
}
