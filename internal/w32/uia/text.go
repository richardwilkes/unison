// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package uia

import (
	"slices"
	"unicode"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/textunit"
)

// This file is the whole of the Text pattern's arithmetic: how a document's composed stream divides into the
// characters, words, lines, paragraphs and formatting runs an assistive technology reads it in, what a range of it says
// about itself, and where on screen it is. Like map.go and patterns.go it is a pure function of the snapshot —
// nothing here touches the OS, allocates COM memory or depends on which thread it runs on — so the file carries no
// build constraint and its tests run on any platform. text_windows.go and textrange_windows.go do nothing but
// turn these answers into COM out-parameters.
//
// Every offset here is a rune index into the stream, which is what accessibility.TextInfo measures in and what both
// other adapters use. UI Automation counts in UTF-16 code units in one place only — the maximum length GetText is given
// — and TextClip is what deals with that.

// textUnitCount is how many text units UI Automation defines, which is what indexes the divisions of a document by
// the unit they divide it into.
const textUnitCount = int(TextUnit_Document) + 1

// textDocument is the view of one Document node's composed stream that the Text pattern answers from: the stream's
// runes, the offsets every unit begins at, the elements that occupy parts of it, and where its lines are on screen.
//
// It is immutable once built, which is what lets one be shared by the arbitrary threads UI Automation calls in on and
// remembered for the snapshot it was built from; see memoizedTextDocument. Everything it needs is worked out up
// front, because the alternative — working a division out the first time it is asked for — would need a lock around
// every answer, and the whole point of remembering one is that a client reading a document asks the same few questions
// thousands of times.
//
// A view is built only for a Document that actually carries a stream. A Document without one hands out no Text pattern
// at all (see rolePatterns), so nothing here is ever reached for it.
type textDocument struct {
	tree       *accessibility.Tree
	node       *accessibility.Node
	info       *accessibility.TextInfo
	runes      []rune
	spans      []span
	boundaries [textUnitCount][]int // the Character unit's entry is left nil; see boundaries
	visible    geom.Rect
	length     int
	focused    bool
}

// span is one element's stretch of a document's stream, along with where it sits among the others. It is the
// snapshot's accessibility.TextSpan with two things added: the spans of elements that have no provider are left out,
// since nothing can be handed to a client for them, and each span knows which one contains it, which is what
// ITextRangeProvider::GetChildren asks for.
type span struct {
	node   accessibility.NodeID
	start  int
	end    int
	parent int
}

// newTextDocument builds the view of a Document node's stream, or nil when the node carries none.
func newTextDocument(t *accessibility.Tree, n *accessibility.Node) *textDocument {
	if t == nil || n == nil || n.Document == nil {
		return nil
	}
	d := &textDocument{tree: t, node: n, info: &n.Document.Text}
	d.runes = []rune(d.info.Text)
	d.length = len(d.runes)
	d.visible = visibleBounds(t, n)
	d.focused = HasKeyboardFocus(t, n)
	d.spans = reportedSpans(t, d.info.Spans)
	for unit := range textUnitCount {
		if TextUnit(unit) != TextUnit_Character {
			d.boundaries[unit] = d.computeBoundaries(TextUnit(unit))
		}
	}
	return d
}

// reportedSpans turns the snapshot's spans into the ones a client can be told about, each knowing which span
// contains it. A span whose node the snapshot no longer holds, or marks Ignored, is left out: such a node has no
// provider, so GetEnclosingElement and GetChildren could not hand it over, and the element that does have one is
// whichever span contains it.
//
// The parent of a span is the innermost span that contains it, which is the last one before it in the snapshot's order
// that covers its whole range: accessibility.TextInfo documents that order as outermost first and then by start offset,
// so among the spans covering one range the later one is the more deeply nested. A span nothing contains is a child of
// the document itself, recorded as -1.
func reportedSpans(t *accessibility.Tree, spans []accessibility.TextSpan) []span {
	if len(spans) == 0 {
		return nil
	}
	reported := make([]span, 0, len(spans))
	for _, sp := range spans {
		if n := t.Node(sp.Node); n == nil || n.Ignored {
			continue
		}
		parent := -1
		for i := range reported {
			if reported[i].start <= sp.Start && reported[i].end >= sp.End {
				parent = i
			}
		}
		reported = append(reported, span{node: sp.Node, start: sp.Start, end: sp.End, parent: parent})
	}
	return reported
}

// memoizedTextDocument returns the view of one Document node's stream, working it out and remembering it whenever
// snapshotMemo does not already hold one for this snapshot. It answers nil for a node that is not a Document or
// carries no stream.
//
// A snapshot the memo will not serve is answered from itself, outside the lock and remembering nothing, exactly as the
// memo's other answers are; see memoSwitchTo for which snapshots those are and why.
//
// The view is built outside the lock and stored under a second check, which is the one answer here that is not worked
// out with the lock held: dividing a long document costs milliseconds and allocates a slice per unit, and holding every
// other thread — and every other window — up for that is exactly what the lock is not for. Two threads that ask for the
// same view at once may each build one; the first to store it is the one both of them, and everything after, are
// answered with, so a client still cannot be handed two views of one snapshot.
func memoizedTextDocument(t *accessibility.Tree, id accessibility.NodeID) *textDocument {
	if t == nil {
		return nil
	}
	node := t.Node(id)
	if node == nil || node.Document == nil {
		return nil
	}
	snapshotMemo.lock.Lock()
	serving := memoSwitchTo(t)
	var held *textDocument
	if serving {
		held = snapshotMemo.documents[id]
	}
	snapshotMemo.lock.Unlock()
	if !serving {
		return newTextDocument(t, node)
	}
	if held != nil {
		return held
	}
	doc := newTextDocument(t, node)
	snapshotMemo.lock.Lock()
	defer snapshotMemo.lock.Unlock()
	if !memoSwitchTo(t) {
		// A newer snapshot of this window arrived while the view was being built, and is the one the memo is answering
		// from now. This view is still the truth about the snapshot it was asked about, so it is answered with and
		// forgotten.
		return doc
	}
	if held = snapshotMemo.documents[id]; held != nil {
		return held
	}
	if snapshotMemo.documents == nil {
		snapshotMemo.documents = make(map[accessibility.NodeID]*textDocument)
	}
	snapshotMemo.documents[id] = doc
	return doc
}

