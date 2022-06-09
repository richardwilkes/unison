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
	"fmt"

	"github.com/richardwilkes/toolbox/xmath"
)

// DefaultPopupMenuTheme holds the default PopupMenuTheme values for PopupMenus. Modifying this data will not alter
// existing PopupMenus, but will alter any PopupMenus created in the future.
var DefaultPopupMenuTheme = PopupMenuTheme{
	Font:            SystemFont,
	BackgroundInk:   ControlColor,
	OnBackgroundInk: OnControlColor,
	EdgeInk:         ControlEdgeColor,
	SelectionInk:    SelectionColor,
	OnSelectionInk:  OnSelectionColor,
	CornerRadius:    4,
	HMargin:         8,
	VMargin:         1,
}

// PopupMenuTheme holds theming data for a PopupMenu.
type PopupMenuTheme struct {
	Font            Font
	BackgroundInk   Ink
	OnBackgroundInk Ink
	EdgeInk         Ink
	SelectionInk    Ink
	OnSelectionInk  Ink
	CornerRadius    float32
	HMargin         float32
	VMargin         float32
}

type popupMenuItem[T comparable] struct {
	item      T
	textCache TextCache
	enabled   bool
	separator bool
}

// PopupMenu represents a clickable button that displays a menu of choices.
type PopupMenu[T comparable] struct {
	Panel
	PopupMenuTheme
	MenuFactory       MenuFactory
	SelectionCallback func(index int, item T)
	items             []*popupMenuItem[T]
	selectedIndex     int
	textCache         TextCache
}

// NewPopupMenu creates a new PopupMenu.
func NewPopupMenu[T comparable]() *PopupMenu[T] {
	p := &PopupMenu[T]{PopupMenuTheme: DefaultPopupMenuTheme}
	p.Self = p
	p.SetFocusable(true)
	p.SetSizer(p.DefaultSizes)
	p.MenuFactory = DefaultMenuFactory()
	p.DrawCallback = p.DefaultDraw
	p.GainedFocusCallback = p.DefaultFocusGained
	p.LostFocusCallback = p.MarkForRedraw
	p.MouseDownCallback = p.DefaultMouseDown
	p.KeyDownCallback = p.DefaultKeyDown
	return p
}

// DefaultSizes provides the default sizing.
func (p *PopupMenu[T]) DefaultSizes(hint Size) (min, pref, max Size) {
	pref = LabelSize(p.textCache.Text("M", p.Font), nil, 0, 0)
	for _, one := range p.items {
		if !one.separator {
			size := LabelSize(one.textCache.Text(fmt.Sprintf("%v", one.item), p.Font), nil, 0, 0)
			if pref.Width < size.Width {
				pref.Width = size.Width
			}
			if pref.Height < size.Height {
				pref.Height = size.Height
			}
		}
	}
	if border := p.Border(); border != nil {
		pref.AddInsets(border.Insets())
	}
	pref.Height += p.VMargin*2 + 2
	pref.Width += p.HMargin*2 + 2 + pref.Height*0.75
	pref.GrowToInteger()
	pref.ConstrainForHint(hint)
	max.Width = xmath.Max(DefaultMaxSize, pref.Width)
	max.Height = pref.Height
	return pref, pref, max
}

// DefaultFocusGained provides the default focus gained handling.
func (p *PopupMenu[T]) DefaultFocusGained() {
	p.ScrollIntoView()
	p.MarkForRedraw()
}

// DefaultDraw provides the default drawing.
func (p *PopupMenu[T]) DefaultDraw(canvas *Canvas, dirty Rect) {
	thickness := float32(1)
	if p.Focused() {
		thickness++
	}
	rect := p.ContentRect(false)
	DrawRoundedRectBase(canvas, rect, p.CornerRadius, thickness, p.BackgroundInk, p.EdgeInk)
	rect.InsetUniform(1.5)
	rect.X += p.HMargin
	rect.Y += p.VMargin
	rect.Width -= p.HMargin * 2
	rect.Height -= p.VMargin * 2
	triWidth := rect.Height * 0.75
	triHeight := triWidth / 2
	rect.Width -= triWidth
	DrawLabel(canvas, rect, StartAlignment, MiddleAlignment, p.textObj(), p.OnBackgroundInk, nil, 0, 0, !p.Enabled())
	rect.Width += triWidth + p.HMargin/2
	path := NewPath()
	path.MoveTo(rect.Right(), rect.Y+(rect.Height-triHeight)/2)
	path.LineTo(rect.Right()-triWidth, rect.Y+(rect.Height-triHeight)/2)
	path.LineTo(rect.Right()-triWidth/2, rect.Y+(rect.Height-triHeight)/2+triHeight)
	path.Close()
	paint := p.OnBackgroundInk.Paint(canvas, rect, Fill)
	if !p.Enabled() {
		paint.SetColorFilter(Grayscale30Filter())
	}
	canvas.DrawPath(path, paint)
}

func (p *PopupMenu[T]) textObj() *Text {
	if p.selectedIndex >= 0 && p.selectedIndex < len(p.items) {
		if one := p.items[p.selectedIndex]; !one.separator {
			return one.textCache.Text(fmt.Sprintf("%v", one.item), p.Font)
		}
	}
	return nil
}

// Text the currently shown text.
func (p *PopupMenu[T]) Text() string {
	if p.selectedIndex >= 0 && p.selectedIndex < len(p.items) {
		if one := p.items[p.selectedIndex]; !one.separator {
			return fmt.Sprintf("%v", one.item)
		}
	}
	return ""
}

