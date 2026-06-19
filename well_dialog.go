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
	"log/slog"
	"math/bits"
	"slices"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/imgfmt"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

type wellDialog struct {
	well        *Well
	originalInk Ink
	ink         Ink
	dialog      *Dialog
	right       *Panel
	popup       *PopupMenu[string]
	options     []WellMask
	current     WellMask
	syncing     bool
}

func showWellDialog(w *Well) {
	if w.Mask&(ColorWellMask|GradientWellMask|PatternWellMask) == 0 {
		slog.Warn("well mask doesn't enable color, gradient, or pattern")
		return
	}
	d := &wellDialog{
		well:        w,
		originalInk: w.Ink(),
		ink:         w.Ink(),
		current:     255,
	}
	content := NewPanel()
	content.SetLayout(&FlexLayout{
		Columns:  2,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})

	left := NewPanel()
	left.SetBorder(NewEmptyBorder(geom.Insets{Right: 2 * StdHSpacing}))
	left.SetLayoutData(&FlexLayoutData{})
	left.SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	content.AddChild(left)
	d.addPreviewBlock(left, i18n.Text("Preview"), 0, func() Ink { return d.ink })
	d.addPreviewBlock(left, i18n.Text("Original"), 16, func() Ink { return d.originalInk })

	d.right = NewPanel()
	d.right.SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing * 2,
	})
	d.right.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Start,
		HGrab:  true,
	})
	content.AddChild(d.right)

	if d.hasMultipleEditors() {
		labels := make([]string, 0, 3)
		d.options = make([]WellMask, 0, 3)
		if d.well.Mask&ColorWellMask != 0 {
			labels = append(labels, i18n.Text("Color"))
			d.options = append(d.options, ColorWellMask)
		}
		if d.well.Mask&GradientWellMask != 0 {
			labels = append(labels, i18n.Text("Gradient"))
			d.options = append(d.options, GradientWellMask)
		}
		if d.well.Mask&PatternWellMask != 0 {
			labels = append(labels, i18n.Text("Pattern"))
			d.options = append(d.options, PatternWellMask)
		}
		d.popup = NewPopupMenu[string]()
		d.popup.AddItem(labels...)
		d.popup.SelectionChangedCallback = func(popup *PopupMenu[string]) {
			d.switchEditor(d.options[WellMask(popup.SelectedIndex())])
		}
		d.popup.SetLayoutData(&FlexLayoutData{
			HAlign: align.Middle,
			HGrab:  true,
		})
		d.right.AddChild(d.popup)
	}
	var current WellMask
	switch d.ink.(type) {
	case Color:
		current = ColorWellMask
	case *Color:
		current = ColorWellMask
	case *Pattern:
		current = PatternWellMask
	case *Gradient:
		current = GradientWellMask
	}
	if current&d.well.Mask == 0 {
		// Data type doesn't match, so select the mask that corresponds to the last enabled bit
		current = WellMask(1 << bits.TrailingZeros8(uint8(d.well.Mask)))
	}
	d.switchEditor(current)

	var err error
	d.dialog, err = NewDialog(nil, nil, content, []*DialogButtonInfo{NewCancelButtonInfo(), NewOKButtonInfo()},
		NotResizableWindowOption())
	if err != nil {
		errs.Log(err)
		return
	}
	d.dialog.Window().SetTitle(i18n.Text("Choose an ink"))
	if d.dialog.RunModal() == ModalResponseOK {
		w.SetInk(d.ink)
	}
}

func (d *wellDialog) hasMultipleEditors() bool {
	return bits.OnesCount8(uint8(d.well.Mask)) > 1
}

func (d *wellDialog) switchEditor(which WellMask) {
	if d.syncing || which == d.current {
		return
	}
	d.syncing = true
	defer func() { d.syncing = false }()
	if d.hasMultipleEditors() {
		d.popup.SelectIndex(slices.Index(d.options, which))
		if len(d.right.Children()) > 1 {
			d.right.RemoveChildAtIndex(1)
		}
	}
	switch which {
	case ColorWellMask:
		d.right.AddChild(d.createColorEditor())
	case GradientWellMask:
		d.right.AddChild(d.createGradientEditor())
	case PatternWellMask:
		d.right.AddChild(d.createPatternSelector())
	}
	if d.dialog != nil {
		w := d.dialog.Window()
		w.Content().MarkForLayoutRecursively()
		w.MarkForRedraw()
		w.Pack()
	}
}