// textContainerFor returns the id of the document whose stream a node sits inside, or zero when it sits in none. It
// is what grants the TextChild pattern, whose two methods report exactly that document and the stretch of its text this
// element occupies.
//
// The nearest Document ancestor that carries a stream is the only candidate, and it has to claim the node: a panel
// inside a Markdown view that the document's composition passed over — a separator draws nothing and occupies no text —
// has no stretch of text to report, so it reports no pattern either rather than one whose get_TextRange would have to
// invent a range.
//
// A Document itself never reports the pattern. It is the text container rather than a child of one, and it hands out
// the Text pattern instead.
func textContainerFor(t *accessibility.Tree, n *accessibility.Node) accessibility.NodeID {
	if t == nil || n == nil || n.Document != nil {
		return 0
	}
	id := n.ID
	for depth := 0; depth < maxTreeDepth; depth++ {
		parentID := t.UnignoredParent(id)
		if parentID == 0 {
			return 0
		}
		parent := t.Node(parentID)
		if parent == nil {
			return 0
		}
		if parent.Document != nil {
			if _, _, ok := textSpanFor(t, parent, n.ID); ok {
				return parentID
			}
			return 0
		}
		id = parentID
	}
	return 0
}

// textSpanFor returns the stretch of a document's stream that a node occupies, and whether it occupies one. A node
// the document's composition gave no span of its own is answered with the span of its nearest ancestor that has one,
// which is the smallest stretch of text that is certainly the node's: a link's label inside a paragraph is part of that
// paragraph's text, and a client asking where the label is is better told the paragraph than nothing at all.
//
// The document itself has no span, and neither has anything outside it. The walk is bounded, and stops at the document,
// so a node in another part of the window cannot be answered with a stretch of this document's text.
//
// It works from the spans alone rather than from a textDocument, so that granting the TextChild pattern — which
// happens for every element a client asks about, in both of the snapshots a publish compares — costs a lookup rather
// than the several passes over the stream that building a view costs. The lookup goes through the snapshot's index of
// them; see memoizedTextSpans.
func textSpanFor(t *accessibility.Tree, doc *accessibility.Node, id accessibility.NodeID) (start, end int, ok bool) {
	if t == nil || doc == nil || doc.Document == nil || id == 0 || id == doc.ID {
		return 0, 0, false
	}
	index := memoizedTextSpans(t)
	for depth := 0; depth < maxTreeDepth; depth++ {
		if spanStart, spanEnd, found := spanOf(index, doc, id); found {
			return spanStart, spanEnd, true
		}
		parentID := t.UnignoredParent(id)
		if parentID == 0 || parentID == doc.ID {
			return 0, 0, false
		}
		id = parentID
	}
	return 0, 0, false
}

// spanOf returns the stretch of one document's stream that a node's own span records, and whether it has one. It
// answers from the snapshot's index when there is one and from the document's span list when there is not, and the two
// agree: the index holds the first span a document records for a node, which is the one a scan of the list finds.
//
// A node the index holds a span of some other document for has none of this one's, which is what stops a node inside a
// nested document from being answered with a stretch of the outer document's text.
func spanOf(index map[accessibility.NodeID]documentSpan, doc *accessibility.Node, id accessibility.NodeID,
) (start, end int, ok bool) {
	if index != nil {
		sp, held := index[id]
		if !held || sp.document != doc.ID {
			return 0, 0, false
		}
		return sp.start, sp.end, true
	}
	spans := doc.Document.Text.Spans
	for i := range spans {
		if spans[i].Node == id {
			return spans[i].Start, spans[i].End, true
		}
	}
	return 0, 0, false
}

// documentSpan is which document's stream an element occupies a stretch of, and which stretch of it. It is what a
// snapshot's spans are indexed by element into; see memoizedTextSpans.
type documentSpan struct {
	document accessibility.NodeID
	start    int
	end      int
}

// memoizedTextSpans returns the index of which stretch of which document's stream each element of a snapshot
// occupies, working it out and remembering it whenever snapshotMemo does not already hold one for this snapshot.
//
// It answers nil for a snapshot in which no document carries a span, and for a snapshot the memo will not serve — see
// memoSwitchTo for which those are and why — which has the caller answer from the document's own span list instead,
// exactly as it did before anything was remembered.
//
// The index is what keeps granting the TextChild pattern cheap. ProvidedPatterns asks where an element sits for
// every element a client so much as looks at — on every QueryInterface, every GetPatternProvider, every
// ReportsProperty check, and twice per node in patternAvailability — and a scan of a document's span list for each
// of those costs a pass over one span per block and inline element the document has, which over a whole tree is
// quadratic. Building the index is one pass over the same spans, made once per snapshot.
//
// Nothing writes to the map once it has been built, and the memo only ever replaces it wholesale, so the caller may
// read it after the lock has been released.
func memoizedTextSpans(t *accessibility.Tree) map[accessibility.NodeID]documentSpan {
	snapshotMemo.lock.Lock()
	defer snapshotMemo.lock.Unlock()
	if !memoSwitchTo(t) {
		return nil
	}
	if !snapshotMemo.textSpansDone {
		snapshotMemo.textSpans = documentSpans(t)
		snapshotMemo.textSpansDone = true
	}
	return snapshotMemo.textSpans
}

// documentSpans indexes every stretch of every document's stream in a snapshot by the element that occupies it, or
// nil when no document in it records one. The first span a document records for an element is the one kept, which is
// what a scan of that document's list finds; an element two documents both claim — which no composition produces, since
// a nested document's elements belong to it alone — is kept for whichever of them the snapshot is walked over first.
func documentSpans(t *accessibility.Tree) map[accessibility.NodeID]documentSpan {
	var index map[accessibility.NodeID]documentSpan
	for _, n := range t.Nodes {
		if n.Document == nil {
			continue
		}
		for _, sp := range n.Document.Text.Spans {
			if index == nil {
				index = make(map[accessibility.NodeID]documentSpan)
			}
			if _, held := index[sp.Node]; !held {
				index[sp.Node] = documentSpan{document: n.ID, start: sp.Start, end: sp.End}
			}
		}
	}
	return index
}

