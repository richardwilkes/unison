// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

// This file answers the search predicate: the pair of parameterized attributes an assistive technology asks a container
// to find things within. It is how VoiceOver's rotor lists a window's headings, links, lists and tables, and how Quick
// Nav's H, L, K and T keys jump from one to the next — without them, moving through a document means stepping over
// every element in it one at a time.
//
// The attributes are AXUIElementsForSearchPredicate and AXResultsForSearchPredicate, asked with a dictionary naming
// what to look for (AXSearchKey), where to look from (AXStartElement), which way to go (AXDirection) and how many
// answers are wanted (AXResultsLimit), optionally narrowed by text (AXSearchText). They were WebKit's private
// arrangement with VoiceOver for years and became public API in the macOS 26 SDK, which is why every name here is a
// literal string: the exported symbols exist only in that SDK, while the strings themselves are what VoiceOver has sent
// and WebKit has answered since Mac OS X 10.6 and are what a binary built against any SDK has to compare against. They
// were verified at runtime on macOS 27.
//
// The search itself is pure Go over the snapshot — axSearchOrder, axSearch and axSearchKeyMatches — so what an
// assistive technology is told can be tested without one. The one thing any of it asks AppKit is whether this system
// honors the AXHeading role, since that decides whether a heading is reported as static text and so whether it answers
// the static-text keys (see axIsStaticText); the answer is settled once per process. The Objective-C side does the
// rest: it only decodes the parameter dictionary and wraps the answer.

