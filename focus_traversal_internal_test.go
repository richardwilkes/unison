// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

func newFocusablePanel() *Panel {
	p := NewPanel()
	p.SetFocusable(true)
	return p
}

// TestCollectFocusablesSkipsHiddenSubtrees verifies that tab traversal does not descend into hidden containers, since
// focusable descendants of a hidden ancestor are not visible even though their own Hidden flag is false. This mirrors
// the dock container case, where non-current dockables are hidden at the container level only.
func TestCollectFocusablesSkipsHiddenSubtrees(t *testing.T) {
	c := check.New(t)
	root := NewPanel()
	visible := newFocusablePanel()
	root.AddChild(visible)
	hiddenContainer := NewPanel()
	hiddenContainer.Hidden = true
	buried := newFocusablePanel()
	hiddenContainer.AddChild(buried)
	root.AddChild(hiddenContainer)
	match, focusables := collectFocusables(root, visible, nil)
	c.Equal(1, len(focusables))
	c.True(focusables[0].Is(visible))
	c.Equal(0, match)

	// Unhiding the container makes its focusable descendant reachable again.
	hiddenContainer.Hidden = false
	match, focusables = collectFocusables(root, buried, nil)
	c.Equal(2, len(focusables))
	c.True(focusables[1].Is(buried))
	c.Equal(1, match)
}

// TestCollectFocusablesSkipsDirectlyHiddenPanels verifies that a hidden focusable panel is excluded even when its
// parent is visible.
func TestCollectFocusablesSkipsDirectlyHiddenPanels(t *testing.T) {
	c := check.New(t)
	root := NewPanel()
	hidden := newFocusablePanel()
	hidden.Hidden = true
	root.AddChild(hidden)
	visible := newFocusablePanel()
	root.AddChild(visible)
	match, focusables := collectFocusables(root, visible, nil)
	c.Equal(1, len(focusables))
	c.True(focusables[0].Is(visible))
	c.Equal(0, match)
}

// TestFirstFocusableChildSkipsHiddenSubtrees verifies that FirstFocusableChild neither returns a focusable panel
// buried in a hidden subtree nor a directly hidden focusable panel.
func TestFirstFocusableChildSkipsHiddenSubtrees(t *testing.T) {
	c := check.New(t)
	root := NewPanel()
	hiddenContainer := NewPanel()
	hiddenContainer.Hidden = true
	buried := newFocusablePanel()
	hiddenContainer.AddChild(buried)
	root.AddChild(hiddenContainer)
	c.Nil(root.FirstFocusableChild())
	visible := newFocusablePanel()
	root.AddChild(visible)
	c.True(root.FirstFocusableChild().Is(visible))
	hiddenContainer.Hidden = false
	c.True(root.FirstFocusableChild().Is(buried))
}

// TestLastFocusableChildSkipsHiddenSubtrees verifies that LastFocusableChild neither returns a focusable panel buried
// in a hidden subtree nor a directly hidden focusable panel.
func TestLastFocusableChildSkipsHiddenSubtrees(t *testing.T) {
	c := check.New(t)
	root := NewPanel()
	visible := newFocusablePanel()
	root.AddChild(visible)
	hiddenContainer := NewPanel()
	hiddenContainer.Hidden = true
	buried := newFocusablePanel()
	hiddenContainer.AddChild(buried)
	root.AddChild(hiddenContainer)
	c.True(root.LastFocusableChild().Is(visible))
	hiddenContainer.Hidden = false
	c.True(root.LastFocusableChild().Is(buried))
}