// visibleBounds returns the part of a node's bounds that is really on screen: its own bounds intersected with those
// of every one of its ancestors, which is what a scroll area does to the content inside it. A node the snapshot reports
// as offscreen has no visible area at all, and neither has one whose bounds fall entirely outside an ancestor's.
//
// An ancestor with no bounds filled in is passed over rather than clipping everything away, since a snapshot is
// entitled to leave the bounds of a node that draws nothing empty.
func visibleBounds(t *accessibility.Tree, n *accessibility.Node) geom.Rect {
	if t == nil || n == nil || n.Offscreen {
		return geom.Rect{}
	}
	bounds := n.Bounds
	for _, id := range t.Path(n.ID) {
		if id == n.ID {
			continue
		}
		if ancestor := t.Node(id); ancestor != nil && !ancestor.Bounds.Empty() {
			bounds = bounds.Intersect(ancestor.Bounds)
		}
	}
	return bounds
}

// Node returns the Document node this view describes.
func (d *textDocument) Node() *accessibility.Node {
	return d.node
}

// Len returns how many runes the stream holds, which is one past the last offset a range endpoint may sit at.
func (d *textDocument) Len() int {
	return d.length
}

// ClampOffset brings an offset within the stream. Every offset a client hands over goes through it, and so does every
// offset a range remembers: a range outlives the snapshot it was made from, and the document it stands for may have
// grown shorter since.
func (d *textDocument) ClampOffset(offset int) int {
	return min(max(offset, 0), d.length)
}

// ClampRange brings both ends of a range within the stream. An inverted range — which nothing here produces, since
// every operation that could cross the endpoints collapses them — is answered as the degenerate range at its start.
func (d *textDocument) ClampRange(start, end int) (clampedStart, clampedEnd int) {
	start = d.ClampOffset(start)
	end = d.ClampOffset(end)
	if end < start {
		end = start
	}
	return start, end
}

// Text returns the stream's content between two offsets.
func (d *textDocument) Text(start, end int) string {
	start, end = d.ClampRange(start, end)
	return string(d.runes[start:end])
}

// Selection returns the selected stretch of the stream, which is the degenerate range at the caret when nothing is
// selected.
func (d *textDocument) Selection() (start, end int) {
	return d.ClampRange(d.info.SelStart, d.info.SelEnd)
}

// Caret returns the offset the caret sits at. It is one end of the selection, and not always the later one: extending a
// selection backwards leaves the caret at its start, and a client told otherwise puts its reading cursor at the wrong
// end of what it has just read out.
func (d *textDocument) Caret() int {
	return d.ClampOffset(d.info.Caret)
}

// Focused reports whether the document holds the keyboard focus, which is what the IsActive text attribute says and
// what ITextProvider2::GetCaretRange reports alongside the caret: a caret in a document the user is not typing into is
// still where reading would resume, but it is not where the keyboard is.
func (d *textDocument) Focused() bool {
	return d.focused
}

// SupportedSelection returns what kind of selection the document allows, which is the answer to
// ITextProvider::get_SupportedTextSelection. A document that does not offer the SetTextSelection action has no caret to
// place — a Markdown view only accepts one while it can take the focus — so a client must be told it cannot select, not
// left to discover it when Select fails.
func (d *textDocument) SupportedSelection() SupportedTextSelection {
	if d.node.Actions.Has(accessibility.SetTextSelection) {
		return SupportedTextSelection_Single
	}
	return SupportedTextSelection_None
}

// Boundaries returns the offsets one text unit divides the stream at, in ascending order. The first is always zero and
// the last is always the length of the stream, so every offset a client can name falls inside exactly one unit, and the
// end of the text is a boundary without being the start of a unit. An empty stream divides into the single boundary
// zero.
//
// A unit this package does not know is answered with the document's own divisions, which is the documented fallback for
// a provider asked for a unit it does not support: report the next larger one it does. textUnitValid is what refuses
// a value that is not a unit at all.
func (d *textDocument) Boundaries(unit TextUnit) []int {
	return d.boundariesFor(unit).Offsets()
}

// boundariesFor returns the divisions of the stream by one unit in the form the arithmetic here asks about them. A unit
// this package does not divide by is answered with the document's own divisions; see Boundaries.
func (d *textDocument) boundariesFor(unit TextUnit) boundaries {
	if !textUnitValid(unit) {
		unit = TextUnit_Document
	}
	if unit == TextUnit_Character {
		return boundaries{length: d.length, runes: true}
	}
	return boundaries{offsets: d.boundaries[unit], length: d.length}
}

// boundaries is where one unit divides a stream, in the three terms every operation in this file is written in: how
// many boundaries there are, what the i'th of them is, and where an offset sits among them.
//
// Every unit but one is a slice of offsets worked out when the view was built. The Character unit is not: its
// boundaries are every offset from zero to the length of the stream, so the i'th of them is i, and an offset is a
// boundary exactly when it is within the stream. Answering it arithmetically is what keeps a document of a few hundred
// thousand runes from paying an int per rune — materialised twice, sorted and compacted — for a list whose contents
// were known before it was built.
type boundaries struct {
	offsets []int
	length  int
	runes   bool
}

// Len returns how many boundaries the unit divides the stream at, which is one more than the number of whole units it
// divides it into. An empty stream divides into the single boundary zero, which is both of its ends.
func (b boundaries) Len() int {
	if b.runes {
		if b.length == 0 {
			return 1
		}
		return b.length + 1
	}
	return len(b.offsets)
}

// At returns the i'th boundary, which the caller has established is one: every index here comes from Len or Search.
func (b boundaries) At(i int) int {
	if b.runes {
		return i
	}
	return b.offsets[i]
}

// Search reports where an offset sits among the boundaries, exactly as slices.BinarySearch does: the index of the
// boundary it is, or of the first one past it, and whether it is one of them.
func (b boundaries) Search(offset int) (i int, found bool) {
	if b.runes {
		switch {
		case offset < 0:
			return 0, false
		case offset > b.length:
			return b.Len(), false
		default:
			return offset, true
		}
	}
	return slices.BinarySearch(b.offsets, offset)
}

// Offsets returns the boundaries as the ascending slice a client asked for them as. The Character unit's is built here
// and nowhere else, since nothing inside this file needs one.
func (b boundaries) Offsets() []int {
	if !b.runes {
		return b.offsets
	}
	offsets := make([]int, 0, b.Len())
	for i := range b.Len() {
		offsets = append(offsets, i)
	}
	return offsets
}

// textUnitValid reports whether a value a client passed is one of the text units UI Automation defines. Anything
// else is a programming error on the client's side, which the provider answers with E_INVALIDARG rather than guessing
// at.
func textUnitValid(unit TextUnit) bool {
	return unit >= TextUnit_Character && unit <= TextUnit_Document
}

