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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xhttp"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/toolbox/v2/xstrings"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/enums/slant"
	"github.com/richardwilkes/unison/enums/weight"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	astex "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// DefaultMarkdownWidth is the default maximum width to use, roughly equivalent to a page at 100dpi.
const DefaultMarkdownWidth = 8 * 100

// DefaultMarkdownTheme holds the default MarkdownTheme values for Markdown. Modifying this data will not alter existing
// Markdown, but will alter any Markdown created in the future.
var DefaultMarkdownTheme MarkdownTheme

const (
	markdownListItemKey = "unison.list.item"
	// markdownLinkHeadKey marks a piece of a link broken across rows, other than the first, with the first.
	markdownLinkHeadKey = "unison.link.head"
)

func init() {
	DefaultMarkdownTheme = MarkdownTheme{
		TextDecoration: TextDecoration{
			Font:            DefaultLabelTheme.Font,
			OnBackgroundInk: DefaultLabelTheme.OnBackgroundInk,
		},
		HeadingFont: [6]Font{
			&DynamicFont{Resolver: func() FontDescriptor { return DeriveMarkdownHeadingFont(nil, 1) }},
			&DynamicFont{Resolver: func() FontDescriptor { return DeriveMarkdownHeadingFont(nil, 2) }},
			&DynamicFont{Resolver: func() FontDescriptor { return DeriveMarkdownHeadingFont(nil, 3) }},
			&DynamicFont{Resolver: func() FontDescriptor { return DeriveMarkdownHeadingFont(nil, 4) }},
			&DynamicFont{Resolver: func() FontDescriptor { return DeriveMarkdownHeadingFont(nil, 5) }},
			&DynamicFont{Resolver: func() FontDescriptor { return DeriveMarkdownHeadingFont(nil, 6) }},
		},
		CodeBlockFont:          &DynamicFont{Resolver: func() FontDescriptor { return DeriveMarkdownCodeBlockFont(nil) }},
		CodeBackground:         ThemeAboveSurface,
		OnCodeBackground:       ThemeOnAboveSurface,
		QuoteBarColor:          ThemeFocus,
		QuoteBarNoteColor:      RGB(9, 105, 218),
		QuoteBarTipColor:       RGB(26, 127, 55),
		QuoteBarImportantColor: RGB(130, 80, 223),
		QuoteBarWarningColor:   RGB(154, 103, 0),
		QuoteBarCautionColor:   RGB(207, 34, 46),
		LinkInk:                DefaultLinkTheme.OnBackgroundInk,
		LinkOnPressedInk:       DefaultLinkTheme.OnPressedInk,
		LinkHandler:            DefaultMarkdownLinkHandler,
		QuoteBarThickness:      2,
		CodeAndQuotePadding:    6,
		Slop:                   4,
	}
}

// DeriveMarkdownHeadingFont derives a FontDescriptor for a heading from another font. Pass in nil for the font to use
// DefaultMarkdownTheme.Font.
func DeriveMarkdownHeadingFont(font Font, level int) FontDescriptor {
	var fd FontDescriptor
	if xreflect.IsNil(font) {
		fd = DefaultMarkdownTheme.Font.Descriptor()
	} else {
		fd = font.Descriptor()
	}
	fd.Weight = weight.Bold
	switch level {
	case 1:
		fd.Size *= 2
	case 2:
		fd.Size *= 1.5
	case 3:
		fd.Size *= 1.25
	case 4:
		fd.Size *= 1
	case 5:
		fd.Size *= 0.875
	default:
		fd.Size *= 0.85
	}
	return fd
}

// DeriveMarkdownCodeBlockFont derives a FontDescriptor for code from another font. Pass in nil for the font to use
// MonospacedFont.
func DeriveMarkdownCodeBlockFont(font Font) FontDescriptor {
	var fd FontDescriptor
	if xreflect.IsNil(font) {
		fd = MonospacedFont.Descriptor()
	} else {
		fd = font.Descriptor()
	}
	fd.Size = DefaultMarkdownTheme.Font.Size()
	return fd
}

// MarkdownTheme holds theming data for a Markdown.
type MarkdownTheme struct {
	TextDecoration
	HeadingFont            [6]Font
	CodeBlockFont          Font
	CodeBackground         Ink
	OnCodeBackground       Ink
	QuoteBarColor          Ink
	QuoteBarNoteColor      Ink
	QuoteBarTipColor       Ink
	QuoteBarImportantColor Ink
	QuoteBarWarningColor   Ink
	QuoteBarCautionColor   Ink
	LinkInk                Ink
	LinkOnPressedInk       Ink
	LinkHandler            func(Paneler, string)
	WorkingDirProvider     func(Paneler) string
	AltLinkPrefixes        []string
	QuoteBarThickness      float32
	CodeAndQuotePadding    float32
	Slop                   float32
	StripBottomEmptyMargin bool
}

// HasAnyPrefix returns true if the target has a prefix matching one of those found in prefixes.
func HasAnyPrefix(prefixes []string, target string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
}

type drawableCacheEntry struct {
	drawable Drawable
	targets  []markdownImageTarget
}

// markdownImageTarget is a placeholder waiting for an image, and the widths constrainImage is to keep the image between
// there.
type markdownImageTarget struct {
	panel    *DrawablePanel
	maxWidth float32
	minWidth float32
}

// Markdown provides markdown display widget.
//
// It has no contextual menu of its own. An application may set ContextMenuCallback; the menu then opens beneath the
// reading caret from the keyboard or at an assistive technology's request (see ContextMenuAnchor). The text is drawn
// with panels of the document's own, which have no menu, and only the panel under the pointer is asked for one, so a
// right-click on the text is an ordinary press; only a right-click on a part of the document no block covers opens the
// menu.
type Markdown struct {
	HTTPClient                 *http.Client // Used when retrieving data from a remote host
	lastParent                 *Panel
	block                      *Panel
	textRow                    *Panel
	text                       *Text
	decoration                 *TextDecoration
	node                       ast.Node
	chainedFrameChangeCallback func()
	content                    []byte
	// columnWidths is how wide each column of the table being built is made, its cells' borders and padding included.
	// Zero makes a column as narrow as its content allows.
	columnWidths  []int
	drawableCache map[string]*drawableCacheEntry
	anchors       map[string]*Panel
	// axDoc is the content composed into one stream of text, built on demand and thrown away whenever anything that
	// would move a rune of it changes. See Markdown.axDocument.
	axDoc *axDocument
	MarkdownTheme
	Panel
	// axGeneration counts how many times the content has moved, which is what the text each block within it worked out
	// for itself is thrown away by. See Markdown.axInvalidate.
	axGeneration      uint64
	drawableCacheLock sync.Mutex
	PerImageByteLimit int64 // Used when retrieving data from a remote host
	index             int
	columnIndex       int
	// tableRow is the row the cells being built belong to, while a table is being built.
	tableRow int
	alert    int
	// caret is where the reading caret sits within the composed stream, and anchor is the end of the selection it was
	// made from. They are equal when nothing is selected. See Markdown.axSetSelection.
	caret  int
	anchor int
	// axWordStart and axWordEnd are the word a double-click landed on, which a drag from it extends rather than
	// shrinking back into. They mean nothing unless axWordDrag is set.
	axWordStart  int
	axWordEnd    int
	maxWidth     float32
	maxLineWidth float32
	// bulletWidth is the width every bullet of the list being built is given. See Markdown.listBulletWidth.
	bulletWidth float32
	ordered     bool
	isHeader    bool
	// axWordDrag reports that the gesture in progress began with a double-click, so it selects whole words.
	axWordDrag bool
}

