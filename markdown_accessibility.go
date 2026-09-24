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
	"strings"
	"unicode"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/textunit"
)

// This file is what makes rendered markdown readable rather than merely reachable. A screen reader reads a document as
// text — line by line, word by word, character by character, jumping from heading to heading and from link to link —
// and none of that is possible from a window full of labels: the words are there, but nothing says where one line ends
// and the next begins, which of them are a paragraph, or where the person is within them.
//
// Three things are built here. Each text-bearing block — a paragraph, a heading, a code block, a table cell — reports
// its own text, with the lines it was laid out into, the styles it was drawn in and the links and images sitting within
// it. The document composes every one of those into a single stream, joined the way the content reads, with a span
// saying which part of it each node beneath it occupies. And the markdown itself carries a reading caret through that
// stream, moved by the keyboard and the mouse, drawn where it is, and published so that a screen reader following it
// speaks what the person has just moved onto.
//
// Nothing here runs unless an assistive technology asks for it, or an application has opted in by making the markdown
// focusable: the caret keys and the mouse handling below all begin by asking, and the text is composed only when
// something asks for the description. See Panel.axTakesFocus.

// markdownRow is one visual line of a block's text: the row the pieces of a line are flowed into. It is not exposed to
// an assistive technology — the block it belongs to reports the whole of its text, a line at a time — but it has a type
// of its own so that the two ways a line can end are remembered. A line broken because the text ran out of room had the
// space that separated it from the next taken off its end, and a line ended by a break in the markdown itself ends in a
// line feed; both have to be put back for the text to read as what was written. See markdownBlock.axText.
type markdownRow struct {
	Panel
	// hardBreak reports that a break in the markdown ended this line rather than the text running out of room.
	hardBreak bool
	// strippedSpace reports that the space which separated this line from the next was sliced off the end of it when the
	// text was broken to fit.
	strippedSpace bool
}

// newMarkdownRow creates the row one visual line of a block's text is flowed into.
func newMarkdownRow() *markdownRow {
	r := &markdownRow{}
	r.Self = r
	r.SetLayout(&FlowLayout{})
	r.Accessibility.Role = role.None
	return r
}

// markdownBlock is one block of a document's content that carries text: a paragraph, a heading, a code block, or a cell
// of a table. It is the unit a screen reader reads, so it reports the whole of its text along with the lines it was
// laid out into, however many panels those lines are actually drawn with, and it holds the part of the document's caret
// and selection that falls within it.
type markdownBlock struct {
	md *Markdown
	// axCache is the text of this block, built the first time it is asked for and thrown away whenever anything within
	// the markdown moves. See markdownBlock.axText.
	axCache *axBlockText
	Panel
	// kind is what this block is: Paragraph, Heading, Code, Cell or ColumnHeader.
	kind role.Enum
	// level is a heading's depth, from 1 to 6, and zero for everything else.
	level int
	// row and col are where a cell sits within its table, and -1 for a block that is not a cell.
	row int
	col int
	// axCacheGen is the generation axCache was built for; see Markdown.axGeneration.
	axCacheGen uint64
	// lastCaret is where this block last reported the caret, which it goes on reporting once the caret has moved into
	// another block. See markdownBlock.axFillSelection.
	lastCaret int
}

// newMarkdownBlock creates a block of the given kind, which is one of Paragraph, Heading, Code, Cell or ColumnHeader.
func newMarkdownBlock(md *Markdown, kind role.Enum) *markdownBlock {
	b := &markdownBlock{
		md:   md,
		kind: kind,
		row:  -1,
		col:  -1,
	}
	b.Self = b
	b.SetLayout(&FlexLayout{Columns: 1})
	b.Accessibility.Role = kind
	return b
}

// markdownContainer is a block of a document's content that holds other blocks rather than text of its own: a list, one
// item of a list, or a quoted passage. It is what tells a screen reader that the blocks within it belong together,
// which is how "list of three items" and a quotation's title are announced.
type markdownContainer struct {
	Panel
	// kind is what this container is: List, ListItem or BlockQuote.
	kind role.Enum
}

// newMarkdownContainer creates a container of the given kind, which is one of List, ListItem or BlockQuote.
func newMarkdownContainer(kind role.Enum) *markdownContainer {
	c := &markdownContainer{kind: kind}
	c.Self = c
	c.SetLayout(&FlexLayout{Columns: 1})
	c.Accessibility.Role = kind
	return c
}

// markdownTable is the grid a table's cells are laid out in. The cells are laid out as one flow of panels in as many
// columns as the table has, which is what gives the table its look, so the rows a person moves through exist only
// because the table describes them. See markdownTable.ProvideAccessibility.
type markdownTable struct {
	Panel
	// columns is how many columns the table has.
	columns int
}

// newMarkdownTable creates the grid a table with the given number of columns is laid out in.
func newMarkdownTable(columns int) *markdownTable {
	t := &markdownTable{columns: columns}
	t.Self = t
	t.SetLayout(&FlexLayout{Columns: columns})
	t.Accessibility.Role = role.Table
	return t
}

// markdownRowKey identifies one row of a markdown table, which has no panel of its own, so that the row keeps the same
// identity from one description to the next. See AccessibilityBuilder.AddVirtualChild.
type markdownRowKey struct {
	// Row is the zero-based index of the row within its table.
	Row int
}

// axPanelSpan says that a panel occupies a range of a block's text: a link, or the single placeholder rune standing in
// for an image.
type axPanelSpan struct {
	panel *Panel
	start int
	end   int
}

// axBlockText is the text of one block: its runes, the lines they were laid out into, the styles they were drawn in and
// the panels sitting within them. Nothing in it is measured twice — the advances come from the widths each piece of
// text was drawn with and the lines from the frames those pieces were placed at — so it costs no more than reading what
// the layout has already worked out.
type axBlockText struct {
	// composed is the text as the one string an assistive technology is handed, kept beside the runes and composed the
	// first time it is asked for. See axBlockText.str.
	composed *string
	text     []rune
	lines    []accessibility.Line
	runs     []accessibility.TextRun
	spans    []axPanelSpan
}

// str returns the text as the one string an assistive technology is handed. A snapshot is built after every redraw
// while a screen reader is attached, so composing it afresh each time would be a pass over the whole of the text for
// runes that have not changed; it is composed once for the layout the text was read from and thrown away with it.
func (t *axBlockText) str() string {
	if t.composed == nil {
		composed := string(t.text)
		t.composed = &composed
	}
	return *t.composed
}

// axText returns the text of this block, building it if it has not been built for the markdown as it is now.
func (b *markdownBlock) axText() *axBlockText {
	if b.axCache == nil || b.axCacheGen != b.md.axGeneration {
		builder := &axBlockTextBuilder{block: b.AsPanel(), text: &axBlockText{}}
		builder.collect(b.AsPanel())
		builder.emit()
		builder.text.runs = axRunsFromDecorations(builder.decorations)
		b.axCache = builder.text
		b.axCacheGen = b.md.axGeneration
	}
	return b.axCache
}

// axBlockLine is one visual line of a block before its text has been read out of it: where the line was drawn, the
// pieces it is drawn with, and how it ends.
type axBlockLine struct {
	chunks []*Panel
	// bounds is where the line was drawn, in the block's own coordinates.
	bounds geom.Rect
	// own reports that the panel holding the line is the whole of it rather than a row the pieces of a line were flowed
	// into, which is what the lines of a code block are. Every one of those ends where it ends, so each but the last is
	// followed by a line feed.
	own           bool
	hardBreak     bool
	strippedSpace bool
}

