/*
 * Copyright ©1998-2022 by Richard A. Wilkes. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, version 2.0. If a copy of the MPL was not distributed with
 * this file, You can obtain one at http://mozilla.org/MPL/2.0/.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as
 * defined by the Mozilla Public License, version 2.0.
 */

package unison

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/desktop"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/txt"
	"github.com/richardwilkes/toolbox/xmath"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	tableAST "github.com/yuin/goldmark/extension/ast"
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
			Font:       LabelFont,
			Foreground: OnBackgroundColor,
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
		CodeBackground:      ContentColor,
		OnCodeBackground:    OnContentColor,
		QuoteBarColor:       AccentColor,
		LinkInk:             LinkColor,
		LinkRolloverInk:     LinkRolloverColor,
		LinkPressedInk:      LinkPressedColor,
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
	if toolbox.IsNil(font) {
		fd = DefaultMarkdownTheme.Font.Descriptor()
	} else {
		fd = font.Descriptor()
	}
	fd.Weight = BlackFontWeight
	switch level {
	case 1:
		fd.Size *= 2.5
	case 2:
		fd.Size *= 2
	case 3:
		fd.Size *= 1.75
	case 4:
		fd.Size *= 1.5
	case 5:
		fd.Size *= 1.25
	default:
	}
	return fd
}

// DeriveMarkdownCodeBlockFont derives a FontDescriptor for code from another font. Pass in nil for the font to use
// MonospacedFont.
func DeriveMarkdownCodeBlockFont(font Font) FontDescriptor {
	var fd FontDescriptor
	if toolbox.IsNil(font) {
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
	LinkRolloverInk     Ink
	LinkPressedInk      Ink
	LinkHandler         func(Paneler, string)
	WorkingDirProvider  func(Paneler) string
	ImageConstrainer    func(size Size) Size
	VSpacing            float32
	QuoteBarThickness   float32
	CodeAndQuotePadding float32
	Slop                float32
}

// Markdown provides markdown display widget.
type Markdown struct {
	Panel
	MarkdownTheme
	lastParent                 *Panel
	chainedFrameChangeCallback func()
	node                       ast.Node
	content                    []byte
	block                      *Panel
	textRow                    *Panel
	text                       *Text
	decoration                 *TextDecoration
	imgCache                   map[string]*Image
	index                      int
	columnIndex                int
	columnWidths               []int
	maxWidth                   float32
	maxLineWidth               float32
	ordered                    bool
	isHeader                   bool
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
	m.decoration = m.TextDecoration.Clone()
	m.index = 0
	m.ordered = false
	m.node = goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser().Parse(text.NewReader(m.content))
	m.walk(m.node)
	m.MarkForLayoutAndRedraw()
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
	case tableAST.KindTable:
		m.processTable()
	case tableAST.KindTableHeader:
		m.processTableHeader()
	case tableAST.KindTableRow:
		m.processTableRow()
	case tableAST.KindTableCell:
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

	default:
		jot.Infof("unhandled markdown element: %v", m.node.Kind())
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
		m.decoration.Font = m.HeadingFont[xmath.Min(xmath.Max(heading.Level, 1), 6)-1]
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
		HAlign: FillAlignment,
		VAlign: MiddleAlignment,
	})
	m.block.AddChild(hr)
}