// NewMarkdown creates a new markdown widget. If autoSizingFromParent is true, then the Markdown will attempt to keep
// its content wrapped to its parent's width. Currently, things like tables don't play nice with width management.
//
// An application that installs a FrameChangeInChildHierarchyCallback of its own on the result must call
// Markdown.DefaultFrameChangeInChildHierarchy from it, or chain whatever this installed: everything the text of the
// document says about itself is read from where the panels holding it were placed, and that callback is the only thing
// that throws it away when they move. Without it a caret, a highlight and the lines a screen reader is told about all
// keep the places they had before the last layout.
func NewMarkdown(autoSizingFromParent bool) *Markdown {
	m := &Markdown{
		MarkdownTheme: DefaultMarkdownTheme,
		drawableCache: make(map[string]*drawableCacheEntry),
		anchors:       make(map[string]*Panel),
	}
	m.SetLayout(&FlexLayout{Columns: 1})
	m.Self = m
	// The content is rich, readable text rather than a set of controls, which is what a document is. Everything the
	// markdown is built out of is a panel in its own right, so there is nothing else to tell an assistive technology
	// here: the headings, links and images below say what they are as they are created.
	m.Accessibility.Role = role.Document
	// A document takes the keyboard focus while a screen reader is running, on the platforms where the screen reader
	// begins from the focus: with nothing in a window holding it, Narrator's cursor stays on the window's own element,
	// from which its scan mode, heading and link navigation refuse to move into the content. Nothing changes for anyone
	// else: a document has never been a tab stop, and it draws no differently for holding the focus. See
	// Panel.axTakesFocus.
	m.axFocusable = true
	// Everything the text of the document says about itself — which runes are on which line, and where each of them is
	// on the screen — is read from where the panels holding it were placed, so all of it is thrown away whenever any of
	// them moves. Whatever was already installed is chained rather than dropped, since a caller that set a callback
	// before the markdown was handed back is entitled to go on being told. See DefaultFrameChangeInChildHierarchy.
	m.FrameChangeInChildHierarchyCallback = m.DefaultFrameChangeInChildHierarchy
	// The reading caret: the keys that move it, the mouse that places it, the drawing of it, and the two commands that
	// act on what it has selected. All of them do nothing while the markdown is not focusable, which keeps an ordinary
	// markdown — one no screen reader is watching and no application has opted in for — exactly as it was.
	m.KeyDownCallback = m.DefaultKeyDown
	m.MouseDownCallback = m.DefaultMouseDown
	m.MouseDragCallback = m.DefaultMouseDrag
	m.MouseUpCallback = m.DefaultMouseUp
	m.DrawOverCallback = m.DefaultDrawOver
	// The caret and the selection are drawn only while the document holds the focus, so the focus arriving and leaving
	// are both things the document is drawn differently for. Nothing else repaints for them: the window marks itself
	// for redraw on a focus change only while an assistive technology is being served, which the application that opted
	// in by making its markdown focusable need not have. See Markdown.DefaultDrawOver and Window.axMarkForPublish.
	m.GainedFocusCallback = m.DefaultFocusGained
	m.LostFocusCallback = m.DefaultFocusLost
	m.InstallCmdHandlers(CopyItemID, func(_ any) bool { return m.CanCopy() }, func(_ any) { m.Copy() })
	m.InstallCmdHandlers(SelectAllItemID, func(_ any) bool { return m.CanSelectAll() }, func(_ any) { m.SelectAll() })
	if autoSizingFromParent {
		m.ParentChangedCallback = m.adjustSizeOnParentChange
	}
	return m
}

// DefaultFrameChangeInChildHierarchy provides the default frame change in child hierarchy handling, which is throwing
// away everything worked out from where the panels drawing the content were placed. A layout moves hundreds of frames
// and this is called for each of them, so it does no more than mark what was worked out as no longer current; it is
// worked out again the next time something asks. See Markdown.axInvalidate.
func (m *Markdown) DefaultFrameChangeInChildHierarchy(_ *Panel) {
	m.axInvalidate()
}

// DefaultFocusGained provides the default focus gained handling, which is redrawing so that the reading caret appears.
func (m *Markdown) DefaultFocusGained() {
	m.MarkForRedraw()
}

// DefaultFocusLost provides the default focus lost handling, which is redrawing so that the reading caret and whatever
// it had selected stop being drawn.
func (m *Markdown) DefaultFocusLost() {
	m.MarkForRedraw()
}

func (m *Markdown) adjustSizeOnParentChange() {
	if p := m.Parent(); p != m.lastParent {
		if m.lastParent != nil {
			m.lastParent.FrameChangeCallback = m.chainedFrameChangeCallback
			m.lastParent = nil
		}
		if p != nil {
			m.lastParent = p
			m.chainedFrameChangeCallback = p.FrameChangeCallback
			p.FrameChangeCallback = m.adjustToParent
		}
	}
}

func (m *Markdown) adjustToParent() {
	m.SetContentBytes(m.content, 0)
	SafeCall(m.chainedFrameChangeCallback)
}

// SetContent replaces the current markdown content.
func (m *Markdown) SetContent(content string, maxWidth float32) {
	m.SetContentBytes([]byte(content), maxWidth)
}

// ContentBytes returns the current markdown content as a byte slice.
func (m *Markdown) ContentBytes() []byte {
	return m.content
}

// SetContentBytes replaces the current markdown content. If maxWidth < 1, then the content will be sized based on the
// parent container or use DefaultMarkdownWidth if no parent is present.
func (m *Markdown) SetContentBytes(content []byte, maxWidth float32) {
	if maxWidth < 1 {
		if p := m.Parent(); p != nil {
			maxWidth = p.ContentRect(false).Width - m.Slop
			if border := m.Border(); border != nil {
				insets := border.Insets()
				maxWidth -= insets.Width()
			}
		} else {
			maxWidth = DefaultMarkdownWidth
		}
	}
	if m.maxWidth == maxWidth && bytes.Equal(m.content, content) {
		return
	}
	if !bytes.Equal(m.content, content) {
		// Different content is a different document, so the reading caret goes back to the top of it. The same content
		// laid out to a different width is the same document re-wrapped, and the caret stays where it was in it —
		// clamped, since the stream it is an offset into is rebuilt from the same runes.
		m.caret = 0
		m.anchor = 0
	}
	m.axInvalidate()
	m.RemoveAllChildren()
	m.maxWidth = maxWidth
	m.maxLineWidth = maxWidth
	m.content = content
	m.block = m.AsPanel()
	m.textRow = nil
	m.text = nil
	m.decoration = m.Clone()
	m.index = 0
	m.ordered = false
	m.anchors = make(map[string]*Panel)
	// extension.GFM, but with a task list check box that has to be followed by whitespace, as GFM requires. See
	// markdownTaskCheckBoxParser.
	m.node = goldmark.New(goldmark.WithExtensions(extension.Linkify, extension.Table, extension.Strikethrough),
		goldmark.WithParserOptions(parser.WithAutoHeadingID(), parser.WithHeadingAttribute(),
			parser.WithInlineParsers(util.Prioritized(markdownTaskCheckBoxParser{
				InlineParser: extension.NewTaskCheckBoxParser(),
			}, 0)))).
		Parser().Parse(text.NewReader(m.content))
	m.walk(m.node)
	if m.StripBottomEmptyMargin && len(m.children) > 0 {
		if border := m.children[len(m.children)-1].Border(); border != nil {
			switch b := border.(type) {
			case *EmptyBorder:
				b.insets.Bottom = 0
			case *CompoundBorder:
				if len(b.borders) != 0 {
					if eb, ok := b.borders[0].(*EmptyBorder); ok {
						eb.insets.Bottom = 0
					}
				}
			}
		}
	}
	m.MarkForLayoutAndRedraw()
}

// Rebuild rebuilds the markdown content. This is useful if the theme has been changed.
func (m *Markdown) Rebuild() {
	maxWidth := m.maxWidth
	m.maxWidth = -1
	content := m.content
	m.content = nil
	m.SetContentBytes(content, maxWidth)
}

func (m *Markdown) walk(node ast.Node) {
	save := m.node
	m.node = node
	switch m.node.Kind() {
	// Block types
	case ast.KindDocument:
		m.processChildren()
	case ast.KindTextBlock, ast.KindParagraph:
		m.processParagraphOrTextBlock()
	case ast.KindHeading:
		m.processHeading()
	case ast.KindThematicBreak:
		m.processThematicBreak()
	case ast.KindCodeBlock, ast.KindFencedCodeBlock:
		m.processCodeBlock()
	case ast.KindBlockquote:
		m.processBlockquote()
	case ast.KindList:
		m.processList()
	case ast.KindListItem:
		m.processListItem()
	case ast.KindHTMLBlock, ast.KindLinkReferenceDefinition:
		// Ignore
	case astex.KindTable:
		m.processTable()
	case astex.KindTableHeader:
		m.processTableHeader()
	case astex.KindTableRow:
		m.processTableRow()
	case astex.KindTableCell:
		m.processTableCell()

	// Inline types
	case ast.KindText:
		m.processText()
	case ast.KindEmphasis:
		m.processEmphasis()
	case ast.KindCodeSpan:
		m.processCodeSpan()
	case ast.KindRawHTML:
		m.processRawHTML()
	case ast.KindString:
		m.processString()
	case ast.KindLink:
		m.processLink()
	case ast.KindImage:
		m.processImage()
	case ast.KindAutoLink:
		m.processAutoLink()
	case astex.KindStrikethrough:
		m.processStrikethrough()
	case astex.KindTaskCheckBox:
		// Shown in place of the item's bullet; see Markdown.createListBullet.

	default:
		errs.Log(errs.New("unhandled markdown element"), "kind", m.node.Kind())
	}
	m.node = save
}