// textEndpointValid reports whether a value a client passed names one end of a range. It is the companion of
// textUnitValid, and is refused in the same places and for the same reason: a method that took anything outside the
// enumeration for the start would move or compare the wrong end of the range and report success, which a client has no
// way to notice.
func textEndpointValid(endpoint TextPatternRangeEndpoint) bool {
	return endpoint == TextPatternRangeEndpoint_Start || endpoint == TextPatternRangeEndpoint_End
}

// computeBoundaries works out where one unit divides the stream. The Character unit is not among them — its boundaries
// are every offset from zero to the length of the stream, which boundaries answers without a slice — and UI
// Automation's character is a code point rather than a grapheme cluster either way, which is what the rest of the
// toolkit counts in as well.
//
//   - Word: the divisions internal/textunit defines, so that a word is the same stretch of text here as it is on AT-SPI
//     — the same screen readers ask both, and an answer that differed would have one desktop read a document
//     differently from the next.
//   - Line: where the layout wrapped, which is what the snapshot's Lines record. A document whose lines were never
//     measured falls back to its paragraphs, since a line a client cannot be told the extent of is worse than a
//     paragraph it can.
//   - Paragraph: the line feeds the stream joins its blocks with, plus the start of every block and container that
//     occupies text. The second half is what makes a heading, a code block, a table cell and a list item each their own
//     paragraph even where the composition put no line feed between them.
//   - Format: where the styling changes, which is every run boundary, plus every span boundary — a link, an image or a
//     block beginning or ending is a change of formatting as far as a client walking the runs of a document is
//     concerned, and it is where the attributes this file answers can change.
//   - Page and Document: the whole stream. Nothing here paginates, so a page is the document, which is the documented
//     answer for a unit a provider does not divide by.
func (d *textDocument) computeBoundaries(unit TextUnit) []int {
	switch unit {
	case TextUnit_Word:
		return sortedBoundaries(d.length, textunit.WordStarts(d.runes))
	case TextUnit_Line:
		if len(d.info.Lines) == 0 {
			return d.computeBoundaries(TextUnit_Paragraph)
		}
		offsets := make([]int, 0, len(d.info.Lines))
		for i := range d.info.Lines {
			offsets = append(offsets, d.info.Lines[i].Start)
		}
		return sortedBoundaries(d.length, offsets)
	case TextUnit_Paragraph:
		offsets := textunit.ParagraphStarts(d.runes)
		for i := range d.spans {
			if spanIsParagraph(d.tree.Node(d.spans[i].node)) {
				offsets = append(offsets, d.spans[i].start)
			}
		}
		return sortedBoundaries(d.length, offsets)
	case TextUnit_Format:
		offsets := make([]int, 0, 2*(len(d.info.Runs)+len(d.spans)))
		for i := range d.info.Runs {
			offsets = append(offsets, d.info.Runs[i].Start, d.info.Runs[i].End)
		}
		for i := range d.spans {
			offsets = append(offsets, d.spans[i].start, d.spans[i].end)
		}
		return sortedBoundaries(d.length, offsets)
	default:
		return sortedBoundaries(d.length, nil)
	}
}

// spanIsParagraph reports whether the element a span stands for begins a paragraph of the stream. These are the
// blocks a document is read as, plus the two containers whose content is a paragraph of its own wherever it begins: a
// list item and a quoted passage.
func spanIsParagraph(n *accessibility.Node) bool {
	if n == nil {
		return false
	}
	switch n.Role {
	case role.Paragraph, role.Heading, role.Code, role.Cell, role.ColumnHeader, role.ListItem, role.BlockQuote:
		return true
	default:
		return false
	}
}

// sortedBoundaries returns the offsets a unit divides a stream of the given length at: those of the offsets handed
// over that fall within the stream, in ascending order, without duplicates, and with both ends of the stream always
// present. An empty stream yields the single boundary zero, which is both of its ends.
func sortedBoundaries(length int, offsets []int) []int {
	bounds := make([]int, 0, len(offsets)+2)
	bounds = append(bounds, 0)
	if length > 0 {
		bounds = append(bounds, length)
	}
	for _, offset := range offsets {
		if offset > 0 && offset < length {
			bounds = append(bounds, offset)
		}
	}
	slices.Sort(bounds)
	return slices.Compact(bounds)
}

// UnitStart returns the offset the unit containing an offset begins at. The end of the stream is the one offset that is
// a boundary without beginning a unit: it belongs to the last unit, which is the unit a caret sitting there is in.
func (d *textDocument) UnitStart(unit TextUnit, offset int) int {
	bounds := d.boundariesFor(unit)
	i, found := bounds.Search(d.ClampOffset(offset))
	if found {
		if bounds.At(i) == d.length && i > 0 {
			return bounds.At(i - 1)
		}
		return bounds.At(i)
	}
	// Zero is always a boundary, so the insertion point of an offset that is not one is never the first entry.
	return bounds.At(i - 1)
}

// UnitEnd returns the offset just past the unit containing an offset, which for an offset that is itself a boundary is
// the end of the unit that follows it.
func (d *textDocument) UnitEnd(unit TextUnit, offset int) int {
	bounds := d.boundariesFor(unit)
	i, found := bounds.Search(d.ClampOffset(offset))
	if found {
		i++
	}
	if i < bounds.Len() {
		return bounds.At(i)
	}
	return d.length
}

// IsUnitBoundary reports whether an offset is where a unit begins or where the stream ends, which is what decides
// whether Expand has anything to do to an endpoint.
func (d *textDocument) IsUnitBoundary(unit TextUnit, offset int) bool {
	_, found := d.boundariesFor(unit).Search(d.ClampOffset(offset))
	return found
}

