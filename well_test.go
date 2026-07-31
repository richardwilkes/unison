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
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/imgfmt"
)

// wellDragInfo is a minimal drag.Info for exercising the Well drop callbacks without a live drag session.
type wellDragInfo struct {
	data  map[string][]byte
	files []string
}

func (f *wellDragInfo) SourceDragOpMask() drag.Op { return drag.Copy }

func (f *wellDragInfo) DataTypes() []string {
	types := make([]string, 0, len(f.data))
	for dt := range f.data {
		types = append(types, dt)
	}
	return types
}

func (f *wellDragInfo) HasString() bool    { return false }
func (f *wellDragInfo) HasFilePaths() bool { return len(f.files) > 0 }
func (f *wellDragInfo) HasURLs() bool      { return false }

func (f *wellDragInfo) HasDataType(dataType string) bool {
	_, ok := f.data[dataType]
	return ok
}

func (f *wellDragInfo) Text() string          { return "" }
func (f *wellDragInfo) FilePaths() []string   { return f.files }
func (f *wellDragInfo) URLs() []*url.URL      { return nil }
func (f *wellDragInfo) Data(dt string) []byte { return f.data[dt] }

// imageDataDrag returns a drag carrying PNG image data.
func imageDataDrag() *wellDragInfo {
	return &wellDragInfo{data: map[string][]byte{imgfmt.PNG.UTI().UTI: {1, 2, 3}}}
}

// imageFileDrag returns a drag carrying a path to a readable image file.
func imageFileDrag() *wellDragInfo {
	return &wellDragInfo{files: []string{"/tmp/sample.png"}}
}

// TestWellCanAcceptDropRequiresPatternMask verifies that a well whose mask excludes patterns declines image drags,
// since any drop would only ever produce a *Pattern ink that SetInk would reject.
func TestWellCanAcceptDropRequiresPatternMask(t *testing.T) {
	c := check.New(t)
	w := unison.NewWell()
	w.Mask = unison.ColorWellMask
	c.False(w.DefaultCanAcceptDrop(imageDataDrag()))
	c.False(w.DefaultCanAcceptDrop(imageFileDrag()))
	c.Equal(drag.None, w.DefaultDragEnter(imageDataDrag(), geom.Point{}, 0))

	w.Mask = unison.ColorWellMask | unison.PatternWellMask
	c.True(w.DefaultCanAcceptDrop(imageDataDrag()))
	c.True(w.DefaultCanAcceptDrop(imageFileDrag()))
	c.Equal(drag.Copy, w.DefaultDragEnter(imageDataDrag(), geom.Point{}, 0))
}

// TestWellDropRequiresPatternMask verifies that DefaultDrop reports failure and leaves the ink untouched when the mask
// excludes patterns, without even attempting to load the dropped image.
func TestWellDropRequiresPatternMask(t *testing.T) {
	c := check.New(t)
	w := unison.NewWell()
	w.Mask = unison.ColorWellMask
	loaded := false
	w.ImageFromSpecCallback = func(_ context.Context, _ *http.Client, _ string, scale geom.Point, _ int64) (*unison.Image, error) {
		loaded = true
		return unison.NewImageFromPixels(2, 2, make([]byte, 2*2*4), scale)
	}
	original := w.Ink()
	c.False(w.DefaultDrop(imageFileDrag(), geom.Point{}, 0))
	c.False(loaded)
	c.Equal(original, w.Ink())
}

// TestWellDropAcceptsImageWhenPatternAllowed verifies the positive path: with patterns permitted, a dropped image file
// is loaded and installed as the well's pattern ink, and the drop reports success.
func TestWellDropAcceptsImageWhenPatternAllowed(t *testing.T) {
	c := check.New(t)
	w := unison.NewWell()
	var img *unison.Image
	w.ImageFromSpecCallback = func(_ context.Context, _ *http.Client, _ string, scale geom.Point, _ int64) (*unison.Image, error) {
		var err error
		img, err = unison.NewImageFromPixels(2, 2, make([]byte, 2*2*4), scale)
		return img, err
	}
	c.True(w.DefaultDrop(imageFileDrag(), geom.Point{}, 0))
	pattern, ok := w.Ink().(*unison.Pattern)
	c.True(ok, "ink should be a *Pattern after a successful drop")
	c.Equal(img, pattern.Image)
}