func (m *Markdown) processChildren() {
	for child := m.node.FirstChild(); child != nil; child = child.NextSibling() {
		m.walk(child)
	}
}

func (m *Markdown) processParagraphOrTextBlock() {
	p := newMarkdownBlock(m, role.Paragraph).AsPanel()
	p.SetBorder(NewEmptyBorder(m.stdBottomMargin()))
	save := m.block
	m.block.AddChild(p)
	m.block = p
	m.text = NewText("", m.decoration)
	m.processChildren()
	m.finishTextRow()
	m.block = save
}

func (m *Markdown) stdBottomMargin() geom.Insets {
	return geom.Insets{Bottom: m.Font.Baseline() / 2}
}

func (m *Markdown) processHeading() {
	if heading, ok := m.node.(*ast.Heading); ok {
		saveDec := m.decoration
		saveBlock := m.block
		level := min(max(heading.Level, 1), 6)
		m.decoration = m.decoration.Clone()
		m.decoration.Font = m.HeadingFont[level-1]
		block := newMarkdownBlock(m, role.Heading)
		block.level = level
		p := block.AsPanel()
		insets := m.stdBottomMargin()
		insets.Top = m.collapseMarginWithPrevious(m.decoration.Font.Baseline())
		if len(m.block.Children()) == 0 {
			// Nothing above it to keep apart from in the document, list item or quote it begins, where a margin would
			// leave an item's bullet beside the space above the heading's text.
			insets.Top = 0
		}
		if heading.Level == 1 || heading.Level == 2 {
			insets.Bottom = 0
		}
		p.SetBorder(NewEmptyBorder(insets))
		m.block.AddChild(p)
		if id, hasID := heading.AttributeString("id"); hasID {
			if idBytes, isBytes := id.([]byte); isBytes {
				m.anchors[string(idBytes)] = p
			}
		}
		m.block = p
		m.text = NewText("", m.decoration)
		m.processChildren()
		m.finishTextRow()
		m.decoration = saveDec
		m.block = saveBlock
		if heading.Level == 1 || heading.Level == 2 {
			hr := NewSeparator()
			hr.SetLayoutData(&FlexLayoutData{
				HGrab:  true,
				HAlign: align.Fill,
				VAlign: align.Middle,
			})
			hr.SetBorder(NewEmptyBorder(m.stdBottomMargin()))
			m.block.AddChild(hr)
		}
	}
}

// markdownHasInlineElement reports whether anything within the panel is something an assistive technology must be able
// to reach in its own right — a link or an image — rather than a piece of the text around it.
func markdownHasInlineElement(p *Panel) bool {
	for _, child := range p.Children() {
		if child.Accessibility.Role == role.Link {
			return true
		}
		if _, ok := child.Self.(*DrawablePanel); ok {
			return true
		}
		if markdownHasInlineElement(child) {
			return true
		}
	}
	return false
}

func (m *Markdown) collapseMarginWithPrevious(desired float32) float32 {
	children := m.block.Children()
	if len(children) == 0 {
		return desired
	}
	if border := children[len(children)-1].Border(); border != nil {
		desired -= border.Insets().Bottom
		if desired < 0 {
			desired = 0
		}
	}
	return desired
}

func (m *Markdown) processThematicBreak() {
	hr := NewSeparator()
	hr.SetLayoutData(&FlexLayoutData{
		HGrab:  true,
		HAlign: align.Fill,
		VAlign: align.Middle,
	})
	insets := m.stdBottomMargin()
	insets.Top = m.collapseMarginWithPrevious(insets.Bottom)
	if _, isItem := m.block.ClientData()[markdownListItemKey]; !isItem && len(m.block.Children()) == 0 {
		// Nothing above it to keep apart from in the document or quote it begins. In a list item, the margin keeps it
		// level with the middle of the bullet.
		insets.Top = 0
	}
	hr.SetBorder(NewEmptyBorder(insets))
	m.block.AddChild(hr)
}

func (m *Markdown) processCodeBlock() {
	saveDec := m.decoration
	saveBlock := m.block
	saveMaxLineWidth := m.maxLineWidth
	m.decoration = m.decoration.Clone()
	m.decoration.Font = m.CodeBlockFont
	m.decoration.OnBackgroundInk = m.OnCodeBackground
	m.maxLineWidth -= m.CodeAndQuotePadding * 2

	// The padded panel the background is drawn behind is the block: it is what holds the lines of code, so it is what
	// an assistive technology reads the code as. The wrapper around it exists only for the margin below it.
	block := newMarkdownBlock(m, role.Code)
	p := block.AsPanel()
	p.DrawCallback = func(gc *Canvas, rect geom.Rect) {
		paint := m.CodeBackground.Paint(gc, rect, paintstyle.Fill)
		gc.DrawRect(rect, paint)
	}
	p.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	p.SetBorder(NewEmptyBorder(geom.NewUniformInsets(m.CodeAndQuotePadding)))
	wrapper := NewPanel()
	wrapper.Accessibility.Role = role.None
	wrapper.SetLayout(&FlexLayout{Columns: 1})
	wrapper.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	wrapper.SetBorder(NewEmptyBorder(m.stdBottomMargin()))
	wrapper.AddChild(p)
	m.block.AddChild(wrapper)
	m.block = p
	lines := m.node.Lines()
	count := lines.Len()
	for i := range count {
		segment := lines.At(i)
		label := NewLabel()
		label.Accessibility.Role = role.None
		label.Text = NewText(string(bytes.TrimRight(segment.Value(m.content), "\n")), m.decoration)
		p.AddChild(label)
	}
	m.text = nil
	m.textRow = nil
	m.decoration = saveDec
	m.block = saveBlock
	m.maxLineWidth = saveMaxLineWidth
}

func (m *Markdown) processBlockquote() {
	saveDec := m.decoration
	saveBlock := m.block
	saveMaxLineWidth := m.maxLineWidth
	m.decoration = m.decoration.Clone()
	m.decoration.OnBackgroundInk = m.OnCodeBackground
	m.maxLineWidth -= m.QuoteBarThickness + m.CodeAndQuotePadding*2

	quote := newMarkdownContainer(role.BlockQuote)
	p := quote.AsPanel()
	p.DrawCallback = func(gc *Canvas, rect geom.Rect) {
		paint := m.CodeBackground.Paint(gc, rect, paintstyle.Fill)
		gc.DrawRect(rect, paint)
	}
	p.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	wrapper := NewPanel()
	wrapper.Accessibility.Role = role.None
	wrapper.SetLayout(&FlexLayout{Columns: 1})
	wrapper.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	wrapper.SetBorder(NewEmptyBorder(m.stdBottomMargin()))
	wrapper.AddChild(p)
	m.block.AddChild(wrapper)
	m.block = p
	saveAlert := m.alert
	m.alert = 1
	m.text = NewText("", m.decoration)
	m.processChildren()
	m.finishTextRow()
	quoteBarColor := m.QuoteBarColor
	if m.alert < 0 {
		var str string
		var svg *SVG
		switch -m.alert {
		case markdownAlertNote:
			quoteBarColor = m.QuoteBarNoteColor
			str = i18n.Text("Note")
			svg = MarkdownNoteSVG
		case markdownAlertTip:
			quoteBarColor = m.QuoteBarTipColor
			str = i18n.Text("Tip")
			svg = MarkdownTipSVG
		case markdownAlertImportant:
			quoteBarColor = m.QuoteBarImportantColor
			str = i18n.Text("Important")
			svg = MarkdownImportantSVG
		case markdownAlertWarning:
			quoteBarColor = m.QuoteBarWarningColor
			str = i18n.Text("Warning")
			svg = MarkdownWarningSVG
		case markdownAlertCaution:
			quoteBarColor = m.QuoteBarCautionColor
			str = i18n.Text("Caution")
			svg = MarkdownCautionSVG
		}
		if str != "" {
			label := NewLabel()
			// What kind of alert this is belongs to the quote rather than to a label within it: the title and its icon
			// are how the quote says so on the screen, and an assistive technology hears it as the name of the passage
			// it introduces. The label's text is still part of the document's stream, read where it is drawn.
			p.Accessibility.Name = str
			label.Accessibility.Role = role.None
			label.Gap *= 2
			label.Font = m.HeadingFont[3]
			label.OnBackgroundInk = quoteBarColor
			label.SetTitle(str)
			label.SetBorder(NewEmptyBorder(m.stdBottomMargin()))
			label.Drawable = &DrawableSVG{
				SVG:  svg,
				Size: geom.NewUniformSize(label.Font.Baseline()),
			}
			p.AddChildAtIndex(label, 0)
		}
	}
	p.SetBorder(NewCompoundBorder(
		NewLineBorder(quoteBarColor, geom.Size{}, geom.Insets{Left: m.QuoteBarThickness}, false),
		NewEmptyBorder(geom.NewUniformInsets(m.CodeAndQuotePadding)),
	))
	removeBottomMarginFromLastChild(p)
	m.alert = saveAlert
	m.decoration = saveDec
	m.block = saveBlock
	m.maxLineWidth = saveMaxLineWidth
}

