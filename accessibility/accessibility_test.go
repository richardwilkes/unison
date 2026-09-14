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

// axActionNames names every Action, in declaration order. Spelling the names out is the point: an adapter's own tables
// are keyed by them and a trace of what a screen reader asked for is read by them, so a name that changed, or a case
// quietly dropped from Action.String, shows up here. Merely checking that a name is not empty guards nothing, since the
// default branch answers "Action(N)" for any value at all.
var axActionNames = []struct {
	name   string
	action accessibility.Action
}{
	{"focus", accessibility.Focus},
	{"press", accessibility.Press},
	{"increment", accessibility.Increment},
	{"decrement", accessibility.Decrement},
	{"set-value", accessibility.SetValue},
	{"expand", accessibility.Expand},
	{"collapse", accessibility.Collapse},
	{"select", accessibility.Select},
	{"add-to-selection", accessibility.AddToSelection},
	{"remove-from-selection", accessibility.RemoveFromSelection},
	{"toggle", accessibility.Toggle},
	{"scroll-into-view", accessibility.ScrollIntoView},
	{"show-context-menu", accessibility.ShowContextMenu},
	{"set-text-selection", accessibility.SetTextSelection},
	{"replace-text", accessibility.ReplaceText},
}

// axEventKindNames names every EventKind, in declaration order, for the same reason axActionNames names the actions.
var axEventKindNames = []struct {
	name string
	kind accessibility.EventKind
}{
	{"focus-changed", accessibility.FocusChanged},
	{"name-changed", accessibility.NameChanged},
	{"description-changed", accessibility.DescriptionChanged},
	{"value-changed", accessibility.ValueChanged},
	{"number-changed", accessibility.NumberChanged},
	{"state-changed", accessibility.StateChanged},
	{"text-inserted", accessibility.TextInserted},
	{"text-deleted", accessibility.TextDeleted},
	{"text-selection-changed", accessibility.TextSelectionChanged},
	{"children-changed", accessibility.ChildrenChanged},
	{"bounds-changed", accessibility.BoundsChanged},
	{"node-added", accessibility.NodeAdded},
	{"node-removed", accessibility.NodeRemoved},
	{"sort-changed", accessibility.SortChanged},
	{"attributes-changed", accessibility.AttributesChanged},
	{"role-changed", accessibility.RoleChanged},
	{"window-activated", accessibility.WindowActivated},
	{"window-deactivated", accessibility.WindowDeactivated},
}

// axStateNames names every State, in declaration order, for the same reason axActionNames names the actions.
var axStateNames = []struct {
	name  string
	state accessibility.State
}{
	{"none", accessibility.StateNone},
	{"disabled", accessibility.StateDisabled},
	{"focusable", accessibility.StateFocusable},
	{"selectable", accessibility.StateSelectable},
	{"selected", accessibility.StateSelected},
	{"multiselectable", accessibility.StateMultiselectable},
	{"pressed", accessibility.StatePressed},
	{"read-only", accessibility.StateReadOnly},
	{"modal", accessibility.StateModal},
	{"busy", accessibility.StateBusy},
	{"invalid", accessibility.StateInvalid},
	{"offscreen", accessibility.StateOffscreen},
	{"expandable", accessibility.StateExpandable},
	{"expanded", accessibility.StateExpanded},
	{"ignored", accessibility.StateIgnored},
	{"protected", accessibility.StateProtected},
	{"checked", accessibility.StateChecked},
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

	// Every action must report the name it is known by, and no two of them may share one. Every bit an ActionSet can
	// hold must round-trip, and nothing above the last action may be accepted.
	seen := make(map[string]bool, len(axActionNames))
	for i, one := range axActionNames {
		c.Equal(accessibility.Action(i), one.action, "the table must be in declaration order")
		c.Equal(one.name, one.action.String())
		c.False(seen[one.name], "duplicate name %q", one.name)
		seen[one.name] = true
		var held accessibility.ActionSet
		held = held.With(one.action)
		c.True(held.Has(one.action), "action %s", one.name)
		c.Equal(one.name, held.String())
	}
	bogus := accessibility.Action(len(axActionNames))
	c.False(accessibility.ActionSet(0xFFFFFFFF).Has(bogus))
	c.Equal("Action("+strconv.Itoa(len(axActionNames))+")", bogus.String())
}

func TestEnumStrings(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	kinds := make(map[string]bool, len(axEventKindNames))
	for i, one := range axEventKindNames {
		c.Equal(accessibility.EventKind(i), one.kind, "the table must be in declaration order")
		c.Equal(one.name, one.kind.String())
		c.False(kinds[one.name], "duplicate name %q", one.name)
		kinds[one.name] = true
	}
	c.Equal("EventKind("+strconv.Itoa(len(axEventKindNames))+")",
		accessibility.EventKind(len(axEventKindNames)).String())

	states := make(map[string]bool, len(axStateNames))
	for i, one := range axStateNames {
		c.Equal(accessibility.State(i), one.state, "the table must be in declaration order")
		c.Equal(one.name, one.state.String())
		c.False(states[one.name], "duplicate name %q", one.name)
		states[one.name] = true
	}
	c.Equal("State("+strconv.Itoa(len(axStateNames))+")", accessibility.State(len(axStateNames)).String())

	c.Equal("none", accessibility.OrientationNone.String())
	c.Equal("horizontal", accessibility.OrientationHorizontal.String())
	c.Equal("vertical", accessibility.OrientationVertical.String())
	c.Equal("Orientation(3)", (accessibility.OrientationVertical + 1).String())
	c.Equal("unsorted", accessibility.SortNone.String())
	c.Equal("ascending", accessibility.SortAscending.String())
	c.Equal("descending", accessibility.SortDescending.String())
	c.Equal("SortDirection(3)", (accessibility.SortDescending + 1).String())
}