import (
	"slices"
	"strings"
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// The two parameterized attributes a search is asked through. The first answers the matching elements themselves, the
// second a dictionary per match, which is how a client that also wants the text range of a match asks for one.
const (
	axAttrUIElementsForSearchPredicate = "AXUIElementsForSearchPredicate"
	axAttrResultsForSearchPredicate    = "AXResultsForSearchPredicate"
)

// The keys of the dictionary a search is asked with, the two directions it may go in, and the key each entry of the
// results form holds its element under.
const (
	axSearchParamKey           = "AXSearchKey"
	axSearchParamStartElement  = "AXStartElement"
	axSearchParamStartRange    = "AXStartRange"
	axSearchParamDirection     = "AXDirection"
	axSearchParamResultsLimit  = "AXResultsLimit"
	axSearchParamText          = "AXSearchText"
	axSearchParamVisibleOnly   = "AXVisibleOnly"
	axSearchParamImmediateOnly = "AXImmediateDescendantsOnly"
	axSearchDirectionNext      = "AXDirectionNext"
	axSearchDirectionPrevious  = "AXDirectionPrevious"
	axSearchResultElementKey   = "AXSearchResultElement"
)

// The search keys. Every one VoiceOver is known to send is named, including the ones nothing in a snapshot can ever
// match, so that the list reads as the whole vocabulary rather than the part of it this adapter happens to answer.
const (
	axSearchKeyAnyType             = "AXAnyTypeSearchKey"
	axSearchKeyHeading             = "AXHeadingSearchKey"
	axSearchKeyHeadingLevel1       = "AXHeadingLevel1SearchKey"
	axSearchKeyHeadingLevel2       = "AXHeadingLevel2SearchKey"
	axSearchKeyHeadingLevel3       = "AXHeadingLevel3SearchKey"
	axSearchKeyHeadingLevel4       = "AXHeadingLevel4SearchKey"
	axSearchKeyHeadingLevel5       = "AXHeadingLevel5SearchKey"
	axSearchKeyHeadingLevel6       = "AXHeadingLevel6SearchKey"
	axSearchKeyHeadingSameLevel    = "AXHeadingSameLevelSearchKey"
	axSearchKeyLink                = "AXLinkSearchKey"
	axSearchKeyVisitedLink         = "AXVisitedLinkSearchKey"
	axSearchKeyUnvisitedLink       = "AXUnvisitedLinkSearchKey"
	axSearchKeyList                = "AXListSearchKey"
	axSearchKeyTable               = "AXTableSearchKey"
	axSearchKeyTableSameLevel      = "AXTableSameLevelSearchKey"
	axSearchKeyGraphic             = "AXGraphicSearchKey"
	axSearchKeyStaticText          = "AXStaticTextSearchKey"
	axSearchKeyPlainText           = "AXPlainTextSearchKey"
	axSearchKeyBoldFont            = "AXBoldFontSearchKey"
	axSearchKeyItalicFont          = "AXItalicFontSearchKey"
	axSearchKeyUnderline           = "AXUnderlineSearchKey"
	axSearchKeyFontChange          = "AXFontChangeSearchKey"
	axSearchKeyFontColorChange     = "AXFontColorChangeSearchKey"
	axSearchKeyStyleChange         = "AXStyleChangeSearchKey"
	axSearchKeyButton              = "AXButtonSearchKey"
	axSearchKeyControl             = "AXControlSearchKey"
	axSearchKeyCheckBox            = "AXCheckBoxSearchKey"
	axSearchKeyTextField           = "AXTextFieldSearchKey"
	axSearchKeyRadioGroup          = "AXRadioGroupSearchKey"
	axSearchKeyBlockquote          = "AXBlockquoteSearchKey"
	axSearchKeyBlockquoteSameLevel = "AXBlockquoteSameLevelSearchKey"
	axSearchKeyLandmark            = "AXLandmarkSearchKey"
	axSearchKeyArticle             = "AXArticleSearchKey"
	axSearchKeyLiveRegion          = "AXLiveRegionSearchKey"
	axSearchKeyOutline             = "AXOutlineSearchKey"
	axSearchKeyFrame               = "AXFrameSearchKey"
	axSearchKeyKeyboardFocusable   = "AXKeyboardFocusableSearchKey"
	axSearchKeyMisspelledWord      = "AXMisspelledWordSearchKey"
	axSearchKeySameType            = "AXSameTypeSearchKey"
	axSearchKeyDifferentType       = "AXDifferentTypeSearchKey"
)

// axSearchHeadingLevelKeys holds the six by-level heading keys in level order, which is how axHeadingLevelOfKey turns
// one into the level it asks for.
var axSearchHeadingLevelKeys = []string{
	axSearchKeyHeadingLevel1, axSearchKeyHeadingLevel2, axSearchKeyHeadingLevel3,
	axSearchKeyHeadingLevel4, axSearchKeyHeadingLevel5, axSearchKeyHeadingLevel6,
}

var (
	axSearchStringsOnce sync.Once
	axSearchAttrIDs     []objc.ID
	axSearchResultKeyID objc.ID
)

// axSearchQuery is one search an assistive technology asked for: what to look for, where to look from, which way to go
// and how many answers are wanted. It is the platform-neutral form of the parameter dictionary, so the search can be
// worked out — and tested — without AppKit.
type axSearchQuery struct {
	// Text narrows the answers to the nodes whose spoken text contains it, compared without regard to case.
	Text string
	// Keys holds the search keys, any one of which matching is enough. It is empty when the predicate names none, which
	// is the same thing as asking for anything at all.
	Keys []string
	// Start is the node the search runs from, which is never itself an answer. It is zero when the predicate names no
	// start element, and the search then begins at the first node of the scope — or, going backwards, at the last.
	Start accessibility.NodeID
	// StartOffset is how far into the start element's own text the search begins, as a rune index, and means anything
	// only while StartRange is true. It is what the VoiceOver cursor parked inside a paragraph has: a position within a
	// run of text rather than a whole element.
	StartOffset int
	// Limit is the greatest number of answers to give back; zero asks for all of them.
	Limit int
	// StartRange reports that the predicate named an AXStartRange, and with it the position StartOffset holds.
	StartRange bool
	// Previous reverses the search: the answers are the matches before the start element, nearest first.
	Previous bool
	// ImmediateOnly confines the search to the scope's own presented children instead of its whole subtree.
	ImmediateOnly bool
	// VisibleOnly leaves out the nodes that are scrolled or clipped out of sight.
	VisibleOnly bool
}

// axSearchOrder returns the nodes within a scope in the order an assistive technology moving forward through the window
// reaches them: the presented hierarchy walked pre-order, which is the same order the VoiceOver cursor travels in and
// the same one accessibilityChildren hands each level over in. The scope itself is not among them, since a search
// answers with what is inside the container it was asked of.
//
// Presented rather than raw: an ignored node is spliced out and its children take its place, a separator is dropped,
// and a cell holding one thing is stood in for by that thing, exactly as axPresentedChildren decides for every other
// answer this adapter gives. A search that walked the snapshot's own children instead would hand a client elements that
// appear nowhere in the hierarchy it can see.
func axSearchOrder(t *accessibility.Tree, scope accessibility.NodeID, immediateOnly bool) []accessibility.NodeID {
	if t == nil {
		return nil
	}
	if immediateOnly {
		return axPresentedChildren(t, scope)
	}
	return axAppendSearchOrder(nil, t, scope, map[accessibility.NodeID]bool{scope: true}, 0)
}

// axAppendSearchOrder appends the presented descendants of a node to ids, depth first. visited holds the nodes already
// reached, and the depth is bounded by axMaxPresentedDepth: between them a malformed snapshot — one whose children form
// a cycle — cannot make the walk run on forever, which matters here because this runs inside an AppKit callback where a
// hang is the whole application hanging.
func axAppendSearchOrder(ids []accessibility.NodeID, t *accessibility.Tree, id accessibility.NodeID,
	visited map[accessibility.NodeID]bool, depth int,
) []accessibility.NodeID {
	if depth >= axMaxPresentedDepth {
		return ids
	}
	for _, childID := range axPresentedChildren(t, id) {
		if visited[childID] {
			continue
		}
		visited[childID] = true
		ids = append(ids, childID)
		ids = axAppendSearchOrder(ids, t, childID, visited, depth+1)
	}
	return ids
}

// axSearch returns the nodes within a scope that answer a query, in the order the query asks for them: forwards from
// the start element, or backwards from it with the nearest match first. The start element is never an answer, which is
// what makes repeating the same search step through the matches rather than stand still; a start element the scope does
// not hold is the same as naming none, so the search begins at whichever end the direction starts from.
//
// A query that also names a position inside the start element's own text — which is what the VoiceOver cursor parked
// inside a paragraph has — begins at that position rather than at the element: the nodes the element's text carries
// before it are already behind the cursor, so they are no answer. See axSpansPassed.
func axSearch(t *accessibility.Tree, scope accessibility.NodeID, q axSearchQuery) []accessibility.NodeID {
	if t == nil {
		return nil
	}
	order := axSearchOrder(t, scope, q.ImmediateOnly)
	if q.Previous {
		slices.Reverse(order)
	}
	from := 0
	if q.Start != 0 {
		if i := slices.Index(order, q.Start); i >= 0 {
			from = i + 1
		}
	}
	start := t.Node(q.Start)
	text := strings.ToLower(q.Text)
	passed := axSpansPassed(start, q)
	var found []accessibility.NodeID
	for _, id := range order[from:] {
		n := t.Node(id)
		// The start element is already behind the slice above; naming it again here is what keeps a malformed snapshot —
		// one that manages to present the same node twice — from answering with the very element the search began at.
		if n == nil || id == q.Start {
			continue
		}
		if passed[id] {
			continue
		}
		if q.VisibleOnly && n.Offscreen {
			continue
		}
		if text != "" && !axSearchTextMatches(n, text) {
			continue
		}
		if !axSearchKeysMatch(t, n, start, q.Keys) {
			continue
		}
		found = append(found, id)
		if q.Limit > 0 && len(found) >= q.Limit {
			break
		}
	}
	return found
}

// axSearchKeysMatch reports whether a node answers any of a query's keys. A query naming none matches everything, which
// is what a search asked with AXSearchText alone means and what AXAnyTypeSearchKey says explicitly.
func axSearchKeysMatch(t *accessibility.Tree, n, start *accessibility.Node, keys []string) bool {
	if len(keys) == 0 {
		return true
	}
	for _, key := range keys {
		if axSearchKeyMatches(t, n, start, key) {
			return true
		}
	}
	return false
}

// axSearchTextMatches reports whether an AXSearchText filter matches a node. The text is already lowered, since one
// search compares it against every node of a scope.
func axSearchTextMatches(n *accessibility.Node, lowered string) bool {
	for _, candidate := range axSearchTextsOf(n) {
		if candidate != "" && strings.Contains(strings.ToLower(candidate), lowered) {
			return true
		}
	}
	return false
}

// axSearchTextsOf returns the strings an AXSearchText filter is compared against: everything an assistive technology
// would be heard saying about the node, each of which is a reason a person searching would expect to be brought to it.
//
// All of them, rather than the first that happens to be there: a field named "Author" holding "Ada Lovelace" is spoken
// as both, so a search for either has to find it, and WebKit searches the description as well. The name is left out for
// a node whose name is reported nowhere — a paragraph or a code block, whose content is their value instead (see
// axHasLabel) — unless the node reports it as that value for want of anything else, since a string spoken nowhere would
// answer with a node for a reason the person searching cannot hear.
func axSearchTextsOf(n *accessibility.Node) []string {
	texts := make([]string, 0, 4)
	if axHasLabel(n) || n.Text == nil && n.Value == "" {
		texts = append(texts, n.Name)
	}
	if n.Text != nil {
		texts = append(texts, n.Text.Text)
	}
	return append(texts, n.Value, n.Description)
}

// axSpansPassed returns the nodes a search asked with a start range has already gone past: the nodes the start
// element's own text carries that end at or before the position the range names, which are behind the cursor the search
// was asked from.
//
// It is what keeps Quick Nav moving forwards. With the VoiceOver cursor inside a paragraph past an inline link, the
// next node in presented order after the paragraph is that very link, so a search that began at the paragraph as a
// whole would answer with the link the cursor has already read and the cursor would jump backwards.
//
// Only the forward direction has anything to skip: going backwards, the start element's own text is walked before the
// element itself and so lies before where the search begins, which leaves a backwards search from a position inside a
// paragraph answering as if it had been asked from the paragraph's start — the same whole-element reading this key
// exists to refine, in the one direction VoiceOver's own cursor does not depend on it.
func axSpansPassed(start *accessibility.Node, q axSearchQuery) map[accessibility.NodeID]bool {
	if !q.StartRange || start == nil || start.Text == nil {
		return nil
	}
	var passed map[accessibility.NodeID]bool
	for _, span := range start.Text.Spans {
		if span.End <= q.StartOffset {
			if passed == nil {
				passed = make(map[accessibility.NodeID]bool, len(start.Text.Spans))
			}
			passed[span.Node] = true
		}
	}
	return passed
}

// axSearchKeyMatches reports whether one node answers one search key.
//
// The keys are WebKit's vocabulary, and each is answered from what a snapshot actually records. Several of them ask
// about something no snapshot carries and are answered false rather than approximated: AXVisitedLinkSearchKey, because
// nothing here tracks where a person has been, which also makes every link an unvisited one; AXMisspelledWordSearchKey,
// because nothing marks spelling; AXFontColorChangeSearchKey, because a run records no color; and
// AXLandmarkSearchKey/AXArticleSearchKey/AXLiveRegionSearchKey, because the document vocabulary has no counterpart of
// an ARIA landmark, article or live region. An unknown key is false for the same reason: a client asking for something
// that cannot be described is better told there is none of it than handed the wrong thing.
//
// The four relative keys — same heading level, same table level, same and different type, a change of font or style —
// compare against the start element, so they are false when the predicate named none: there is nothing for them to be
// relative to.
//
//nolint:gocognit // one case per search key; splitting the table apart would only obscure it
func axSearchKeyMatches(t *accessibility.Tree, n, start *accessibility.Node, key string) bool {
	if level := axHeadingLevelOfKey(key); level > 0 {
		return n.Role == role.Heading && n.Level == level
	}
	switch key {
	case axSearchKeyAnyType:
		return true
	case axSearchKeyHeading:
		return n.Role == role.Heading
	case axSearchKeyHeadingSameLevel:
		return n.Role == role.Heading && start != nil && start.Role == role.Heading && n.Level == start.Level
	case axSearchKeyLink, axSearchKeyUnvisitedLink:
		return n.Role == role.Link
	case axSearchKeyVisitedLink, axSearchKeyLandmark, axSearchKeyArticle, axSearchKeyLiveRegion,
		axSearchKeyMisspelledWord, axSearchKeyFontColorChange:
		// The keys nothing in a snapshot can answer; see the comment above for what each of them asks about.
		return false
	case axSearchKeyList:
		return n.Role == role.List
	case axSearchKeyTable:
		return n.Role == role.Table
	case axSearchKeyTableSameLevel:
		return n.Role == role.Table && start != nil && start.Role == role.Table &&
			axNestingLevelOf(t, n, role.Table) == axNestingLevelOf(t, start, role.Table)
	case axSearchKeyGraphic:
		return n.Role == role.Image
	case axSearchKeyStaticText:
		return axIsStaticText(n)
	case axSearchKeyPlainText:
		// Plain text is static text drawn in one unremarkable style throughout, which is what a client stepping through
		// a document by "plain text" is trying to skip to: the prose between the bold runs, the links and the code.
		return axIsStaticText(n) && !axHasRun(n, axRunIsStyled)
	case axSearchKeyBoldFont:
		return axHasRun(n, func(run *accessibility.TextRun) bool { return run.Weight >= axBoldWeight })
	case axSearchKeyItalicFont:
		return axHasRun(n, func(run *accessibility.TextRun) bool { return run.Italic })
	case axSearchKeyUnderline:
		return axHasRun(n, func(run *accessibility.TextRun) bool { return run.Underline })
	case axSearchKeyFontChange:
		return axFontDiffers(n, start)
	case axSearchKeyStyleChange:
		return axStyleDiffers(n, start)
	case axSearchKeyButton:
		// Everything a person presses, which is what WebKit's own button test answers with rather than only the roles
		// axRoleFor reports as AXButton: a button, a column header (a button with the sort subrole), a pop-up button —
		// an AXPopUpButton, which would otherwise be reachable by no key but AXControlSearchKey, so VoiceOver's
		// "Buttons" rotor and Quick Nav's B skipped every one of them — and a toggle button, which is pressed like a
		// button whatever the check box role it is reported with. A toggle button answers the check box key as well,
		// since that is what an assistive technology is told it is.
		return n.Role == role.Button || n.Role == role.ColumnHeader || n.Role == role.PopupButton ||
			n.Role == role.ToggleButton
	case axSearchKeyCheckBox:
		// Both roles axRoleFor reports as AXCheckBox; a toggle button is one with the toggle subrole.
		return n.Role == role.CheckBox || n.Role == role.ToggleButton
	case axSearchKeyTextField:
		// Everything a person types into, rather than only the role reported as AXTextField: a text area, a combo box
		// and a spin button are all fields to someone moving through a window looking for somewhere to type, and each of
		// them answers the whole text protocol.
		return n.Role == role.TextField || n.Role == role.TextArea || n.Role == role.ComboBox ||
			n.Role == role.SpinButton
	case axSearchKeyRadioGroup:
		return axHoldsRadioButtons(t, n)
	case axSearchKeyControl:
		return axIsControl(n)
	case axSearchKeyBlockquote:
		return n.Role == role.BlockQuote
	case axSearchKeyBlockquoteSameLevel:
		return n.Role == role.BlockQuote && start != nil && start.Role == role.BlockQuote &&
			axNestingLevelOf(t, n, role.BlockQuote) == axNestingLevelOf(t, start, role.BlockQuote)
	case axSearchKeyOutline:
		return n.Role == role.Tree
	case axSearchKeyFrame:
		// A frame is the container a body of text lives in, which is what a document is here: it is the element
		// VoiceOver moves between when a window holds more than one of them.
		return n.Role == role.Document
	case axSearchKeyKeyboardFocusable:
		// The same rule setAccessibilityFocused: is offered by, so what a client is told it can reach with the keyboard
		// is exactly what it will be allowed to focus.
		return n.Actions.Has(accessibility.Focus)
	case axSearchKeySameType:
		return start != nil && axSearchTypeOf(n) == axSearchTypeOf(start)
	case axSearchKeyDifferentType:
		return start != nil && axSearchTypeOf(n) != axSearchTypeOf(start)
	default:
		return false
	}
}

// axHeadingLevelOfKey returns the heading level a by-level search key asks for, and 0 for every other key.
func axHeadingLevelOfKey(key string) int {
	return slices.Index(axSearchHeadingLevelKeys, key) + 1
}

// axIsStaticText reports whether a node is presented as static text, which is decided from what axRoleFor actually
// reports for it rather than from a list of roles: a label, and the paragraphs and code blocks a document is made of,
// and a heading as well on a system where the AXHeading role is not honored, since axHeadingRole falls back to static
// text there. An assistive technology on such a system sees that heading as one more block of text, so the static-text
// key, the plain-text key and the same-type key all have to agree with it — a heading a client is shown as AXStaticText
// while the static-text rotor refuses it is the drift between what is advertised and what is answered that every other
// answer here is written to avoid.
func axIsStaticText(n *accessibility.Node) bool {
	switch n.Role {
	case role.Label, role.Paragraph, role.Code:
		return true
	case role.Heading:
		return axHeadingIsStaticText()
	default:
		return false
	}
}

// axSearchTypeOf returns the kind of thing a node is presented as, which is what the same-type and different-type keys
// compare. The static text family is folded together, since a paragraph, a code block and a label are one kind of
// element on this platform whatever the snapshot calls them: moving by "same type" from a paragraph reaches the next
// block of text rather than stopping at the next node that happens to carry the identical role.
func axSearchTypeOf(n *accessibility.Node) role.Enum {
	if axIsStaticText(n) {
		return role.Label
	}
	return n.Role
}

// axHasRun reports whether any run of a node's text satisfies a predicate. A node holding no text, or text with no
// runs, has none: an unstyled stretch of text is not a bold one.
func axHasRun(n *accessibility.Node, match func(run *accessibility.TextRun) bool) bool {
	if n.Text == nil {
		return false
	}
	for i := range n.Text.Runs {
		if match(&n.Text.Runs[i]) {
			return true
		}
	}
	return false
}

// axRunIsStyled reports whether a run is drawn as anything other than plain text.
func axRunIsStyled(run *accessibility.TextRun) bool {
	return run.Weight >= axBoldWeight || run.Italic || run.Underline || run.Strikethrough
}

// axFirstRun returns the first run of a node's text, or nil for a node whose text carries none — which is what a node
// the snapshot describes without measuring its style has.
func axFirstRun(n *accessibility.Node) *accessibility.TextRun {
	if n == nil || n.Text == nil || len(n.Text.Runs) == 0 {
		return nil
	}
	return &n.Text.Runs[0]
}

// axFontDiffers reports whether a node is drawn in a different font from the start element, which is what a client
// stepping by "font change" is looking for. It compares the first run of each, since that is the font the element is
// read as beginning in, and is false when either of them says nothing about its font.
func axFontDiffers(n, start *accessibility.Node) bool {
	a, b := axFirstRun(n), axFirstRun(start)
	if a == nil || b == nil {
		return false
	}
	return a.Family != b.Family || a.Size != b.Size || a.Monospace != b.Monospace
}

// axStyleDiffers is axFontDiffers for the style rather than the face: a run counts as differing when it is bold where
// the start element is not, or italic, underlined or struck through where it is not.
func axStyleDiffers(n, start *accessibility.Node) bool {
	a, b := axFirstRun(n), axFirstRun(start)
	if a == nil || b == nil {
		return false
	}
	return (a.Weight >= axBoldWeight) != (b.Weight >= axBoldWeight) || a.Italic != b.Italic ||
		a.Underline != b.Underline || a.Strikethrough != b.Strikethrough
}

// axIsControl reports whether a node is something a person operates rather than something they read, which is what the
// control search key asks for — VoiceOver's way of listing a window's form controls.
//
// The roles are named rather than derived from whether the node can take the focus or advertises an action: a control
// is a control whether or not it is enabled, and a disabled one advertises nothing at all, while a document or a scroll
// area that can be focused is still not something a person operates. Everything named here is one of the roles
// axRoleFor reports as an AppKit control.
func axIsControl(n *accessibility.Node) bool {
	switch n.Role {
	case role.Button, role.ToggleButton, role.CheckBox, role.RadioButton, role.Link, role.PopupButton, role.ComboBox,
		role.Slider, role.SpinButton, role.TextField, role.TextArea, role.ColorWell, role.Tab, role.MenuItem,
		role.DisclosureTriangle, role.ScrollBar, role.ColumnHeader:
		return true
	default:
		return false
	}
}

// axHoldsRadioButtons reports whether a node stands in for a radio group. There is no role for one: a set of radio
// buttons is a panel holding them, so what a client asking for a radio group is offered is any presented container with
// a radio button directly inside it — the same thing ARIA's radiogroup role names, spelled the way this toolkit builds
// it.
func axHoldsRadioButtons(t *accessibility.Tree, n *accessibility.Node) bool {
	if n.Role == role.RadioButton {
		return false
	}
	for _, id := range axPresentedChildren(t, n.ID) {
		if child := t.Node(id); child != nil && child.Role == role.RadioButton {
			return true
		}
	}
	return false
}

// axElementSearchMethods returns the two overrides that let an element be searched: the one that says the search
// attributes can be asked of it, and the one that answers them.
//
// The elements are the whole of where a search is answered, and that is a deliberate decision rather than an omission.
// What the superclass lists is preserved, but it lists nothing: a UnisonAXElement asked for
// accessibilityParameterizedAttributeNames gets an empty array from NSAccessibilityElement even with every method of
// the text protocol implemented, and the legacy accessibilityAttributeValue:forParameter: answers nil for
// AXStringForRange — both verified at runtime on macOS 27. The text protocol's own parameterized attributes reach a
// client all the same, because AppKit merges the list it derives from the modern protocol methods with whatever this
// selector answers: an AXUIElement client in another process sees a paragraph list AXStringForRange,
// AXAttributedStringForRange, AXRangeForLine, AXLineForIndex, AXBoundsForRange, AXRangeForIndex and AXRangeForPosition
// alongside the two search attributes, and answering each of them lands in the methods of axElementTextMethods. So
// nothing has to be listed here twice; see TestAXParameterizedAttributeNamesIncludeSearch, which pins both halves of
// that.
//
// The other receivers a client could conceivably ask are the content view and the window, and neither is answered. The
// content view answers isAccessibilityElement with false so that it is spliced out of the hierarchy (see
// view_darwin.go), and AppKit takes it at its word: the same client sees the window's children as the document and the
// panel of controls themselves, with no element standing for the view, so an override on it could never be reached. The
// window is an element, and it reports both search attributes unsupported; answering them there would mean guessing
// that VoiceOver asks a window rather than the element its cursor is on, which nothing on this side of the API can find
// out without a screen reader to watch, so it is deliberately not guessed at. Every element answering for its own
// subtree is what VoiceOver's rotor and Quick Nav actually walk.
//
// Both overrides check that the superclass implements the selector before they use it, since an unrecognized selector
// raises an Objective-C exception, and one raised inside a Go callback cannot be caught and takes the process with it —
// the hazard axElementAllowedMethods documents for accessibilityIsAttributeSettable:. NSAccessibilityElement implements
// both of these on every macOS this has been tried on, verified at runtime; the guard is what keeps a release that
// stops implementing one from being fatal rather than merely quiet.
func axElementSearchMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{
			Cmd: Sel("accessibilityParameterizedAttributeNames"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				var names objc.ID
				if axSuperRespondsTo(axElementClass, cmd) {
					names = SendSuper(self, axElementClass, cmd)
				}
				if a, n := axElementTarget(self, cmd); a == nil || n == nil {
					// An element whose node has left the tree speaks for nothing, so there is nothing to search within
					// it either.
					return names
				}
				return axArrayByAdding(names, axSearchAttributeStrings())
			},
		},
		{
			Cmd: Sel("accessibilityAttributeValue:forParameter:"),
			Fn: func(self objc.ID, cmd objc.SEL, attribute, parameter objc.ID) objc.ID {
				name := GoStringFromNSString(attribute)
				if name != axAttrUIElementsForSearchPredicate && name != axAttrResultsForSearchPredicate {
					if axSuperRespondsTo(axElementClass, cmd) {
						return SendSuper(self, axElementClass, cmd, attribute, parameter)
					}
					return 0
				}
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return 0
				}
				return a.searchAnswer(n.ID, parameter, name == axAttrResultsForSearchPredicate)
			},
		},
	}
}