func removeBottomMarginFromLastChild(p *Panel) {
	if children := p.Children(); len(children) != 0 {
		p = children[len(children)-1]
		if border := p.Border(); border != nil {
			if b, ok := border.(*EmptyBorder); ok {
				b.insets.Bottom = 0
				return
			}
		}
	}
}

func (m *Markdown) processList() {
	if list, ok := m.node.(*ast.List); ok {
		saveIndex := m.index
		saveOrdered := m.ordered
		saveBulletWidth := m.bulletWidth
		saveBlock := m.block
		m.index = list.Start
		m.ordered = list.IsOrdered()
		// A list is a list of items, each of which pairs a bullet with its content, rather than one grid of alternating
		// bullets and content: an assistive technology moves from item to item, says which of how many it is on, and
		// reads the whole of it. The look is unchanged because each item lays its bullet and content out in the same
		// two columns the one grid used, with every bullet given the width of the widest so that the numbers of an
		// ordered list still line up. See Markdown.listBulletWidth.
		m.bulletWidth = m.listBulletWidth(list)
		p := newMarkdownContainer(role.List).AsPanel()
		p.SetLayoutData(&FlexLayoutData{
			HAlign: align.Fill,
			HGrab:  true,
		})
		insets := m.stdBottomMargin()
		insets.Left = m.Font.Baseline() / 2
		p.SetBorder(NewEmptyBorder(insets))
		var data any
		if data, ok = m.block.ClientData()[markdownListItemKey]; ok {
			var is bool
			if is, ok = data.(bool); ok && is {
				removeBottomMarginFromLastChild(m.block)
			}
		}
		m.block.AddChild(p)
		m.block = p
		saveMaxLineWidth := m.maxLineWidth
		m.maxLineWidth -= insets.Left
		m.processChildren()
		m.maxLineWidth = saveMaxLineWidth
		m.index = saveIndex
		m.ordered = saveOrdered
		m.bulletWidth = saveBulletWidth
		m.block = saveBlock
	}
}

// listBulletWidth returns the width every bullet of a list is given, which is that of the widest of them: it keeps the
// numbers of an ordered list aligned with one another now that each item lays its own bullet out rather than all of
// them sharing one column of one grid. The bullets are measured rather than the width being guessed, and before any
// item is built, since the text of every item is wrapped to the room the column leaves.
func (m *Markdown) listBulletWidth(list *ast.List) float32 {
	var widest float32
	index := list.Start
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		_, prefSize, _ := m.createListBullet(item, index).Sizes(geom.Size{})
		widest = max(widest, prefSize.Width)
		index++
	}
	return widest
}

// markdownTaskCheckBoxParser parses the check box of a task list item only where GFM has one, which goldmark's own
// parser does not: GFM requires whitespace after the "]", so "[x](url)" begins with a link and "[ ]todo" with text.
type markdownTaskCheckBoxParser struct {
	parser.InlineParser
}

// Parse implements parser.InlineParser.
func (p markdownTaskCheckBoxParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	if line, _ := block.PeekLine(); len(line) > 3 && !util.IsSpace(line[3]) {
		return nil
	}
	return p.InlineParser.Parse(parent, block, pc)
}

// markdownTaskCheckBox returns the check box of a task list item, or nil if the item is not a task.
func markdownTaskCheckBox(item ast.Node) *astex.TaskCheckBox {
	if block := item.FirstChild(); block != nil {
		if box, ok := block.FirstChild().(*astex.TaskCheckBox); ok {
			return box
		}
	}
	return nil
}

// markdownTaskBox draws the check box of a task list item.
type markdownTaskBox struct {
	font    Font
	checked bool
}

// LogicalSize implements Drawable. The height is that of a line, which keeps the box level with the text beside it.
func (b *markdownTaskBox) LogicalSize() geom.Size {
	return geom.NewSize(xmath.Ceil(b.font.Baseline()*0.8), b.font.LineHeight())
}

// boxRect returns where within rect the box is drawn. The box keeps its size and sits at the end of rect, since every
// bullet is given the width of the widest; see Markdown.listBulletWidth.
func (b *markdownTaskBox) boxRect(rect geom.Rect) geom.Rect {
	side := b.LogicalSize().Width
	return geom.NewRect(rect.Right()-side, rect.Y+xmath.Round((rect.Height-side)/2), side, side)
}

// DrawInRect implements Drawable.
func (b *markdownTaskBox) DrawInRect(canvas *Canvas, rect geom.Rect, _ *SamplingOptions, paint *Paint) {
	rect = b.boxRect(rect)
	side := rect.Width
	if paint == nil {
		paint = NewPaint()
	} else {
		paint = paint.Clone()
	}
	paint.SetStyle(paintstyle.Stroke)
	paint.SetStrokeWidth(1)
	canvas.DrawRoundedRect(rect.Inset(geom.NewUniformInsets(0.5)), geom.NewUniformSize(side/5), paint)
	if b.checked {
		paint.SetStrokeWidth(side / 7)
		mark := NewPath()
		mark.MoveTo(geom.NewPoint(rect.X+side*0.25, rect.Y+side*0.55))
		mark.LineTo(geom.NewPoint(rect.X+side*0.45, rect.Y+side*0.7))
		mark.LineTo(geom.NewPoint(rect.X+side*0.75, rect.Y+side*0.3))
		canvas.DrawPath(mark, paint)
	}
}

// createTaskBox creates the check box that takes the place of a task list item's bullet or number, as it does on
// GitHub. It is an image named for its state, which is how an assistive technology is told whether the task is done.
func (m *Markdown) createTaskBox(checked bool) *DrawablePanel {
	box := NewDrawablePanel()
	box.Drawable = &markdownTaskBox{font: m.decoration.Font, checked: checked}
	box.Ink = m.decoration.OnBackgroundInk
	if checked {
		box.Accessibility.Name = i18n.Text("Checked")
	} else {
		box.Accessibility.Name = i18n.Text("Unchecked")
	}
	return box
}

// createListBullet creates what marks a list item: the check box of a task, or else its number or bullet.
func (m *Markdown) createListBullet(item ast.Node, index int) *Panel {
	if task := markdownTaskCheckBox(item); task != nil {
		return m.createTaskBox(task.IsChecked).AsPanel()
	}
	bullet := "•"
	if m.ordered {
		bullet = fmt.Sprintf("%d.", index)
	}
	label := NewLabel()
	// The bullet is not something to move to or hear announced on its own: it says which item this is, which the item
	// itself reports, and it is read where it is drawn as the first thing in the item's text.
	label.Accessibility.Role = role.None
	// The text is aligned to the end of the label rather than the label to the end of its cell, since every bullet is
	// given the width of the widest; see Markdown.listBulletWidth.
	label.HAlign = align.End
	label.Text = NewText(bullet, m.decoration)
	return label.AsPanel()
}

func (m *Markdown) processListItem() {
	saveMaxLineWidth := m.maxLineWidth
	bullet := m.createListBullet(m.node, m.index)
	if m.ordered {
		m.index++
	}
	bullet.SetLayoutData(&FlexLayoutData{
		HAlign:   align.End,
		SizeHint: geom.NewSize(m.bulletWidth, 0),
	})
	spacing := m.decoration.Font.Baseline() / 3
	// Rounded down, since the labels the text is drawn in round their widths up.
	m.maxLineWidth = xmath.Floor(m.maxLineWidth - (m.bulletWidth + spacing))
	item := newMarkdownContainer(role.ListItem).AsPanel()
	item.SetLayout(&FlexLayout{
		Columns:  2,
		HSpacing: spacing,
	})
	item.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	m.block.AddChild(item)
	item.AddChild(bullet)
	saveBlock := m.block
	p := NewPanel()
	p.Accessibility.Role = role.None
	p.SetLayout(&FlexLayout{Columns: 1})
	p.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	p.ClientData()[markdownListItemKey] = true
	item.AddChild(p)
	m.block = p
	m.processChildren()
	removeBottomMarginFromLastChild(p)
	m.block = saveBlock
	m.maxLineWidth = saveMaxLineWidth
}

