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
	"slices"
	"sort"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/toolbox/v2/xstrings"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/role"
)

// DefaultTableHeaderTheme holds the default TableHeaderTheme values for TableHeaders. Modifying this data will not
// alter existing TableHeaders, but will alter any TableHeaders created in the future.
var DefaultTableHeaderTheme = TableHeaderTheme{
	BackgroundInk:        ThemeAboveSurface,
	InteriorDividerColor: ThemeAboveSurface,
}

// TableHeaderTheme holds theming data for a TableHeader.
type TableHeaderTheme struct {
	BackgroundInk        Ink
	InteriorDividerColor Ink
	HeaderBorder         Border
}

// TableHeader provides a header for a Table.
type TableHeader[T TableRowConstraint[T]] struct {
	table         *Table[T]
	ColumnHeaders []TableColumnHeader[T]
	Less          func(s1, s2 string) bool
	TableHeaderTheme
	Panel
	interactionColumn    int
	columnResizeStart    float32
	columnResizeBase     float32
	columnResizeOverhead float32
	inHeader             bool
}

// NewTableHeader creates a new TableHeader.
func NewTableHeader[T TableRowConstraint[T]](table *Table[T], columnHeaders ...TableColumnHeader[T]) *TableHeader[T] {
	h := &TableHeader[T]{
		TableHeaderTheme: DefaultTableHeaderTheme,
		table:            table,
		ColumnHeaders:    columnHeaders,
		Less:             func(s1, s2 string) bool { return xstrings.NaturalLess(s1, s2, true) },
	}
	h.Self = h
	h.SetSizer(h.DefaultSizes)
	h.SetBorder(h.HeaderBorder)
	h.DrawCallback = h.DefaultDraw
	h.UpdateCursorCallback = h.DefaultUpdateCursorCallback
	h.UpdateTooltipCallback = h.DefaultUpdateTooltipCallback
	h.MouseMoveCallback = h.DefaultMouseMove
	h.MouseDownCallback = h.DefaultMouseDown
	h.MouseDragCallback = h.DefaultMouseDrag
	h.MouseUpCallback = h.DefaultMouseUp
	h.table.header = h
	return h
}

// DefaultSizes provides the default sizing.
func (h *TableHeader[T]) DefaultSizes(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
	prefSize.Width = h.table.FrameRect().Width
	prefSize.Height = h.heightForColumns()
	if border := h.Border(); border != nil {
		insets := border.Insets()
		prefSize.Height += insets.Height()
	}
	return geom.NewSize(16, prefSize.Height), prefSize, prefSize
}

// ColumnFrame returns the frame of the given column.
func (h *TableHeader[T]) ColumnFrame(col int) geom.Rect {
	if col < 0 || col >= len(h.table.Columns) {
		return geom.Rect{}
	}
	insets := h.combinedInsets()
	x := insets.Left + h.table.leadingColumnDividerWidth()
	for c := range col {
		x += h.table.Columns[c].Current
		if h.table.ShowColumnDivider && (h.table.ShowLastColumnDivider || c < len(h.table.Columns)-1) {
			x++
		}
	}
	return geom.NewRect(x, insets.Top, h.table.Columns[col].Current, h.FrameRect().Height-insets.Height()).
		Inset(h.table.Padding)
}

func (h *TableHeader[T]) heightForColumns() float32 {
	var height float32
	for i := range h.table.Columns {
		w := h.table.Columns[i].Current
		if w <= 0 {
			continue
		}
		w -= h.table.Padding.Left + h.table.Padding.Right
		if i < len(h.ColumnHeaders) {
			_, cpref, _ := h.ColumnHeaders[i].AsPanel().Sizes(geom.NewSize(w, 0))
			cpref.Height += h.table.Padding.Top + h.table.Padding.Bottom
			if height < cpref.Height {
				height = cpref.Height
			}
		}
	}
	return max(xmath.Ceil(height), h.table.MinimumRowHeight)
}

func (h *TableHeader[T]) combinedInsets() geom.Insets {
	var insets geom.Insets
	if border := h.Border(); border != nil {
		insets = border.Insets()
	}
	if border := h.table.Border(); border != nil {
		insets2 := border.Insets()
		if insets.Left < insets2.Left {
			insets.Left = insets2.Left
		}
		if insets.Right < insets2.Right {
			insets.Right = insets2.Right
		}
	}
	return insets
}

