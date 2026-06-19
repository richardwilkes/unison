// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
)

// transformPanel returns a panel positioned at (x, y) in its parent's coordinate system with the given scale. Only the
// frame origin and scale matter to the coordinate transforms, so the frame size is left at zero. A zero scale leaves
// the panel at its default scale of 1.
func transformPanel(x, y float32, scale geom.Point) *unison.Panel {
	p := unison.NewPanel()
	if scale != (geom.Point{}) {
		p.SetScale(scale)
	}
	p.SetFrameRect(geom.NewRect(x, y, 0, 0))
	return p
}

func TestPointToRootSinglePanel(t *testing.T) {
	c := check.New(t)
	// An unparented panel at the origin with no scale is the identity transform.
	p := transformPanel(0, 0, geom.Point{})
	c.Equal(geom.NewPoint(3, 4), p.PointToRoot(geom.NewPoint(3, 4)))

	// An unparented panel offset from the origin adds its frame position.
	p = transformPanel(10, 20, geom.Point{})
	c.Equal(geom.NewPoint(13, 24), p.PointToRoot(geom.NewPoint(3, 4)))
}

func TestPointToRootHierarchy(t *testing.T) {
	c := check.New(t)
	root := transformPanel(0, 0, geom.Point{})
	child := transformPanel(10, 20, geom.Point{})
	grandchild := transformPanel(5, 5, geom.Point{})
	root.AddChild(child)
	child.AddChild(grandchild)
	// (1,2) -> +grandchild(5,5) = (6,7) -> +child(10,20) = (16,27) -> +root(0,0) = (16,27).
	c.Equal(geom.NewPoint(16, 27), grandchild.PointToRoot(geom.NewPoint(1, 2)))
}

func TestPointToRootWithScale(t *testing.T) {
	c := check.New(t)
	root := transformPanel(0, 0, geom.Point{})
	child := transformPanel(10, 20, geom.NewPoint(2, 2))
	grandchild := transformPanel(5, 5, geom.Point{})
	root.AddChild(child)
	child.AddChild(grandchild)
	// (1,2) -> +grandchild(5,5) = (6,7) -> *child scale(2) +child(10,20) = (22,34) -> +root = (22,34).
	c.Equal(geom.NewPoint(22, 34), grandchild.PointToRoot(geom.NewPoint(1, 2)))
}

func TestPointFromRootRoundTrip(t *testing.T) {
	c := check.New(t)
	root := transformPanel(0, 0, geom.Point{})
	child := transformPanel(10, 20, geom.NewPoint(2, 2))
	grandchild := transformPanel(5, 5, geom.Point{})
	root.AddChild(child)
	child.AddChild(grandchild)
	local := geom.NewPoint(1, 2)
	c.Equal(local, grandchild.PointFromRoot(grandchild.PointToRoot(local)))
}

func TestPointFromRootValues(t *testing.T) {
	c := check.New(t)
	root := transformPanel(0, 0, geom.Point{})
	child := transformPanel(10, 20, geom.NewPoint(2, 2))
	root.AddChild(child)
	// (22,34) -> root identity -> child: (22-10, 34-20)/2 = (6,7).
	c.Equal(geom.NewPoint(6, 7), child.PointFromRoot(geom.NewPoint(22, 34)))
}

func TestPointTo(t *testing.T) {
	c := check.New(t)
	parent := transformPanel(0, 0, geom.Point{})
	childA := transformPanel(10, 10, geom.Point{})
	childB := transformPanel(50, 5, geom.Point{})
	parent.AddChild(childA)
	parent.AddChild(childB)
	// (1,1) in childA is (11,11) in parent; relative to childB at (50,5) that is (-39,6).
	c.Equal(geom.NewPoint(-39, 6), childA.PointTo(geom.NewPoint(1, 1), childB))
}

func TestPointToSelfIsIdentity(t *testing.T) {
	c := check.New(t)
	parent := transformPanel(0, 0, geom.Point{})
	child := transformPanel(10, 20, geom.NewPoint(2, 2))
	parent.AddChild(child)
	pt := geom.NewPoint(3, 4)
	c.Equal(pt, child.PointTo(pt, child))
}

func TestRectToRootNoScale(t *testing.T) {
	c := check.New(t)
	parent := transformPanel(10, 20, geom.Point{})
	child := transformPanel(5, 5, geom.Point{})
	parent.AddChild(child)
	// Without scale the rect is simply translated by the accumulated offsets (5,5)+(10,20) = (15,25).
	c.Equal(geom.NewRect(16, 27, 3, 4), child.RectToRoot(geom.NewRect(1, 2, 3, 4)))
}

func TestRectToRootWithScale(t *testing.T) {
	c := check.New(t)
	parent := transformPanel(10, 20, geom.Point{})
	child := transformPanel(5, 5, geom.NewPoint(2, 2))
	parent.AddChild(child)
	// Origin (1,2) -> *2 +(5,5) +(10,20) = (17,29); size (3,4) scales to (6,8).
	c.Equal(geom.NewRect(17, 29, 6, 8), child.RectToRoot(geom.NewRect(1, 2, 3, 4)))
}

func TestRectFromRootRoundTrip(t *testing.T) {
	c := check.New(t)
	parent := transformPanel(10, 20, geom.Point{})
	child := transformPanel(5, 5, geom.NewPoint(2, 2))
	parent.AddChild(child)
	local := geom.NewRect(1, 2, 3, 4)
	c.Equal(local, child.RectFromRoot(child.RectToRoot(local)))
}

func TestRectTo(t *testing.T) {
	c := check.New(t)
	parent := transformPanel(0, 0, geom.Point{})
	childA := transformPanel(10, 10, geom.Point{})
	childB := transformPanel(50, 5, geom.Point{})
	parent.AddChild(childA)
	parent.AddChild(childB)
	// (1,1,2,2) in childA is (11,11,2,2) in parent; relative to childB at (50,5) the origin is (-39,6).
	c.Equal(geom.NewRect(-39, 6, 2, 2), childA.RectTo(geom.NewRect(1, 1, 2, 2), childB))
}