// axBlockTextBuilder accumulates the text of one block as the rows that draw it are walked in order.
type axBlockTextBuilder struct {
	block *Panel
	text  *axBlockText
	lines []axBlockLine
	// decorations holds what each rune was drawn with, which is what the styled runs are worked out from once the whole
	// of the text is known.
	decorations []*TextDecoration
}

// collect finds every line beneath p, in the order they are drawn in, descending through the panels that are there only
// to hold them.
func (bb *axBlockTextBuilder) collect(p *Panel) {
	for _, child := range p.Children() {
		if child.Hidden {
			continue
		}
		switch c := child.Self.(type) {
		case *markdownRow:
			bb.row(c)
		case *Label, *DrawablePanel:
			// A block whose content is not flowed into rows at all: the lines of a code block are a label apiece.
			bb.lines = append(bb.lines, axBlockLine{
				bounds: child.RectTo(child.ContentRect(true), bb.block),
				chunks: []*Panel{child},
				own:    true,
			})
		case *markdownBlock, *markdownContainer, *markdownTable, *Separator:
			// Content of the document in its own right rather than part of this block's text. A rule across the page
			// does reach here: processRawHTML turns an <hr> written inside a paragraph into a Separator added to the
			// paragraph itself. Skipping it is right — it is not text and there is nothing to read in it — and the
			// consequence is that the block's lines jump over the vertical band the rule occupies, which is a gap
			// between two lines of one block rather than the line boundary it looks like. See axLineAt, which resolves a
			// point in such a gap to the line nearest it.
		default:
			bb.collect(child)
		}
	}
}

// row adds the lines one row of the block was drawn as. A row holds the pieces of a line in a FlowLayout, so it is one
// visual line only as long as every piece of it fitted: a piece that did not was flowed onto a row of its own beneath
// the others, and each of those rows is a line in its own right. The code that fills the rows keeps a piece from
// overflowing in the first place — see Markdown.flushText, Markdown.createLink and Markdown.processImage — and this is
// what keeps a row that overflowed anyway from being reported as one line two rows tall, whose rune boundaries walk
// back to the left partway along it and whose caret and highlight then land on the wrong words.
func (bb *axBlockTextBuilder) row(r *markdownRow) {
	row := r.AsPanel()
	var band []*Panel
	var bounds geom.Rect
	for _, chunk := range row.Children() {
		if chunk.Hidden {
			continue
		}
		frame := chunk.FrameRect()
		if len(band) != 0 && frame.Y > bounds.Y && frame.Y >= bounds.Bottom() {
			// This piece was flowed onto a row of its own below the ones before it, so those are a line and this begins
			// the next. Only the last line of the row ends the way the row does, since the line feed a hard break put
			// there and the space the text was broken at both belong to the end of the row.
			bb.lines = append(bb.lines, axBlockLine{bounds: row.RectTo(bounds, bb.block), chunks: band})
			band = nil
		}
		if len(band) == 0 {
			bounds = frame
		} else {
			bounds = bounds.Union(frame)
		}
		band = append(band, chunk)
	}
	if len(band) == 0 {
		// A row with nothing visible on it is still a line, and still as tall as one.
		bounds = row.ContentRect(true)
	}
	bb.lines = append(bb.lines, axBlockLine{
		bounds:        row.RectTo(bounds, bb.block),
		chunks:        band,
		hardBreak:     r.hardBreak,
		strippedSpace: r.strippedSpace,
	})
}

// emit turns the lines that were found into the block's text.
func (bb *axBlockTextBuilder) emit() {
	if len(bb.lines) == 0 {
		// A block with nothing in it at all — an empty cell of a table — still took up the room for a line, and is still
		// somewhere the caret can sit and something an assistive technology may ask to select. A block left out of the
		// stream would refuse the very actions it advertises, so it is given the one empty line it drew. See
		// Label.axTextLine, which reports the one place a caret can sit in an empty label the same way.
		bb.line(bb.block.ContentRect(true), nil, false, false)
		return
	}
	for i, line := range bb.lines {
		hardBreak := line.hardBreak
		if line.own && i != len(bb.lines)-1 {
			// A line that is a panel of its own was written as a line, so it ends in a line feed rather than in the space
			// a wrapped line was broken at. The last of them ends the block, which the document itself puts a line feed
			// after.
			hardBreak = true
		}
		bb.line(line.bounds, line.chunks, hardBreak, line.strippedSpace)
	}
}

// line adds one line, drawn within the given bounds and made of the given chunks in the order they are drawn in. The
// bounds are in the block's own coordinates.
func (bb *axBlockTextBuilder) line(bounds geom.Rect, chunks []*Panel, hardBreak, strippedSpace bool) {
	line := accessibility.Line{
		Start:  len(bb.text.text),
		Bounds: bounds,
	}
	for _, chunk := range chunks {
		if !chunk.Hidden {
			bb.chunk(chunk, &line)
		}
	}
	if strippedSpace {
		// The space that separated this line from the next was taken off the end of it when the text was broken to fit,
		// so it is put back. It is given no width of its own, since nothing was drawn for it: what it does is make the
		// text read, and copy, as the one run of words it was written as.
		bb.separator(&line, ' ')
	}
	if hardBreak {
		bb.separator(&line, '\n')
	}
	if len(line.Advances) == 0 {
		// A line with nothing on it is still somewhere a caret can sit.
		line.Advances = append(line.Advances, 0)
	}
	// The advances are measured from the line's own left edge, which is where its first rune begins rather than where
	// the row holding it does: a centered cell, or a bullet aligned to the end of its column, starts further in.
	if first := line.Advances[0]; first != 0 {
		line.Bounds.X += first
		for i := range line.Advances {
			line.Advances[i] -= first
		}
	}
	line.Bounds.Width = line.Advances[len(line.Advances)-1]
	line.End = len(bb.text.text)
	bb.text.lines = append(bb.text.lines, line)
}

// separator adds a rune that nothing was drawn for to the end of a line: the space a soft wrap took off it, or the line
// feed a hard break ended it with.
func (bb *axBlockTextBuilder) separator(line *accessibility.Line, r rune) {
	bb.text.text = append(bb.text.text, r)
	bb.decorations = append(bb.decorations, nil)
	if len(line.Advances) == 0 {
		line.Advances = append(line.Advances, 0)
	}
	line.Advances = append(line.Advances, line.Advances[len(line.Advances)-1])
}

// chunk adds one piece of a line: a run of text, a link, or an image.
func (bb *axBlockTextBuilder) chunk(chunk *Panel, line *accessibility.Line) {
	start := len(bb.text.text)
	if _, isImage := chunk.Self.(*DrawablePanel); isImage {
		// An image is not text, so it occupies the one rune that stands in for something that is not: a screen reader
		// walking the stream lands on it, says what the image is from the element occupying it, and moves on.
		left := chunk.PointTo(geom.Point{}, bb.block).X - line.Bounds.X
		bb.boundary(line, left)
		bb.text.text = append(bb.text.text, textunit.ObjectReplacement)
		bb.decorations = append(bb.decorations, nil)
		line.Advances = append(line.Advances, left+chunk.FrameRect().Width)
		bb.text.spans = append(bb.text.spans, axPanelSpan{panel: chunk, start: start, end: len(bb.text.text)})
		return
	}
	label, ok := chunk.Self.(*Label)
	if !ok {
		return
	}
	runes, decorations, sub := label.axTextLine()
	left := label.PointTo(sub.Bounds.Point, bb.block).X - line.Bounds.X
	bb.boundary(line, left)
	for _, advance := range sub.Advances[1:] {
		line.Advances = append(line.Advances, left+advance)
	}
	bb.text.text = append(bb.text.text, runes...)
	bb.decorations = append(bb.decorations, decorations...)
	if len(runes) != 0 && chunk.Accessibility.Role == role.Link {
		// A link is an element of its own within the text: an assistive technology offers it among the document's links,
		// says where it leads and follows it, all of which needs to know which words it is.
		bb.text.spans = append(bb.text.spans, axPanelSpan{panel: chunk, start: start, end: len(bb.text.text)})
	}
}