// DefaultDraw provides the default drawing.
func (h *TableHeader[T]) DefaultDraw(canvas *Canvas, dirty geom.Rect) {
	backgroundPaint := h.BackgroundInk.Paint(canvas, dirty, paintstyle.Fill)
	canvas.DrawRect(dirty, backgroundPaint)

	var firstCol int
	insets := h.combinedInsets()
	x := insets.Left + h.table.leadingColumnDividerWidth()
	for i := range h.table.Columns {
		x1 := x + h.table.Columns[i].Current
		if h.table.ShowColumnDivider && (h.table.ShowLastColumnDivider || i < len(h.table.Columns)-1) {
			x1++
		}
		if x1 >= dirty.X {
			break
		}
		x = x1
		firstCol = i + 1
	}

	if h.table.ShowColumnDivider {
		rect := dirty
		rect.Width = 1
		if firstCol == 0 && h.table.leadingColumnDividerWidth() > 0 {
			rect.X = insets.Left
			paint := h.InteriorDividerColor.Paint(canvas, rect, paintstyle.Fill)
			canvas.DrawRect(rect, paint)
		}
		rect.X = x
		lastCol := len(h.table.Columns)
		if !h.table.ShowLastColumnDivider {
			lastCol--
		}
		for c := firstCol; c < lastCol; c++ {
			rect.X += h.table.Columns[c].Current
			paint := h.InteriorDividerColor.Paint(canvas, rect, paintstyle.Fill)
			canvas.DrawRect(rect, paint)
			rect.X++
		}
	}

	rect := dirty
	rect.X = x
	rect.Y = insets.Top
	rect.Height = h.heightForColumns()
	lastX := dirty.Right()
	for c := firstCol; c < len(h.table.Columns) && rect.X < lastX; c++ {
		rect.Width = h.table.Columns[c].Current
		cellRect := rect.Inset(h.table.Padding)
		if c < len(h.ColumnHeaders) {
			cell := h.ColumnHeaders[c].AsPanel()
			h.installCell(cell, cellRect)
			canvas.Save()
			canvas.Translate(cellRect.Point)
			cellRect.X = 0
			cellRect.Y = 0
			cell.Draw(canvas, cellRect)
			h.uninstallCell(cell)
			canvas.Restore()
		}
		rect.X += h.table.Columns[c].Current
		if h.table.ShowColumnDivider {
			rect.X++
		}
	}
}

func (h *TableHeader[T]) installCell(cell *Panel, frame geom.Rect) {
	cell.SetFrameRect(frame)
	cell.ValidateLayout()
	cell.parent = h.AsPanel()
}

// uninstallCell detaches a column header that installCell() attached, with one exception. A widget inside a custom
// column header may have taken the keyboard focus while handling whatever the header was installed for — a click, or
// an assistive technology's Focus or Press request, which axPerformInColumnHeader carries out the same way — and a
// panel with no parent cannot find its window, so Window.CurrentFocus() would answer nil, the description published
// next would report nothing focused, and the person's focus would be silently gone until they pressed Tab. A header
// holding the focus is therefore left attached, exactly as Table.uninstallCell leaves the focused cell attached.
//
// Unlike a table cell, a column header is a panel the header owns for as long as it lives rather than one built afresh
// on each call, so there is nothing to adopt and no bookkeeping to keep: leaving it attached is the whole of it, and
// the next install and uninstall, once the focus has gone elsewhere, detaches it.
//
// The window's raw focus pointer is consulted rather than Window.CurrentFocus() for the reason given in
// Table.adoptCellIfFocused: CurrentFocus() reports nil whenever the focused panel cannot find its window, which is
// exactly the state this exists to avoid creating.
func (h *TableHeader[T]) uninstallCell(cell *Panel) {
	if wnd := h.Window(); wnd != nil && wnd.focus != nil && panelContains(cell, wnd.focus) {
		return
	}
	cell.parent = nil
}

