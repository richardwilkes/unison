// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"strconv"
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xmath/geom32"
)

type wellDialog struct {
	well             *Well
	ink              Ink
	dialog           *Dialog
	panel            *Panel
	preview          *Panel
	right            *Panel
	redField         *Field
	greenField       *Field
	blueField        *Field
	alphaField       *Field
	validationFields []*Field
}

// TODO: Implement gradient selection

func showWellDialog(w *Well) {
	d := &wellDialog{
		well:    w,
		ink:     w.Ink(),
		panel:   NewPanel(),
		preview: NewPanel(),
		right:   NewPanel(),
	}
	d.preview.SetBorder(NewCompoundBorder(NewLineBorder(OnBackgroundColor, 0, geom32.NewUniformInsets(1), false),
		NewLineBorder(BackgroundColor, 0, geom32.NewUniformInsets(1), false)))
	d.panel.SetLayout(&FlexLayout{
		Columns:  2,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	d.preview.SetLayoutData(&FlexLayoutData{
		SizeHint: geom32.NewSize(64, 64),
		HSpan:    1,
		VSpan:    1,
	})
	d.preview.DrawCallback = func(canvas *Canvas, dirty geom32.Rect) {
		r := d.preview.ContentRect(false)
		if pattern, ok := d.ink.(*Pattern); ok {
			canvas.DrawImageInRect(pattern.Image, r, nil, nil)
		} else {
			canvas.DrawRect(r, d.ink.Paint(canvas, r, Fill))
		}
	}
	d.panel.AddChild(d.preview)
	d.right.SetLayout(&FlexLayout{
		Columns:  2,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	d.right.SetLayoutData(&FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: FillAlignment,
		VAlign: MiddleAlignment,
		HGrab:  true,
	})
	d.panel.AddChild(d.right)
	if w.Mask&ColorWellMask != 0 {
		color := Black
		switch inkColor := d.ink.(type) {
		case Color:
			color = inkColor
		case *Color:
			color = *inkColor
		default:
		}
		d.redField = d.addEntryField(i18n.Text("Red:"), color.Red())
		d.greenField = d.addEntryField(i18n.Text("Green:"), color.Green())
		d.blueField = d.addEntryField(i18n.Text("Blue:"), color.Blue())
		d.alphaField = d.addEntryField(i18n.Text("Alpha:"), color.Alpha())
	}
	if w.Mask&PatternWellMask != 0 {
		b := NewButton()
		b.Text = i18n.Text("Select Image…")
		b.SetLayoutData(&FlexLayoutData{
			HSpan:  2,
			VSpan:  1,
			HAlign: MiddleAlignment,
			VAlign: MiddleAlignment,
		})
		b.ClickCallback = func() {
			openDialog := NewOpenDialog()
			openDialog.SetAllowedExtensions(KnownImageFormatExtensions...)
			if openDialog.RunModal() {
				unable := i18n.Text("Unable to load image")
				paths := openDialog.Paths()
				if len(paths) == 0 {
					ErrorDialogWithMessage(unable, "Invalid path")
					return
				}
				imageSpec := DistillImageSpecFor(paths[0])
				if imageSpec == "" {
					ErrorDialogWithMessage(unable, "Invalid image file")
					return
				}
				img, err := d.well.ImageFromSpecCallback(imageSpec, d.well.ImageScale)
				if err != nil {
					ErrorDialogWithError(unable, err)
					return
				}
				if d.well.ValidateImageCallback != nil {
					img = d.well.ValidateImageCallback(img)
				}
				if img == nil {
					ErrorDialogWithMessage(unable, "")
					return
				}
				d.ink = &Pattern{Image: img}
				d.preview.MarkForRedraw()
			}
		}
		if len(d.right.Children()) > 0 {
			b.SetBorder(NewEmptyBorder(geom32.Insets{Top: 10}))
		}
		d.right.AddChild(b)
	}
	var err error
	d.dialog, err = NewDialog(nil, d.panel, []*DialogButtonInfo{NewCancelButtonInfo(), NewOKButtonInfo()})
	if err != nil {
		jot.Error(err)
		return
	}
	d.dialog.Window().SetTitle(i18n.Text("Choose an ink"))
	if d.dialog.RunModal() == ModalResponseOK {
		w.SetInk(d.ink)
	}
}

func (d *wellDialog) addEntryField(title string, value int) *Field {
	l := NewLabel()
	l.Text = title
	l.HAlign = EndAlignment
	l.SetLayoutData(&FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: EndAlignment,
		VAlign: MiddleAlignment,
	})
	d.right.AddChild(l)
	field := NewField()
	field.SetText(strconv.Itoa(value))
	field.Watermark = "0"
	field.MinimumTextWidth = 50
	field.SetLayoutData(&FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: FillAlignment,
		VAlign: MiddleAlignment,
		HGrab:  true,
	})
	field.ValidateCallback = func() bool {
		_, valid := d.parseField(field)
		if valid {
			var r int
			if r, valid = d.parseField(d.redField); valid {
				var g int
				if g, valid = d.parseField(d.greenField); valid {
					var b int
					if b, valid = d.parseField(d.blueField); valid {
						var a int
						if a, valid = d.parseField(d.alphaField); valid {
							d.ink = ARGB(float32(a)/255, r, g, b)
							d.preview.MarkForRedraw()
						}
					}
				}
			}
		}
		d.adjustOKButton(field, valid)
		return valid
	}
	d.validationFields = append(d.validationFields, field)
	d.right.AddChild(field)
	return field
}

func (d *wellDialog) parseField(field *Field) (int, bool) {
	text := strings.TrimSpace(field.Text())
	if text == "" {
		text = "0"
	}
	v, err := strconv.Atoi(text)
	return v, err == nil && v >= 0 && v <= 255
}

func (d *wellDialog) adjustOKButton(field *Field, valid bool) {
	if d.dialog != nil {
		enabled := valid
		if enabled {
			for _, f := range d.validationFields {
				if f != field && f.Invalid() {
					enabled = false
					break
				}
			}
		}
		d.dialog.Button(ModalResponseOK).SetEnabled(enabled)
	}
}