// boundary records where the next chunk of a line begins.
//
// The chunks are measured from where each of them was actually drawn rather than by summing the widths of the ones
// before it, since every chunk sits in a panel whose frame was rounded up to a whole unit: after a few of them the sum
// of the widths and the place the text is on the screen have drifted apart, and it is the place on the screen that a
// caret dropped there, and a highlight drawn over it, have to agree with.
func (bb *axBlockTextBuilder) boundary(line *accessibility.Line, at float32) {
	if len(line.Advances) == 0 {
		line.Advances = append(line.Advances, at)
		return
	}
	last := len(line.Advances) - 1
	line.Advances[last] = max(line.Advances[last], at)
}

// axDocSpan says that a node occupies a range of the document's stream. The node is named by the panel it was described
// from, and by the key that panel identifies it by when it is one of that panel's virtual children — a row of a table
// has no panel of its own. The ids themselves are resolved when the document is described, since a panel that has not
// been described yet still has to be pointed at.
type axDocSpan struct {
	panel *Panel
	key   any
	start int
	end   int
	// depth is how deeply nested the node is, which is what the spans are ordered by: a span always follows the spans
	// that contain it.
	depth int
}

// axDocBlock is where one block's text sits within the document's stream.
type axDocBlock struct {
	block *markdownBlock
	start int
	end   int
}

// axDocument is the content of a markdown composed into the one stream of text a screen reader reads it as: every block
// in reading order, joined by line feeds, with a line per visual line, the styles they were drawn in, and a span per
// node saying which part of the stream that node occupies.
type axDocument struct {
	// index says where in blocks each block is, so that a block asked about its own part of the stream does not have to
	// search for it.
	index map[*markdownBlock]int
	// composed is the stream as the one string an assistive technology is handed, kept beside the runes and composed
	// the first time it is asked for. See axDocument.str.
	composed *string
	text     []rune
	lines    []accessibility.Line
	runs     []accessibility.TextRun
	spans    []axDocSpan
	blocks   []axDocBlock
}

// str returns the stream as the one string an assistive technology is handed. A snapshot is built after every redraw
// while a screen reader is attached, so composing it afresh each time would be a copy of the whole document's text for
// runes that have not changed; it is composed once for the layout the stream was read from and thrown away with it.
func (d *axDocument) str() string {
	if d.composed == nil {
		composed := string(d.text)
		d.composed = &composed
	}
	return *d.composed
}

// blockRange returns where a block's text sits within the stream, and false if the block is not part of it.
func (d *axDocument) blockRange(b *markdownBlock) (start, end int, ok bool) {
	i, exists := d.index[b]
	if !exists {
		return 0, 0, false
	}
	return d.blocks[i].start, d.blocks[i].end, true
}

// blockAt returns the range of the block the given offset falls in, and false when the offset falls between blocks.
func (d *axDocument) blockAt(offset int) (start, end int, ok bool) {
	for _, block := range d.blocks {
		if offset >= block.start && offset <= block.end {
			return block.start, block.end, true
		}
	}
	return 0, 0, false
}

// linkAt returns the link occupying the given offset of the stream, or nil when nothing there is a link.
func (d *axDocument) linkAt(offset int) *Panel {
	var found *Panel
	for _, span := range d.spans {
		if span.key != nil || span.panel == nil || span.panel.Accessibility.Role != role.Link {
			continue
		}
		if offset >= span.start && offset < span.end {
			// The spans are ordered outermost first, so the last one that covers the offset is the innermost, which is
			// the link a person on those words means.
			found = span.panel
		}
	}
	return found
}

// wordAt returns the range of the word at the given offset, which is what a double-click selects. The spaces that
// follow the word are left out of it, since a person selecting a word means the word.
func (d *axDocument) wordAt(offset int) (start, end int) {
	if len(d.text) == 0 {
		return 0, 0
	}
	pos := min(max(offset, 0), len(d.text)-1)
	start = pos
	for start > 0 && !textunit.IsWordStart(d.text, start) {
		start--
	}
	end = start + 1
	for end < len(d.text) && !textunit.IsWordStart(d.text, end) {
		end++
	}
	for end > start+1 && unicode.IsSpace(d.text[end-1]) {
		end--
	}
	return start, end
}

// axDocBuilder composes the stream as the panels holding the content are walked in the order they are drawn in.
type axDocBuilder struct {
	md  *Markdown
	doc *axDocument
	// pending is the rune that joins whatever comes next to what has already been added: a line feed between two blocks,
	// or the space after a list item's bullet. It is added only once there is something for it to join to, so the stream
	// neither begins nor ends with one.
	pending rune
	// depth is how deeply nested the content being walked is, which orders the spans.
	depth int
}

// axDocument returns the markdown's content composed into one stream of text, building it if anything has moved since
// it was last built.
func (m *Markdown) axDocument() *axDocument {
	if m.axDoc == nil {
		builder := &axDocBuilder{
			md:  m,
			doc: &axDocument{index: make(map[*markdownBlock]int)},
		}
		builder.walk(m.AsPanel())
		// Outermost first and then by where each begins, which is the order an adapter needs to be able to tell which
		// node is the innermost one at an offset without sorting them itself.
		slices.SortStableFunc(builder.doc.spans, func(a, b axDocSpan) int {
			if a.depth != b.depth {
				return a.depth - b.depth
			}
			return a.start - b.start
		})
		m.axDoc = builder.doc
	}
	return m.axDoc
}

// axInvalidate throws the composed stream away, along with the text every block within it had worked out, so that the
// next thing to ask is given the content as it now is. Everything is rebuilt on demand rather than here: a layout moves
// hundreds of frames and nothing may be described in the middle of one.
func (m *Markdown) axInvalidate() {
	m.axDoc = nil
	m.axGeneration++
}

// walk adds everything beneath p to the stream, in the order it is drawn in.
func (db *axDocBuilder) walk(p *Panel) {
	listItem := false
	if container, ok := p.Self.(*markdownContainer); ok && container.kind == role.ListItem {
		listItem = true
	}
	for i, child := range p.Children() {
		if child.Hidden {
			continue
		}
		switch c := child.Self.(type) {
		case *markdownBlock:
			db.block(c)
		case *markdownContainer:
			db.container(c)
		case *markdownTable:
			db.table(c)
		case *Label:
			// A list item's bullet, or the title of an alert: text that is drawn as part of the document but is not a
			// block of its own. A bullet is followed by the content beside it rather than by a line feed, since the two
			// are one line of the document as it reads.
			db.label(c, listItem && i == 0)
		case *markdownRow:
			// A row of text that no block owns. Nothing builds one today — a row is added to whatever block is being
			// filled — but a row is one visual line rather than a line per piece of it, so it is composed the way a
			// block's rows are rather than walked into, which would make each of its labels a line of the document.
			db.row(c)
		case *DrawablePanel:
			// An image that no block's text holds. Nothing builds one today either, and walking past it would leave the
			// stream describing less than the document holds and an assistive technology reading the text no way to
			// reach the image within it.
			db.drawable(child)
		case *Separator:
			// A rule across the page is not text and contributes nothing to read.
		default:
			db.walk(child)
		}
	}
}