func (m *Markdown) processTable() {
	if table, ok := m.node.(*astex.Table); ok {
		if len(table.Alignments) != 0 {
			saveBlock := m.block
			saveTableRow := m.tableRow
			columns := len(table.Alignments)
			// The grid is kept as it is — one flow of cells in as many columns as the table has — because its column
			// alignment is what the table looks like. The rows an assistive technology moves through are described over
			// the top of it; see markdownTable.ProvideAccessibility.
			p := newMarkdownTable(columns).AsPanel()
			p.SetBorder(NewCompoundBorder(NewEmptyBorder(m.stdBottomMargin()),
				NewLineBorder(ThemeSurfaceEdge, geom.Size{}, geom.NewUniformInsets(1), false)))
			m.block.AddChild(p)
			build := func() {
				p.RemoveAllChildren()
				m.block = p
				m.tableRow = 0
				m.processChildren()
				m.block = saveBlock
			}
			m.columnWidths = make([]int, columns)
			for i := range m.columnWidths {
				m.columnWidths[i] = int(xmath.Floor(m.maxLineWidth))
			}
			build()
			room := int(xmath.Floor(m.maxLineWidth - p.Border().Insets().Width()))
			if natural := markdownColumnWidths(p, columns); markdownSum(natural) > room {
				// Too wide, so built again with every column as narrow as its content allows, which says how narrow each
				// can be made: a long word, or an image, which is never scaled narrower than its placeholder, keeps its
				// column wider than asked.
				clear(m.columnWidths)
				build()
				// A table that cannot fit even so is left that narrow.
				if least := markdownColumnWidths(p, columns); markdownSum(least) < room {
					m.columnWidths = markdownFitColumns(natural, least, room)
					build()
				}
			}
			m.tableRow = saveTableRow
			m.MarkForLayoutRecursively()
		}
	}
}

// markdownColumnWidths returns how wide each column of a table is, which is as wide as the widest of its cells.
func markdownColumnWidths(table *Panel, columns int) []int {
	widths := make([]int, columns)
	for i, cell := range table.Children() {
		_, prefSize, _ := cell.Sizes(geom.Size{})
		widths[i%columns] = max(widths[i%columns], int(xmath.Ceil(prefSize.Width)))
	}
	return widths
}

// markdownSum returns the sum of values.
func markdownSum(values []int) int {
	total := 0
	for _, v := range values {
		total += v
	}
	return total
}

// markdownFitColumns returns how wide to make each column of a table that is too wide for room when each column is
// natural wide and fits it when each is least wide: the widest are narrowed first, down to a level that no column is
// made wider than and none is made narrower than least, and whatever room that leaves goes to the narrowed columns.
func markdownFitColumns(natural, least []int, room int) []int {
	width := func(i, level int) int { return max(least[i], min(natural[i], level)) }
	total := func(level int) int {
		sum := 0
		for i := range natural {
			sum += width(i, level)
		}
		return sum
	}
	// The widest level at which the columns fit room, which is below the widest of them since together they do not.
	level := sort.Search(slices.Max(natural)+1, func(level int) bool { return total(level) > room }) - 1
	extra := room - total(level)
	widths := make([]int, len(natural))
	for i := range widths {
		widths[i] = width(i, level)
		// The columns one more unit of level would widen, of which there are more than extra.
		if extra > 0 && least[i] <= level && level < natural[i] {
			widths[i]++
			extra--
		}
	}
	return widths
}

func (m *Markdown) processTableHeader() {
	if m.hasNonEmptyContentInTree(m.node) {
		m.isHeader = true
		m.columnIndex = 0
		m.processChildren()
		m.isHeader = false
		m.tableRow++
	}
}

func (m *Markdown) hasNonEmptyContentInTree(node ast.Node) bool {
	switch node.Kind() {
	case ast.KindTextBlock, ast.KindParagraph, ast.KindHeading, ast.KindCodeBlock, ast.KindFencedCodeBlock,
		ast.KindBlockquote, ast.KindList, ast.KindText, ast.KindEmphasis, ast.KindCodeSpan, ast.KindRawHTML,
		ast.KindString, ast.KindLink, ast.KindImage, ast.KindAutoLink:
		return true
	}
	if node.HasChildren() {
		child := node.FirstChild()
		for !xreflect.IsNil(child) {
			if m.hasNonEmptyContentInTree(child) {
				return true
			}
			child = child.NextSibling()
		}
	}
	return false
}

func (m *Markdown) processTableRow() {
	m.columnIndex = 0
	m.processChildren()
	m.tableRow++
}

func (m *Markdown) processTableCell() {
	if cell, ok := m.node.(*astex.TableCell); ok {
		saveDec := m.decoration
		saveBlock := m.block
		hAlign := m.alignment(cell.Alignment)
		m.decoration = m.decoration.Clone()
		if m.isHeader {
			m.decoration.Font = m.HeadingFont[5]
			if hAlign != align.End {
				hAlign = align.Middle
			}
		}
		kind := role.Cell
		if m.isHeader {
			kind = role.ColumnHeader
		}
		cell := newMarkdownBlock(m, kind)
		cell.row = m.tableRow
		cell.col = m.columnIndex
		p := cell.AsPanel()
		p.SetBorder(NewLineBorder(ThemeSurfaceEdge, geom.Size{}, geom.NewUniformInsets(1), false))
		p.SetLayout(&FlexLayout{
			Columns: 1,
			HAlign:  hAlign,
		})
		p.SetLayoutData(&FlexLayoutData{
			HAlign: align.Fill,
			VAlign: align.Fill,
			VGrab:  true,
		})
		m.block.AddChild(p)

		inner := NewPanel()
		inner.Accessibility.Role = role.None
		inner.SetBorder(NewEmptyBorder(StdInsets()))
		inner.SetLayout(&FlexLayout{
			Columns: 1,
			HAlign:  hAlign,
		})
		inner.SetLayoutData(&FlexLayoutData{
			HAlign: hAlign,
		})
		p.AddChild(inner)

		m.block = inner
		saveMaxLineWidth := m.maxLineWidth
		// The column's width takes in the cell's border and padding, which the text is not wrapped into.
		m.maxLineWidth = max(float32(m.columnWidths[m.columnIndex])-
			p.Border().Insets().Width()-inner.Border().Insets().Width(), 0)
		m.text = NewText("", m.decoration)
		m.processChildren()
		m.finishTextRow()
		m.maxLineWidth = saveMaxLineWidth
		m.decoration = saveDec
		m.block = saveBlock
	}
	m.columnIndex++
	if m.columnIndex >= len(m.columnWidths) {
		m.columnIndex = 0
	}
}

func (m *Markdown) alignment(alignment astex.Alignment) align.Enum {
	switch alignment {
	case astex.AlignLeft:
		return align.Start
	case astex.AlignRight:
		return align.End
	case astex.AlignCenter:
		return align.Middle
	default:
		return align.Start
	}
}

const (
	markdownAlertNote = iota + 3
	markdownAlertTip
	markdownAlertImportant
	markdownAlertWarning
	markdownAlertCaution
)

func (m *Markdown) processText() {
	if t, ok := m.node.(*ast.Text); ok {
		b := util.UnescapePunctuations(t.Value(m.content))
		b = util.ResolveNumericReferences(b)
		str := string(util.ResolveEntityNames(b))
		if m.alert > 0 {
			switch m.alert {
			case 1: // Looking for '['
				if str == "[" {
					m.alert = 2
				} else {
					m.alert = 0
				}
			case 2: // Looking for '!' followed by one of 'NOTE', 'TIP', 'IMPORTANT', 'WARNING', or 'CAUTION'
				switch str {
				case "!NOTE":
					m.alert = markdownAlertNote
				case "!TIP":
					m.alert = markdownAlertTip
				case "!IMPORTANT":
					m.alert = markdownAlertImportant
				case "!WARNING":
					m.alert = markdownAlertWarning
				case "!CAUTION":
					m.alert = markdownAlertCaution
				default:
					m.alert = 0
				}
			case 3, 4, 5, 6, 7: // Looking for terminating ']'
				if str == "]" {
					m.text = NewText("", m.decoration)
					m.alert = -m.alert
					return
				}
				m.alert = 0
			}
		}
		if t.SoftLineBreak() {
			str += " "
		}
		m.text.AddString(str, m.decoration)
		if t.HardLineBreak() {
			m.alert = 0
			m.flushAndIssueLineBreak()
		}
	}
}

