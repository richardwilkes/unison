// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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
	"slices"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/slant"
)

// DefaultPopupMenuTheme holds the default PopupMenuTheme values for PopupMenus. Modifying this data will not alter
// existing PopupMenus, but will alter any PopupMenus created in the future.
var DefaultPopupMenuTheme = PopupMenuTheme{
	TextDecoration: TextDecoration{
		Font:            SystemFont,
		BackgroundInk:   ThemeAboveSurface,
		OnBackgroundInk: ThemeOnAboveSurface,
	},
	EdgeInk:        ThemeSurfaceEdge,
	SelectionInk:   ThemeFocus,
	OnSelectionInk: ThemeOnFocus,
	CornerRadius:   4,
	HMargin:        8,
	VMargin:        1,
}

// PopupMenuTheme holds theming data for a PopupMenu.
type PopupMenuTheme struct {
	EdgeInk        Ink
	SelectionInk   Ink
	OnSelectionInk Ink
	TextDecoration
	CornerRadius float32
	HMargin      float32
	VMargin      float32
}

type popupMenuItem[T comparable] struct {
	item      T
	enabled   bool
	separator bool
}

// PopupMenu represents a clickable button that displays a menu of choices.
type PopupMenu[T comparable] struct {
	MenuFactory              MenuFactory
	WillShowMenuCallback     func(popup *PopupMenu[T])
	ChoiceMadeCallback       func(popup *PopupMenu[T], index int, item T)
	SelectionChangedCallback func(popup *PopupMenu[T])
	items                    []*popupMenuItem[T]
	selection                map[int]bool
	PopupMenuTheme
	Panel
	pressed bool
}

// NewPopupMenu creates a new PopupMenu.
func NewPopupMenu[T comparable]() *PopupMenu[T] {
	p := &PopupMenu[T]{
		PopupMenuTheme: DefaultPopupMenuTheme,
		selection:      make(map[int]bool),
	}
	p.Self = p
	p.SetFocusable(true)
	p.SetSizer(p.DefaultSizes)
	p.MenuFactory = DefaultMenuFactory()
	p.DrawCallback = p.DefaultDraw
	p.GainedFocusCallback = p.DefaultFocusGained
	p.LostFocusCallback = p.MarkForRedraw
	p.MouseDownCallback = p.DefaultMouseDown
	p.MouseDragCallback = p.DefaultMouseDrag
	p.MouseUpCallback = p.DefaultMouseUp
	p.KeyDownCallback = p.DefaultKeyDown
	p.UpdateCursorCallback = p.DefaultUpdateCursor
	p.ChoiceMadeCallback = func(popup *PopupMenu[T], index int, _ T) { popup.SelectIndex(index) }
	return p
}

// DefaultSizes provides the default sizing.
func (p *PopupMenu[T]) DefaultSizes(hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	prefSize, _ = LabelContentSizes(nil, nil, p.Font, 0, 0)
	for _, one := range p.items {
		if one.separator {
			continue
		}
		size, _ := LabelContentSizes(NewText(fmt.Sprintf("%v", one.item), &TextDecoration{
			Font:            p.Font,
			OnBackgroundInk: p.OnBackgroundInk,
		}), nil, p.Font, 0, 0)
		if prefSize.Width < size.Width {
			prefSize.Width = size.Width
		}
		if prefSize.Height < size.Height {
			prefSize.Height = size.Height
		}
	}
	if border := p.Border(); border != nil {
		prefSize = prefSize.Add(border.Insets().Size())
	}
	prefSize.Height += p.VMargin*2 + 2
	prefSize.Width += p.HMargin*2 + 2 + prefSize.Height*0.75
	prefSize = prefSize.Ceil().ConstrainForHint(hint)
	maxSize.Width = max(DefaultMaxSize, prefSize.Width)
	maxSize.Height = prefSize.Height
	return prefSize, prefSize, maxSize
}

// DefaultFocusGained provides the default focus gained handling.
func (p *PopupMenu[T]) DefaultFocusGained() {
	p.ScrollIntoView()
	p.MarkForRedraw()
}