// contentStart returns the offset the next rune added will sit at, which is where a container's content begins.
func (db *axDocBuilder) contentStart() int {
	if db.pending != 0 {
		return len(db.doc.text) + 1
	}
	return len(db.doc.text)
}

// flushPending adds the rune that joins what comes next to what is already there, as part of the line it ends.
func (db *axDocBuilder) flushPending() {
	if db.pending == 0 {
		return
	}
	r := db.pending
	db.pending = 0
	if len(db.doc.lines) == 0 {
		// Nothing has been added yet, so there is nothing to join to.
		return
	}
	db.doc.text = append(db.doc.text, r)
	line := &db.doc.lines[len(db.doc.lines)-1]
	line.End++
	line.Advances = append(line.Advances, line.Advances[len(line.Advances)-1])
	db.doc.runs = axCoverRuns(db.doc.runs, len(db.doc.text))
}

// span records that a node occupies a range of the stream. An empty range is not recorded: a container holding nothing
// that reads as text occupies no part of the document.
func (db *axDocBuilder) span(panel *Panel, key any, start, end, depth int) {
	if panel == nil || end <= start {
		return
	}
	db.doc.spans = append(db.doc.spans, axDocSpan{
		panel: panel,
		key:   key,
		start: start,
		end:   end,
		depth: depth,
	})
}

// block adds one block's text to the stream, as its own lines rebased into the markdown's coordinates, and records
// where the block sits within it so that the block can be asked about its own part of it.
func (db *axDocBuilder) block(blk *markdownBlock) {
	start := db.addText(blk.AsPanel(), blk.AsPanel(), blk.axText())
	db.doc.index[blk] = len(db.doc.blocks)
	db.doc.blocks = append(db.doc.blocks, axDocBlock{block: blk, start: start, end: len(db.doc.text)})
}

// row adds the text of a row that no block owns, composed as one visual line per row it was flowed onto. See
// axDocBuilder.walk, and axBlockTextBuilder.row for the grouping.
func (db *axDocBuilder) row(r *markdownRow) {
	builder := &axBlockTextBuilder{block: r.AsPanel(), text: &axBlockText{}}
	builder.row(r)
	builder.emit()
	builder.text.runs = axRunsFromDecorations(builder.decorations)
	// The row itself is not an element of the document — its role is role.None, so nothing describes it — and a span
	// naming a node that is not in the tree is worse than no span at all.
	db.addText(r.AsPanel(), nil, builder.text)
}

// drawable adds an image that no block's text holds: the one rune standing in for something that is not text, with the
// span saying the image is what occupies it. See axDocBuilder.walk.
func (db *axDocBuilder) drawable(p *Panel) {
	builder := &axBlockTextBuilder{block: p, text: &axBlockText{}}
	builder.lines = append(builder.lines, axBlockLine{
		bounds: p.ContentRect(true),
		chunks: []*Panel{p},
		own:    true,
	})
	builder.emit()
	db.addText(p, nil, builder.text)
}

// addText adds text that was composed against one panel's coordinates to the stream, rebased into the markdown's, and
// returns the offset it begins at. owner is the panel the lines were measured in, and element the node that occupies
// the text, which is nil for text no element of its own is described for.
func (db *axDocBuilder) addText(owner, element *Panel, text *axBlockText) int {
	// A bullet and the content beside it are one line of the document, so the first line continues the line the bullet
	// opened rather than beginning a new one.
	merge := db.pending == ' '
	db.flushPending()
	start := len(db.doc.text)
	for i, line := range text.lines {
		bounds := owner.RectTo(line.Bounds, db.md.AsPanel())
		if i == 0 && merge {
			db.mergeLine(bounds, line.Advances, start+line.End-line.Start)
			continue
		}
		db.addLine(bounds, line.Advances, start+line.Start, start+line.End)
	}
	db.doc.text = append(db.doc.text, text.text...)
	db.doc.runs = axAppendRuns(db.doc.runs, text.runs, start)
	db.span(element, nil, start, len(db.doc.text), db.depth)
	for _, s := range text.spans {
		db.span(s.panel, nil, start+s.start, start+s.end, db.depth+1)
	}
	db.pending = '\n'
	return start
}

// label adds a piece of text that is drawn as part of the document without being a block of its own: a list item's
// bullet, or the title of an alert. inline says that what follows it continues the same line, which is what a bullet
// and the content beside it are.
func (db *axDocBuilder) label(label *Label, inline bool) {
	runes, decorations, line := label.axTextLine()
	db.flushPending()
	start := len(db.doc.text)
	db.addLine(label.RectTo(line.Bounds, db.md.AsPanel()), line.Advances, start, start+len(runes))
	db.doc.text = append(db.doc.text, runes...)
	db.doc.runs = axAppendRuns(db.doc.runs, axRunsFromDecorations(decorations), start)
	db.pending = '\n'
	if inline {
		db.pending = ' '
	}
}

// container adds everything within a list, a list item or a quoted passage, and records the part of the stream it
// occupies.
func (db *axDocBuilder) container(c *markdownContainer) {
	start := db.contentStart()
	depth := db.depth
	db.depth++
	db.walk(c.AsPanel())
	db.depth = depth
	db.span(c.AsPanel(), nil, start, len(db.doc.text), depth)
}

// table adds the cells of a table, row by row, and records the part of the stream the table and each of its rows
// occupies. A row has no panel of its own, so it is named by the table and the key the table invents for it.
func (db *axDocBuilder) table(t *markdownTable) {
	start := db.contentStart()
	depth := db.depth
	rows := t.axRows()
	for r, cells := range rows {
		if len(cells) == 0 {
			continue
		}
		rowStart := db.contentStart()
		db.depth = depth + 2
		for _, cell := range cells {
			db.block(cell.block)
		}
		db.depth = depth
		db.span(t.AsPanel(), markdownRowKey{Row: r}, rowStart, len(db.doc.text), depth+1)
	}
	db.span(t.AsPanel(), nil, start, len(db.doc.text), depth)
}

// addLine adds one line of the stream. The advances arrive measured from the line's own left edge and are kept that
// way.
func (db *axDocBuilder) addLine(bounds geom.Rect, advances []float32, start, end int) {
	line := accessibility.Line{
		Advances: make([]float32, len(advances)),
		Start:    start,
		End:      end,
		Bounds:   bounds,
	}
	copy(line.Advances, advances)
	db.doc.lines = append(db.doc.lines, line)
}

// mergeLine folds a line into the one already open, which is how a list item's bullet and the first line of its content
// read as the one line they are drawn as.
func (db *axDocBuilder) mergeLine(bounds geom.Rect, advances []float32, end int) {
	if len(advances) == 0 {
		return
	}
	if len(db.doc.lines) == 0 {
		db.addLine(bounds, advances, end-len(advances)+1, end)
		return
	}
	line := &db.doc.lines[len(db.doc.lines)-1]
	if bounds.X < line.Bounds.X {
		// Everything already measured on this line was measured from further right, so it is measured again from the
		// line's new left edge.
		shift := line.Bounds.X - bounds.X
		for i := range line.Advances {
			line.Advances[i] += shift
		}
	}
	offset := bounds.X - min(line.Bounds.X, bounds.X)
	if len(line.Advances) == 0 {
		line.Advances = append(line.Advances, offset+advances[0])
	} else {
		last := len(line.Advances) - 1
		line.Advances[last] = max(line.Advances[last], offset+advances[0])
	}
	for _, advance := range advances[1:] {
		line.Advances = append(line.Advances, offset+advance)
	}
	line.Bounds = line.Bounds.Union(bounds)
	line.End = end
}

