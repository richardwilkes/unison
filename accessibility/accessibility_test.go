// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package accessibility_test

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/accessibility"
)

// rootPackage is the import path this package must never depend on.
const rootPackage = "github.com/richardwilkes/unison"

// Names the fixtures in these tests give their nodes.
const (
	windowName = "Window"
	groupName  = "Group"
)

// Short aliases keep the tree literals these tests are built from readable.
type (
	axID    = accessibility.NodeID
	axNode  = accessibility.Node
	axEvent = accessibility.Event
)

// newTree builds a Tree from the supplied nodes, keyed by their ids, using the first node as the root. The nodes are
// used as given, so a test can hand the same node layout to two trees and change only what it means to change.
func newTree(focus axID, nodes ...*axNode) *accessibility.Tree {
	t := &accessibility.Tree{
		Nodes:      make(map[axID]*axNode, len(nodes)),
		Root:       nodes[0].ID,
		Focus:      focus,
		Generation: 1,
	}
	for _, n := range nodes {
		t.Nodes[n.ID] = n
	}
	return t
}

// TestSchemaNeverImportsTheRootPackage verifies the claim the package documentation makes about itself: the schema and
// its diffing depend on nothing in unison but the two enumerations they need, so that the root package can import this
// one and every adapter can be tested without a window, a display, or an assistive technology. An import of the root
// package would turn that dependency around and make the cycle impossible to break later, so it is checked here rather
// than being left to be discovered when some adapter needs it.
func TestSchemaNeverImportsTheRootPackage(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	entries, err := os.ReadDir(".")
	c.NoError(err)
	fset := token.NewFileSet()
	examined := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		f, parseErr := parser.ParseFile(fset, name, nil, parser.ImportsOnly|parser.SkipObjectResolution)
		c.NoError(parseErr)
		if f == nil {
			continue
		}
		examined++
		for _, imported := range f.Imports {
			path, unquoteErr := strconv.Unquote(imported.Path.Value)
			c.NoError(unquoteErr)
			c.NotEqual(rootPackage, path, "%s must not import the root unison package",
				fset.Position(imported.Pos()))
		}
	}
	c.True(examined > 0, "no source files were examined")
}

func TestActionSet(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	var set accessibility.ActionSet
	c.False(set.Has(accessibility.Focus))
	c.Equal("", set.String())

	set = set.With(accessibility.Focus, accessibility.Press, accessibility.ScrollIntoView)
	c.True(set.Has(accessibility.Focus))
	c.True(set.Has(accessibility.Press))
	c.True(set.Has(accessibility.ScrollIntoView))
	c.False(set.Has(accessibility.Toggle))
	c.Equal("focus,press,scroll-into-view", set.String())

	set = set.Without(accessibility.Press)
	c.False(set.Has(accessibility.Press))
	c.Equal("focus,scroll-into-view", set.String())

	// Every bit an ActionSet can hold must round-trip, and nothing above the last action may be accepted.
	for _, a := range []accessibility.Action{
		accessibility.Focus, accessibility.Press, accessibility.Increment, accessibility.Decrement,
		accessibility.SetValue, accessibility.Expand, accessibility.Collapse, accessibility.Select,
		accessibility.AddToSelection, accessibility.RemoveFromSelection, accessibility.Toggle,
		accessibility.ScrollIntoView, accessibility.ShowContextMenu, accessibility.SetTextSelection,
		accessibility.ReplaceText,
	} {
		var one accessibility.ActionSet
		one = one.With(a)
		c.True(one.Has(a), "action %v", a)
		c.NotEqual("", a.String())
	}
	bogus := accessibility.ReplaceText + 1
	c.False(accessibility.ActionSet(0xFFFFFFFF).Has(bogus))
	c.Equal("Action(15)", bogus.String())
}

func TestEnumStrings(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal("focus-changed", accessibility.FocusChanged.String())
	c.Equal("window-deactivated", accessibility.WindowDeactivated.String())
	c.Equal("EventKind(18)", (accessibility.Announcement + 1).String())
	c.Equal("role-changed", accessibility.RoleChanged.String())
	c.Equal("none", accessibility.StateNone.String())
	c.Equal("checked", accessibility.StateChecked.String())
	c.Equal("State(17)", (accessibility.StateChecked + 1).String())
	c.Equal("horizontal", accessibility.OrientationHorizontal.String())
	c.Equal("Orientation(3)", (accessibility.OrientationVertical + 1).String())
	c.Equal("unsorted", accessibility.SortNone.String())
	c.Equal("descending", accessibility.SortDescending.String())
	c.Equal("SortDirection(3)", (accessibility.SortDescending + 1).String())
}
