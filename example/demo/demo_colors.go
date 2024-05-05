// Copyright ©2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package demo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
)

var (
	colorsWindow  *unison.Window
	currentColors []*themedColor
)

type themedColor struct {
	ID    string
	Title string
	Color *unison.ThemeColor
}

func init() {
	currentColors = []*themedColor{
		{ID: "primary", Title: "Primary", Color: &unison.PrimaryTheme.Primary},
		{ID: "on_primary", Title: "On Primary", Color: &unison.PrimaryTheme.OnPrimary},
		{ID: "primary_variant", Title: "Primary Variant", Color: &unison.PrimaryTheme.PrimaryVariant},
		{ID: "secondary", Title: "Secondary", Color: &unison.PrimaryTheme.Secondary},
		{ID: "on_secondary", Title: "On Secondary", Color: &unison.PrimaryTheme.OnSecondary},
		{ID: "secondary_variant", Title: "Secondary Variant", Color: &unison.PrimaryTheme.SecondaryVariant},
		{ID: "tertiary", Title: "Tertiary", Color: &unison.PrimaryTheme.Tertiary},
		{ID: "on_tertiary", Title: "On Tertiary", Color: &unison.PrimaryTheme.OnTertiary},
		{ID: "tertiary_variant", Title: "Tertiary Variant", Color: &unison.PrimaryTheme.TertiaryVariant},
		{ID: "surface", Title: "Surface", Color: &unison.PrimaryTheme.Surface},
		{ID: "on_surface", Title: "On Surface", Color: &unison.PrimaryTheme.OnSurface},
		{ID: "surface_above", Title: "Surface Above", Color: &unison.PrimaryTheme.SurfaceAbove},
		{ID: "surface_below", Title: "Surface Below", Color: &unison.PrimaryTheme.SurfaceBelow},
		{ID: "error", Title: "Error", Color: &unison.PrimaryTheme.Error},
		{ID: "on_error", Title: "On Error", Color: &unison.PrimaryTheme.OnError},
		{ID: "warning", Title: "Warning", Color: &unison.PrimaryTheme.Warning},
		{ID: "on_warning", Title: "On Warning", Color: &unison.PrimaryTheme.OnWarning},
		{ID: "outline", Title: "Outline", Color: &unison.PrimaryTheme.Outline},
		{ID: "outline_variant", Title: "Outline Variant", Color: &unison.PrimaryTheme.OutlineVariant},
		{ID: "shadow", Title: "Shadow", Color: &unison.PrimaryTheme.Shadow},
	}
}

// NewDemoColorsWindow creates and displays our demo colors window.
func NewDemoColorsWindow(where unison.Point) (*unison.Window, error) {
	if colorsWindow != nil {
		if colorsWindow.IsVisible() {
			return colorsWindow, nil
		}
		colorsWindow.Dispose()
		colorsWindow = nil
	}

	// Create the window
	wnd, err := unison.NewWindow("Colors")
	if err != nil {
		return nil, err
	}

	// Install our menus
	installDefaultMenus(wnd)

	content := wnd.Content()
	content.SetLayout(&unison.FlexLayout{Columns: 1})
	content.SetBorder(unison.NewEmptyBorder(unison.NewUniformInsets(10)))

	// Create the colors panel
	colorsPanel := unison.NewPanel()
	colorsPanel.SetLayout(&unison.FlexLayout{
		Columns:  6,
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})
	onColumn := false
	for _, one := range currentColors {
		on := strings.HasPrefix(one.ID, "on_")
		if on != onColumn {
			p := unison.NewPanel()
			p.SetLayoutData(&unison.FlexLayoutData{HSpan: 3})
			colorsPanel.AddChild(p)
			onColumn = !onColumn
		}
		label := unison.NewLabel()
		label.Text = one.Title
		label.SetLayoutData(&unison.FlexLayoutData{
			HAlign: align.End,
			VAlign: align.Middle,
		})
		if on {
			label.SetBorder(unison.NewEmptyBorder(unison.Insets{Left: unison.StdHSpacing}))
		}
		colorsPanel.AddChild(label)
		colorsPanel.AddChild(createColorWellField(one, true))
		colorsPanel.AddChild(createColorWellField(one, false))
		onColumn = !onColumn
	}

	// Create a scroll panel and place a table panel inside it
	scrollArea := unison.NewScrollPanel()
	scrollArea.SetContent(colorsPanel, behavior.Fill, behavior.Fill)
	scrollArea.SetLayoutData(&unison.FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Fill,
		HGrab:  true,
		VGrab:  true,
	})
	content.AddChild(scrollArea)

	jsonButton := unison.NewButton()
	jsonButton.Text = "Copy Theme As JSON"
	jsonButton.ClickCallback = func() {
		d, err := json.MarshalIndent(unison.PrimaryTheme, "", "  ")
		if err != nil {
			unison.ErrorDialogWithError("Unable to marshal the colors", err)
		} else {
			unison.GlobalClipboard.SetText(string(d))
		}
	}
	jsonButton.SetBorder(unison.NewEmptyBorder(unison.Insets{Top: 10}))
	jsonButton.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Middle})
	content.AddChild(jsonButton)

	goCodeButton := unison.NewButton()
	goCodeButton.Text = "Copy Theme As Go Code"
	goCodeButton.ClickCallback = func() {
		var buffer strings.Builder
		buffer.WriteString("var MyTheme = unison.Theme{\n")
		for _, one := range currentColors {
			fmt.Fprintf(&buffer, "\t%s: unison.ThemeColor{Light: %s, Dark: %s},\n",
				strings.ReplaceAll(one.Title, " ", ""),
				colorToRGBString(one.Color.Light),
				colorToRGBString(one.Color.Dark),
			)
		}
		buffer.WriteString("}\n")
		unison.GlobalClipboard.SetText(buffer.String())
	}
	goCodeButton.SetBorder(unison.NewEmptyBorder(unison.Insets{Top: 10}))
	goCodeButton.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Middle})
	content.AddChild(goCodeButton)

	// Pack our window to fit its content, then set its location on the display and make it visible.
	wnd.Pack()
	rect := wnd.FrameRect()
	rect.Point = where
	wnd.SetFrameRect(rect)
	wnd.ToFront()

	return wnd, nil
}

func colorToRGBString(c unison.Color) string {
	if c.HasAlpha() {
		return fmt.Sprintf("ARGB(%f, %d, %d, %d)", c.AlphaIntensity(), c.Red(), c.Green(), c.Blue())
	}
	return fmt.Sprintf("RGB(%d, %d, %d)", c.Red(), c.Green(), c.Blue())
}

func createColorWellField(c *themedColor, light bool) *unison.Well {
	w := unison.NewWell()
	w.Mask = unison.ColorWellMask
	if light {
		w.SetInk(c.Color.Light)
		w.Tooltip = unison.NewTooltipWithText("The color to use when light mode is enabled")
		w.InkChangedCallback = func() {
			if clr, ok := w.Ink().(unison.Color); ok {
				c.Color.Light = clr
				unison.ThemeChanged()
			}
		}
	} else {
		w.SetInk(c.Color.Dark)
		w.Tooltip = unison.NewTooltipWithText("The color to use when dark mode is enabled")
		w.InkChangedCallback = func() {
			if clr, ok := w.Ink().(unison.Color); ok {
				c.Color.Dark = clr
				unison.ThemeChanged()
			}
		}
	}
	return w
}