// axLineAt returns the index of the line a point falls on, which is the whole of what resolving a point to an offset of
// the text comes down to once the line is known.
//
// The lines of a document are neither vertically ordered nor disjoint. Every cell of one row of a table contributes its
// lines to the same vertical band, as do the outer and inner bullets of a nested list, so a point is resolved by taking
// the lines whose band holds it and then the one horizontally nearest it: without that, a click anywhere in a table
// resolves to the first column and the caret can never be placed in any other. A point in a gap between two bands, or
// past the ends of the document, goes to the line nearest it.
func axLineAt(lines []accessibility.Line, pt geom.Point) int {
	best := 0
	var bestV, bestH float32
	for i, line := range lines {
		vertical := axDistanceOutside(line.Bounds.Y, line.Bounds.Bottom(), pt.Y)
		horizontal := axDistanceOutside(line.Bounds.X, line.Bounds.Right(), pt.X)
		if i == 0 || vertical < bestV || (vertical == bestV && horizontal < bestH) {
			best, bestV, bestH = i, vertical, horizontal
		}
	}
	return best
}

// axDistanceOutside returns how far a position lies outside the span from low to high, and zero when it is within it.
func axDistanceOutside(low, high, at float32) float32 {
	if at < low {
		return low - at
	}
	if at > high {
		return at - high
	}
	return 0
}

// axLineEnd returns the offset just past the last rune of a line that a caret may sit at, which is before the runes
// ending it that nothing was drawn for: the line feed a hard break put there, and the space a soft-wrapped line was
// broken at and had put back. A caret past either of those is the beginning of the next line — the space's own offset
// is one before that line's Start — so without this, End on a wrapped line, and Command and the right arrow with it,
// would leave the caret at the start of the line below and have a screen reader read that line instead of this one.
//
// A rune nothing was drawn for takes up no room on the line, which is what tells the space a line was broken at apart
// from a space that was drawn: its advance is the advance of the rune before it.
func axLineEnd(lines []accessibility.Line, text []rune, index int) int {
	if index < 0 || index >= len(lines) {
		return 0
	}
	line := lines[index]
	end := min(line.End, len(text))
	for end > line.Start {
		at := end - line.Start
		switch {
		case text[end-1] == '\n':
		case text[end-1] == ' ' && at < len(line.Advances) && line.Advances[at] == line.Advances[at-1]:
		default:
			return end
		}
		end--
	}
	return end
}

// axOffsetNearestX returns the offset on a line whose rune boundary is closest to x, which is what dropping a caret at
// a point, and carrying one from line to line, both come down to.
func axOffsetNearestX(lines []accessibility.Line, text []rune, index int, x float32) int {
	if index < 0 || index >= len(lines) {
		return 0
	}
	line := lines[index]
	best := 0
	first := true
	var bestDelta float32
	for i, advance := range line.Advances {
		delta := xmath.Abs(line.Bounds.X + advance - x)
		if first || delta < bestDelta {
			first = false
			bestDelta = delta
			best = i
		}
	}
	return min(line.Start+best, axLineEnd(lines, text, index))
}

// axCaretX returns where on the screen, in the coordinates the lines are in, the caret at an offset sits.
func axCaretX(lines []accessibility.Line, offset int) float32 {
	if len(lines) == 0 {
		return 0
	}
	line := lines[axLineIndexFor(lines, offset)]
	if len(line.Advances) == 0 {
		return line.Bounds.X
	}
	return line.Bounds.X + line.Advances[min(max(offset-line.Start, 0), len(line.Advances)-1)]
}

// axSelection returns the selected range of the stream, in ascending order, clamped to the stream as it now is.
func (m *Markdown) axSelection() (start, end int) {
	limit := len(m.axDocument().text)
	anchor := min(max(m.anchor, 0), limit)
	caret := min(max(m.caret, 0), limit)
	return min(anchor, caret), max(anchor, caret)
}

// axCaret returns where the reading caret sits within the stream, clamped to the stream as it now is: the same content
// laid out to a different width is the same document, and the caret stays where it was in it.
func (m *Markdown) axCaret() int {
	return min(max(m.caret, 0), len(m.axDocument().text))
}

// axSetSelection moves the selection, from the end it was made from to the end that moves, and brings the caret into
// view. Both offsets are into the composed stream and are clamped to it.
func (m *Markdown) axSetSelection(anchor, caret int) {
	m.axApplySelection(anchor, caret, true)
}

// axApplySelection moves the selection and, if scroll is true, brings the caret into view. Every move of the caret
// scrolls to it but selecting the whole document, which is the one selection that was not made by moving.
//
// Nothing happens when neither end moved. An assistive technology that re-asserts the selection it already has — which
// Orca's SetCaretOffset and Narrator's Select both do routinely — would otherwise cost a scroll, a full repaint and the
// description published after it, each time, for a caret that is where it already was.
func (m *Markdown) axApplySelection(anchor, caret int, scroll bool) {
	limit := len(m.axDocument().text)
	anchor = min(max(anchor, 0), limit)
	caret = min(max(caret, 0), limit)
	if m.anchor == anchor && m.caret == caret {
		return
	}
	m.anchor = anchor
	m.caret = caret
	if scroll {
		m.axScrollCaretIntoView()
	}
	// The caret and the selection are drawn, and the description published after a window is drawn is what tells a
	// screen reader the caret moved, so one call covers both.
	m.MarkForRedraw()
}

// axMoveCaret moves the caret, extending the selection from where it was made rather than replacing it when asked to.
func (m *Markdown) axMoveCaret(to int, extend bool) {
	if extend {
		m.axSetSelection(m.anchor, to)
		return
	}
	m.axSetSelection(to, to)
}

// axScrollCaretIntoView brings the line the caret is on into view, which is what makes reading with the keyboard work
// in a document taller than the view showing it.
func (m *Markdown) axScrollCaretIntoView() {
	doc := m.axDocument()
	if rect := axCaretRect(doc.lines, m.axCaret()); !rect.Empty() {
		m.ScrollRectIntoView(rect)
	}
}

// axPageHeight is how far a page up or down moves the caret: the height of the view the document is being read through,
// or of the window when it is not in one.
func (m *Markdown) axPageHeight() float32 {
	for p := m.Parent(); p != nil; p = p.Parent() {
		if scroller, ok := p.Self.(*ScrollPanel); ok {
			if height := scroller.ContentView().ContentRect(false).Height; height > 0 {
				return height
			}
		}
	}
	if wnd := m.Window(); wnd != nil {
		if height := wnd.LocalContentRect().Height; height > 0 {
			return height
		}
	}
	return m.ContentRect(false).Height
}

// axOSMenuCommandIsControl reports whether the key standing for the operating system's menu commands is Control, which
// it is everywhere but macOS. It is what tells the two conventions for moving through text apart: Control and an arrow
// key moves by word on Windows and Linux, while on macOS that is Option and an arrow key, and Command with one moves to
// the end of the line or of the document.
func axOSMenuCommandIsControl() bool {
	return mod.OSMenuCommand() == mod.Control
}

