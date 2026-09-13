// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package role

import (
	"strings"

	"github.com/richardwilkes/toolbox/v2/i18n"
)

// Possible values.
const (
	Auto               Enum = iota // Derive the role from the widget
	None                           // Not exposed at all; the children are promoted to the parent
	Window                         // A top-level window
	Dialog                         // A window that asks for a response before work can continue
	Group                          // A generic grouping of other elements
	Button                         // A control that performs an action when pressed
	ToggleButton                   // A button that stays in an on or off state
	DisclosureTriangle             // A control that shows or hides the children of a row
	CheckBox                       // A control with an on, off, or mixed state
	RadioButton                    // One choice within a mutually exclusive set
	Link                           // A control that navigates somewhere else
	Label                          // Static text that cannot be edited
	Heading                        // Static text that introduces a section; Level holds its depth
	TextField                      // A single-line editable text control
	TextArea                       // A multi-line editable text control
	SpinButton                     // An editable numeric control with increment and decrement
	ComboBox                       // An editable text control paired with a list of choices
	PopupButton                    // A button showing the current choice that pops up the list of choices
	Slider                         // A control that picks a value from a range by position
	ProgressBar                    // An indicator of how much of a task has been completed
	ScrollBar                      // A control that scrolls something else
	ScrollArea                     // A container presenting a scrollable view onto its content
	Separator                      // A divider between elements
	List                           // A container of selectable items
	ListItem                       // One item within a list
	Table                          // A container of rows divided into columns
	Tree                           // A table whose rows form a hierarchy
	Row                            // One row within a table or tree
	Cell                           // One cell within a row
	ColumnHeader                   // The header for one column
	TableHeader                    // The container holding a table's column headers
	TabList                        // A container of tabs
	Tab                            // One tab within a tab list
	TabPanel                       // The content shown for the selected tab
	MenuBar                        // The container holding a window's top-level menus
	Menu                           // A list of menu items
	MenuItem                       // One item within a menu
	Image                          // A graphic whose name describes what it shows
	ColorWell                      // A control that shows and edits a color
	Tooltip                        // Transient explanatory text for another element
	Document                       // A container of rich, readable content
	Toolbar                        // A container of frequently used controls
	Unknown                        // A role with no better match
)

// All possible values.
var All = []Enum{
	Auto,
	None,
	Window,
	Dialog,
	Group,
	Button,
	ToggleButton,
	DisclosureTriangle,
	CheckBox,
	RadioButton,
	Link,
	Label,
	Heading,
	TextField,
	TextArea,
	SpinButton,
	ComboBox,
	PopupButton,
	Slider,
	ProgressBar,
	ScrollBar,
	ScrollArea,
	Separator,
	List,
	ListItem,
	Table,
	Tree,
	Row,
	Cell,
	ColumnHeader,
	TableHeader,
	TabList,
	Tab,
	TabPanel,
	MenuBar,
	Menu,
	MenuItem,
	Image,
	ColorWell,
	Tooltip,
	Document,
	Toolbar,
	Unknown,
}