// DefaultDraw provides the default drawing.
func (p *PopupMenu[T]) DefaultDraw(canvas *Canvas, _ geom.Rect) {
	thickness := float32(1)
	edge := p.EdgeInk
	if p.Focused() || p.pressed {
		thickness++
		edge = p.SelectionInk
	}
	rect := p.ContentRect(false)
	DrawRoundedRectBase(canvas, rect, p.CornerRadius, thickness, p.BackgroundInk, edge)
	rect = rect.Inset(geom.NewUniformInsets(1.5))
	rect.X += p.HMargin
	rect.Y += p.VMargin
	rect.Width -= p.HMargin * 2
	rect.Height -= p.VMargin * 2
	triWidth := rect.Height * 0.75
	triHeight := triWidth / 2
	rect.Width -= triWidth
	DrawLabel(canvas, rect, align.Start, align.Middle, p.Font, p.textObj(), p.OnBackgroundInk, nil, nil, 0, 0,
		!p.Enabled())
	rect.Width += triWidth + p.HMargin/2
	path := NewPath()
	path.MoveTo(rect.Right(), rect.Y+(rect.Height-triHeight)/2)
	path.LineTo(rect.Right()-triWidth, rect.Y+(rect.Height-triHeight)/2)
	path.LineTo(rect.Right()-triWidth/2, rect.Y+(rect.Height-triHeight)/2+triHeight)
	path.Close()
	paint := p.OnBackgroundInk.Paint(canvas, rect, paintstyle.Fill)
	if !p.Enabled() {
		paint.SetColorFilter(Grayscale30Filter())
	}
	canvas.DrawPath(path, paint)
}

func (p *PopupMenu[T]) textObj() *Text {
	indexes := p.SelectedIndexes()
	switch len(indexes) {
	case 0:
		return nil
	case 1:
		one := p.items[indexes[0]]
		return NewText(fmt.Sprintf("%v", one.item), &TextDecoration{
			Font:            p.Font,
			OnBackgroundInk: p.OnBackgroundInk,
		})
	default:
		desc := p.Font.Descriptor()
		desc.Slant = slant.Italic
		return NewText(i18n.Text("Multiple"), &TextDecoration{
			Font:            desc.Font(),
			OnBackgroundInk: p.OnBackgroundInk,
		})
	}
}

// Text the currently shown text.
func (p *PopupMenu[T]) Text() string {
	indexes := p.SelectedIndexes()
	switch len(indexes) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("%v", p.items[indexes[0]].item)
	default:
		return i18n.Text("Multiple")
	}
}

// Click performs any animation associated with a click and triggers the popup menu to appear.
func (p *PopupMenu[T]) Click() {
	if p.WillShowMenuCallback != nil {
		xos.SafeCall(func() { p.WillShowMenuCallback(p) }, nil)
	}
	hasItem := false
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
		index := 0
		if indexes := p.SelectedIndexes(); len(indexes) > 0 {
			index = indexes[0]
		}
		m.Popup(p.RectToRoot(p.ContentRect(true)), index)
	}
}

func (p *PopupMenu[T]) createMenuItem(m Menu, index int, entry *popupMenuItem[T]) MenuItem {
	item := m.Factory().NewItem(PopupMenuTemporaryBaseID+index+1,
		fmt.Sprintf("%v", entry.item), KeyBinding{}, func(_ MenuItem) bool {
			return entry.enabled
		}, func(_ MenuItem) {
			if p.ChoiceMadeCallback != nil {
				p.ChoiceMadeCallback(p, index, p.items[index].item)
			}
		})
	if p.selection[index] {
		item.SetCheckState(check.On)
	}
	return item
}

// AddItem appends one or more menu items to the end of the PopupMenu.
func (p *PopupMenu[T]) AddItem(item ...T) {
	for _, one := range item {
		p.items = append(p.items, &popupMenuItem[T]{
			item:    one,
			enabled: true,
		})
	}
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
	p.selection = make(map[int]bool)
	p.items = nil
	p.MarkForRedraw()
}

// RemoveItem from the PopupMenu.
func (p *PopupMenu[T]) RemoveItem(item T) {
	index := p.IndexOfItem(item)
	indexes := p.SelectedIndexes()
	p.RemoveItemAt(index)
	for _, one := range indexes {
		if one >= index {
			delete(p.selection, one)
		}
	}
	for _, one := range indexes {
		if one > index {
			p.selection[one-1] = true
		}
	}
}