// DefaultUpdateCursorCallback provides the default cursor update handling.
func (h *TableHeader[T]) DefaultUpdateCursorCallback(where geom.Point) *Cursor {
	if !h.table.PreventUserColumnResize {
		if over := h.table.OverColumnDivider(where.X); over != -1 {
			if h.table.Columns[over].resizable() {
				return ResizeHorizontalCursor()
			}
		}
	}
	if col := h.table.OverColumn(where.X); col != -1 && col < len(h.ColumnHeaders) {
		cell := h.ColumnHeaders[col].AsPanel()
		if cell.UpdateCursorCallback != nil {
			rect := h.ColumnFrame(col)
			h.installCell(cell, rect)
			var cursor *Cursor
			SafeCall(func() { cursor = cell.UpdateCursorCallback(where.Sub(rect.Point)) })
			h.uninstallCell(cell)
			return cursor
		}
	}
	return nil
}

// DefaultUpdateTooltipCallback provides the default tooltip update handling. The tooltip of the column header the
// pointer is over is handed to the window through Panel.borrowedTooltip rather than through the header's own Tooltip:
// a column header is not a child of the header — it is installed only long enough to be drawn or handed an event —
// so what it has to say is not the header's to keep, and a description of the header built while the borrowed tooltip
// sat in Tooltip would be a description of one of its columns.
func (h *TableHeader[T]) DefaultUpdateTooltipCallback(where geom.Point, _ geom.Rect) geom.Rect {
	if col := h.table.OverColumn(where.X); col != -1 && col < len(h.ColumnHeaders) {
		cell := h.ColumnHeaders[col].AsPanel()
		if cell.UpdateTooltipCallback != nil {
			rect := h.ColumnFrame(col)
			h.installCell(cell, rect)
			var avoid geom.Rect
			SafeCall(func() { avoid = cell.UpdateTooltipCallback(where.Sub(rect.Point), h.RectToRoot(rect).Align()) })
			h.borrowedTooltip = cell.Tooltip
			h.uninstallCell(cell)
			return avoid
		}
		if cell.Tooltip != nil {
			h.borrowedTooltip = cell.Tooltip
			return h.RectToRoot(h.ColumnFrame(col)).Align()
		}
	}
	h.borrowedTooltip = nil
	return geom.Rect{}
}

// DefaultMouseMove provides the default mouse move handling.
func (h *TableHeader[T]) DefaultMouseMove(where geom.Point, mods mod.Modifiers) bool {
	stop := false
	if col := h.table.OverColumn(where.X); col != -1 && col < len(h.ColumnHeaders) {
		cell := h.ColumnHeaders[col].AsPanel()
		if cell.MouseMoveCallback != nil {
			rect := h.ColumnFrame(col)
			h.installCell(cell, rect)
			SafeCall(func() { stop = cell.MouseMoveCallback(where.Sub(rect.Point), mods) })
			h.uninstallCell(cell)
		}
	}
	return stop
}

// DefaultMouseDown provides the default mouse down handling.
func (h *TableHeader[T]) DefaultMouseDown(where geom.Point, button, clickCount int, mods mod.Modifiers) bool {
	h.interactionColumn = -1
	h.inHeader = false
	if !h.table.PreventUserColumnResize {
		if over := h.table.OverColumnDivider(where.X); over != -1 {
			if h.table.Columns[over].resizable() {
				if clickCount == 2 {
					h.table.SizeColumnToFit(over, true)
					h.MarkForRedraw()
					h.Window().UpdateCursorNow()
					return true
				}
				h.interactionColumn = over
				h.columnResizeStart = where.X
				h.columnResizeBase = h.table.Columns[over].Current
				h.columnResizeOverhead = h.table.Padding.Left + h.table.Padding.Right
				if h.table.Columns[over].ID == h.table.HierarchyColumnID {
					if hierarchyIndent := h.table.CurrentHierarchyIndent(); hierarchyIndent > 0 {
						depth := 0
						for _, cache := range h.table.rowCache {
							if depth < cache.depth {
								depth = cache.depth
							}
						}
						h.columnResizeOverhead += h.table.Padding.Left + hierarchyIndent*float32(depth+1)
					}
				}
				return true
			}
		}
	}
	stop := true
	if col := h.table.OverColumn(where.X); col != -1 {
		h.interactionColumn = col
		h.inHeader = true
		if col < len(h.ColumnHeaders) {
			cell := h.ColumnHeaders[col].AsPanel()
			if cell.MouseDownCallback != nil {
				rect := h.ColumnFrame(col)
				h.installCell(cell, rect)
				SafeCall(func() { stop = cell.MouseDownCallback(where.Sub(rect.Point), button, clickCount, mods) })
				h.uninstallCell(cell)
			}
		}
	}
	return stop
}