// Enum holds the semantic role a panel plays when it is presented to assistive technologies.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e <= Unknown {
		return e
	}
	return Auto
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Auto:
		return "auto"
	case None:
		return "none"
	case Window:
		return "window"
	case Dialog:
		return "dialog"
	case Group:
		return "group"
	case Button:
		return "button"
	case ToggleButton:
		return "toggle-button"
	case DisclosureTriangle:
		return "disclosure-triangle"
	case CheckBox:
		return "check-box"
	case RadioButton:
		return "radio-button"
	case Link:
		return "link"
	case Label:
		return "label"
	case Heading:
		return "heading"
	case TextField:
		return "text-field"
	case TextArea:
		return "text-area"
	case SpinButton:
		return "spin-button"
	case ComboBox:
		return "combo-box"
	case PopupButton:
		return "popup-button"
	case Slider:
		return "slider"
	case ProgressBar:
		return "progress-bar"
	case ScrollBar:
		return "scroll-bar"
	case ScrollArea:
		return "scroll-area"
	case Separator:
		return "separator"
	case List:
		return "list"
	case ListItem:
		return "list-item"
	case Table:
		return "table"
	case Tree:
		return "tree"
	case Row:
		return "row"
	case Cell:
		return "cell"
	case ColumnHeader:
		return "column-header"
	case TableHeader:
		return "table-header"
	case TabList:
		return "tab-list"
	case Tab:
		return "tab"
	case TabPanel:
		return "tab-panel"
	case MenuBar:
		return "menu-bar"
	case Menu:
		return "menu"
	case MenuItem:
		return "menu-item"
	case Image:
		return "image"
	case ColorWell:
		return "color-well"
	case Tooltip:
		return "tooltip"
	case Document:
		return "document"
	case Toolbar:
		return "toolbar"
	case Unknown:
		return "unknown"
	default:
		return Auto.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Auto:
		return i18n.Text("Auto")
	case None:
		return i18n.Text("None")
	case Window:
		return i18n.Text("Window")
	case Dialog:
		return i18n.Text("Dialog")
	case Group:
		return i18n.Text("Group")
	case Button:
		return i18n.Text("Button")
	case ToggleButton:
		return i18n.Text("Toggle-Button")
	case DisclosureTriangle:
		return i18n.Text("Disclosure-Triangle")
	case CheckBox:
		return i18n.Text("Check-Box")
	case RadioButton:
		return i18n.Text("Radio-Button")
	case Link:
		return i18n.Text("Link")
	case Label:
		return i18n.Text("Label")
	case Heading:
		return i18n.Text("Heading")
	case TextField:
		return i18n.Text("Text-Field")
	case TextArea:
		return i18n.Text("Text-Area")
	case SpinButton:
		return i18n.Text("Spin-Button")
	case ComboBox:
		return i18n.Text("Combo-Box")
	case PopupButton:
		return i18n.Text("Popup-Button")
	case Slider:
		return i18n.Text("Slider")
	case ProgressBar:
		return i18n.Text("Progress-Bar")
	case ScrollBar:
		return i18n.Text("Scroll-Bar")
	case ScrollArea:
		return i18n.Text("Scroll-Area")
	case Separator:
		return i18n.Text("Separator")
	case List:
		return i18n.Text("List")
	case ListItem:
		return i18n.Text("List-Item")
	case Table:
		return i18n.Text("Table")
	case Tree:
		return i18n.Text("Tree")
	case Row:
		return i18n.Text("Row")
	case Cell:
		return i18n.Text("Cell")
	case ColumnHeader:
		return i18n.Text("Column-Header")
	case TableHeader:
		return i18n.Text("Table-Header")
	case TabList:
		return i18n.Text("Tab-List")
	case Tab:
		return i18n.Text("Tab")
	case TabPanel:
		return i18n.Text("Tab-Panel")
	case MenuBar:
		return i18n.Text("Menu-Bar")
	case Menu:
		return i18n.Text("Menu")
	case MenuItem:
		return i18n.Text("Menu-Item")
	case Image:
		return i18n.Text("Image")
	case ColorWell:
		return i18n.Text("Color-Well")
	case Tooltip:
		return i18n.Text("Tooltip")
	case Document:
		return i18n.Text("Document")
	case Toolbar:
		return i18n.Text("Toolbar")
	case Unknown:
		return i18n.Text("Unknown")
	default:
		return Auto.String()
	}
}

// MarshalText implements the encoding.TextMarshaler interface.
func (e Enum) MarshalText() (text []byte, err error) {
	return []byte(e.Key()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (e *Enum) UnmarshalText(text []byte) error {
	*e = Extract(string(text))
	return nil
}

// Extract the value from a string.
func Extract(str string) Enum {
	for _, e := range All {
		if strings.EqualFold(e.Key(), str) {
			return e
		}
	}
	return Auto
}