// searchAnswer answers one of the two search attributes for a scope: an NSArray of the matching elements, or, for the
// results form, an NSArray of one dictionary per match holding that element under AXSearchResultElement. A parameter
// that is not a dictionary is answered with nil, which is the only thing to say about a question that was not asked
// properly.
//
// The results carry no AXSearchResultRange. That key exists for a client searching within a run of text, and every
// answer here is a whole element; a range covering the whole of an element's own text would say nothing the element
// does not already say for itself.
func (a *AXAdapter) searchAnswer(scope accessibility.NodeID, parameter objc.ID, asResults bool) objc.ID {
	if a.tree == nil {
		return 0
	}
	q, ok := a.searchQueryFromParameter(parameter)
	if !ok {
		return 0
	}
	ids := axSearch(a.tree, scope, q)
	if axTraceOn {
		axTrace("  search scope=%d keys=%v text=%q start=%d previous=%v limit=%d answered %d", scope, q.Keys, q.Text,
			q.Start, q.Previous, q.Limit, len(ids))
	}
	if !asResults {
		return a.elementsFor(ids)
	}
	results := make([]objc.ID, 0, len(ids))
	for _, id := range ids {
		if element := a.elementFor(id); element != 0 {
			results = append(results, NSDictionaryFromPairs(axSearchResultElementString(), element))
		}
	}
	return NSArrayFromIDs(results...)
}

