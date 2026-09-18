// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package role_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/enums/role"
)

// TestMembership names the full membership of every predicate. Spelling the sets out here rather than deriving them
// from the predicates is the point: the platform adapters branch on these, so a role quietly joining or leaving a set
// would change what an assistive technology is told, and this is where that shows up.
func TestMembership(t *testing.T) {
	c := check.New(t)
	for _, one := range []struct {
		name    string
		fn      func(role.Enum) bool
		members []role.Enum
	}{
		{
			name: "IsText",
			fn:   role.Enum.IsText,
			members: []role.Enum{
				role.TextField, role.TextArea, role.SpinButton, role.ComboBox, role.Document, role.Paragraph,
				role.Heading, role.Code, role.Cell, role.ColumnHeader,
			},
		},
		{
			name:    "IsRowLike",
			fn:      role.Enum.IsRowLike,
			members: []role.Enum{role.ListItem, role.Row},
		},
	} {
		in := make(map[role.Enum]bool, len(one.members))
		for _, member := range one.members {
			in[member] = true
		}
		for _, e := range role.All {
			c.Equal(in[e], one.fn(e), "%s(%s)", one.name, e.Key())
		}
	}
}

// TestAutoAndNoneAreNeverClassified checks that the two roles which are instructions to the snapshot builder rather
// than real roles stay out of every set, since they never reach a published tree.
func TestAutoAndNoneAreNeverClassified(t *testing.T) {
	c := check.New(t)
	for _, e := range []role.Enum{role.Auto, role.None} {
		c.False(e.IsText(), e.Key())
		c.False(e.IsRowLike(), e.Key())
	}
	c.Equal(role.Auto, role.Enum(0), "auto must be the zero value")
	c.Equal(role.Auto, role.Enum(len(role.All)).EnsureValid(), "an out-of-range value must fall back to auto")
}

// TestEveryRoleHasADistinctKey guards the serialization keys, which are what the generated Extract round-trips through.
func TestEveryRoleHasADistinctKey(t *testing.T) {
	c := check.New(t)
	seen := make(map[string]bool, len(role.All))
	for _, e := range role.All {
		key := e.Key()
		c.NotEqual("", key)
		c.False(seen[key], "duplicate key %q", key)
		seen[key] = true
		c.Equal(e, role.Extract(key))
	}
}
