// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// GroupPanel is a panel that can be used in a Group. A GroupPanel is typically embedded into another widget that wants
// to participate in group selection. If used standalone, .Self should be set appropriately.
type GroupPanel struct {
	Panel
	group    *Group
	selected bool
}

// AsGroupPanel returns the object as a GroupPanel.
func (p *GroupPanel) AsGroupPanel() *GroupPanel {
	return p
}

// Selected returns true if the panel is currently selected.
func (p *GroupPanel) Selected() bool {
	return p.selected
}

// SetSelected sets the panel's selected state.
func (p *GroupPanel) SetSelected(selected bool) {
	if p.group != nil {
		p.group.Select(p)
	} else {
		p.setSelected(selected)
	}
}

func (p *GroupPanel) setSelected(selected bool) {
	if p.selected != selected {
		p.selected = selected
		p.MarkForRedraw()
	}
}
