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

	"github.com/richardwilkes/canvas/canvas"
	skgeom "github.com/richardwilkes/canvas/geom"
	"github.com/richardwilkes/canvas/maskfilter"
	"github.com/richardwilkes/canvas/path"
	skpatheffect "github.com/richardwilkes/canvas/patheffect"
	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/canvas/shaders"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/enums/arcsize"
	"github.com/richardwilkes/unison/enums/blendmode"
	"github.com/richardwilkes/unison/enums/blur"
	"github.com/richardwilkes/unison/enums/direction"
	"github.com/richardwilkes/unison/enums/filltype"
	"github.com/richardwilkes/unison/enums/filtermode"
	"github.com/richardwilkes/unison/enums/mipmapmode"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/patheffect"
	"github.com/richardwilkes/unison/enums/strokecap"
	"github.com/richardwilkes/unison/enums/strokejoin"
	"github.com/richardwilkes/unison/enums/tilemode"
	"github.com/richardwilkes/unison/enums/trimmode"
)

// enumPair records one unison enum value and the canvas enum value it is bridged to by a bare numeric cast somewhere in
// the package. The int fields are the raw underlying values of the two constants.
type enumPair struct {
	unison int
	canvas int
}

// checkEnumAlignment asserts that every unison↔canvas enum value agrees numerically and that the pair table stays in
// lock-step with the unison enum's own All slice. The unison enums and the canvas enums are two independently
// maintained iota orderings joined only by unchecked casts (e.g. paint.go's canvas.Style(style)); if the canvas side
// ever inserts or reorders a value, the cast would silently map to the wrong constant with no compile error. Listing
// each pair explicitly here catches that drift, and comparing len(pairs) against allLen forces any newly added unison
// value to be accounted for in this guard.
func checkEnumAlignment(c check.Checker, name string, allLen int, pairs []enumPair) {
	c.Equal(allLen, len(pairs), "%s: pair count must match len(All); update the alignment table", name)
	for i, p := range pairs {
		c.Equal(p.unison, p.canvas, "%s: value %d bridged by a cast no longer agrees with canvas", name, i)
	}
}

