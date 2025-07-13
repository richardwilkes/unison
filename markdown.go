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
	"bytes"
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/toolbox/v2/xstrings"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/slant"
	"github.com/richardwilkes/unison/enums/weight"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	astex "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// DefaultMarkdownWidth is the default maximum width to use, roughly equivalent to a page at 100dpi.
const DefaultMarkdownWidth = 8 * 100

// DefaultMarkdownTheme holds the default MarkdownTheme values for Markdown. Modifying this data will not alter existing
// Markdown, but will alter any Markdown created in the future.
var DefaultMarkdownTheme MarkdownTheme

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
		CodeBlockFont:       &DynamicFont{Resolver: func() FontDescriptor { return DeriveMarkdownCodeBlockFont(nil) }},
		CodeBackground:      ThemeAboveSurface,
		OnCodeBackground:    ThemeOnAboveSurface,
		QuoteBarColor:       ThemeFocus,
		LinkInk:             DefaultLinkTheme.OnBackgroundInk,
		LinkOnPressedInk:    DefaultLinkTheme.OnPressedInk,
		LinkHandler:         DefaultMarkdownLinkHandler,
		VSpacing:            10,
		QuoteBarThickness:   2,
		CodeAndQuotePadding: 6,
		Slop:                4,
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
	fd.Weight = weight.Black
	// 20% relative growth per level
	switch level {
	case 1:
		fd.Size *= 2.48832
	case 2:
		fd.Size *= 2.0736
	case 3:
		fd.Size *= 1.728
	case 4:
		fd.Size *= 1.44
	case 5:
		fd.Size *= 1.2
	default:
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
	HeadingFont         [6]Font
	CodeBlockFont       Font
	CodeBackground      Ink
	OnCodeBackground    Ink
	QuoteBarColor       Ink
	LinkInk             Ink
	LinkOnPressedInk    Ink
	LinkHandler         func(Paneler, string)
	WorkingDirProvider  func(Paneler) string
	AltLinkPrefixes     []string
	VSpacing            float32
	QuoteBarThickness   float32
	CodeAndQuotePadding float32
	Slop                float32
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

// Markdown provides markdown display widget.
type Markdown struct {
	lastParent                 *Panel
	block                      *Panel
	textRow                    *Panel
	text                       *Text
	decoration                 *TextDecoration
	node                       ast.Node
	chainedFrameChangeCallback func()
	content                    []byte
	columnWidths               []int
	imgCache                   map[string]*Image
	MarkdownTheme
	Panel
	index        int
	columnIndex  int
	maxWidth     float32
	maxLineWidth float32
	ordered      bool
	isHeader     bool
}

// NewMarkdown creates a new markdown widget. If autoSizingFromParent is true, then the Markdown will attempt to keep
// its content wrapped to its parent's width. Currently, things like tables don't play nice with width management.
func NewMarkdown(autoSizingFromParent bool) *Markdown {
	m := &Markdown{
		MarkdownTheme: DefaultMarkdownTheme,
		imgCache:      make(map[string]*Image),
	}
	m.SetVSpacing(m.VSpacing)
	m.Self = m
	if autoSizingFromParent {
		m.ParentChangedCallback = m.adjustSizeOnParentChange
	}
	return m
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
	if m.chainedFrameChangeCallback != nil {
		m.chainedFrameChangeCallback()
	}
}

// SetVSpacing sets the vertical spacing between blocks. Use this function rather than setting VSpacing directly, since
// this will also adjust the layout to match.
func (m *Markdown) SetVSpacing(spacing float32) {
	m.VSpacing = spacing
	m.SetLayout(&FlexLayout{
		Columns:  1,
		VSpacing: m.VSpacing,
	})
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
	m.node = goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser().Parse(text.NewReader(m.content))
	m.walk(m.node)
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
	case ast.KindHTMLBlock:
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
	p := NewPanel()
	p.SetLayout(&FlexLayout{Columns: 1})
	save := m.block
	m.block.AddChild(p)
	m.block = p
	m.text = NewText("", m.decoration)
	m.processChildren()
	m.finishTextRow()
	m.block = save
}

func (m *Markdown) processHeading() {
	if heading, ok := m.node.(*ast.Heading); ok {
		saveDec := m.decoration
		saveBlock := m.block
		m.decoration = m.decoration.Clone()
		m.decoration.Font = m.HeadingFont[min(max(heading.Level, 1), 6)-1]
		p := NewPanel()
		p.SetLayout(&FlexLayout{Columns: 1})
		m.block.AddChild(p)
		m.block = p
		m.text = NewText("", m.decoration)
		m.processChildren()
		m.finishTextRow()
		m.decoration = saveDec
		m.block = saveBlock
	}
}

func (m *Markdown) processThematicBreak() {
	hr := NewSeparator()
	hr.SetLayoutData(&FlexLayoutData{
		HGrab:  true,
		HAlign: align.Fill,
		VAlign: align.Middle,
	})
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

	p := NewPanel()
	p.DrawCallback = func(gc *Canvas, rect geom.Rect) {
		gc.DrawRect(rect, m.CodeBackground.Paint(gc, rect, paintstyle.Fill))
	}
	p.SetLayout(&FlexLayout{Columns: 1})
	p.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	p.SetBorder(NewEmptyBorder(geom.NewUniformInsets(m.CodeAndQuotePadding)))
	m.block.AddChild(p)
	m.block = p
	lines := m.node.Lines()
	count := lines.Len()
	for i := 0; i < count; i++ {
		segment := lines.At(i)
		label := NewLabel()
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

	p := NewPanel()
	p.DrawCallback = func(gc *Canvas, rect geom.Rect) {
		gc.DrawRect(rect, m.CodeBackground.Paint(gc, rect, paintstyle.Fill))
	}
	p.SetLayout(&FlexLayout{
		Columns:  1,
		VSpacing: m.VSpacing,
	})
	p.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	p.SetBorder(NewCompoundBorder(
		NewLineBorder(m.QuoteBarColor, 0, geom.Insets{Left: m.QuoteBarThickness}, false),
		NewEmptyBorder(geom.NewUniformInsets(m.CodeAndQuotePadding)),
	))
	m.block.AddChild(p)
	m.block = p
	m.text = NewText("", m.decoration)
	m.processChildren()
	m.finishTextRow()
	m.decoration = saveDec
	m.block = saveBlock
	m.maxLineWidth = saveMaxLineWidth
}

func (m *Markdown) processList() {
	if list, ok := m.node.(*ast.List); ok {
		saveIndex := m.index
		saveOrdered := m.ordered
		saveBlock := m.block
		m.index = list.Start
		m.ordered = list.IsOrdered()
		p := NewPanel()
		p.SetLayout(&FlexLayout{
			Columns:  2,
			HSpacing: m.decoration.Font.SimpleWidth(" "),
		})
		p.SetLayoutData(&FlexLayoutData{
			HAlign: align.Fill,
			HGrab:  true,
		})
		m.block.AddChild(p)
		m.block = p
		m.processChildren()
		m.index = saveIndex
		m.ordered = saveOrdered
		m.block = saveBlock
	}
}

func (m *Markdown) processListItem() {
	var bullet string
	saveMaxLineWidth := m.maxLineWidth
	if m.ordered {
		bullet = fmt.Sprintf("%d.", m.index)
		m.index++
		m.maxLineWidth -= m.decoration.Font.SimpleWidth("999. ") // This isn't right, but is a reasonable approximation
	} else {
		bullet = "•"
		m.maxLineWidth -= m.decoration.Font.SimpleWidth("• ")
	}
	label := NewLabel()
	label.Text = NewText(bullet, m.decoration)
	label.SetLayoutData(&FlexLayoutData{HAlign: align.End})
	m.block.AddChild(label)

	saveBlock := m.block
	p := NewPanel()
	p.SetLayout(&FlexLayout{Columns: 1})
	p.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	m.block.AddChild(p)
	m.block = p
	m.processChildren()
	m.block = saveBlock
	m.maxLineWidth = saveMaxLineWidth
}

func (m *Markdown) processTable() {
	if table, ok := m.node.(*astex.Table); ok {
		if len(table.Alignments) != 0 {
			saveBlock := m.block
			m.columnWidths = make([]int, len(table.Alignments))
			for i := 0; i < len(m.columnWidths); i++ {
				m.columnWidths[i] = int(xmath.Floor(m.maxLineWidth))
			}
			p := NewPanel()
			p.SetBorder(NewLineBorder(ThemeSurfaceEdge, 0, geom.NewUniformInsets(1), false))
			p.SetLayout(&FlexLayout{Columns: len(table.Alignments)})
			m.block.AddChild(p)
			m.block = p
			m.processChildren()
			m.block = saveBlock

			m.MarkForLayoutRecursively()
			m.ValidateLayout()
			if over := int(xmath.Ceil(p.FrameRect().Width - (m.maxLineWidth - (4 + StdHSpacing*float32(1+len(m.columnWidths)))))); over > 0 {
				children := p.Children()
				count := 0
				for i := 0; i < len(m.columnWidths); i++ {
					if i < len(children) {
						m.columnWidths[i] = int(xmath.Ceil(children[i].FrameRect().Width))
						if m.columnWidths[i] > 0 {
							count++
						}
					} else {
						m.columnWidths[i] = 0
					}
				}
				if count > 0 {
					widths := make([]int, len(m.columnWidths))
					copy(widths, m.columnWidths)
					slices.Sort(widths)
					for i := len(widths) - 1; i > 0; i-- {
						delta := widths[i] - widths[i-1]
						qty := 0
						for j := 0; j < len(m.columnWidths); j++ {
							if m.columnWidths[j] == widths[i] {
								qty++
							}
						}
						if qty*delta > over {
							amt := over / qty
							extra := over - amt*qty
							for j := 0; j < len(m.columnWidths); j++ {
								if m.columnWidths[j] == widths[i] {
									m.columnWidths[j] -= amt
									if extra > 0 {
										m.columnWidths[j]--
										extra--
									}
								}
							}
							over = 0
							break
						}
						for j := 0; j < len(m.columnWidths); j++ {
							if m.columnWidths[j] == widths[i] {
								m.columnWidths[j] -= delta
								over -= delta
							}
						}
					}
					if over > 0 {
						count = 0
						for j := 0; j < len(m.columnWidths); j++ {
							if m.columnWidths[j] > 0 {
								count++
							}
						}
						amt := over / count
						extra := over - amt*count
						for j := 0; j < len(m.columnWidths); j++ {
							if m.columnWidths[j] > 0 {
								m.columnWidths[j] -= amt
								if extra > 0 {
									m.columnWidths[j]--
									extra--
								}
								if m.columnWidths[j] < 0 {
									m.columnWidths[j] = 0
								}
							}
						}
					}
				}
				p.RemoveAllChildren()
				m.block = p
				m.processChildren()
				m.block = saveBlock
			}
			m.MarkForLayoutRecursively()
		}
	}
}

func (m *Markdown) processTableHeader() {
	if m.hasNonEmptyContentInTree(m.node) {
		m.isHeader = true
		m.processChildren()
		m.isHeader = false
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
		p := NewPanel()
		p.SetBorder(NewLineBorder(ThemeSurfaceEdge, 0, geom.NewUniformInsets(1), false))
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
		m.maxLineWidth = float32(m.columnWidths[m.columnIndex])
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

func (m *Markdown) processText() {
	if t, ok := m.node.(*ast.Text); ok {
		b := util.UnescapePunctuations(t.Value(m.content))
		b = util.ResolveNumericReferences(b)
		str := string(util.ResolveEntityNames(b))
		if t.SoftLineBreak() {
			str += " "
		}
		m.text.AddString(str, m.decoration)
		if t.HardLineBreak() {
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
			fd.Weight = weight.Black
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
		for i := 0; i < count; i++ {
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
		m.flushText()
		p := m.createLink(m.extractText(link), string(link.Destination), string(link.Title))
		m.addToTextRow(p)
	}
}

func (m *Markdown) createLink(label, target, tooltip string) *Label {
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
	return NewLink(label, tooltip, target, theme, m.linkHandler)
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
	if workingDir == "" {
		workingDir = "."
	}
	if revised, err = filepath.Abs(filepath.Join(workingDir, revised)); err != nil {
		return target, errs.Wrap(err)
	}
	return revised, nil
}

func (m *Markdown) linkHandler(_ Paneler, target string) {
	m.LinkHandler(m, target)
}

func (m *Markdown) retrieveImage(target string, label *Label) *Image {
	workingDir := ""
	if m.WorkingDirProvider != nil {
		workingDir = m.WorkingDirProvider(m)
	}
	revisedTarget, err := ReviseTarget(workingDir, target, m.AltLinkPrefixes)
	if err != nil {
		errs.Log(err, "workingDir", workingDir, "target", target, "altLinkPrefixes", m.AltLinkPrefixes)
		return nil
	}
	img, ok := m.imgCache[revisedTarget]
	if !ok {
		result := make(chan *Image, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			scale := 1 / PrimaryDisplay().ScaleX
			if img, err = NewImageFromFilePathOrURLWithContext(ctx, revisedTarget, scale); err != nil {
				result <- nil
				errs.Log(err, "path", revisedTarget, "scale", scale)
			} else {
				result <- img
				InvokeTask(func() {
					m.imgCache[revisedTarget] = img
					label.Drawable = m.constrainImage(img)
					label.MarkForRedraw()
					label.MarkForLayoutRecursivelyUpward()
				})
			}
		}()
		timer := time.NewTimer(time.Second)
		select {
		case one := <-result:
			if one != nil {
				img = one
				m.imgCache[revisedTarget] = img
			}
		case <-timer.C:
		}
		timer.Stop()
	}
	return img
}

func (m *Markdown) constrainImage(drawable Drawable) Drawable {
	size := drawable.LogicalSize()
	if size.Width <= m.maxWidth {
		return drawable
	}
	if size.Width > 0 && size.Width > m.maxWidth {
		size.Height *= m.maxWidth / size.Width
		if size.Height < 1 {
			size.Height = 1
		}
		size.Width = m.maxWidth
	}
	return &SizedDrawable{
		Drawable: drawable,
		Size:     size,
	}
}

func (m *Markdown) extractText(node ast.Node) string {
	str := ""
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b := util.UnescapePunctuations(t.Value(m.content))
			b = util.ResolveNumericReferences(b)
			str += string(util.ResolveEntityNames(b))
			if t.SoftLineBreak() {
				str += " "
			}
		}
	}
	return str
}

func (m *Markdown) processImage() {
	if image, ok := m.node.(*ast.Image); ok {
		m.flushText()
		label := NewLabel()
		img := m.retrieveImage(string(image.Destination), label)
		if img == nil {
			size := max(m.decoration.Font.Size(), 24)
			label.Drawable = &DrawableSVG{
				SVG:  BrokenImageSVG,
				Size: geom.Size{Width: size, Height: size},
			}
		} else {
			label.Drawable = m.constrainImage(img)
		}
		primary := m.extractText(image)
		secondary := string(image.Title)
		if primary == "" && secondary != "" {
			primary = secondary
			secondary = ""
		}
		if primary != "" {
			if secondary != "" {
				label.Tooltip = NewTooltipWithSecondaryText(primary, secondary)
			} else {
				label.Tooltip = NewTooltipWithText(primary)
			}
		}
		m.addToTextRow(label)
	}
}

func (m *Markdown) processAutoLink() {
	if link, ok := m.node.(*ast.AutoLink); ok {
		m.flushText()
		u := string(link.URL(m.content))
		p := m.createLink(u, u, "")
		m.addToTextRow(p)
	}
}

func (m *Markdown) addToTextRow(p Paneler) {
	if m.textRow == nil {
		m.textRow = NewPanel()
		m.textRow.SetLayout(&FlowLayout{})
		m.textRow.SetLayoutData(&FlexLayoutData{
			HAlign: align.Fill,
			HGrab:  true,
		})
		m.block.AddChild(m.textRow)
	}
	m.textRow.AddChild(p)
}

func (m *Markdown) addLabelToTextRow(t *Text) {
	label := NewLabel()
	label.Text = t
	m.addToTextRow(label)
}

func (m *Markdown) flushAndIssueLineBreak() {
	m.flushText()
	m.issueLineBreak()
}

func (m *Markdown) issueLineBreak() {
	var children []*Panel
	if m.textRow != nil {
		children = m.textRow.Children()
	}
	if len(children) == 0 {
		m.addToTextRow(NewLabel())
	} else if child, ok := children[len(children)-1].Self.(*Label); ok && !child.Text.Empty() {
		if r := child.Text.Runes(); len(r) > 1 && r[len(r)-1] == ' ' {
			child.Text = child.Text.Slice(0, len(r)-1)
		}
	}
	m.textRow = nil
}

func (m *Markdown) flushText() {
	if m.text != nil && len(m.text.Runes()) != 0 {
		remaining := m.maxLineWidth
		if m.textRow != nil {
			_, prefSize, _ := m.textRow.Sizes(geom.Size{Width: m.maxLineWidth})
			remaining -= prefSize.Width
		}
		minWidth := m.decoration.Font.SimpleWidth("W")
		if remaining < minWidth {
			// Remaining space is less than the width of a W, so go to the next line
			m.issueLineBreak()
			remaining = m.maxLineWidth
		}
		if remaining < m.text.Width() {
			// Remaining space isn't large enough for the text we have, so put a chunk that will fit on this line, then
			// go to the next line
			part := m.text.BreakToWidth(remaining)[0]
			m.text = m.text.Slice(len(part.Runes()), len(m.text.Runes()))
			m.addLabelToTextRow(part)
			m.issueLineBreak()
			// Now break the remaining text up to the max width size and add each line
			if parts := m.text.BreakToWidth(m.maxLineWidth); len(parts) != 0 {
				for i := 0; i < len(parts)-1; i++ {
					m.addLabelToTextRow(parts[i])
					m.issueLineBreak()
				}
				m.addLabelToTextRow(parts[len(parts)-1])
			}
		} else {
			m.addLabelToTextRow(m.text)
		}
		m.text = NewText("", m.decoration)
	}
}

func (m *Markdown) finishTextRow() {
	m.flushText()
	m.text = nil
	m.textRow = nil
}

// DefaultMarkdownLinkHandler provides the default link handler, which handles opening a browsers for http and https
// links.
func DefaultMarkdownLinkHandler(_ Paneler, target string) {
	if HasURLPrefix(target) {
		if err := xos.OpenBrowser(target); err != nil {
			ErrorDialogWithError(i18n.Text("Opening the link failed"), err)
		}
	}
}