func (m *Markdown) processEmphasis() {
	if emphasis, ok := m.node.(*ast.Emphasis); ok {
		save := m.decoration
		m.decoration = save.Clone()
		fd := m.decoration.Font.Descriptor()
		if emphasis.Level == 1 {
			fd.Slant = slant.Italic
		} else {
			fd.Weight = weight.Bold
		}
		m.decoration.Font = fd.Font()
		m.processChildren()
		m.decoration = save
	}
}

func (m *Markdown) processCodeSpan() {
	save := m.decoration
	m.decoration = save.Clone()
	m.decoration.OnBackgroundInk = m.OnCodeBackground
	m.decoration.BackgroundInk = m.CodeBackground
	m.decoration.Font = m.CodeBlockFont
	m.processChildren()
	m.decoration = save
}

func (m *Markdown) processStrikethrough() {
	if _, ok := m.node.(*astex.Strikethrough); ok {
		save := m.decoration
		m.decoration = save.Clone()
		m.decoration.StrikeThrough = true
		m.processChildren()
		m.decoration = save
	}
}

func (m *Markdown) processRawHTML() {
	if raw, ok := m.node.(*ast.RawHTML); ok {
		count := raw.Segments.Len()
		for i := range count {
			segment := raw.Segments.At(i)
			switch xstrings.CollapseSpaces(strings.ToLower(string(segment.Value(m.content)))) {
			case "<br>", "<br/>", "<br />":
				m.flushAndIssueLineBreak()
				if next := m.node.NextSibling(); next != nil {
					if t, ok2 := next.(*ast.Text); ok2 {
						t.SetSoftLineBreak(false)
					}
				}
			case "<hr>", "<hr/>", "<hr />":
				m.flushAndIssueLineBreak()
				m.processThematicBreak()
				m.flushAndIssueLineBreak()
			}
		}
	}
}

func (m *Markdown) processString() {
	if t, ok := m.node.(*ast.String); ok {
		b := util.UnescapePunctuations(t.Value)
		b = util.ResolveNumericReferences(b)
		str := string(util.ResolveEntityNames(b))
		m.text.AddString(str, m.decoration)
	}
}

func (m *Markdown) processLink() {
	if link, ok := m.node.(*ast.Link); ok {
		m.createLink(m.extractText(link), string(link.Destination), string(link.Title))
	}
}

// createLink adds a link to the row being filled, broken across rows as text is when it does not fit. Each piece is a
// link of its own to the mouse, but only the first is one to an assistive technology: it is named for the whole of the
// link, and its span in the block's text takes in every piece. See axBlockTextBuilder.chunk.
func (m *Markdown) createLink(label, target, tooltip string) {
	m.flushText()
	theme := LinkTheme{
		LabelTheme: LabelTheme{
			TextDecoration: *m.decoration.Clone(),
		},
		PressedInk:   m.LinkInk,
		OnPressedInk: m.LinkOnPressedInk,
	}
	theme.OnBackgroundInk = m.LinkInk
	if tooltip == "" && target != "" {
		tooltip = target
	}
	var head *Label
	var rest []*Panel
	addPiece := func(part *Text) {
		link := NewLink(part.String(), tooltip, target, &theme, m.linkHandler)
		if head == nil {
			head = link
		} else {
			link.Accessibility.Role = role.None
			link.ClientData()[markdownLinkHeadKey] = head.AsPanel()
			if len(rest) == 0 {
				own := head.AsPanel()
				// Set by a callback rather than as Accessibility.Name, which would put the rest of the link's words into
				// the name of a heading holding it twice. A node has one rectangle, so the link's takes in every piece,
				// along with whatever else is on the rows it begins and ends on. Whether it is offscreen was worked out
				// from the first piece alone, and it is not while any other piece shows.
				head.Accessibility.Callback = func(node *accessibility.Node) {
					node.Name = label
					node.Bounds = markdownLinkBounds(node.Bounds, own, rest)
					node.Offscreen = node.Offscreen && !slices.ContainsFunc(rest, markdownPanelShows)
				}
			}
			rest = append(rest, link.AsPanel())
		}
		// The document takes the keyboard focus and follows the link under its reading caret itself (see
		// Markdown.DefaultKeyDown), so the link takes no focus and needs no room for the outline.
		link.SetFocusable(false)
		link.SetBorder(nil)
		// A press on the link never reaches the document, so a document that can take the focus is given it here,
		// with its caret put on the link unless it is there already, as it is when Return or Space is what is
		// following the link. A screen reader then finds the person on the link they followed.
		press := link.MouseDownCallback
		first := head
		link.MouseDownCallback = func(where geom.Point, button, clickCount int, mods mod.Modifiers) bool {
			if button == ButtonLeft && m.Focusable() {
				m.RequestFocus()
				doc := m.axDocument()
				if !first.Is(doc.linkAt(m.axCaret())) {
					pos := doc.offsetAt(link.PointTo(where, m.AsPanel()))
					m.axSetSelection(pos, pos)
				}
			}
			return press(where, button, clickCount, mods)
		}
		m.addToTextRow(link)
	}
	m.flowText(NewText(label, &theme.TextDecoration), addPiece)
	if head == nil {
		// An empty link has nothing to draw, nor has one of nothing but spaces beginning a line, which are not drawn (see
		// markdownRow.leadingSpaces), and either is still a link.
		addPiece(NewText("", &theme.TextDecoration))
	}
}

// markdownLinkBounds returns the smallest rectangle holding every piece of a link broken across rows: head, the first,
// and the rest. bounds is what head's node was given, which is its area mapped into the space of the window being
// described; that only scales and moves it, so the other pieces are mapped the same way.
func markdownLinkBounds(bounds geom.Rect, head *Panel, rest []*Panel) geom.Rect {
	own := head.RectToRoot(head.ContentRect(true))
	if own.Width <= 0 || own.Height <= 0 {
		return bounds
	}
	all := own
	for _, piece := range rest {
		all = all.Union(piece.RectToRoot(piece.ContentRect(true)))
	}
	scale := geom.NewPoint(bounds.Width/own.Width, bounds.Height/own.Height)
	return geom.Rect{
		Point: bounds.Point.Add(all.Point.Sub(own.Point).MulPt(scale)),
		Size:  all.Size.MulPt(scale),
	}
}

// markdownPanelShows reports whether any of p is left in view by the panels it sits within, each of which clips what it
// draws to its own frame.
func markdownPanelShows(p *Panel) bool {
	visible := p.RectToRoot(p.ContentRect(true))
	for parent := p.Parent(); parent != nil && !visible.Empty(); parent = parent.Parent() {
		visible = visible.Intersect(parent.RectToRoot(parent.ContentRect(true)))
	}
	return !visible.Empty()
}

// HasURLPrefix returns true if the target has a prefix of "http://" or "https://".
func HasURLPrefix(target string) bool {
	return strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://")
}

// ReviseTarget returns a sanitized target with either a link or an absolute path.
func ReviseTarget(workingDir, target string, altLinkPrefixes []string) (string, error) {
	if HasURLPrefix(target) {
		return target, nil
	}
	revised, err := url.PathUnescape(target)
	if err != nil {
		return target, errs.Wrap(err)
	}
	if HasAnyPrefix(altLinkPrefixes, revised) {
		return revised, nil
	}
	if !filepath.IsAbs(revised) {
		if workingDir == "" {
			workingDir = "."
		}
		revised = filepath.Join(workingDir, revised)
	}
	if revised, err = filepath.Abs(revised); err != nil {
		return target, errs.Wrap(err)
	}
	return revised, nil
}

func (m *Markdown) linkHandler(_ Paneler, target string) {
	if strings.HasPrefix(target, "#") {
		if m.ScrollToAnchor(target) {
			return
		}
	}
	m.LinkHandler(m, target)
}