func TestEnumAlignmentWithCanvas(t *testing.T) {
	c := check.New(t)

	// paintstyle.Enum ↔ canvas.Style (paint.go)
	checkEnumAlignment(c, "paintstyle", len(paintstyle.All), []enumPair{
		{int(paintstyle.Fill), int(canvas.StyleFill)},
		{int(paintstyle.Stroke), int(canvas.StyleStroke)},
		{int(paintstyle.StrokeAndFill), int(canvas.StyleStrokeAndFill)},
	})

	// strokecap.Enum ↔ canvas.StrokeCap (paint.go)
	checkEnumAlignment(c, "strokecap", len(strokecap.All), []enumPair{
		{int(strokecap.Butt), int(canvas.CapButt)},
		{int(strokecap.Round), int(canvas.CapRound)},
		{int(strokecap.Square), int(canvas.CapSquare)},
	})

	// strokejoin.Enum ↔ canvas.StrokeJoin (paint.go)
	checkEnumAlignment(c, "strokejoin", len(strokejoin.All), []enumPair{
		{int(strokejoin.Miter), int(canvas.JoinMiter)},
		{int(strokejoin.Round), int(canvas.JoinRound)},
		{int(strokejoin.Bevel), int(canvas.JoinBevel)},
	})

	// blendmode.Enum ↔ raster.BlendMode (paint.go, shader.go, color_filter.go, canvas.go)
	checkEnumAlignment(c, "blendmode", len(blendmode.All), []enumPair{
		{int(blendmode.Clear), int(raster.BlendClear)},
		{int(blendmode.Src), int(raster.BlendSrc)},
		{int(blendmode.Dst), int(raster.BlendDst)},
		{int(blendmode.SrcOver), int(raster.BlendSrcOver)},
		{int(blendmode.DstOver), int(raster.BlendDstOver)},
		{int(blendmode.SrcIn), int(raster.BlendSrcIn)},
		{int(blendmode.DstIn), int(raster.BlendDstIn)},
		{int(blendmode.SrcOut), int(raster.BlendSrcOut)},
		{int(blendmode.DstOut), int(raster.BlendDstOut)},
		{int(blendmode.SrcAtop), int(raster.BlendSrcATop)},
		{int(blendmode.DstAtop), int(raster.BlendDstATop)},
		{int(blendmode.Xor), int(raster.BlendXor)},
		{int(blendmode.Plus), int(raster.BlendPlus)},
		{int(blendmode.Modulate), int(raster.BlendModulate)},
		{int(blendmode.Screen), int(raster.BlendScreen)},
		{int(blendmode.Overlay), int(raster.BlendOverlay)},
		{int(blendmode.Darken), int(raster.BlendDarken)},
		{int(blendmode.Lighten), int(raster.BlendLighten)},
		{int(blendmode.ColorDodge), int(raster.BlendColorDodge)},
		{int(blendmode.ColorBurn), int(raster.BlendColorBurn)},
		{int(blendmode.HardLight), int(raster.BlendHardLight)},
		{int(blendmode.SoftLight), int(raster.BlendSoftLight)},
		{int(blendmode.Difference), int(raster.BlendDifference)},
		{int(blendmode.Exclusion), int(raster.BlendExclusion)},
		{int(blendmode.Multiply), int(raster.BlendMultiply)},
		{int(blendmode.Hue), int(raster.BlendHue)},
		{int(blendmode.Saturation), int(raster.BlendSaturation)},
		{int(blendmode.Color), int(raster.BlendColor)},
		{int(blendmode.Luminosity), int(raster.BlendLuminosity)},
	})

	// filltype.Enum ↔ path.FillType (path.go)
	checkEnumAlignment(c, "filltype", len(filltype.All), []enumPair{
		{int(filltype.Winding), int(path.FillWinding)},
		{int(filltype.EvenOdd), int(path.FillEvenOdd)},
		{int(filltype.InverseWinding), int(path.FillInverseWinding)},
		{int(filltype.InverseEvenOdd), int(path.FillInverseEvenOdd)},
	})

	// arcsize.Enum ↔ path.ArcSize (path.go)
	checkEnumAlignment(c, "arcsize", len(arcsize.All), []enumPair{
		{int(arcsize.Small), int(path.ArcSizeSmall)},
		{int(arcsize.Large), int(path.ArcSizeLarge)},
	})

	// direction.Enum ↔ skgeom.PathDirection (path.go)
	checkEnumAlignment(c, "direction", len(direction.All), []enumPair{
		{int(direction.Clockwise), int(skgeom.DirectionCW)},
		{int(direction.CounterClockwise), int(skgeom.DirectionCCW)},
	})

	// blur.Enum ↔ maskfilter.BlurStyle (mask_filter.go)
	checkEnumAlignment(c, "blur", len(blur.All), []enumPair{
		{int(blur.Normal), int(maskfilter.BlurNormal)},
		{int(blur.Solid), int(maskfilter.BlurSolid)},
		{int(blur.Outer), int(maskfilter.BlurOuter)},
		{int(blur.Inner), int(maskfilter.BlurInner)},
	})

	// tilemode.Enum ↔ shaders.TileMode (shader.go, image_filter.go)
	checkEnumAlignment(c, "tilemode", len(tilemode.All), []enumPair{
		{int(tilemode.Clamp), int(shaders.TileClamp)},
		{int(tilemode.Repeat), int(shaders.TileRepeat)},
		{int(tilemode.Mirror), int(shaders.TileMirror)},
		{int(tilemode.Decal), int(shaders.TileDecal)},
	})

	// filtermode.Enum ↔ shaders.FilterMode (sampling_options.go, canvas.go)
	checkEnumAlignment(c, "filtermode", len(filtermode.All), []enumPair{
		{int(filtermode.Nearest), int(shaders.FilterNearest)},
		{int(filtermode.Linear), int(shaders.FilterLinear)},
	})

	// mipmapmode.Enum ↔ shaders.MipmapMode (sampling_options.go)
	checkEnumAlignment(c, "mipmapmode", len(mipmapmode.All), []enumPair{
		{int(mipmapmode.None), int(shaders.MipmapNone)},
		{int(mipmapmode.Nearest), int(shaders.MipmapNearest)},
		{int(mipmapmode.Linear), int(shaders.MipmapLinear)},
	})

	// patheffect.Enum ↔ skpatheffect.Path1DStyle (path_effect.go)
	checkEnumAlignment(c, "patheffect", len(patheffect.All), []enumPair{
		{int(patheffect.Translate), int(skpatheffect.Path1DTranslate)},
		{int(patheffect.Rotate), int(skpatheffect.Path1DRotate)},
		{int(patheffect.Morph), int(skpatheffect.Path1DMorph)},
	})

	// trimmode.Enum ↔ skpatheffect.TrimMode (path_effect.go)
	checkEnumAlignment(c, "trimmode", len(trimmode.All), []enumPair{
		{int(trimmode.Normal), int(skpatheffect.TrimNormal)},
		{int(trimmode.Inverted), int(skpatheffect.TrimInverted)},
	})
}