// DefaultKeyDown provides the default key down handling, which is the reading caret: the arrow keys move it by
// character, word, line and page, Home and End to the ends of a line or of the document, shift extends the selection to
// wherever it goes, Return or Space follows the link it is on, and the copy and select-all commands act on what it has
// selected.
//
// Anything else is left alone — Tab still moves the focus, Escape still closes what it closes, and the keys a scroll
// panel around the document handles still reach it — as is every key while the document does not hold the focus, which
// is whenever no assistive technology is being served and no application has asked for a document its text can be read
// from.
func (m *Markdown) DefaultKeyDown(keyCode KeyCode, mods mod.Modifiers, _ bool) bool {
	if !m.Focused() {
		return false
	}
	doc := m.axDocument()
	caret := m.axCaret()
	extend := mods.ShiftDown()
	byWord := mods.OptionDown() || (mods.ControlDown() && axOSMenuCommandIsControl())
	toEnds := mods.OSMenuCommandDown() && !axOSMenuCommandIsControl()
	index := axLineIndexFor(doc.lines, caret)
	switch keyCode {
	case KeyLeft:
		switch {
		case byWord:
			m.axMoveCaret(doc.wordStartBefore(caret), extend)
		case toEnds:
			m.axMoveCaret(axLineStart(doc.lines, index), extend)
		default:
			// A selection collapses to the end the caret is moving towards rather than the caret stepping out of it,
			// which is what every text control does.
			if start, end := m.axSelection(); start != end && !extend {
				m.axMoveCaret(start, false)
			} else {
				m.axMoveCaret(caret-1, extend)
			}
		}
	case KeyRight:
		switch {
		case byWord:
			m.axMoveCaret(doc.wordStartAfter(caret), extend)
		case toEnds:
			m.axMoveCaret(axLineEnd(doc.lines, doc.text, index), extend)
		default:
			if start, end := m.axSelection(); start != end && !extend {
				m.axMoveCaret(end, false)
			} else {
				m.axMoveCaret(caret+1, extend)
			}
		}
	case KeyUp:
		if toEnds {
			m.axMoveCaret(0, extend)
		} else {
			m.axMoveCaret(m.axCaretOnLine(index-1, caret), extend)
		}
	case KeyDown:
		if toEnds {
			m.axMoveCaret(len(doc.text), extend)
		} else {
			m.axMoveCaret(m.axCaretOnLine(index+1, caret), extend)
		}
	case KeyHome:
		if mods.OSMenuCommandDown() {
			m.axMoveCaret(0, extend)
		} else {
			m.axMoveCaret(axLineStart(doc.lines, index), extend)
		}
	case KeyEnd:
		if mods.OSMenuCommandDown() {
			m.axMoveCaret(len(doc.text), extend)
		} else {
			m.axMoveCaret(axLineEnd(doc.lines, doc.text, index), extend)
		}
	case KeyPageUp:
		m.axMoveCaret(m.axCaretOnPage(caret, -1), extend)
	case KeyPageDown:
		m.axMoveCaret(m.axCaretOnPage(caret, 1), extend)
	case KeyReturn, KeyNumPadEnter, KeySpace:
		// A document is read rather than typed into, so these are free to mean what a person on a link means by them.
		// With the caret anywhere else they are left to whatever holds the document, where Space still scrolls.
		link := doc.linkAt(caret)
		if link == nil {
			return false
		}
		return link.axSynthesizeClick()
	default:
		// Handled directly, as a field handles them, so that they work whether or not the application has a menu.
		if mods&mod.NonSticky == mod.OSMenuCommand() {
			switch keyCode {
			case KeyA:
				if m.CanSelectAll() {
					m.SelectAll()
					return true
				}
			case KeyC:
				if m.CanCopy() {
					m.Copy()
					return true
				}
			}
		}
		return false
	}
	return true
}

// axLineStart returns the offset a line begins at.
func axLineStart(lines []accessibility.Line, index int) int {
	if index < 0 || index >= len(lines) {
		return 0
	}
	return lines[index].Start
}

// axCaretOnLine returns the offset on another line that is nearest, horizontally, to where the caret is now, which is
// what moving up and down a document means. Moving past either end of the content goes to that end.
func (m *Markdown) axCaretOnLine(index, caret int) int {
	doc := m.axDocument()
	if index < 0 {
		return 0
	}
	if index >= len(doc.lines) {
		return len(doc.text)
	}
	return axOffsetNearestX(doc.lines, doc.text, index, axCaretX(doc.lines, caret))
}

// axCaretOnPage returns the offset a page up (-1) or down (1) from the caret: the same place on the line that far above
// or below the one the caret is on.
func (m *Markdown) axCaretOnPage(caret int, direction float32) int {
	doc := m.axDocument()
	rect := axCaretRect(doc.lines, caret)
	if rect.Empty() {
		return caret
	}
	x := axCaretX(doc.lines, caret)
	y := rect.CenterY() + direction*m.axPageHeight()
	return axOffsetNearestX(doc.lines, doc.text, axLineAt(doc.lines, geom.NewPoint(x, y)), x)
}

// wordStartBefore returns the start of the word before an offset, or the start of the stream when there is none.
func (d *axDocument) wordStartBefore(offset int) int {
	pos := min(max(offset, 0), len(d.text))
	for pos > 0 {
		pos--
		if textunit.IsWordStart(d.text, pos) {
			return pos
		}
	}
	return 0
}

// wordStartAfter returns the start of the word after an offset, or the end of the stream when there is none.
func (d *axDocument) wordStartAfter(offset int) int {
	pos := min(max(offset, 0), len(d.text))
	for pos < len(d.text) {
		pos++
		if pos < len(d.text) && textunit.IsWordStart(d.text, pos) {
			return pos
		}
	}
	return len(d.text)
}

// offsetAt returns the offset of the stream nearest a point in the markdown's own coordinates, which is where a click
// places the caret.
func (d *axDocument) offsetAt(pt geom.Point) int {
	if len(d.lines) == 0 {
		return 0
	}
	return axOffsetNearestX(d.lines, d.text, axLineAt(d.lines, pt), pt.X)
}

// DefaultMouseDown provides the default mouse down handling: a click places the reading caret, a shift-click extends
// the selection to it, a double-click takes the word and a triple-click the block.
//
// Nothing happens while the document is not focusable, which is whenever no assistive technology is being served and no
// application has asked for a document whose text can be read: an ordinary markdown is untouched by any of this. The
// links and images within it are panels with callbacks of their own and are offered the press first, since the window
// delivers one to the deepest panel under the pointer, so clicking a link still follows it.
func (m *Markdown) DefaultMouseDown(where geom.Point, button, clickCount int, mods mod.Modifiers) bool {
	if !m.Focusable() {
		return false
	}
	if button != ButtonLeft {
		// Any other button is not a gesture the document has anything to do with, so it is left to whatever else wants
		// it — without taking the keyboard focus out of whatever holds it, which claiming nothing afterwards would make a
		// press that did nothing but move the focus.
		return false
	}
	m.RequestFocus()
	doc := m.axDocument()
	pos := doc.offsetAt(where)
	m.axWordDrag = false
	switch clickCount {
	case 2:
		start, end := doc.wordAt(pos)
		m.axWordStart = start
		m.axWordEnd = end
		m.axWordDrag = true
		m.axSetSelection(start, end)
	case 3:
		if start, end, ok := doc.blockAt(pos); ok {
			m.axSetSelection(start, end)
		} else {
			m.axSetSelection(pos, pos)
		}
	default:
		if mods.ShiftDown() {
			// The anchor is the fixed end of the gesture, so a shift-click extends from wherever the selection was made
			// and leaves that end where it is.
			m.axSetSelection(m.anchor, pos)
		} else {
			m.axSetSelection(pos, pos)
		}
	}
	return true
}