// ScrollToAnchor attempts to scroll the heading associated with the given anchor into view. The anchor may optionally
// be prefixed with '#' and may be URL-escaped. Matching is first attempted case-sensitively, then case-insensitively.
// Returns true if a matching anchor was found.
func (m *Markdown) ScrollToAnchor(anchor string) bool {
	anchor = strings.TrimPrefix(anchor, "#")
	if unescaped, err := url.PathUnescape(anchor); err == nil {
		anchor = unescaped
	}
	if p := m.panelForAnchor(anchor); p != nil {
		// Bring the top of the heading to the top of the view (when the scroll range allows), revealing the section
		// that follows it rather than leaving the heading pinned to the bottom of the view. Inflating the height of the
		// rect we request to be made visible to the full height of the document forces a top alignment: the rect can
		// never fully fit, so the scroll logic aligns its top edge with the top of the view (clamped to the available
		// scroll range). When the heading itself is taller than the view, this still shows its top rather than its
		// bottom.
		rect := p.ContentRect(true)
		rect.Height = max(rect.Height, m.ContentRect(true).Height)
		p.ScrollRectIntoView(rect)
		return true
	}
	return false
}

func (m *Markdown) panelForAnchor(anchor string) *Panel {
	if p, ok := m.anchors[anchor]; ok {
		return p
	}
	for id, p := range m.anchors {
		if strings.EqualFold(id, anchor) {
			return p
		}
	}
	return nil
}

// markdownPrimaryDisplay indirects PrimaryDisplay solely so that tests can verify the display query performed by
// retrieveImage happens on the calling goroutine and not on the image-loading goroutine, since displays may only be
// queried on the UI thread. Production code must never reassign it.
var markdownPrimaryDisplay = PrimaryDisplay

// retrieveImage returns the image at target, or nil when it is not to hand within a second. One that arrives after that
// is put in panel, constrained to maxWidth and minWidth by constrainImage.
func (m *Markdown) retrieveImage(target string, panel *DrawablePanel, maxWidth, minWidth float32) Drawable {
	workingDir := ""
	if m.WorkingDirProvider != nil {
		workingDir = m.WorkingDirProvider(m)
	}
	revisedTarget, err := ReviseTarget(workingDir, target, m.AltLinkPrefixes)
	if err != nil {
		errs.Log(err, "workingDir", workingDir, "target", target, "altLinkPrefixes", m.AltLinkPrefixes)
		return nil
	}
	m.drawableCacheLock.Lock()
	entry, ok := m.drawableCache[revisedTarget]
	if ok {
		d := entry.drawable
		if d == nil {
			entry.targets = append(entry.targets, markdownImageTarget{panel: panel, maxWidth: maxWidth, minWidth: minWidth})
		}
		m.drawableCacheLock.Unlock()
		return d
	}
	m.drawableCache[revisedTarget] = &drawableCacheEntry{
		targets: []markdownImageTarget{{panel: panel, maxWidth: maxWidth, minWidth: minWidth}},
	}
	m.drawableCacheLock.Unlock()
	isSVG := strings.ToLower(path.Ext(revisedTarget)) == ".svg"
	// Displays may only be queried on the UI thread (on macOS the lookup calls into AppKit, and the platform
	// implementations use unsynchronized state), so capture the scale here, on the calling thread, rather than inside
	// the image-loading goroutine below.
	scale := geom.NewPoint(1, 1)
	if !isSVG {
		if d := markdownPrimaryDisplay(); d != nil {
			scale = scale.DivPt(d.Scale)
		}
	}
	result := make(chan Drawable, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		// On failure, the cache entry must be removed. Leaving it in place would permanently poison the cache: the
		// load would never be retried and every subsequent rebuild would append another dead placeholder panel to the
		// entry's target list, retaining those detached panels for the life of the Markdown widget.
		fail := func(failure error, args ...any) {
			result <- nil
			errs.Log(failure, args...)
			m.drawableCacheLock.Lock()
			delete(m.drawableCache, revisedTarget)
			m.drawableCacheLock.Unlock()
		}
		var d Drawable
		if isSVG {
			var r io.ReadCloser
			if r, err = xhttp.StreamData(ctx, nil, revisedTarget); err != nil {
				fail(err, "path", revisedTarget)
				return
			}
			defer xio.CloseIgnoringErrors(r)
			var svg *SVG
			if svg, err = NewSVGFromReader(r); err != nil {
				fail(err, "path", revisedTarget)
				return
			}
			d = &DrawableSVG{
				SVG:  svg,
				Size: svg.SuggestedSize(),
			}
		} else {
			var img *Image
			if img, err = NewImageFromFilePathOrURL(ctx, m.HTTPClient, revisedTarget, scale, m.PerImageByteLimit); err != nil {
				fail(err, "path", revisedTarget, "scale", scale)
				return
			}
			d = img
		}
		result <- d
		InvokeTask(func() { m.updateDrawable(revisedTarget, d) })
	}()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case one := <-result:
		return one
	case <-timer.C:
		return nil
	}
}

func (m *Markdown) updateDrawable(target string, d Drawable) {
	m.drawableCacheLock.Lock()
	defer m.drawableCacheLock.Unlock()
	entry, ok := m.drawableCache[target]
	if !ok {
		// This should never happen
		return
	}
	entry.drawable = d
	// An image that has just arrived takes up room where a placeholder was, so every rune after it on its line has
	// moved. The frames it changes would say so on their own, but the drawables are replaced here before the layout
	// runs, so the stream is thrown away here as well.
	m.axInvalidate()
	for _, one := range entry.targets {
		one.panel.Drawable = constrainImage(d, one.maxWidth, one.minWidth)
		one.panel.Ink = nil
		one.panel.MarkForRedraw()
		one.panel.MarkForLayoutRecursivelyUpward()
	}
	entry.targets = nil
}

// constrainImage returns the drawable scaled down, if need be, to be no wider than maxWidth, though never narrower than
// minWidth, which keeps it visible on a line with little or no room. One no wider than minWidth is left as it is.
func constrainImage(drawable Drawable, maxWidth, minWidth float32) Drawable {
	maxWidth = max(maxWidth, minWidth)
	size := drawable.LogicalSize()
	if size.Width <= maxWidth {
		return drawable
	}
	if size.Width > 0 && size.Width > maxWidth {
		size.Height *= maxWidth / size.Width
		if size.Height < 1 {
			size.Height = 1
		}
		size.Width = maxWidth
	}
	return &SizedDrawable{
		Drawable: drawable,
		Size:     size,
	}
}

func (m *Markdown) extractText(node ast.Node) string {
	var str strings.Builder
	m.collectText(node, &str)
	return str.String()
}

func (m *Markdown) collectText(node ast.Node, str *strings.Builder) {
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		switch t := c.(type) {
		case *ast.Text:
			b := util.UnescapePunctuations(t.Value(m.content))
			b = util.ResolveNumericReferences(b)
			str.Write(util.ResolveEntityNames(b))
			if t.SoftLineBreak() {
				str.WriteByte(' ')
			}
		case *ast.String:
			b := util.UnescapePunctuations(t.Value)
			b = util.ResolveNumericReferences(b)
			str.Write(util.ResolveEntityNames(b))
		default:
			m.collectText(c, str)
		}
	}
}

func (m *Markdown) processImage() {
	if image, ok := m.node.(*ast.Image); ok {
		m.flushText()
		panel := NewDrawablePanel()
		// The placeholder's size, which is also the narrowest an image is scaled down to.
		size := max(m.decoration.Font.Size(), 24)
		d := m.retrieveImage(string(image.Destination), panel, m.maxLineWidth, size)
		if d == nil {
			panel.Drawable = &DrawableSVG{
				SVG:  BrokenImageSVG,
				Size: geom.NewSize(size, size),
			}
			panel.Ink = m.OnBackgroundInk
		} else {
			panel.Drawable = constrainImage(d, m.maxLineWidth, size)
			panel.Ink = nil
		}
		primary := m.extractText(image)
		secondary := string(image.Title)
		if primary == "" && secondary != "" {
			primary = secondary
			secondary = ""
		}
		if primary != "" {
			// The alternative text is what the image is called, which is the only thing about it an assistive
			// technology can pass on; without it the image is skipped entirely rather than announced as a picture of
			// nothing.
			panel.Accessibility.Name = primary
			panel.Accessibility.Description = secondary
			if secondary != "" {
				panel.Tooltip = NewTooltipWithSecondaryText(primary, secondary)
			} else {
				panel.Tooltip = NewTooltipWithText(primary)
			}
		}
		if m.text != nil {
			// An image that will not fit in what is left of the line begins a new one. Without this the row's
			// FlowLayout wraps it onto a second visual row of its own while the row still counts as the one line it was
			// built as, and everything read from where that line was drawn — every rune boundary on it, the caret
			// dropped at one and the highlight drawn from one — describes two rows as if they were one. See
			// axBlockTextBuilder.collect.
			_, prefSize, _ := panel.Sizes(geom.Size{})
			m.prepareToFlushText(prefSize.Width)
		}
		m.addToTextRow(panel)
	}
}