// DefaultMouseDrag provides the default mouse drag handling.
func (h *TableHeader[T]) DefaultMouseDrag(where geom.Point, _ int, _ mod.Modifiers) bool {
	if !h.table.PreventUserColumnResize && !h.inHeader && h.interactionColumn != -1 {
		width := h.columnResizeBase + where.X - h.columnResizeStart
		if width < h.columnResizeOverhead {
			width = h.columnResizeOverhead
		}
		minimum := h.table.Columns[h.interactionColumn].Minimum
		if minimum > 0 && width < minimum+h.columnResizeOverhead {
			width = minimum + h.columnResizeOverhead
		} else {
			maximum := h.table.Columns[h.interactionColumn].Maximum
			if maximum > 0 && width > maximum+h.columnResizeOverhead {
				width = maximum + h.columnResizeOverhead
			}
		}
		if h.table.Columns[h.interactionColumn].Current != width {
			h.table.Columns[h.interactionColumn].Current = width
			h.table.SyncToModel()
			h.MarkForRedraw()
		}
		return true
	}
	return false
}

// DefaultMouseUp provides the default mouse up handling.
func (h *TableHeader[T]) DefaultMouseUp(where geom.Point, button int, mods mod.Modifiers) bool {
	stop := false
	if h.inHeader && h.interactionColumn != -1 && h.interactionColumn < len(h.ColumnHeaders) {
		cell := h.ColumnHeaders[h.interactionColumn].AsPanel()
		if cell.MouseUpCallback != nil {
			rect := h.ColumnFrame(h.interactionColumn)
			h.installCell(cell, rect)
			SafeCall(func() { stop = cell.MouseUpCallback(where.Sub(rect.Point), button, mods) })
			h.uninstallCell(cell)
		}
	}
	return stop
}

// SortOn adjusts the sort such that the specified header is the primary sort column. If the header was already the
// primary sort column, then its ascending/descending flag will be flipped instead.
func (h *TableHeader[T]) SortOn(header TableColumnHeader[T]) {
	if header.SortState().Sortable {
		headers := make([]TableColumnHeader[T], len(h.ColumnHeaders))
		copy(headers, h.ColumnHeaders)
		sort.Slice(headers, func(i, j int) bool {
			if headers[i] == header {
				return true
			}
			if headers[j] == header {
				return false
			}
			s1 := headers[i].SortState()
			if !s1.Sortable || s1.Order < 0 {
				return false
			}
			s2 := headers[j].SortState()
			if !s2.Sortable || s2.Order < 0 {
				return true
			}
			return s1.Order < s2.Order
		})
		for i, hdr := range headers {
			s := hdr.SortState()
			if s.Sortable {
				if i == 0 {
					if s.Order == 0 {
						s.Ascending = !s.Ascending
					} else {
						s.Order = 0
					}
				} else if s.Order >= 0 {
					s.Order = i
				}
			} else {
				s.Order = -1
			}
			hdr.SetSortState(s)
		}
	}
}

type headerWithIndex[T TableRowConstraint[T]] struct {
	header TableColumnHeader[T]
	index  int
}

// HasSort returns true if at least one column is marked for sorting.
func (h *TableHeader[T]) HasSort() bool {
	for _, hdr := range h.ColumnHeaders {
		if ss := hdr.SortState(); ss.Sortable && ss.Order >= 0 {
			return true
		}
	}
	return false
}