// DefaultMouseDrag provides the default mouse drag handling, which extends the selection to the pointer and keeps the
// end that is moving in view. A drag that began with a double-click takes whole words, growing from the word the
// double-click landed on rather than shrinking back into it.
func (m *Markdown) DefaultMouseDrag(where geom.Point, button int, _ mod.Modifiers) bool {
	if !m.Focusable() {
		return false
	}
	if button != ButtonLeft {
		return true
	}
	doc := m.axDocument()
	pos := doc.offsetAt(where)
	if m.axWordDrag {
		start, end := doc.wordAt(pos)
		start = min(start, m.axWordStart)
		end = max(end, m.axWordEnd)
		if pos < m.axWordStart {
			m.axSetSelection(end, start)
		} else {
			m.axSetSelection(start, end)
		}
		return true
	}
	m.axSetSelection(m.anchor, pos)
	return true
}

// DefaultMouseUp provides the default mouse up handling, which has nothing to do.
func (m *Markdown) DefaultMouseUp(_ geom.Point, _ int, _ mod.Modifiers) bool {
	return false
}

// CanCopy returns true if the document has a selection that can be copied.
func (m *Markdown) CanCopy() bool {
	start, end := m.axSelection()
	return start < end
}

// Copy places the selected text on the clipboard. The placeholders standing in for the images within it are left out,
// since they are not text and there is nothing to paste for them.
func (m *Markdown) Copy() {
	start, end := m.axSelection()
	if start >= end {
		return
	}
	ClipboardSetText(strings.ReplaceAll(string(m.axDocument().text[start:end]),
		string(textunit.ObjectReplacement), ""))
}

// CanSelectAll returns true if the document has text to select.
func (m *Markdown) CanSelectAll() bool {
	return len(m.axDocument().text) > 0
}

// SelectAll selects all of the document's text, leaving the view where it is: selecting the whole of a document is not
// asking to be taken to the end of it, and a person reading the middle of one would otherwise find the view jerked to
// the bottom by the command and by the menu item both.
func (m *Markdown) SelectAll() {
	m.axApplySelection(0, len(m.axDocument().text), false)
}

// ContextMenuAnchor implements ContextMenuAnchorer: the menu opens beneath the reading caret's line, brought into view
// first, so that it neither covers the text it acts on nor opens out of sight. A document that is not focusable, or
// has nothing laid out, has no usable caret and falls back to DefaultContextMenuAnchor.
func (m *Markdown) ContextMenuAnchor() geom.Point {
	if !m.Focusable() {
		return m.DefaultContextMenuAnchor()
	}
	caret := axCaretRect(m.axDocument().lines, m.axCaret())
	if caret.Empty() {
		return m.DefaultContextMenuAnchor()
	}
	m.ScrollRectIntoView(caret)
	return geom.NewPoint(caret.X, caret.Bottom())
}

// DefaultDrawOver provides the default drawing over the content, which is the reading caret and the selection it has
// made. Neither is drawn unless the document holds the keyboard focus, so a markdown nothing is reading is drawn
// exactly as it always was. The caret does not blink: a document that redrew itself twice a second would cost every
// application showing one something, whether or not anybody was reading it.
func (m *Markdown) DefaultDrawOver(gc *Canvas, _ geom.Rect) {
	if !m.Focused() {
		return
	}
	doc := m.axDocument()
	if len(doc.lines) == 0 {
		return
	}
	if start, end := m.axSelection(); start < end {
		// Translucent, since the words beneath it have already been drawn and are what the person is reading.
		selection := &ColorFilteredInk{
			OriginalInk: ThemeFocus,
			ColorFilter: Alpha30Filter(),
		}
		for _, rect := range axLineRects(doc.lines, start, end) {
			gc.DrawRect(rect, selection.Paint(gc, rect, paintstyle.Fill))
		}
	}
	rect := axCaretRect(doc.lines, m.axCaret())
	ink := m.OnBackgroundInk
	if xreflect.IsNil(ink) {
		ink = ThemeOnSurface
	}
	if !rect.Empty() {
		gc.DrawRect(rect, ink.Paint(gc, rect, paintstyle.Fill))
	}
}

// ProvideAccessibility describes the markdown to assistive technologies as the document it is: the content of every
// block within it composed into one stream of text, with the lines it was laid out into, the styles it was drawn in, a
// span saying which part of it each element beneath it occupies, and the reading caret's place in it.
//
// The stream is carried apart from the node's own text so that an adapter has to opt into presenting it. The elements
// beneath the document describe the same content, and an assistive technology told to read both would read everything
// twice; only the platforms that present a document as text — Windows, through the UI Automation text pattern — are
// given the stream, and the value is left empty for the same reason.
func (m *Markdown) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	if node.Role == role.Auto {
		node.Role = role.Document
	}
	// Pressing a document is not activating it: the default behavior would synthesize a click in the middle of it, which
	// now does no more than drop the caret there.
	node.Actions = node.Actions.Without(accessibility.Press).
		With(accessibility.SetTextSelection, accessibility.ScrollRangeIntoView)
	// A document is read, not written: the caret may be moved through it and the text may be copied out of it, and
	// nothing an assistive technology offers may change a rune of it. Every block within it says the same.
	node.ReadOnly = true
	doc := m.axDocument()
	start, end := m.axSelection()
	info := accessibility.TextInfo{
		Text:      doc.str(),
		Lines:     doc.lines,
		Runs:      doc.runs,
		SelStart:  start,
		SelEnd:    end,
		Caret:     m.axCaret(),
		Multiline: true,
	}
	info.Spans = make([]accessibility.TextSpan, 0, len(doc.spans))
	for _, span := range doc.spans {
		var id accessibility.NodeID
		if span.key != nil {
			// A row of a table has no panel of its own, so it is pointed at through the key the table will describe it
			// under, whether or not the table has been described yet.
			id = b.virtualIDOf(span.panel, span.key)
		} else {
			id = b.IDFor(span.panel)
		}
		if id == 0 {
			continue
		}
		info.Spans = append(info.Spans, accessibility.TextSpan{Node: id, Start: span.start, End: span.end})
	}
	node.Document = &accessibility.DocumentInfo{Text: info}
	node.Text = nil
	node.Value = ""
}

// PerformAccessibilityAction carries out a request from an assistive technology: the reading caret or the selection may
// be moved anywhere in the stream, and a range of it may be brought into view. Showing the contextual menu is left to
// the default behavior; a request for the menu that names a range has the selection placed on it first (see
// axPlaceContextMenuRange).
func (m *Markdown) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	switch req.Action {
	case accessibility.SetTextSelection:
		m.axSetSelection(req.Start, req.End)
		return true
	case accessibility.ScrollRangeIntoView:
		doc := m.axDocument()
		start := min(req.Start, req.End)
		end := max(req.Start, req.End)
		if rect := axRangeRect(doc.lines, start, end); !rect.Empty() {
			m.ScrollRectIntoView(rect)
		}
		return true
	case accessibility.ShowContextMenu:
		// Declined so that the default behavior opens the menu, at the caret.
		m.axPlaceMenuRange(req)
		return false
	default:
		return false
	}
}

// axPlaceMenuRange implements axContextMenuRangePlacer.
func (m *Markdown) axPlaceMenuRange(req accessibility.ActionRequest) {
	axPlaceContextMenuRange(m.AsPanel(), req, m.axSelection, m.axSetSelection)
}