func (m *Markdown) processAutoLink() {
	if link, ok := m.node.(*ast.AutoLink); ok {
		u := string(link.URL(m.content))
		m.createLink(u, u, "")
	}
}

func (m *Markdown) addToTextRow(p Paneler) {
	if m.textRow == nil {
		m.startTextRow()
	}
	m.textRow.AddChild(p)
}

// startTextRow starts a new row to fill.
func (m *Markdown) startTextRow() *markdownRow {
	row := newMarkdownRow()
	m.textRow = row.AsPanel()
	m.textRow.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	m.block.AddChild(m.textRow)
	return row
}

func (m *Markdown) addLabelToTextRow(t *Text) {
	label := NewLabel()
	// A fragment of a line is not an element of its own: the block it belongs to reports the whole of its text, and a
	// label per piece would have an assistive technology read the line a few words at a time.
	label.Accessibility.Role = role.None
	label.Text = t
	m.addToTextRow(label)
}

func (m *Markdown) flushAndIssueLineBreak() {
	m.flushText()
	m.endTextRow(true)
}

func (m *Markdown) issueLineBreak() {
	m.endTextRow(false)
}

// endTextRow finishes the row being filled. hard says the line was ended by a break in the markdown itself rather than
// by the text running out of room, which is the difference between the line ending in a line feed and the spaces that
// were there being taken off the end of it — the row remembers both so that the block's text reads as what was written.
// See markdownRow and markdownBlock.axText.
func (m *Markdown) endTextRow(hard bool) {
	var children []*Panel
	if m.textRow != nil {
		children = m.textRow.Children()
	}
	if len(children) == 0 {
		// A line with nothing on it is still a line, and still as tall as one, so it is kept as an empty label rather
		// than left out. See Label.axTextLine, which reports the one place a caret can sit in it.
		empty := NewLabel()
		empty.Accessibility.Role = role.None
		m.addToTextRow(empty)
	} else if child, ok := children[len(children)-1].Self.(*Label); ok && !child.Text.Empty() {
		// Text.BreakToWidth leaves every space at the break on the line, past the width it was broken to. A piece of
		// text that is nothing but those spaces is emptied if it is all the line holds, and otherwise removed: left
		// empty after something that fills the row, the row's FlowLayout would put it on a line of its own.
		r := child.Text.Runes()
		end := len(r)
		for end > 0 && isBreakableSpace(r[end-1]) {
			end--
		}
		if end < len(r) {
			if end == 0 && len(children) > 1 && child.Accessibility.Role != role.Link {
				m.textRow.RemoveChild(child)
			} else {
				child.Text = child.Text.Slice(0, end)
			}
			if row, is := m.textRow.Self.(*markdownRow); is {
				row.strippedSpaces = r[end:]
			}
		}
	}
	if hard {
		if row, ok := m.textRow.Self.(*markdownRow); ok {
			row.hardBreak = true
		}
	}
	m.textRow = nil
}

func (m *Markdown) flushText() {
	if m.text != nil && len(m.text.Runes()) != 0 {
		m.flowText(m.text, m.addLabelToTextRow)
		m.text = NewText("", m.decoration)
	}
}

// flowText adds t to the row being filled, breaking it across as many rows as it needs. add puts one piece of it on
// the row.
func (m *Markdown) flowText(t *Text, add func(part *Text)) {
	if m.textRow == nil || len(m.textRow.Children()) == 0 {
		// The line was begun by a break in the markdown or by the block itself, and the spaces after either are not
		// drawn, however many pieces of text they are split between. A row with nothing on it was started here for
		// the spaces of an earlier piece. See markdownRow.leadingSpaces.
		if lead := markdownLeadingSpaces(t); lead != 0 {
			if m.textRow == nil {
				m.startTextRow()
			}
			if row, ok := m.textRow.Self.(*markdownRow); ok {
				row.leadingSpaces = append(row.leadingSpaces, t.Runes()[:lead]...)
			}
			t = t.Slice(lead, len(t.Runes()))
		}
	}
	for !t.Empty() {
		remaining, started := m.remainingOnTextRow()
		if !started {
			m.flowTextOnEmptyRows(t, remaining, add)
			return
		}
		// BreakToWidth keeps a first word that is too wide, which goes on the next row instead.
		part := t.BreakToWidth(remaining)[0]
		if !markdownPieceFits(part, t, remaining) {
			// The text begins the next row, and the spaces it begins with are where the line breaks, so they end this
			// row as the spaces a piece of text was broken at do. See markdownRow.strippedSpaces.
			row := m.textRow
			m.issueLineBreak()
			if lead := markdownLeadingSpaces(t); lead != 0 {
				if r, ok := row.Self.(*markdownRow); ok {
					r.strippedSpaces = append(slices.Clip(r.strippedSpaces), t.Runes()[:lead]...)
				}
				t = t.Slice(lead, len(t.Runes()))
			}
			continue
		}
		add(part)
		// The last piece stays on the row even when the spaces after its last word go past the width, which whatever
		// ends the row takes off it.
		if t = t.Slice(len(part.Runes()), len(t.Runes())); !t.Empty() {
			m.issueLineBreak()
		}
	}
}

// flowTextOnEmptyRows is flowText for a row with nothing on it, width wide. Every row after it begins empty too, so t
// is broken once rather than a row at a time.
func (m *Markdown) flowTextOnEmptyRows(t *Text, width float32, add func(part *Text)) {
	lines := t.BreakToWidth(width)
	// BreakToWidth keeps text that fits width exactly on one line only when that is all it was given, which the last
	// two lines are when broken a row at a time.
	if n := len(lines); n > 1 {
		end := len(t.Runes())
		if tail := t.Slice(end-len(lines[n-2].Runes())-len(lines[n-1].Runes()), end); tail.Width() <= width {
			lines = append(lines[:n-2], tail)
		}
	}
	for i, line := range lines {
		if i != 0 {
			m.issueLineBreak()
		}
		add(line)
	}
}

// markdownLeadingSpaces returns how many breakable spaces t begins with.
func markdownLeadingSpaces(t *Text) int {
	runes := t.Runes()
	lead := 0
	for lead < len(runes) && isBreakableSpace(runes[lead]) {
		lead++
	}
	return lead
}

// markdownPieceFits reports whether part, the first piece BreakToWidth broke t into for the room remaining on a row,
// fits there. The spaces after its last word need not.
func markdownPieceFits(part, t *Text, remaining float32) bool {
	runes := part.Runes()
	end := len(runes)
	for end > 0 && isBreakableSpace(runes[end-1]) {
		end--
	}
	if end == 0 {
		// Nothing but spaces: the whole of t, which need not fit either, since whatever ends the row takes them off it
		// and only then knows whether the line was broken there, or the spaces before a first word that did not fit.
		return len(runes) == len(t.Runes())
	}
	return part.PositionForRuneIndex(end) <= remaining
}

// remainingOnTextRow returns the room left on the row being filled, which is the whole width when nothing has been put
// on it, and whether anything has.
func (m *Markdown) remainingOnTextRow() (remaining float32, started bool) {
	// Rounded down, since the labels the text is drawn in round their widths up.
	width := xmath.Floor(m.maxLineWidth)
	if m.textRow == nil || len(m.textRow.Children()) == 0 {
		return width, false
	}
	// Measured unwrapped, since the spaces that end the row may already have taken it past the width, and measuring it
	// wrapped to the width would move them to a line of their own rather than count them.
	_, prefSize, _ := m.textRow.Sizes(geom.Size{})
	return xmath.Floor(width - prefSize.Width), true
}

// prepareToFlushText ends the row being filled if it has something on it and less than minRemaining of room left,
// and returns the room left on the row that is then being filled.
func (m *Markdown) prepareToFlushText(minRemaining float32) float32 {
	remaining, started := m.remainingOnTextRow()
	// A row with nothing on it is left alone, since ending it would leave a blank line.
	if started && remaining < minRemaining {
		m.issueLineBreak()
		remaining = xmath.Floor(m.maxLineWidth)
	}
	return remaining
}

func (m *Markdown) finishTextRow() {
	m.flushText()
	if m.textRow != nil {
		m.endTextRow(false)
	}
	m.text = nil
}

// DefaultMarkdownLinkHandler is the default link handler, which opens http and https links via OpenBrowser.
func DefaultMarkdownLinkHandler(_ Paneler, target string) {
	if HasURLPrefix(target) {
		if err := OpenBrowser(target); err != nil {
			ErrorDialogWithError(i18n.Text("Opening the link failed"), err)
		}
	}
}