// ApplySort sorts the table according to the current sort criteria.
func (h *TableHeader[T]) ApplySort() {
	headers := make([]*headerWithIndex[T], len(h.ColumnHeaders))
	for i, hdr := range h.ColumnHeaders {
		headers[i] = &headerWithIndex[T]{
			index:  i,
			header: hdr,
		}
	}
	sort.Slice(headers, func(i, j int) bool {
		s1 := headers[i].header.SortState()
		if !s1.Sortable || s1.Order < 0 {
			return false
		}
		s2 := headers[j].header.SortState()
		if !s2.Sortable || s2.Order < 0 {
			return true
		}
		return s1.Order < s2.Order
	})
	for i, hdr := range headers {
		s := hdr.header.SortState()
		if !s.Sortable || s.Order < 0 {
			headers = headers[:i]
			break
		}
	}
	switch {
	case h.table.hierarchicalFilter:
		// The rows a hierarchical filter shows are the table's own lists, so they are sorted in place, leaving the
		// model's order alone.
		h.applySort(headers, h.table.filterRoots)
	case h.table.filteredRows == nil:
		roots := slices.Clone(h.table.RootRows())
		h.applySort(headers, roots)
		h.table.Model.SetRootRows(roots) // Avoid resetting the selection by directly updating the model
	default:
		h.applySort(headers, h.table.filteredRows)
	}
	h.table.SyncToModel()
}

func (h *TableHeader[T]) applySort(headers []*headerWithIndex[T], rows []T) {
	if len(headers) > 0 && len(rows) > 0 {
		sort.Slice(rows, func(i, j int) bool {
			for _, hdr := range headers {
				d1 := rows[i].CellDataForSort(hdr.index)
				d2 := rows[j].CellDataForSort(hdr.index)
				if d1 != d2 {
					ascending := hdr.header.SortState().Ascending
					less := hdr.header.Less()
					if less == nil {
						less = h.Less
					}
					if less(d1, d2) {
						return ascending
					}
					return !ascending
				}
			}
			return false
		})
		switch {
		case h.table.hierarchicalFilter:
			for _, row := range rows {
				if children := h.table.filterChildren[row.ID()]; len(children) > 1 {
					h.applySort(headers, children)
				}
			}
		case h.table.filteredRows == nil:
			for _, row := range rows {
				if row.CanHaveChildren() {
					if children := row.Children(); len(children) > 1 {
						children = slices.Clone(children)
						h.applySort(headers, children)
						row.SetChildren(children)
					}
				}
			}
		}
	}
}

// ProvideAccessibility describes the header to assistive technologies. The column headers are described directly, as
// virtual children keyed by their column index: they are panels, but the header does not hold them as children — it
// installs one just long enough to draw it or to forward an event to it and then detaches it again — so nothing would
// otherwise place them in the hierarchy or know where they sit.
//
// A column header that is nothing but a label has nothing within it to describe: what it has to say is its text, which
// is the name of the column. Anything else — a header built around a button, or one holding a filter control beside its
// title — is described with its content beneath it, since an assistive technology has to be able to reach whatever is
// in there and act on it. Such a header takes its name from that content when it has none of its own.
func (h *TableHeader[T]) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	if node.Role == role.Auto {
		node.Role = role.TableHeader
	}
	node.ColumnCount = len(h.table.Columns)
	// Pressing the header is not activating it; the default behavior would synthesize a click at the center of the
	// header, which DefaultMouseDown and DefaultMouseUp would forward to whichever column header happens to sit there,
	// re-sorting the table on an arbitrary column — or, if that point falls within ColumnResizeSlop of a divider,
	// starting a column resize.
	node.Actions = node.Actions.Without(accessibility.Press)
	for col, header := range h.ColumnHeaders {
		if col >= len(h.table.Columns) {
			// A header may have fewer column headers than the table has columns, exactly as drawing tolerates.
			break
		}
		panel := header.AsPanel()
		// A column header built around a label — the library's own is, and so is a custom one written the way the
		// documentation describes, by embedding a *Label and pointing Self at itself — is named by that label's text.
		// Asking a label for its text is not the same as asking a panel for it: every panel answers String() with the
		// name of its own type, so a header built around anything else with a title of its own would be announced as
		// the name of the Go type it was written as. Anything that is not a label is read from the labels it is built
		// out of, and then, failing that, from whatever its content turns out to be called.
		label := axLabelOf(panel)
		name := panel.Accessibility.Name
		if name == "" {
			if label != nil {
				name = label.String()
			} else {
				name = axLabelText(panel)
			}
		}
		description := axTooltipText(panel)
		if description == name {
			description = ""
		}
		state := header.SortState()
		frame := h.ColumnFrame(col)
		colID := b.AddVirtualChild(col, func(n *accessibility.Node) {
			n.Role = role.ColumnHeader
			n.Name = name
			n.Description = description
			n.Bounds = frame
			n.ColumnIndex = col
			n.Sort = axSortDirection(state)
			n.Actions = n.Actions.With(accessibility.ScrollIntoView)
			if state.Sortable {
				n.Actions = n.Actions.With(accessibility.Press)
			}
		})
		if colID == 0 {
			continue
		}
		if !axColumnHeaderContentIsReachable(panel, label) {
			h.axDescribeColumnHeaderText(b, colID, panel, label, frame)
			continue
		}
		h.installCell(panel, frame)
		b.addColumnHeaderPanel(colID, col, panel)
		h.uninstallCell(panel)
		if name == "" {
			if content := b.snapshot.tree.UnignoredChildren(colID); len(content) == 1 {
				// The header holds one thing, and whatever that is called is what the column is called. A header
				// holding more than one has nothing to single out, so it stays unnamed rather than being named after
				// an arbitrary piece of itself.
				if only := b.snapshot.tree.Node(content[0]); only != nil {
					if colNode := b.snapshot.tree.Node(colID); colNode != nil {
						colNode.Name = only.Name
					}
				}
			}
		}
	}
}