// searchQueryFromParameter turns the dictionary a search attribute was asked with into a query, and reports false for a
// parameter that is not a dictionary at all.
//
// Every entry is optional, and an entry holding something other than what it is documented to hold is treated as
// absent: the dictionary comes from another process, so nothing in it can be taken on trust, and sending an NSString's
// selector to something that is not one would raise inside a Go callback, which cannot be caught.
//
// AXStartRange names a position inside the start element's own text to search from, which is where the VoiceOver cursor
// sits while it is reading a paragraph. It is read as an NSValue holding a range — AppKit hands the AXValue a client
// sends over as one — and what it is worth is that the nodes the text carries before that position are not answered
// again; see axSpansPassed. AXSearchResultRange, its counterpart in the answer, is still not given back: every answer
// here is a whole element, and a range covering the whole of an element's own text would say nothing the element does
// not already say for itself.
func (a *AXAdapter) searchQueryFromParameter(parameter objc.ID) (q axSearchQuery, ok bool) {
	if parameter == 0 || !objc.Send[bool](parameter, Sel("isKindOfClass:"), Cls("NSDictionary")) {
		return axSearchQuery{}, false
	}
	q.Keys = axSearchKeysFromParameter(parameter)
	q.Text = axSearchStringEntry(parameter, axSearchParamText)
	q.Start = a.searchStartOf(NSDictionaryObjectForKey(parameter, NSStringFromGo(axSearchParamStartElement)))
	q.StartOffset, q.StartRange = a.searchStartOffsetOf(q.Start,
		NSDictionaryObjectForKey(parameter, NSStringFromGo(axSearchParamStartRange)))
	q.Previous = axSearchStringEntry(parameter, axSearchParamDirection) == axSearchDirectionPrevious
	q.Limit = int(Int64FromNSNumber(axSearchNumberEntry(parameter, axSearchParamResultsLimit)))
	if q.Limit < 0 {
		q.Limit = 0
	}
	q.VisibleOnly = BoolFromNSNumber(axSearchNumberEntry(parameter, axSearchParamVisibleOnly))
	q.ImmediateOnly = BoolFromNSNumber(axSearchNumberEntry(parameter, axSearchParamImmediateOnly))
	return q, true
}