func (m *Markdown) processCodeBlock() {
	saveDec := m.decoration
	saveBlock := m.block
	saveMaxLineWidth := m.maxLineWidth
	m.decoration = m.decoration.Clone()
	m.decoration.Font = m.CodeBlockFont
	m.decoration.Foreground = m.OnCodeBackground
	m.maxLineWidth -= m.CodeAndQuotePadding * 2

	p := NewPanel()
	p.DrawCallback = func(gc *Canvas, rect Rect) {
		gc.DrawRect(rect, m.CodeBackground.Paint(gc, rect, Fill))
	}
	p.SetLayout(&FlexLayout{Columns: 1})
	p.SetLayoutData(&FlexLayoutData{
		HAlign: FillAlignment,
		HGrab:  true,
	})
	p.SetBorder(NewEmptyBorder(NewUniformInsets(m.CodeAndQuotePadding)))
	m.block.AddChild(p)
	m.block = p
	lines := m.node.Lines()
	count := lines.Len()
	for i := 0; i < count; i++ {
		segment := lines.At(i)
		label := NewRichLabel()
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
	m.decoration.Foreground = m.OnCodeBackground
	m.maxLineWidth -= m.QuoteBarThickness + m.CodeAndQuotePadding*2

	p := NewPanel()
	p.DrawCallback = func(gc *Canvas, rect Rect) {
		gc.DrawRect(rect, m.CodeBackground.Paint(gc, rect, Fill))
	}
	p.SetLayout(&FlexLayout{
		Columns:  1,
		VSpacing: m.VSpacing,
	})
	p.SetLayoutData(&FlexLayoutData{
		HAlign: FillAlignment,
		HGrab:  true,
	})
	p.SetBorder(NewCompoundBorder(NewLineBorder(m.QuoteBarColor, 0,
		Insets{Left: m.QuoteBarThickness}, false),
		NewEmptyBorder(NewUniformInsets(m.CodeAndQuotePadding))))
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
			HAlign: FillAlignment,
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
	label := NewRichLabel()
	label.Text = NewText(bullet, m.decoration)
	label.SetLayoutData(&FlexLayoutData{HAlign: EndAlignment})
	m.block.AddChild(label)

	saveBlock := m.block
	p := NewPanel()
	p.SetLayout(&FlexLayout{Columns: 1})
	p.SetLayoutData(&FlexLayoutData{
		HAlign: FillAlignment,
		HGrab:  true,
	})
	m.block.AddChild(p)
	m.block = p
	m.processChildren()
	m.block = saveBlock
	m.maxLineWidth = saveMaxLineWidth
}

func (m *Markdown) processTable() {
	if table, ok := m.node.(*tableAST.Table); ok {
		if len(table.Alignments) != 0 {
			saveBlock := m.block
			m.columnWidths = make([]int, len(table.Alignments))
			for i := 0; i < len(m.columnWidths); i++ {
				m.columnWidths[i] = int(xmath.Floor(m.maxLineWidth))
			}
			p := NewPanel()
			p.SetBorder(NewLineBorder(DividerColor, 0, NewUniformInsets(1), false))
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
					sort.IntSlice(widths).Sort()
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
		for !toolbox.IsNil(child) {
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
	if cell, ok := m.node.(*tableAST.TableCell); ok {
		saveDec := m.decoration
		saveBlock := m.block
		align := m.alignment(cell.Alignment)
		m.decoration = m.decoration.Clone()
		if m.isHeader {
			m.decoration.Font = m.HeadingFont[5]
			if align != EndAlignment {
				align = MiddleAlignment
			}
		}
		p := NewPanel()
		p.SetBorder(NewLineBorder(DividerColor, 0, NewUniformInsets(1), false))
		p.SetLayout(&FlexLayout{
			Columns: 1,
			HAlign:  align,
		})
		p.SetLayoutData(&FlexLayoutData{
			HAlign: FillAlignment,
			VAlign: FillAlignment,
			VGrab:  true,
		})
		m.block.AddChild(p)

		inner := NewPanel()
		inner.SetBorder(NewEmptyBorder(StdInsets()))
		inner.SetLayout(&FlexLayout{
			Columns: 1,
			HAlign:  align,
		})
		inner.SetLayoutData(&FlexLayoutData{
			HAlign: align,
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

func (m *Markdown) alignment(alignment tableAST.Alignment) Alignment {
	switch alignment {
	case tableAST.AlignLeft:
		return StartAlignment
	case tableAST.AlignRight:
		return EndAlignment
	case tableAST.AlignCenter:
		return MiddleAlignment
	default:
		return StartAlignment
	}
}

func (m *Markdown) processText() {
	if t, ok := m.node.(*ast.Text); ok {
		b := util.UnescapePunctuations(t.Text(m.content))
		b = util.ResolveNumericReferences(b)
		str := string(util.ResolveEntityNames(b))
		if t.SoftLineBreak() {
			str += " "
		}
		m.text.AddString(str, m.decoration)
		if t.HardLineBreak() {
			m.flushText()
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
			fd.Slant = ItalicSlant
		} else {
			fd.Weight = BlackFontWeight
		}
		m.decoration.Font = fd.Font()
		m.processChildren()
		m.decoration = save
	}
}

func (m *Markdown) processCodeSpan() {
	save := m.decoration
	m.decoration = save.Clone()
	m.decoration.Foreground = m.OnCodeBackground
	m.decoration.Background = m.CodeBackground
	m.decoration.Font = m.CodeBlockFont
	m.processChildren()
	m.decoration = save
}

func (m *Markdown) processRawHTML() {
	if raw, ok := m.node.(*ast.RawHTML); ok {
		count := raw.Segments.Len()
		for i := 0; i < count; i++ {
			segment := raw.Segments.At(i)
			switch txt.CollapseSpaces(strings.ToLower(string(segment.Value(m.content)))) {
			case "<br>", "<br/>", "<br />":
				m.flushText()
				m.flushAndIssueLineBreak()
			case "<hr>", "<hr/>", "<hr />":
				m.flushText()
				m.flushAndIssueLineBreak()
				m.processThematicBreak()
				m.flushText()
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
		p := m.createLink(string(link.Text(m.content)), string(link.Destination), string(link.Title))
		m.addToTextRow(p)
	}
}

func (m *Markdown) createLink(label, target, tooltip string) *RichLabel {
	theme := LinkTheme{
		TextDecoration: *m.decoration.Clone(),
		RolloverInk:    m.LinkRolloverInk,
		PressedInk:     m.LinkPressedInk,
	}
	theme.Foreground = m.LinkInk
	theme.Underline = true
	if tooltip == "" && target != "" {
		tooltip = target
	}
	return NewLink(label, tooltip, target, theme, m.linkHandler)
}

func hasURLPrefix(target string) bool {
	return strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://")
}

func (m *Markdown) reviseTarget(target string) string {
	if m.WorkingDirProvider != nil && !hasURLPrefix(target) {
		workingDir := m.WorkingDirProvider(m)
		if workingDir == "" {
			workingDir = fmt.Sprintf(".%c", filepath.Separator)
		}
		p := target
		if revised, err := url.PathUnescape(p); err == nil {
			p = revised
		}
		var err error
		if p, err = filepath.Abs(filepath.Join(workingDir, p)); err == nil {
			target = p
		}
	}
	return target
}

func (m *Markdown) linkHandler(_ Paneler, target string) {
	m.LinkHandler(m, m.reviseTarget(target))
}

func (m *Markdown) retrieveImage(target string, label *Label) *Image {
	target = m.reviseTarget(target)
	img, ok := m.imgCache[target]
	if !ok {
		result := make(chan *Image, 1)
		go func() {
			var err error
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if img, err = NewImageFromFilePathOrURLWithContext(ctx, target, 1); err != nil {
				result <- nil
				jot.Error(errs.Wrap(err))
			} else {
				result <- img
				InvokeTask(func() {
					m.imgCache[target] = img
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
				m.imgCache[target] = img
			}
		case <-timer.C:
		}
		timer.Stop()
	}
	return img
}

func (m *Markdown) constrainImage(drawable Drawable) Drawable {
	if m.ImageConstrainer != nil {
		drawable = &SizedDrawable{
			Drawable: drawable,
			Size:     m.ImageConstrainer(drawable.LogicalSize()),
		}
	}
	return drawable
}

func (m *Markdown) processImage() {
	if image, ok := m.node.(*ast.Image); ok {
		m.flushText()
		label := NewLabel()
		img := m.retrieveImage(string(image.Destination), label)
		if img == nil {
			size := xmath.Max(m.decoration.Font.Size(), 24)
			label.Drawable = m.constrainImage(&DrawableSVG{
				SVG:  BrokenImageSVG,
				Size: NewSize(size, size),
			})
		} else {
			label.Drawable = m.constrainImage(img)
		}
		primary := string(image.Text(m.content))
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
			HAlign: FillAlignment,
			HGrab:  true,
		})
		m.block.AddChild(m.textRow)
	}
	m.textRow.AddChild(p)
}

func (m *Markdown) addLabelToTextRow(t *Text) {
	label := NewRichLabel()
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
		m.addToTextRow(NewRichLabel())
	} else if child, ok := children[len(children)-1].Self.(*RichLabel); ok {
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
			_, prefSize, _ := m.textRow.Sizes(Size{Width: m.maxLineWidth})
			remaining -= prefSize.Width
		}
		min := m.decoration.Font.SimpleWidth("W")
		if remaining < min {
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
	if hasURLPrefix(target) {
		if err := desktop.Open(target); err != nil {
			ErrorDialogWithError(i18n.Text("Opening the link failed"), err)
		}
	}
}