// axDescribeColumnHeaderText gives the node standing for a column that is nothing but a title the text of that title,
// along with the one line it was drawn on. A column header is static text like any other, so a screen reader reads it
// by line, by word and by character and puts its review cursor where each character actually is, and it can only do
// that for text it has been given. The header holds no node of its own here — a header built around a label is one
// element, which is why nothing beneath it was described — so the text goes on the column's node.
//
// The panel is installed at the column's frame for the measurement, exactly as it is for drawing, since where the text
// sits is worked out from the size the panel has been given. What comes back is in the panel's own coordinates, which
// are the column node's own coordinates too — the node stands for that same frame — so the line needs no rebasing.
//
// The line comes from the panel's Self rather than from the label, so that a header drawing its title somewhere else
// answers for itself: the library's own shrinks the title by the sort indicator. Everything that arrives here is built
// around a label — that is what it takes for axColumnHeaderContentIsReachable to have answered false — and the only
// thing that answers axLabel is a *Label, which carries axTextLine too, so Self satisfies axTextLiner through that
// embedded label. The guard therefore stands only for a *Label-embedding type that shadowed axTextLine with an
// incompatible signature: such a header is left without text rather than being measured as though its label were the
// whole of it, and a blind type assertion would panic on it.
//
// The text itself comes from that same call rather than from the label, exactly as Label.ProvideAccessibility takes it:
// the line boundaries, the advances and the styled runs all index the runes that came back, so publishing the label's
// own string beside them would leave every offset an adapter derives addressing a different set of characters. It is
// also what decides whether there is any text to publish, so a header drawing words of its own is described by them
// even when the label it embeds holds nothing.
func (h *TableHeader[T]) axDescribeColumnHeaderText(b *AccessibilityBuilder, colID accessibility.NodeID, panel *Panel,
	label *Label, frame geom.Rect,
) {
	if label == nil {
		return
	}
	liner, ok := panel.Self.(axTextLiner)
	if !ok {
		return
	}
	colNode := b.snapshot.tree.Node(colID)
	if colNode == nil {
		return
	}
	h.installCell(panel, frame)
	runes, decorations, line := liner.axTextLine()
	h.uninstallCell(panel)
	if len(runes) == 0 {
		return
	}
	colNode.Text = axStaticTextInfo(string(runes), decorations, line)
}