// Expand returns the range ITextRangeProvider::ExpandToEnclosingUnit makes of one, which is the range grown outwards
// until both of its ends are unit boundaries. These are the eight cases UI Automation documents, in the order this
// implements them:
//
//  1. An empty stream: the range is the whole of it, which is degenerate, and no unit can be expanded to.
//  2. A degenerate range inside a unit: the unit containing it.
//  3. A degenerate range on a unit boundary: the unit that follows, since a boundary is where a unit begins.
//  4. A degenerate range at the end of the stream: the last unit, since nothing follows it. This is the case a caret at
//     the end of a document is in, and answering it with an empty range would leave a client with nothing to read.
//  5. A range that already spans whole units: unchanged. A client that expands twice must get the same answer, which is
//     what lets it walk a document by expanding and then moving.
//  6. A range whose start is inside a unit: the start moves back to that unit's beginning.
//  7. A range whose end is inside a unit: the end moves forward to that unit's end.
//  8. A range with both ends inside units: both move, which is cases 6 and 7 together.
func (d *textDocument) Expand(unit TextUnit, start, end int) (expandedStart, expandedEnd int) {
	start, end = d.ClampRange(start, end)
	if d.length == 0 {
		return 0, 0
	}
	if start == end {
		if start >= d.length {
			return d.UnitStart(unit, d.length), d.length
		}
		return d.UnitStart(unit, start), d.UnitEnd(unit, start)
	}
	expandedEnd = end
	if !d.IsUnitBoundary(unit, end) {
		expandedEnd = d.UnitEnd(unit, end)
	}
	return d.UnitStart(unit, start), expandedEnd
}

// Move returns the range ITextRangeProvider::Move makes of one, along with how many units it moved, which is fewer than
// were asked for when the stream ran out.
//
// It is the two sequences UI Automation documents, one for each kind of range:
//
// A degenerate range is a caret, and Move simply moves it by the requested number of unit boundaries, which leaves it
// degenerate. The first move in either direction lands on the nearest boundary that way whether or not the caret was
// already on one, exactly as MoveEndpointByUnit's does, so a caret inside a word moving back one lands on that word's
// beginning rather than on the previous word's. The end of the stream is somewhere a caret can sit, and a caret already
// there reports no movement at all: that is how a client stepping a caret through a document — NVDA's say-all and any
// review cursor — is told it has reached the end, and telling it otherwise would leave it moving forever.
//
// A range that covers text is collapsed to the beginning of the unit its start is in, stepped, and expanded by one unit
// afterwards, so moving by one unit from a range covering three words leaves a range covering the word after the first,
// not after the third — surprising until one remembers that Move is how a client walks a document one unit at a time,
// having expanded to a unit to begin with.
//
// Such a range therefore stops at the last unit rather than at the end of the stream: there is no unit past the last
// one for it to cover, and a count of zero is how a client walking a document is told it has reached the end. A client
// that was moved onto the end of the stream instead would be handed an empty range and have to work out for itself that
// the empty string it read was not a line.
func (d *textDocument) Move(unit TextUnit, count, start, end int) (movedStart, movedEnd, moved int) {
	start, end = d.ClampRange(start, end)
	if start == end {
		at, steps := d.stepEndpoint(unit, start, count)
		return at, at, steps
	}
	limit := d.boundariesFor(unit).Len() - 1
	if limit > 0 {
		limit--
	}
	at, steps := d.step(unit, d.UnitStart(unit, start), count, limit)
	return at, d.UnitEnd(unit, at), steps
}

// step moves an offset that sits on a unit boundary by count units, stopping at the boundary whose index is limit,
// reporting where it ended up and how many units it really moved — negative for a backward move, and short of what was
// asked for when it reached an end of the stream. It is how a range that covers text is moved; a caret goes through
// stepEndpoint, which does not need its offset to be on a boundary.
func (d *textDocument) step(unit TextUnit, at, count, limit int) (offset, moved int) {
	bounds := d.boundariesFor(unit)
	i, found := bounds.Search(at)
	if !found {
		// Nothing reaches this: the one caller normalizes to a boundary first. Taking the boundary below keeps the
		// answer inside the stream if anything ever does.
		i--
	}
	target := min(max(i+count, 0), min(limit, bounds.Len()-1))
	return bounds.At(target), target - i
}

// MoveEndpoint returns the range ITextRangeProvider::MoveEndpointByUnit makes of one, along with how many units the
// endpoint really moved.
//
// An endpoint that is not on a unit boundary reaches the next one as the first of its moves, which is what UI
// Automation documents and is what lets a client walk away from a range it built by hand. An endpoint moved past the
// other one takes it along, leaving the range degenerate: a range whose start had crossed its end would stand for text
// that runs backwards.
func (d *textDocument) MoveEndpoint(unit TextUnit, endpoint TextPatternRangeEndpoint, count, start, end int,
) (movedStart, movedEnd, moved int) {
	start, end = d.ClampRange(start, end)
	if endpoint == TextPatternRangeEndpoint_End {
		at, steps := d.stepEndpoint(unit, end, count)
		return min(start, at), at, steps
	}
	at, steps := d.stepEndpoint(unit, start, count)
	return at, max(end, at), steps
}

// stepEndpoint moves one offset by count units, reporting where it ended up and how many units it really moved. The
// first move in either direction goes to the nearest boundary that way, whether or not the offset was already on one,
// and a count of zero moves nothing at all: an endpoint a client did not ask to move, or a caret it asked to move by
// zero units, must come back where it was. It is what moves both an endpoint of a range and a caret; see MoveEndpoint
// and Move.
func (d *textDocument) stepEndpoint(unit TextUnit, at, count int) (offset, moved int) {
	if count == 0 {
		return at, 0
	}
	bounds := d.boundariesFor(unit)
	i, found := bounds.Search(at)
	if count > 0 {
		if found {
			i++
		}
		if i >= bounds.Len() {
			return at, 0
		}
		target := min(i+count-1, bounds.Len()-1)
		return bounds.At(target), target - i + 1
	}
	i--
	if i < 0 {
		return at, 0
	}
	target := max(i+count+1, 0)
	return bounds.At(target), target - i - 1
}

// OffsetAt returns the offset of the character at a point given in the window-local logical coordinates the snapshot's
// bounds are in, which is what a hit test from UI Automation goes through before it reaches the stream.
//
// Every point has an answer, unlike the AT-SPI method this is the twin of: ITextProvider::RangeFromPoint is documented
// to report the range nearest the point rather than to fail, so a point above the text answers with its beginning, one
// below it with its end, and one beside a line with the character at that end of the line. A document whose lines were
// never measured answers with its beginning, since there is no telling where anything in it is.
func (d *textDocument) OffsetAt(pt geom.Point) int {
	if len(d.info.Lines) == 0 {
		return 0
	}
	pt.X -= d.node.Bounds.X
	pt.Y -= d.node.Bounds.Y
	best := -1
	bestDistance := float32(0)
	for i := range d.info.Lines {
		line := &d.info.Lines[i]
		if pt.Y >= line.Bounds.Y && pt.Y < line.Bounds.Bottom() {
			best = i
			break
		}
		distance := line.Bounds.Y - pt.Y
		if distance < 0 {
			distance = pt.Y - line.Bounds.Bottom()
		}
		if best < 0 || distance < bestDistance {
			best, bestDistance = i, distance
		}
	}
	line := &d.info.Lines[best]
	return d.ClampOffset(offsetOnLine(line, pt.X-line.Bounds.X, d.length))
}

