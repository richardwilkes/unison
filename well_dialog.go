// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/imgfmt"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

type wellDialog struct {
	well            *Well
	originalInk     Ink
	ink             Ink
	dialog          *Dialog
	panel           *Panel
	redField        *Field
	greenField      *Field
	blueField       *Field
	alphaField      *Field
	hueField        *Field
	saturationField *Field
	brightnessField *Field
	cssField        *Field
	syncing         bool
}

// TODO: Implement gradient selection

func showWellDialog(w *Well) {
	d := &wellDialog{
		well:        w,
		originalInk: w.Ink(),
		ink:         w.Ink(),
		panel:       NewPanel(),
	}
	d.panel.SetLayout(&FlexLayout{
		Columns:  2,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})

	left := NewPanel()
	left.SetBorder(NewEmptyBorder(Insets{Right: 2 * StdHSpacing}))
	left.SetLayoutData(&FlexLayoutData{})
	left.SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	d.panel.AddChild(left)
	d.addPreviewBlock(left, i18n.Text("Preview"), 0, func() Ink { return d.ink })
	d.addPreviewBlock(left, i18n.Text("Original"), 16, func() Ink { return d.originalInk })

	right := NewPanel()
	right.SetLayout(&FlexLayout{
		Columns:  2,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	right.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Middle,
		HGrab:  true,
	})
	d.panel.AddChild(right)

	if w.Mask&PatternWellMask != 0 {
		d.addPatternSelector(right)
	}
	if w.Mask&ColorWellMask != 0 {
		d.addColorSelector(right)
	}

	var err error
	d.dialog, err = NewDialog(nil, nil, d.panel, []*DialogButtonInfo{NewCancelButtonInfo(), NewOKButtonInfo()})
	if err != nil {
		errs.Log(err)
		return
	}
	d.dialog.Window().SetTitle(i18n.Text("Choose an ink"))
	if d.dialog.RunModal() == ModalResponseOK {
		w.SetInk(d.ink)
	}
}

func (d *wellDialog) addPatternSelector(parent *Panel) {
	b := NewButton()
	b.Text = i18n.Text("Select Image…")
	b.SetLayoutData(&FlexLayoutData{
		HSpan:  2,
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
				img = d.well.ValidateImageCallback(img)
			}
			if img == nil {
				ErrorDialogWithMessage(unable, "")
				return
			}
			d.ink = &Pattern{Image: img}
			d.dialog.Window().MarkForRedraw()
		}
	}
	if d.well.Mask&^PatternWellMask != 0 {
		b.SetBorder(NewEmptyBorder(Insets{Bottom: 2 * StdHSpacing}))
	}
	parent.AddChild(b)
}