// ProvideAccessibility describes one block of a document's content: what kind of block it is, the whole of its text
// with the lines it was laid out into and the styles it was drawn in, the links and images sitting within that text,
// and the part of the document's caret and selection that falls in it.
//
// A heading holding a link or an image is described with its content as well. Such a thing is not text: it is announced
// as what it is, moved to on its own and, for a link, followed, and a heading that swallowed it would leave the person
// who heard the heading with no way to reach what was in it. The heading still reads as the whole of its text, since
// its name is gathered from everything within it either way.
func (b *markdownBlock) ProvideAccessibility(builder *AccessibilityBuilder) {
	node := builder.Node()
	if node.Role == role.Auto {
		node.Role = b.kind
	}
	if b.kind == role.Heading {
		node.Level = b.level
	}
	if b.row >= 0 {
		node.RowIndex = b.row
		node.ColumnIndex = b.col
	}
	// A document is read, not written: the caret may be moved through it and the text may be copied out of it, and
	// nothing an assistive technology offers may change a rune of it.
	node.ReadOnly = true
	node.Actions = node.Actions.With(accessibility.SetTextSelection, accessibility.ScrollRangeIntoView)
	text := b.axText()
	info := &accessibility.TextInfo{
		Text:      text.str(),
		Lines:     text.lines,
		Runs:      text.runs,
		Multiline: len(text.lines) > 1,
	}
	for _, span := range text.spans {
		if id := builder.IDFor(span.panel); id != 0 {
			info.Spans = append(info.Spans, accessibility.TextSpan{Node: id, Start: span.start, End: span.end})
		}
	}
	b.axFillSelection(info, len(text.text))
	node.Text = info
	if axCaretBlockReportsFocus && b.axHoldsCaret() && b.md.Focused() {
		// The document holds the keyboard focus; this says the caret within it is here as well, which is the only way
		// Orca presents a caret the application moved. Everywhere else the builder takes the claim away, since UI
		// Automation and AppKit have one focused element apiece. See axCaretBlockReportsFocus.
		node.Focused = true
	}
	if b.kind == role.Heading && markdownHasInlineElement(b.AsPanel()) {
		builder.DescribeChildren()
	}
}

// axDocRange returns where this block's text sits within the document's stream.
func (b *markdownBlock) axDocRange() (start, end int, ok bool) {
	return b.md.axDocument().blockRange(b)
}

// axHoldsCaret reports whether the document's reading caret is within this block.
func (b *markdownBlock) axHoldsCaret() bool {
	start, end, ok := b.axDocRange()
	if !ok {
		return false
	}
	caret := b.md.axCaret()
	return caret >= start && caret <= end
}

// axFillSelection fills in the caret and selection this block reports, which are its own offsets rather than the
// document's.
//
// A block the caret has left goes on reporting where it last was. Moving the caret out of a block changes nothing about
// that block — the person has not moved within it — and reporting the caret back at its start would have an assistive
// technology hear a caret move in the block being left as well as in the one being entered, and read the wrong line.
func (b *markdownBlock) axFillSelection(info *accessibility.TextInfo, length int) {
	keep := min(max(b.lastCaret, 0), length)
	start, end, ok := b.axDocRange()
	if !ok {
		info.SelStart, info.SelEnd, info.Caret = keep, keep, keep
		return
	}
	selStart, selEnd := b.md.axSelection()
	caret := b.md.axCaret()
	if (caret < start || caret > end) && (selStart >= end || selEnd <= start) {
		info.SelStart, info.SelEnd, info.Caret = keep, keep, keep
		b.lastCaret = keep
		return
	}
	info.SelStart = min(max(selStart-start, 0), length)
	info.SelEnd = min(max(selEnd-start, 0), length)
	info.Caret = min(max(caret-start, 0), length)
	if info.Caret != info.SelStart && info.Caret != info.SelEnd {
		// The caret is always one end of the selection, and a selection reaching past both ends of this block has its
		// own caret outside it, so the end of what falls within this block that the caret lies beyond is where the caret
		// is as far as this block is concerned.
		if caret <= start {
			info.Caret = info.SelStart
		} else {
			info.Caret = info.SelEnd
		}
	}
	b.lastCaret = info.Caret
}

// PerformAccessibilityAction carries out a request aimed at this block. The offsets of a request are the block's own,
// so moving the caret or the selection turns them into the document's, which is where the one caret a document has
// lives.
func (b *markdownBlock) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	switch req.Action {
	case accessibility.SetTextSelection:
		start, _, ok := b.axDocRange()
		if !ok {
			return false
		}
		b.md.axSetSelection(start+req.Start, start+req.End)
		return true
	case accessibility.ScrollRangeIntoView:
		text := b.axText()
		if rect := axRangeRect(text.lines, min(req.Start, req.End), max(req.Start, req.End)); !rect.Empty() {
			b.ScrollRectIntoView(rect)
		}
		return true
	default:
		return false
	}
}

// ProvideAccessibility describes the table and the rows within it. The cells are laid out as one flow of panels, since
// that is what gives the table its look, so each row is described as a child that has no panel of its own with the
// cells of that row beneath it: a screen reader moves from row to row and from cell to cell within one, and is told
// which row and column each cell is in.
func (t *markdownTable) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	if node.Role == role.Auto {
		node.Role = role.Table
	}
	node.ColumnCount = t.columns
	rows := t.axRows()
	node.RowCount = len(rows)
	for r, cells := range rows {
		if len(cells) == 0 {
			continue
		}
		bounds := t.axRowBounds(cells)
		rowID := b.AddVirtualChild(markdownRowKey{Row: r}, func(n *accessibility.Node) {
			n.Role = role.Row
			n.RowIndex = r
			n.Bounds = bounds
			n.Actions = n.Actions.With(accessibility.ScrollIntoView)
		})
		if rowID == 0 {
			continue
		}
		children := make([]axChildAt, 0, len(cells))
		for _, cell := range cells {
			children = append(children, axChildAt{panel: cell.block.AsPanel(), index: cell.index})
		}
		b.describeChildrenUnder(rowID, children)
	}
	// The cells have been described beneath the rows they belong to, so nothing may describe them a second time as
	// children of the table itself. See AccessibilityBuilder.describeChildrenUnder.
	b.snapshot.markChildrenDescribed(node.ID)
}

// PerformAccessibilityAction carries out a request aimed at one of the rows the table described, which have no panels
// of their own: bringing a row into view is scrolling the area its cells occupy into view.
func (t *markdownTable) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	key, ok := req.Key.(markdownRowKey)
	if !ok || req.Action != accessibility.ScrollIntoView {
		return false
	}
	rows := t.axRows()
	if key.Row < 0 || key.Row >= len(rows) || len(rows[key.Row]) == 0 {
		return false
	}
	t.ScrollRectIntoView(t.axRowBounds(rows[key.Row]))
	return true
}

// axTableCell is one of a table's cells, together with the index it occupies among the table's children, which is the
// index it is described at. See AccessibilityBuilder.describeChildrenUnder, which is handed that index rather than
// searching the table's child list for each cell in turn.
type axTableCell struct {
	block *markdownBlock
	index int
}

// axRows returns the table's cells grouped by the row each of them sits in.
func (t *markdownTable) axRows() [][]axTableCell {
	var rows [][]axTableCell
	for i, child := range t.Children() {
		cell, ok := child.Self.(*markdownBlock)
		if !ok || cell.row < 0 {
			continue
		}
		for len(rows) <= cell.row {
			rows = append(rows, nil)
		}
		rows[cell.row] = append(rows[cell.row], axTableCell{block: cell, index: i})
	}
	return rows
}

// axRowBounds returns the area a row's cells occupy, in the table's own coordinates.
func (t *markdownTable) axRowBounds(cells []axTableCell) geom.Rect {
	var bounds geom.Rect
	for i, cell := range cells {
		frame := cell.block.FrameRect()
		if i == 0 {
			bounds = frame
			continue
		}
		bounds = bounds.Union(frame)
	}
	return bounds
}