// Click performs any animation associated with a click and triggers the popup menu to appear.
func (p *PopupMenu[T]) Click() {
	hasItem := false //nolint:ifshort // Cannot collapse this into the if statement, despite what the linter says
	m := p.MenuFactory.NewMenu(PopupMenuTemporaryBaseID, "", nil)
	defer m.Dispose()
	for i, one := range p.items {
		if one.separator {
			m.InsertSeparator(-1, false)
		} else {
			hasItem = true
			m.InsertItem(-1, p.createMenuItem(m, i, one))
		}
	}
	if hasItem {
		m.Popup(p.RectToRoot(p.ContentRect(true)), p.selectedIndex)
	}
}

func (p *PopupMenu[T]) createMenuItem(m Menu, index int, entry *popupMenuItem[T]) MenuItem {
	return m.Factory().NewItem(PopupMenuTemporaryBaseID+index+1,
		fmt.Sprintf("%v", entry.item), KeyBinding{}, func(mi MenuItem) bool {
			return entry.enabled
		}, func(mi MenuItem) {
			if index != p.SelectedIndex() {
				p.SelectIndex(index)
				mi.SetCheckState(OnCheckState)
			}
		})
}

// AddItem appends a menu item to the end of the PopupMenu.
func (p *PopupMenu[T]) AddItem(item T) {
	p.items = append(p.items, &popupMenuItem[T]{
		item:    item,
		enabled: true,
	})
}

// AddDisabledItem appends a disabled menu item to the end of the PopupMenu.
func (p *PopupMenu[T]) AddDisabledItem(item T) {
	p.items = append(p.items, &popupMenuItem[T]{item: item})
}

// AddSeparator adds a separator to the end of the PopupMenu.
func (p *PopupMenu[T]) AddSeparator() {
	p.items = append(p.items, &popupMenuItem[T]{separator: true})
}

// IndexOfItem returns the index of the specified menu item. -1 will be returned if the menu item isn't present.
func (p *PopupMenu[T]) IndexOfItem(item T) int {
	for i, one := range p.items {
		if !one.separator && one.item == item {
			return i
		}
	}
	return -1
}

// RemoveAllItems removes all items from the PopupMenu.
func (p *PopupMenu[T]) RemoveAllItems() {
	p.selectedIndex = 0
	p.items = nil
	p.MarkForRedraw()
}

// RemoveItem from the PopupMenu.
func (p *PopupMenu[T]) RemoveItem(item T) {
	p.RemoveItemAt(p.IndexOfItem(item))
}

// RemoveItemAt the specified index from the PopupMenu.
func (p *PopupMenu[T]) RemoveItemAt(index int) {
	if index >= 0 {
		length := len(p.items)
		if index < length {
			if p.selectedIndex == index {
				if p.selectedIndex > length-2 {
					p.selectedIndex = length - 2
					if p.selectedIndex < 0 {
						p.selectedIndex = 0
					}
				}
				p.MarkForRedraw()
			} else if p.selectedIndex > index {
				p.selectedIndex--
			}
			copy(p.items[index:], p.items[index+1:])
			length--
			p.items[length] = nil
			p.items = p.items[:length]
		}
	}
}

// ItemCount returns the number of items in this PopupMenu.
func (p *PopupMenu[T]) ItemCount() int {
	return len(p.items)
}

// ItemAt returns the item at the specified index. 'ok' will be false if the index is out of range or the specified
// index contains a separator.
func (p *PopupMenu[T]) ItemAt(index int) (item T, ok bool) {
	if index >= 0 && index < len(p.items) {
		one := p.items[index]
		if !one.separator {
			return one.item, true
		}
	}
	return item, false
}

// SetItemAt sets the item at the specified index.
func (p *PopupMenu[T]) SetItemAt(index int, item T, enabled bool) {
	if index >= 0 && index < len(p.items) {
		one := p.items[index]
		if one.separator || one.item != item {
			one.item = item
			one.enabled = enabled
			one.separator = false
			p.MarkForRedraw()
		}
	}
}

// Selected returns the currently selected item. 'ok' will be false if there is no selection.
func (p *PopupMenu[T]) Selected() (item T, ok bool) {
	return p.ItemAt(p.selectedIndex)
}

// SelectedIndex returns the currently selected item index.
func (p *PopupMenu[T]) SelectedIndex() int {
	return p.selectedIndex
}

// Select an item.
func (p *PopupMenu[T]) Select(item T) {
	p.SelectIndex(p.IndexOfItem(item))
}

// SelectIndex selects an item by its index.
func (p *PopupMenu[T]) SelectIndex(index int) {
	if index != p.selectedIndex && index >= 0 && index < len(p.items) && !p.items[index].separator {
		p.selectedIndex = index
		p.MarkForRedraw()
		if p.SelectionCallback != nil {
			p.SelectionCallback(index, p.items[index].item)
		}
	}
}

// DefaultMouseDown provides the default mouse down handling.
func (p *PopupMenu[T]) DefaultMouseDown(where Point, button, clickCount int, mod Modifiers) bool {
	p.Click()
	return true
}

// DefaultKeyDown provides the default key down handling.
func (p *PopupMenu[T]) DefaultKeyDown(keyCode KeyCode, mod Modifiers, repeat bool) bool {
	if IsControlAction(keyCode, mod) {
		p.Click()
		return true
	}
	return false
}