// offsetOnLine returns the offset of the character at a horizontal position on a line, measured from the line's own
// left edge. A position beyond either end answers with the character at that end, so that every point on the line has
// an offset, and a line whose advances were never filled in answers with its first character. count is how many runes
// the whole stream holds.
func offsetOnLine(line *accessibility.Line, x float32, count int) int {
	last := min(line.End, count) - 1
	switch {
	case last < line.Start || len(line.Advances) < 2:
		return line.Start
	default:
		for i := range len(line.Advances) - 1 {
			if x < line.Advances[i+1] {
				return min(line.Start+i, last)
			}
		}
		return last
	}
}

// VisibleRange returns the stretch of the stream that is on screen, and whether any of it is. It is what
// ITextProvider::GetVisibleRanges reports, which is how a client that reads what the user can see — rather than the
// whole of a long document — knows where to start.
//
// A document whose lines were never measured reports the whole stream while it is on screen at all, since there is no
// telling which part of it is where; one that is scrolled out of view, or clipped away by an ancestor, reports nothing.
func (d *textDocument) VisibleRange() (start, end int, ok bool) {
	if d.visible.Empty() {
		return 0, 0, false
	}
	if len(d.info.Lines) == 0 {
		return 0, d.length, true
	}
	first, last := -1, -1
	for i := range d.info.Lines {
		if !d.lineBounds(&d.info.Lines[i]).Intersects(d.visible) {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	if first < 0 {
		return 0, 0, false
	}
	return d.ClampOffset(d.info.Lines[first].Start), d.ClampOffset(d.info.Lines[last].End), true
}

// Rectangles returns one rectangle per line the range covers, in the window-local logical coordinates the snapshot's
// bounds are in, clipped to what is actually visible. A degenerate range covers no text and therefore has no
// rectangles, which is what UI Automation documents; neither has a range in a document that is scrolled out of view.
//
// A line whose advances were never filled in contributes its whole width, since there is no telling which part of it
// the range covers, and that is a better answer for a client drawing a highlight than none at all.
func (d *textDocument) Rectangles(start, end int) []geom.Rect {
	start, end = d.ClampRange(start, end)
	if start >= end || d.visible.Empty() {
		return nil
	}
	var rects []geom.Rect
	for i := range d.info.Lines {
		line := &d.info.Lines[i]
		if line.End <= start || line.Start >= end {
			continue
		}
		rect := d.lineRect(line, max(start, line.Start), min(end, line.End)).Intersect(d.visible)
		if !rect.Empty() {
			rects = append(rects, rect)
		}
	}
	return rects
}

// lineBounds returns a line's area in the window-local coordinates the node's own bounds are in. The snapshot measures
// a line from the top left corner of the node that drew it.
func (d *textDocument) lineBounds(line *accessibility.Line) geom.Rect {
	bounds := line.Bounds
	bounds.X += d.node.Bounds.X
	bounds.Y += d.node.Bounds.Y
	return bounds
}

// lineRect returns the area of one line between two offsets of the stream, in window-local coordinates. The offsets
// must both fall within the line.
func (d *textDocument) lineRect(line *accessibility.Line, from, to int) geom.Rect {
	bounds := d.lineBounds(line)
	if first, last := from-line.Start, to-line.Start; first >= 0 && last > first && last < len(line.Advances) {
		left, right := line.Advances[first], line.Advances[last]
		bounds.X += left
		bounds.Width = right - left
	}
	return bounds
}

// EnclosingSpan returns the id of the element ITextRangeProvider::GetEnclosingElement reports for a range: the
// innermost element whose own stretch of the stream covers the whole range. The document itself is the answer when no
// element's does, since the document encloses all of its text.
//
// A degenerate range is treated as the character that follows it, which is the element a caret sitting there is in.
// That is what makes GetEnclosingElement useful to a client reading with the caret: it is how the link, image or block
// the caret has just moved into is identified.
func (d *textDocument) EnclosingSpan(start, end int) accessibility.NodeID {
	if i := d.enclosingSpanIndex(start, end); i >= 0 {
		return d.spans[i].node
	}
	return d.node.ID
}

// enclosingSpanIndex returns the index within d.spans of the innermost span covering a range, or -1 when none does. The
// spans are in nesting order, so the last one that covers the range is the innermost.
func (d *textDocument) enclosingSpanIndex(start, end int) int {
	start, end = d.ClampRange(start, end)
	if start == end {
		end = min(start+1, d.length)
	}
	enclosing := -1
	for i := range d.spans {
		if d.spans[i].start <= start && d.spans[i].end >= end {
			enclosing = i
		}
	}
	return enclosing
}

// Children returns the ids of the elements ITextRangeProvider::GetChildren reports for a range: the elements one level
// down from the one that encloses it that occupy any of the range's text. A degenerate range covers no text and so has
// no children.
//
// One level down, rather than every element within the range, is what the method means: a client walks a document by
// asking for the children of a range and then narrowing to one of them, and a flat list of every link inside every
// paragraph of a page would have it read the same text twice.
func (d *textDocument) Children(start, end int) []accessibility.NodeID {
	start, end = d.ClampRange(start, end)
	if start == end {
		return nil
	}
	parent := d.enclosingSpanIndex(start, end)
	var ids []accessibility.NodeID
	for i := range d.spans {
		sp := &d.spans[i]
		if sp.parent != parent || sp.end <= start || sp.start >= end {
			continue
		}
		ids = append(ids, sp.node)
	}
	return ids
}

// SpanFor returns the stretch of the stream an element occupies, and whether it occupies one. See textSpanFor, which
// it defers to: an element with no stretch of its own is answered with its nearest ancestor's, and the document itself
// is answered with none, which ITextProvider::RangeFromChild reports as an invalid argument.
func (d *textDocument) SpanFor(id accessibility.NodeID) (start, end int, ok bool) {
	return textSpanFor(d.tree, d.node, id)
}

// attributeKind says what sort of value an answer about a text attribute carries, which is what decides how the
// Windows layer stores it into the VARIANT a client reads it from.
type attributeKind uint8

// Possible attributeKind values. The first two are the reserved values UI Automation defines for the two things a
// provider may have to say instead of a value: that it will never answer this attribute, and that the range covers text
// the attribute differs over. See textRangeGetAttributeValue.
const (
	attributeUnsupported attributeKind = iota
	attributeMixed
	attributeEmpty
	attributeBoolean
	attributeInteger
	attributeNumber
	attributeString
	attributeRange
)

// attribute is what a range has to say about one text attribute. Which fields carry information depends on Kind; the
// rest are left at their zero values. It is comparable, which is what lets two stretches of a range be checked for
// agreeing about an attribute, and what lets FindAttribute compare a client's value against the text's.
type attribute struct {
	// Text is the value of a string-valued attribute.
	Text string
	// Number is the value of a floating-point attribute, which is the font size.
	Number float64
	// Start is the first offset of the stretch a range-valued attribute names, which is the Link attribute.
	Start int
	// End is the offset just past that stretch.
	End int
	// Int is the value of an integer-valued attribute, which includes every enumeration UI Automation defines.
	Int int32
	// Kind says which of the others carries the answer.
	Kind attributeKind
	// Bool is the value of a boolean attribute.
	Bool bool
}

// Attribute returns what a range has to say about one text attribute.
//
// The answer is worked out for every formatting unit the range covers and is the value they agree on; where they do not
// agree it is the reserved mixed value, which is how a client is told that the range covers more than one kind of text.
// An attribute no document here records is the reserved not-supported value rather than a made-up one: a client asked
// to read a passage in the wrong language, or to announce a spelling error that was never reported, is worse served by
// a guess than by being told there is no answer.
func (d *textDocument) Attribute(start, end int, attr TextAttributeID) attribute {
	start, end = d.ClampRange(start, end)
	var answer attribute
	for i, offset := range d.formatOffsets(start, end) {
		value := d.attributeAt(offset, attr)
		if i == 0 {
			answer = value
			continue
		}
		if value != answer {
			return attribute{Kind: attributeMixed}
		}
	}
	return answer
}

// FindAttribute returns the stretch of a range over which one attribute has the value a client is looking for, and
// whether there is one. It is how a client jumps to the next link, the next heading or the next passage in a different
// font without reading everything in between.
//
// The stretch is as long as the attribute goes on agreeing, and searching backward finds the last such stretch rather
// than the first. A value that is not one a stretch of text can have — the reserved mixed and not-supported values, and
// the range a link is reported as — matches nothing: the first two are answers about a range rather than properties of
// the text, and no two links are the same range.
func (d *textDocument) FindAttribute(start, end int, attr TextAttributeID, want attribute, backward bool,
) (foundStart, foundEnd int, ok bool) {
	start, end = d.ClampRange(start, end)
	if start >= end || !attributeSearchable(want.Kind) {
		return 0, 0, false
	}
	offsets := d.formatOffsets(start, end)
	var stretchStart, stretchEnd int
	inStretch := false
	for i, offset := range offsets {
		limit := end
		if i+1 < len(offsets) {
			limit = offsets[i+1]
		}
		switch {
		case d.attributeAt(offset, attr) != want:
			if inStretch {
				foundStart, foundEnd, ok = stretchStart, stretchEnd, true
				inStretch = false
				if !backward {
					return foundStart, foundEnd, true
				}
			}
		case inStretch:
			stretchEnd = limit
		default:
			stretchStart, stretchEnd, inStretch = offset, limit, true
		}
	}
	if inStretch {
		return stretchStart, stretchEnd, true
	}
	return foundStart, foundEnd, ok
}

// attributeSearchable reports whether a value is one a stretch of text can be looked for by. The reserved mixed and
// not-supported values are answers about a range rather than properties of the text, an empty answer says the text has
// nothing to report, and the range a link is answered with is unique to that link, so none of the four can match
// anything.
func attributeSearchable(kind attributeKind) bool {
	switch kind {
	case attributeBoolean, attributeInteger, attributeNumber, attributeString:
		return true
	default:
		return false
	}
}

// formatOffsets returns one offset within each formatting unit a range covers, which is where every attribute answer is
// worked out from. A degenerate range covers the unit its offset is in, and an empty stream has the single offset zero,
// which no run or span covers.
func (d *textDocument) formatOffsets(start, end int) []int {
	if d.length == 0 {
		return []int{0}
	}
	if start >= end {
		return []int{min(start, d.length-1)}
	}
	offsets := []int{start}
	for _, boundary := range d.Boundaries(TextUnit_Format) {
		if boundary > start && boundary < end {
			offsets = append(offsets, boundary)
		}
	}
	return offsets
}

// attributeAt returns what one offset of the stream has to say about one text attribute. See Attribute, which is what
// asks it, and constants.go for which attributes are answered and which are deliberately refused.
func (d *textDocument) attributeAt(offset int, attr TextAttributeID) attribute {
	run := d.runAt(offset)
	switch attr {
	case FontNameAttributeId:
		if run == nil || run.Family == "" {
			return attribute{Kind: attributeUnsupported}
		}
		return attribute{Kind: attributeString, Text: run.Family}
	case FontSizeAttributeId:
		if run == nil || run.Size <= 0 {
			return attribute{Kind: attributeUnsupported}
		}
		return attribute{Kind: attributeNumber, Number: float64(run.Size)}
	case FontWeightAttributeId:
		if run == nil || run.Weight <= 0 {
			return attribute{Kind: attributeUnsupported}
		}
		return attribute{Kind: attributeInteger, Int: int32(run.Weight)}
	case IsItalicAttributeId:
		return attribute{Kind: attributeBoolean, Bool: run != nil && run.Italic}
	case UnderlineStyleAttributeId:
		return attribute{Kind: attributeInteger, Int: int32(lineStyle(run != nil && run.Underline))}
	case StrikethroughStyleAttributeId:
		return attribute{Kind: attributeInteger, Int: int32(lineStyle(run != nil && run.Strikethrough))}
	case IsReadOnlyAttributeId:
		// Every document this adapter presents is read-only: a Markdown view lets a person select and read its text,
		// never edit it, and an editable control reports its content through the Value pattern instead.
		return attribute{Kind: attributeBoolean, Bool: true}
	case IsHiddenAttributeId:
		// Nothing in a stream is hidden: the composition leaves out what is not drawn, so text that is in the stream at
		// all is text a person could read.
		return attribute{Kind: attributeBoolean, Bool: false}
	case IsActiveAttributeId:
		return attribute{Kind: attributeBoolean, Bool: d.focused}
	case StyleIdAttributeId:
		style, _ := d.styleAt(offset)
		return attribute{Kind: attributeInteger, Int: int32(style)}
	case StyleNameAttributeId:
		if _, name := d.styleAt(offset); name != "" {
			return attribute{Kind: attributeString, Text: name}
		}
		return attribute{Kind: attributeUnsupported}
	case LinkAttributeId:
		if start, end, ok := d.linkAt(offset); ok {
			return attribute{Kind: attributeRange, Start: start, End: end}
		}
		// Text that is not part of a link has no link, which is an empty answer rather than a refusal: the attribute is
		// one this document answers, and a client that asked is told there is nothing here.
		return attribute{Kind: attributeEmpty}
	default:
		return attribute{Kind: attributeUnsupported}
	}
}

// lineStyle turns the presence of an underline or a strikethrough into the style UI Automation reports it as. A
// snapshot records that a run is decorated and not how the line is drawn, so the answer is the plain single line.
func lineStyle(decorated bool) TextDecorationLineStyle {
	if decorated {
		return TextDecorationLineStyle_Single
	}
	return TextDecorationLineStyle_None
}

// runAt returns the styled run covering an offset, or nil when the document records none. Runs tile the stream in
// ascending order, so the one covering an offset is found by bisection.
func (d *textDocument) runAt(offset int) *accessibility.TextRun {
	i, found := slices.BinarySearchFunc(d.info.Runs, offset, func(run accessibility.TextRun, target int) int {
		switch {
		case run.End <= target:
			return -1
		case run.Start > target:
			return 1
		default:
			return 0
		}
	})
	if found {
		return &d.info.Runs[i]
	}
	return nil
}

// styleAt returns the style of the text at an offset, along with the name of that style when UI Automation has no
// identifier for it.
//
// It is the innermost element covering the offset that has a style of its own that decides, rather than simply the
// innermost element: a link inside a heading is still heading text, and a paragraph inside a list item is still part of
// a list. Text no element claims a style for is body text, which is what Normal means.
func (d *textDocument) styleAt(offset int) (style StyleID, name string) {
	for i := len(d.spans) - 1; i >= 0; i-- {
		sp := &d.spans[i]
		if sp.start > offset || sp.end <= offset {
			continue
		}
		n := d.tree.Node(sp.node)
		if n == nil {
			continue
		}
		switch n.Role {
		case role.Heading:
			return headingStyle(n.Level), ""
		case role.BlockQuote:
			return StyleId_Quote, ""
		case role.Code:
			// UI Automation has no style identifier for preformatted source, so it is a custom style with a name. A
			// client speaks the name, which is how "Code" is announced at all.
			return StyleId_Custom, "Code"
		case role.ListItem:
			// Every list item reports the bulleted style. A snapshot does not record whether a list is numbered — the
			// number is drawn into the item's own text, where a person reads it — so reporting NumberedList for some of
			// them would be a guess.
			return StyleId_BulletedList, ""
		default:
		}
	}
	return StyleId_Normal, ""
}

// headingStyle returns the style identifier for a heading of the given level. UI Automation defines nine, so a
// deeper heading reports the ninth, and a heading whose level the snapshot never filled in reports the first: it is a
// heading either way, and a client walking a document by heading must not be made to skip it.
func headingStyle(level int) StyleID {
	switch {
	case level < 1:
		return StyleId_Heading1
	case level > 9:
		return StyleId_Heading9
	default:
		return StyleId_Heading1 + StyleID(level-1)
	}
}

// linkAt returns the stretch of the stream the innermost link covering an offset occupies, and whether there is one.
func (d *textDocument) linkAt(offset int) (start, end int, ok bool) {
	for i := len(d.spans) - 1; i >= 0; i-- {
		sp := &d.spans[i]
		if sp.start > offset || sp.end <= offset {
			continue
		}
		if n := d.tree.Node(sp.node); n != nil && n.Role == role.Link {
			return sp.start, sp.end, true
		}
	}
	return 0, 0, false
}

// FindText returns the stretch of a range that holds a piece of text, and whether it holds it at all. Searching
// backward finds the last occurrence rather than the first, which is what lets a client walk the matches in either
// direction.
func (d *textDocument) FindText(start, end int, text string, backward, ignoreCase bool) (
	foundStart, foundEnd int, ok bool,
) {
	start, end = d.ClampRange(start, end)
	needle := []rune(text)
	if len(needle) == 0 {
		return 0, 0, false
	}
	haystack := d.runes[start:end]
	if ignoreCase {
		haystack = foldRunes(haystack)
		needle = foldRunes(needle)
	}
	i := indexRunes(haystack, needle, backward)
	if i < 0 {
		return 0, 0, false
	}
	return start + i, start + i + len(needle), true
}

// foldRunes returns the runes with each one lowercased on its own, which is what keeps the answer the same length as
// the text it came from. strings.ToLower does not: a handful of characters lowercase into more than one, and an offset
// worked out from a string of a different length would name the wrong character.
func foldRunes(runes []rune) []rune {
	folded := make([]rune, len(runes))
	for i, r := range runes {
		folded[i] = unicode.ToLower(r)
	}
	return folded
}

// indexRunes returns the index of the first occurrence of needle within haystack, or of the last when backward is
// true, or -1 when there is none.
func indexRunes(haystack, needle []rune, backward bool) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
	if backward {
		for i := len(haystack) - len(needle); i >= 0; i-- {
			if slices.Equal(haystack[i:i+len(needle)], needle) {
				return i
			}
		}
		return -1
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if slices.Equal(haystack[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

// TextClip shortens a string to the maximum length ITextRangeProvider::GetText was given, which is a count of UTF-16
// code units rather than of characters — a BSTR's length — and is -1 for no limit at all.
//
// A character that needs two code units is dropped whole rather than cut in half when only one unit is left: half of a
// surrogate pair is not a character, and a client handed one either shows a replacement glyph or refuses the string.
func TextClip(s string, maxLength int) string {
	if maxLength < 0 {
		return s
	}
	if maxLength == 0 {
		return ""
	}
	units := 0
	for i, r := range s {
		width := 1
		if r > 0xFFFF {
			width = 2
		}
		if units+width > maxLength {
			return s[:i]
		}
		units += width
	}
	return s
}