// axSearchKeysFromParameter returns the search keys a parameter dictionary names. VoiceOver sends either a single
// string or an array of them, so both are read; anything else is no keys at all, which asks for everything.
func axSearchKeysFromParameter(parameter objc.ID) []string {
	value := NSDictionaryObjectForKey(parameter, NSStringFromGo(axSearchParamKey))
	switch {
	case value == 0:
		return nil
	case objc.Send[bool](value, Sel("isKindOfClass:"), Cls("NSString")):
		return []string{GoStringFromNSString(value)}
	case objc.Send[bool](value, Sel("isKindOfClass:"), Cls("NSArray")):
		return GoStringsFromNSArray(value)
	default:
		return nil
	}
}

// axSearchStringEntry returns the string an entry of a parameter dictionary holds, and "" when the entry is absent or
// holds something else.
func axSearchStringEntry(parameter objc.ID, key string) string {
	value := NSDictionaryObjectForKey(parameter, NSStringFromGo(key))
	if value == 0 || !objc.Send[bool](value, Sel("isKindOfClass:"), Cls("NSString")) {
		return ""
	}
	return GoStringFromNSString(value)
}

// axSearchNumberEntry returns the NSNumber an entry of a parameter dictionary holds, and 0 when the entry is absent or
// holds something else.
func axSearchNumberEntry(parameter objc.ID, key string) objc.ID {
	value := NSDictionaryObjectForKey(parameter, NSStringFromGo(key))
	if value == 0 || !objc.Send[bool](value, Sel("isKindOfClass:"), Cls("NSNumber")) {
		return 0
	}
	return value
}