// RemoveItemAt the specified index from the PopupMenu.
func (p *PopupMenu[T]) RemoveItemAt(index int) {
	if index >= 0 {
		length := len(p.items)
		if index < length {
			indexes := p.SelectedIndexes()
			p.items = slices.Delete(p.items, index, index+1)
			for _, one := range indexes {
				if one >= index {
					delete(p.selection, one)
				}
			}
			for _, one := range indexes {
				if one > index {
					p.selection[one-1] = true
				}
			}
			p.MarkForRedraw()
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

// ItemEnabledAt returns whether the item at the specified index is enabled or not.
func (p *PopupMenu[T]) ItemEnabledAt(index int) bool {
	if index >= 0 && index < len(p.items) {
		return p.items[index].enabled
	}
	return false
}

// SetItemEnabledAt sets the enabled state of the item at the specified index.
func (p *PopupMenu[T]) SetItemEnabledAt(index int, enabled bool) {
	if index >= 0 && index < len(p.items) {
		one := p.items[index]
		if !one.separator {
			one.enabled = enabled
			p.MarkForRedraw()
		}
	}
}

// Selected returns the currently selected item. 'ok' will be false if there is no selection. The first selected item
// will be returned if there are multiple.
func (p *PopupMenu[T]) Selected() (item T, ok bool) {
	return p.ItemAt(p.SelectedIndex())
}

// SelectedIndex returns the currently selected item index. -1 will be returned if no selection is present. The first
// selected item will be returned if there are multiple.
func (p *PopupMenu[T]) SelectedIndex() int {
	if indexes := p.SelectedIndexes(); len(indexes) != 0 {
		return indexes[0]
	}
	return -1
}

// SelectedIndexes returns the currently selected item indexes.
func (p *PopupMenu[T]) SelectedIndexes() []int {
	var indexes []int
	for sel := range p.selection {
		if sel >= 0 && sel < len(p.items) {
			if one := p.items[sel]; !one.separator {
				indexes = append(indexes, sel)
			}
		}
	}
	slices.Sort(indexes)
	return indexes
}

// Select one or more items, replacing any existing selection.
func (p *PopupMenu[T]) Select(item ...T) {
	var indexes []int
	for _, one := range item {
		if index := p.IndexOfItem(one); index != -1 {
			indexes = append(indexes, index)
		}
	}
	p.SelectIndex(indexes...)
}

// SelectIndex selects one or more items by their indexes, replacing any existing selection.
func (p *PopupMenu[T]) SelectIndex(index ...int) {
	indexes := p.SelectedIndexes()
	p.selection = make(map[int]bool)
	for _, one := range index {
		if one >= 0 && one < len(p.items) && !p.items[one].separator {
			p.selection[one] = true
		}
	}
	newIndexes := p.SelectedIndexes()
	if !slices.Equal(indexes, newIndexes) {
		p.MarkForRedraw()
		if p.SelectionChangedCallback != nil {
			p.SelectionChangedCallback(p)
		}
	}
}

// DefaultMouseDown provides the default mouse down handling.
func (p *PopupMenu[T]) DefaultMouseDown(_ geom.Point, _, _ int, _ Modifiers) bool {
	p.pressed = true
	p.MarkForRedraw()
	return true
}

// DefaultMouseDrag is the default implementation of the MouseDragCallback.
func (p *PopupMenu[T]) DefaultMouseDrag(where geom.Point, _ int, _ Modifiers) bool {
	if p.pressed != where.In(p.ContentRect(true)) {
		p.pressed = !p.pressed
		p.MarkForRedraw()
	}
	return true
}

// DefaultMouseUp is the default implementation of the MouseUpCallback.
func (p *PopupMenu[T]) DefaultMouseUp(where geom.Point, _ int, _ Modifiers) bool {
	if where.In(p.ContentRect(true)) {
		p.Click()
	}
	p.pressed = false
	p.MarkForRedraw()
	return true
}

// DefaultKeyDown provides the default key down handling.
func (p *PopupMenu[T]) DefaultKeyDown(keyCode KeyCode, mod Modifiers, _ bool) bool {
	if IsControlAction(keyCode, mod) {
		p.Click()
		return true
	}
	return false
}

// DefaultUpdateCursor provides the default cursor for popup menus.
func (p *PopupMenu[T]) DefaultUpdateCursor(_ geom.Point) *Cursor {
	if !p.Enabled() {
		return ArrowCursor()
	}
	return PointingCursor()
}