// axColumnHeaderContentIsReachable reports whether describing a column header's own panel beneath the node for the
// column would reach anything. Only content that will actually be visited is worth describing: the panel is added to
// the snapshot solely so that what is inside a header holding more than a title — a filter control beside it, say — can
// be got at, and a panel that adds nothing beyond that leaves a second node carrying the column's own name under the
// column header, which is the title announced twice.
//
// A header that is not built around a label is described, whatever it holds, since what it is described as and what it
// hands on to its children are its own business. One that is built around a label — the library's own is, and so is a
// custom one written the documented way, by embedding a *Label and pointing Self at itself — is described only when it
// has children that will be visited. A label with nothing beneath it has nothing to reach; a label described as static
// text has nothing reachable either, however much it holds, since static text is one element however many panels it is
// built from and the snapshot returns without visiting what is beneath it.
func axColumnHeaderContentIsReachable(panel *Panel, label *Label) bool {
	if label == nil {
		return true
	}
	if len(panel.Children()) == 0 {
		return false
	}
	// What the label resolves to, as axDescribeStaticContent resolves it: an explicitly set role is left alone — which
	// is how a header could be made a group whose content is visited — and one that has not been set becomes an image
	// when a drawable is all the label holds and static text otherwise.
	resolved := panel.Accessibility.Role
	if resolved == role.Auto {
		if label.String() == "" && label.Drawable != nil {
			resolved = role.Image
		} else {
			resolved = role.Label
		}
	}
	return resolved != role.Label && resolved != role.Heading
}

// PerformAccessibilityAction carries out a request from an assistive technology. Pressing a column header sorts the
// table on that column, which is what clicking it does, flipping the direction when it is already the primary sort key.
// A request aimed at something within a column header is passed on to it.
func (h *TableHeader[T]) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	if key, isPanel := req.Key.(axCellPanelKey); isPanel {
		return h.axPerformInColumnHeader(key, req)
	}
	// Bounded by the columns as well as by the column headers, which is what decides the set of column headers that are
	// described: a header holding more column headers than the table has columns would otherwise be asked to sort on a
	// column that does not exist, and CellDataForSort would be handed an out-of-range index.
	col, ok := req.Key.(int)
	if !ok || col < 0 || col >= len(h.ColumnHeaders) || col >= len(h.table.Columns) {
		return false
	}
	switch req.Action {
	case accessibility.Press:
		header := h.ColumnHeaders[col]
		if !header.SortState().Sortable {
			return false
		}
		h.SortOn(header)
		h.ApplySort()
		return true
	case accessibility.ScrollIntoView:
		h.ScrollRectIntoView(h.ColumnFrame(col))
		return true
	default:
		return false
	}
}

// axPerformInColumnHeader carries out a request aimed at a panel inside one of the column headers. The header is
// installed at its column's frame, exactly as it is to hand it a mouse event, so that the panel the request is for
// exists where it was described and can reach its window; the panel is then found at the position within the header it
// was described at. A widget that takes the keyboard focus while handling the request — which Focus does outright, and
// Press does on its way to the click it synthesizes — leaves the header attached, as one that took it from a click
// would; see uninstallCell.
//
// A key naming a cell of something other than a column header is refused: the key travels with the request from
// whatever described the panel, and only the widget that put it there knows how to read it.
func (h *TableHeader[T]) axPerformInColumnHeader(key axCellPanelKey, req accessibility.ActionRequest) bool {
	cellKey, ok := key.Cell.(accessibility.CellKey)
	if !ok {
		return false
	}
	col := cellKey.Col
	if col < 0 || col >= len(h.ColumnHeaders) || col >= len(h.table.Columns) {
		return false
	}
	// The key that brought the request here named one of the header's virtual children. What it is being handed to is a
	// real panel, for which ActionRequest.Key is nil: a panel within a column header would otherwise be given a key it
	// never handed out, and one that keys virtual children of its own would take this one for one of them.
	req.Key = nil
	panel := h.ColumnHeaders[col].AsPanel()
	h.installCell(panel, h.ColumnFrame(col))
	handled := false
	if target := axPanelAtPath(panel, key.Path); target != nil {
		handled = target.axDispatchAction(req, false)
	}
	h.uninstallCell(panel)
	h.MarkForRedraw()
	return handled
}

// axSortDirection returns the direction to report for a column header's sort state. Only the primary sort column is
// reported as sorted, since that is the one whose order the rows visibly follow and the one the header draws an
// indicator for.
func axSortDirection(state SortState) accessibility.SortDirection {
	if !state.Sortable || state.Order != 0 {
		return accessibility.SortNone
	}
	if state.Ascending {
		return accessibility.SortAscending
	}
	return accessibility.SortDescending
}