// searchStartOf returns the node a search starts from: the node of one of this adapter's own elements, and zero for
// anything else, including an element of another window, whose node ids mean nothing in this snapshot. Zero is the same
// as naming no start element, so a search asked from something this window knows nothing about answers from the
// beginning of the scope rather than not at all.
func (a *AXAdapter) searchStartOf(element objc.ID) accessibility.NodeID {
	switch {
	case element == 0:
		return 0
	case axElementClass != 0 && objc.Send[bool](element, Sel("isKindOfClass:"), objc.ID(axElementClass)):
		if View(element.Send(Sel("axView"))) != a.view {
			return 0
		}
		return accessibility.NodeID(objc.Send[uint64](element, Sel("axNodeID")))
	default:
		return 0
	}
}

// searchStartOffsetOf returns the position within the start element's own text a search begins at, as a rune index, and
// false when the predicate named no usable one: an entry that is not an NSValue holding a range, a start element that
// holds no text, or no entry at all. The range arrives in the UTF-16 code units NSAccessibility works in and is
// converted to the rune index the snapshot works in, the same conversion every other offset crossing that boundary
// makes.
func (a *AXAdapter) searchStartOffsetOf(start accessibility.NodeID, value objc.ID) (offset int, ok bool) {
	r, ok := NSRangeFromNSValue(value)
	if !ok {
		return 0, false
	}
	n := a.tree.Node(start)
	if n == nil || n.Text == nil {
		return 0, false
	}
	return axRuneFromUTF16(n.Text.Text, int(r.Location)), true
}