func (d *wellDialog) createColorEditor() *ColorEditor {
	c := Black
	switch v := d.ink.(type) {
	case Color:
		c = v
	case *Color:
		c = *v
	case *Gradient:
		if len(v.Stops) > 0 {
			c = v.Stops[0].Color.GetColor()
		}
	default:
	}
	e := NewColorEditor(c)
	e.ChangedCallback = func() { d.ink = e.Color() }
	return e
}

func (d *wellDialog) createGradientEditor() *GradientEditor {
	var g *Gradient
	switch v := d.ink.(type) {
	case Color:
		g = &Gradient{
			Stops:     NewEvenlySpacedGradientStopsForColors(v, v),
			EndPt:     geom.NewPoint(1, 0),
			Transform: geom.NewIdentityMatrix(),
		}
	case *Color:
		g = &Gradient{
			Stops:     NewEvenlySpacedGradientStopsForColors(*v, *v),
			EndPt:     geom.NewPoint(1, 0),
			Transform: geom.NewIdentityMatrix(),
		}
	case *Pattern:
	case *Gradient:
		g = v
	}
	e := NewGradientEditor(g)
	e.ChangedCallback = func() { d.ink = e.Gradient() }
	return e
}

func (d *wellDialog) createPatternSelector() *Button {
	b := NewButton()
	b.SetTitle(i18n.Text("Select Image…"))
	b.SetLayoutData(&FlexLayoutData{
		HAlign: align.Middle,
		VAlign: align.Middle,
	})
	b.ClickCallback = func() {
		openDialog := NewOpenDialog()
		openDialog.SetAllowedExtensions(imgfmt.AllReadableExtensions()...)
		if openDialog.RunModal() {
			unable := i18n.Text("Unable to load image")
			paths := openDialog.Paths()
			if len(paths) == 0 {
				ErrorDialogWithMessage(unable, "Invalid path")
				return
			}
			imageSpec := imgfmt.Distill(paths[0])
			if imageSpec == "" {
				ErrorDialogWithMessage(unable, "Invalid image file")
				return
			}
			img, err := d.well.loadImage(imageSpec)
			if err != nil {
				ErrorDialogWithError(unable, err)
				return
			}
			if d.well.ValidateImageCallback != nil {
				SafeCall(func() { img = d.well.ValidateImageCallback(img) })
			}
			if img == nil {
				ErrorDialogWithMessage(unable, "")
				return
			}
			d.ink = &Pattern{Image: img}
			d.dialog.Window().MarkForRedraw()
		}
	}
	return b
}

func (d *wellDialog) addPreviewBlock(parent *Panel, title string, spaceBefore float32, inkRetriever func() Ink) {
	label := NewLabel()
	label.SetTitle(title)
	label.HAlign = align.Middle
	label.SetLayoutData(&FlexLayoutData{
		HAlign: align.Middle,
		VAlign: align.Middle,
	})
	if spaceBefore > 0 {
		label.SetBorder(NewEmptyBorder(geom.Insets{Top: spaceBefore}))
	}
	parent.AddChild(label)

	preview := NewPanel()
	preview.SetBorder(NewCompoundBorder(
		NewLineBorder(ThemeOnSurface, geom.Size{}, geom.NewUniformInsets(1), false),
		NewLineBorder(ThemeSurface, geom.Size{}, geom.NewUniformInsets(1), false),
	))
	preview.SetLayoutData(&FlexLayoutData{
		SizeHint: geom.NewSize(64, 64),
	})
	preview.DrawCallback = func(canvas *Canvas, _ geom.Rect) {
		r := preview.ContentRect(false)
		ink := inkRetriever()
		if pattern, ok := ink.(*Pattern); ok {
			canvas.DrawImageInRect(pattern.Image, r, nil, nil)
		} else {
			paint := ink.Paint(canvas, r, paintstyle.Fill)
			canvas.DrawRect(r, paint)
			paint.Dispose()
		}
	}
	parent.AddChild(preview)
}
