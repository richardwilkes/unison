// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// Group is used to ensure only one panel in a group is selected at a time.
type Group struct {
	panel []*GroupPanel
}

// NewGroup creates a new group for the specified set of panels. Each panel is removed from any other group it may be in
// and placed in the newly created one.
func NewGroup(panel ...*GroupPanel) *Group {
	sg := &Group{panel: panel}
	for _, one := range panel {
		sg.Add(one)
	}
	return sg
}

// Add a panel to the group, removing it from any group it may have previously been associated with.
func (sg *Group) Add(panel *GroupPanel) {
	if panel.group != nil {
		panel.group.Remove(panel)
	}
	panel.group = sg
	sg.panel = append(sg.panel, panel)
}

// Remove a panel from the group.
func (sg *Group) Remove(panel *GroupPanel) {
	if sg == panel.group {
		for i, one := range sg.panel {
			if !one.Is(panel) {
				continue
			}
			copy(sg.panel[i:], sg.panel[i+1:])
			sg.panel[len(sg.panel)-1] = nil
			sg.panel = sg.panel[:len(sg.panel)-1]
			panel.group = nil
			break
		}
	}
}

// Select a panel, deselecting all others in the group.
func (sg *Group) Select(panel *GroupPanel) {
	if panel.group == sg {
		for _, one := range sg.panel {
			one.setSelected(one.Is(panel))
		}
	}
}