// axSearchAttributeStrings returns the NSString for each of the two search attribute names, in the order an element
// lists them. See axSearchStrings.
func axSearchAttributeStrings() []objc.ID {
	axSearchStrings()
	return axSearchAttrIDs
}

// axSearchResultElementString returns the NSString for the key each entry of the results form holds its element under.
// See axSearchStrings.
func axSearchResultElementString() objc.ID {
	axSearchStrings()
	return axSearchResultKeyID
}

// axSearchStrings creates the NSStrings the search attributes are answered with. They are created once and never
// released, since they are handed out on every attribute-name query and with every result.
func axSearchStrings() {
	axSearchStringsOnce.Do(func() {
		axSearchAttrIDs = []objc.ID{
			NewNSString(axAttrUIElementsForSearchPredicate),
			NewNSString(axAttrResultsForSearchPredicate),
		}
		axSearchResultKeyID = NewNSString(axSearchResultElementKey)
	})
}

// axArrayByAdding returns an NSArray holding the contents of an array followed by some more objects, which is how an
// override adds to the list its superclass answered with instead of replacing it. A nil array with something to add
// yields just the additions, since a superclass that answered nothing has nothing to preserve.
func axArrayByAdding(array objc.ID, extra []objc.ID) objc.ID {
	if len(extra) == 0 {
		return array
	}
	added := NSArrayFromIDs(extra...)
	if array == 0 {
		return added
	}
	return array.Send(Sel("arrayByAddingObjectsFromArray:"), added)
}

// axSuperRespondsTo reports whether the superclass of a class registered by this package implements a selector, which
// has to be known before any SendSuper to it: the Objective-C runtime raises an unrecognized-selector exception for a
// selector nothing implements, and an exception raised inside a Go callback cannot be caught and takes the process with
// it — the same hazard axElementAllowedMethods documents for accessibilityIsAttributeSettable:.
func axSuperRespondsTo(cls objc.Class, selector objc.SEL) bool {
	super := objc.ID(cls).Send(Sel("superclass"))
	if super == 0 {
		return false
	}
	return objc.Send[bool](super, Sel("instancesRespondToSelector:"), selector)
}