func (d *wellDialog) addColorSelector(parent *Panel) {
	color := Black
	switch inkColor := d.ink.(type) {
	case Color:
		color = inkColor
	case *Color:
		color = *inkColor
	default:
	}

	left := NewPanel()
	left.SetLayout(&FlexLayout{
		Columns:  3,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	left.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	parent.AddChild(left)

	d.redField = d.addChannelField(left, i18n.Text("Red:"), color.Red(), func(value int, color Color) Color {
		return color.SetRed(value)
	})
	d.greenField = d.addChannelField(left, i18n.Text("Green:"), color.Green(), func(value int, color Color) Color {
		return color.SetGreen(value)
	})
	d.blueField = d.addChannelField(left, i18n.Text("Blue:"), color.Blue(), func(value int, color Color) Color {
		return color.SetBlue(value)
	})
	d.alphaField = d.addChannelField(left, i18n.Text("Alpha:"), color.Alpha(), func(value int, color Color) Color {
		return color.SetAlpha(value)
	})

	right := NewPanel()
	right.SetBorder(NewEmptyBorder(Insets{Left: 2 * StdHSpacing}))
	right.SetLayout(&FlexLayout{
		Columns:  3,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	right.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	parent.AddChild(right)

	d.hueField = d.addHueField(right, color)
	d.saturationField = d.addPercentageField(right, i18n.Text("Saturation:"), color.Saturation(), func(value float32, color Color) Color {
		return color.SetSaturation(value)
	})
	d.brightnessField = d.addPercentageField(right, i18n.Text("Brightness:"), color.Brightness(), func(value float32, color Color) Color {
		return color.SetBrightness(value)
	})

	bottom := NewPanel()
	bottom.SetBorder(NewEmptyBorder(Insets{Top: 2 * StdHSpacing}))
	bottom.SetLayout(&FlexLayout{
		Columns:  2,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	bottom.SetLayoutData(&FlexLayoutData{
		HSpan:  2,
		HAlign: align.Fill,
		HGrab:  true,
	})
	parent.AddChild(bottom)

	d.cssField = d.addCSSField(bottom, color)
}

func (d *wellDialog) addChannelField(parent *Panel, title string, value int, adjuster func(value int, color Color) Color) *Field {
	l := NewLabel()
	l.Text = title
	l.HAlign = align.End
	l.SetLayoutData(&FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	parent.AddChild(l)
	field := NewField()
	field.SetText(strconv.Itoa(value))
	field.Watermark = "0"
	field.SetMinimumTextWidthUsing("255", "100%")
	field.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Middle,
		HGrab:  true,
	})
	field.ValidateCallback = func() bool {
		text := strings.TrimSpace(field.Text())
		if text == "" {
			text = "0"
		}
		var v int
		if strings.HasSuffix(text, "%") {
			percentage, err := extractColorPercentage(text)
			if err != nil {
				return false
			}
			v = clamp0To1AndScale255(percentage)
		} else {
			var err error
			if v, err = strconv.Atoi(text); err != nil || v < 0 || v > 255 {
				return false
			}
		}
		if !d.syncing {
			color, ok := d.ink.(Color)
			if !ok {
				color = Black
			}
			d.ink = adjuster(v, color)
			d.sync()
		}
		return true
	}
	parent.AddChild(field)
	l = NewLabel()
	l.Text = i18n.Text("0-255 or 0-100%")
	l.SetEnabled(false)
	l.SetLayoutData(&FlexLayoutData{
		VAlign: align.Middle,
	})
	parent.AddChild(l)
	return field
}

func (d *wellDialog) addHueField(parent *Panel, color Color) *Field {
	l := NewLabel()
	l.Text = i18n.Text("Hue:")
	l.HAlign = align.End
	l.SetLayoutData(&FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	parent.AddChild(l)
	field := NewField()
	field.SetText(strconv.Itoa(int(color.Hue()*360 + 0.5)))
	field.Watermark = "0"
	field.SetMinimumTextWidthUsing("360", "100%")
	field.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Middle,
		HGrab:  true,
	})
	field.ValidateCallback = func() bool {
		text := strings.TrimSpace(field.Text())
		if text == "" {
			text = "0"
		}
		var percentage float32
		if strings.HasSuffix(text, "%") {
			var err error
			if percentage, err = extractColorPercentage(text); err != nil {
				return false
			}
		} else {
			v, err := strconv.Atoi(text)
			if err != nil || v < 0 || v > 360 {
				return false
			}
			percentage = float32(v) / 360
		}
		if !d.syncing {
			c, ok := d.ink.(Color)
			if !ok {
				c = Black
			}
			d.ink = c.SetHue(percentage)
			d.sync()
		}
		return true
	}
	parent.AddChild(field)
	l = NewLabel()
	l.Text = i18n.Text("0-360 or 0-100%")
	l.SetEnabled(false)
	l.SetLayoutData(&FlexLayoutData{
		VAlign: align.Middle,
	})
	parent.AddChild(l)
	return field
}

func (d *wellDialog) addPercentageField(parent *Panel, title string, value float32, adjuster func(value float32, color Color) Color) *Field {
	l := NewLabel()
	l.Text = title
	l.HAlign = align.End
	l.SetLayoutData(&FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	parent.AddChild(l)
	field := NewField()
	field.SetText(strconv.Itoa(int(value*100+0.5)) + "%")
	field.Watermark = "0%"
	field.SetMinimumTextWidthUsing("100%")
	field.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Middle,
		HGrab:  true,
	})
	field.ValidateCallback = func() bool {
		text := strings.TrimSpace(field.Text())
		if text == "" {
			text = "0%"
		}
		if !strings.HasSuffix(text, "%") {
			text += "%"
		}
		percentage, err := extractColorPercentage(text)
		if err != nil {
			return false
		}
		if !d.syncing {
			color, ok := d.ink.(Color)
			if !ok {
				color = Black
			}
			d.ink = adjuster(percentage, color)
			d.sync()
		}
		return true
	}
	parent.AddChild(field)
	l = NewLabel()
	l.Text = i18n.Text("0-100%")
	l.SetEnabled(false)
	l.SetLayoutData(&FlexLayoutData{
		VAlign: align.Middle,
	})
	parent.AddChild(l)
	return field
}

func (d *wellDialog) addCSSField(parent *Panel, color Color) *Field {
	l := NewLabel()
	l.Text = i18n.Text("CSS:")
	l.HAlign = align.End
	l.SetLayoutData(&FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	parent.AddChild(l)
	field := NewField()
	field.SetText(color.String())
	field.Watermark = "CSS"
	field.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Middle,
		HGrab:  true,
	})
	field.ValidateCallback = func() bool {
		if !d.syncing {
			adjustedColor, err := ColorDecode(field.Text())
			if err != nil {
				return false
			}
			d.ink = adjustedColor
			d.sync()
		}
		return true
	}
	parent.AddChild(field)
	for _, txt := range []string{
		i18n.Text(`- predefined color name, e.g. "Yellow"`),
		i18n.Text(`- rgb(), e.g. "rgb(255, 255, 0)"`),
		i18n.Text(`- rgba(), e.g. "rgba(255, 255, 0, 0.3)"`),
		i18n.Text(`- short hexadecimal colors, e.g. "#FF0"`),
		i18n.Text(`- long hexadecimal colors, e.g. "#FFFF00"`),
		i18n.Text(`- hsl(), e.g. "hsl(120, 100%, 50%)"`),
		i18n.Text(`- hsla(), e.g. "hsla(120, 100%, 50%, 0.3)"`),
	} {
		parent.AddChild(NewLabel())
		l = NewLabel()
		l.Text = txt
		l.SetEnabled(false)
		l.SetBorder(NewEmptyBorder(Insets{Left: StdHSpacing}))
		l.SetLayoutData(&FlexLayoutData{
			VAlign: align.Middle,
		})
		parent.AddChild(l)
	}
	return field
}

func (d *wellDialog) sync() {
	d.syncing = true
	switch t := d.ink.(type) {
	case Color:
		d.syncText(d.redField, strconv.Itoa(t.Red()))
		d.syncText(d.greenField, strconv.Itoa(t.Green()))
		d.syncText(d.blueField, strconv.Itoa(t.Blue()))
		d.syncText(d.alphaField, strconv.Itoa(t.Alpha()))
		d.syncText(d.hueField, strconv.Itoa(int(t.Hue()*360+0.5)))
		d.syncText(d.saturationField, strconv.Itoa(int(t.Saturation()*100+0.5))+"%")
		d.syncText(d.brightnessField, strconv.Itoa(int(t.Brightness()*100+0.5))+"%")
		d.syncText(d.cssField, t.String())
	default:
	}
	d.syncing = false
}

func (d *wellDialog) syncText(field *Field, text string) {
	if !field.Focused() {
		field.SetText(text)
	}
}

func (d *wellDialog) addPreviewBlock(parent *Panel, title string, spaceBefore float32, inkRetriever func() Ink) {
	label := NewLabel()
	label.Text = title
	label.HAlign = align.Middle
	label.SetLayoutData(&FlexLayoutData{
		HAlign: align.Middle,
		VAlign: align.Middle,
	})
	if spaceBefore > 0 {
		label.SetBorder(NewEmptyBorder(Insets{Top: spaceBefore}))
	}
	parent.AddChild(label)

	preview := NewPanel()
	preview.SetBorder(NewCompoundBorder(
		NewLineBorder(&PrimaryTheme.OnSurface, 0, NewUniformInsets(1), false),
		NewLineBorder(&PrimaryTheme.Surface, 0, NewUniformInsets(1), false),
	))
	preview.SetLayoutData(&FlexLayoutData{
		SizeHint: Size{Width: 64, Height: 64},
	})
	preview.DrawCallback = func(canvas *Canvas, _ Rect) {
		r := preview.ContentRect(false)
		ink := inkRetriever()
		if pattern, ok := ink.(*Pattern); ok {
			canvas.DrawImageInRect(pattern.Image, r, nil, nil)
		} else {
			canvas.DrawRect(r, ink.Paint(canvas, r, paintstyle.Fill))
		}
	}
	parent.AddChild(preview)
}
