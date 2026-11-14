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
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/richardwilkes/canvas/codecs"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/behavior"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/testenv"
)

// collectMarkdownText concatenates the text of every Label in the panel subtree, in tree order, so tests can assert on
// rendered content without depending on the exact panel structure the builder produces.
func collectMarkdownText(p *Panel) string {
	var sb strings.Builder
	var walk func(panel *Panel)
	walk = func(panel *Panel) {
		if label, ok := panel.Self.(*Label); ok {
			if s := label.String(); s != "" {
				if sb.Len() > 0 {
					sb.WriteByte(' ')
				}
				sb.WriteString(s)
			}
		}
		for _, child := range panel.Children() {
			walk(child)
		}
	}
	walk(p)
	return sb.String()
}

func TestMarkdownContentRoundTrips(t *testing.T) {
	c := check.New(t)
	const content = "Hello world\n"
	m := NewMarkdown(false)
	m.SetContent(content, 400)
	c.Equal(content, string(m.ContentBytes()))
}

func TestMarkdownSetContentIsIdempotent(t *testing.T) {
	c := check.New(t)
	m := NewMarkdown(false)
	m.SetContent("# Heading\n\nA paragraph.\n", 400)
	first := m.Children()

	// Re-setting identical content and width must be a no-op: the existing child panels are retained rather than
	// rebuilt.
	m.SetContent("# Heading\n\nA paragraph.\n", 400)
	second := m.Children()
	c.Equal(len(first), len(second))
	if len(first) == len(second) {
		for i := range first {
			c.True(first[i] == second[i])
		}
	}

	// Changing the content rebuilds the tree.
	m.SetContent("Different content.\n", 400)
	c.Contains(collectMarkdownText(m.AsPanel()), "Different")

	// So does changing it within the slice it was given in.
	buf := []byte("First.\n")
	m.SetContentBytes(buf, 400)
	copy(buf, "Other.\n")
	m.SetContentBytes(buf, 400)
	c.Contains(collectMarkdownText(m.AsPanel()), "Other")

	// Or within the slice ContentBytes returned.
	buf = m.ContentBytes()
	copy(buf, "Third.\n")
	m.SetContentBytes(buf, 400)
	c.Contains(collectMarkdownText(m.AsPanel()), "Third")
}

// TestMarkdownHandlesGFMConstructs exercises every block/inline handler so a panic or unhandled-element regression in
// any of them is caught, the latter by the error it logs. Assertions on the content are intentionally light because
// the exact rendered structure is not part of the public contract.
func TestMarkdownHandlesGFMConstructs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    string
	}{
		{name: "heading", content: "# A Heading\n", want: "Heading"},
		{name: "thematic break", content: "before\n\n---\n\nafter\n", want: "after"},
		{name: "fenced code", content: "```go\nfmt.Println()\n```\n", want: "Println"},
		{name: "blockquote", content: "> quoted text\n", want: "quoted"},
		{name: "ordered list", content: "1. one\n2. two\n", want: "two"},
		{name: "unordered list", content: "- alpha\n- beta\n", want: "beta"},
		{name: "nested unordered lists", content: "- a\n  - b\n    - c\n      - d\n", want: "• a ◦ b ▪ c ▪ d"},
		{name: "unordered list in an ordered one", content: "1. a\n   - b\n", want: "1. a ◦ b"},
		{name: "task list", content: "- [ ] todo\n- [x] done\n", want: "todo done"},
		{name: "ordered task list", content: "1. [x] done\n2. plain\n", want: "done 2. plain"},
		{name: "link beginning a list item", content: "- [X](https://x.com)\n", want: "• X"},
		{name: "brackets beginning a list item", content: "- [ ]todo\n", want: "• [ ]todo"},
		{name: "link", content: "a [link label](http://example.com) here\n", want: "link label"},
		{name: "reference link", content: "a [ref label][] here\n\n[ref label]: http://example.com\n", want: "ref label"},
		{name: "autolink", content: "see http://example.com today\n", want: "example.com"},
		{name: "emphasis", content: "this is **strong** and *weak*\n", want: "strong"},
		{name: "underscore emphasis", content: "some _italic_ words\n", want: "italic"},
		{name: "code span", content: "use `code` inline\n", want: "code"},
		{name: "strikethrough", content: "this is ~~gone~~\n", want: "gone"},
		{
			name:    "table",
			content: "| H1 | H2 |\n| -- | -- |\n| a | b |\n",
			want:    "H1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			var logged bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
			defer slog.SetDefault(prev)
			m := NewMarkdown(false)
			m.SetContent(tc.content, 400)
			c.True(len(m.Children()) > 0)
			c.Contains(collectMarkdownText(m.AsPanel()), tc.want)
			c.Equal("", logged.String())
		})
	}
}

// TestMarkdownStyledLinkLabels verifies that link labels whose text is nested inside styled inline nodes (emphasis,
// code spans, strikethrough, etc.) still surface their text, drawn in the styles written as it would be outside a link
// but in the link's ink.
func TestMarkdownStyledLinkLabels(t *testing.T) {
	decorations := func(m *Markdown) []*TextDecoration {
		var all []*TextDecoration
		var walk func(p *Panel)
		walk = func(p *Panel) {
			if label, ok := p.Self.(*Label); ok && label.Text != nil {
				all = append(all, label.Text.decorations...)
			}
			for _, child := range p.Children() {
				walk(child)
			}
		}
		walk(m.AsPanel())
		return all
	}
	for _, tc := range []struct {
		name   string
		styled string
		want   string
	}{
		{name: "bold label", styled: "**bold label**", want: "bold label"},
		{name: "italic label", styled: "*italic label*", want: "italic label"},
		{name: "code label", styled: "`code label`", want: "code label"},
		{name: "strikethrough label", styled: "~~struck label~~", want: "struck label"},
		{name: "nested styles", styled: "**outer *inner* text**", want: "outer inner text"},
		{name: "mixed plain and styled", styled: "pre **mid** post", want: "pre mid post"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			m := NewMarkdown(false)
			m.SetContent("["+tc.styled+"](http://example.com)\n", 400)
			c.Contains(collectMarkdownText(m.AsPanel()), tc.want)
			plain := NewMarkdown(false)
			plain.SetContent(tc.styled+"\n", 400)
			got := decorations(m)
			want := decorations(plain)
			c.Equal(len(want), len(got), "how many runes are drawn")
			if len(want) != len(got) {
				return
			}
			for i, d := range got {
				c.Equal(want[i].Font.Descriptor(), d.Font.Descriptor(), "the font of rune %d", i)
				c.Equal(want[i].BackgroundInk, d.BackgroundInk, "the background of rune %d", i)
				c.Equal(want[i].StrikeThrough, d.StrikeThrough, "whether rune %d is struck through", i)
				c.Equal(m.LinkInk, d.OnBackgroundInk, "the ink of rune %d", i)
			}
		})
	}
}

// TestMarkdownLinkTargets verifies the name of a link, where it leads and the tooltip it shows, which an assistive
// technology is given as well.
func TestMarkdownLinkTargets(t *testing.T) {
	const oneTwo = "one two"
	const mailto = "mailto:foo@bar.com"
	for _, tc := range []struct {
		name     string
		content  string
		linkName string
		target   string
		tooltip  string
	}{
		{
			name:     "escapes and entities in a destination",
			content:  "[a](http://e.com/\\_x?a=1&amp;b=2)\n",
			linkName: "a",
			target:   "http://e.com/_x?a=1&b=2",
			tooltip:  "http://e.com/_x?a=1&b=2",
		},
		{
			name:     "escapes and references resolved in one pass",
			content:  "[\\&amp; &#38;amp;](u\\&amp; \"\\&amp;\")\n",
			linkName: "&amp; &amp;",
			target:   "u&amp;",
			tooltip:  "&amp;",
		},
		{
			name:     "entities in a reference's destination",
			content:  "[a][r]\n\n[r]: http://e.com/?a=1&amp;b=2\n",
			linkName: "a",
			target:   "http://e.com/?a=1&b=2",
			tooltip:  "http://e.com/?a=1&b=2",
		},
		{name: "br in a label", content: "[one<br>two](u)\n", linkName: oneTwo, target: "u", tooltip: "u"},
		{name: "br ending a line of a label", content: "[one<br>\ntwo](u)\n", linkName: oneTwo, target: "u", tooltip: "u"},
		{name: "br over two lines in a label", content: "[one<br\n/>two](u)\n", linkName: oneTwo, target: "u", tooltip: "u"},
		{name: "hard line break in a label", content: "[one\\\ntwo](u)\n", linkName: oneTwo, target: "u", tooltip: "u"},
		{
			name:     "img tag in a label",
			content:  "[site <img src=\"missing-image-for-test.png\" alt=\"GCS\">](u)\n",
			linkName: "site GCS",
			target:   "u",
			tooltip:  "u",
		},
		{
			name:     "autolink in a label broken across rows",
			content:  "[see <http://e.com>" + strings.Repeat(" and more", 20) + "](u)\n",
			linkName: "see http://e.com" + strings.Repeat(" and more", 20),
			target:   "u",
			tooltip:  "u",
		},
		{
			name:     "escapes and entities in a reference's title",
			content:  "[a][r]\n\n[r]: http://e.com \"T \\\" &amp; t\"\n",
			linkName: "a",
			target:   "http://e.com",
			tooltip:  "T \" & t",
		},
		{
			name:     "bare address beginning with www",
			content:  "see www.example.com now\n",
			linkName: "www.example.com",
			target:   "http://www.example.com",
			tooltip:  "http://www.example.com",
		},
		{
			name:     "email autolink",
			content:  "<foo@bar.com>\n",
			linkName: "foo@bar.com",
			target:   mailto,
			tooltip:  mailto,
		},
		{
			name:     "bare email address",
			content:  "mail foo@bar.com now\n",
			linkName: "foo@bar.com",
			target:   mailto,
			tooltip:  mailto,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			m := NewMarkdown(false)
			m.SetContent(tc.content, 400)
			var link *Panel
			var find func(p *Panel)
			find = func(p *Panel) {
				for _, child := range p.Children() {
					if link == nil && child.Accessibility.Role == role.Link {
						link = child
					}
					find(child)
				}
			}
			find(m.AsPanel())
			c.NotNil(link)
			if link == nil {
				return
			}
			node := &accessibility.Node{Name: link.Accessibility.Name}
			if label, ok := link.Self.(*Label); ok {
				node.Name = label.String()
			}
			if link.Accessibility.Callback != nil {
				link.Accessibility.Callback(node)
			}
			c.Equal(tc.linkName, node.Name, "the name")
			c.Equal(tc.target, link.Accessibility.URL, "the target")
			c.NotNil(link.Tooltip)
			if link.Tooltip != nil {
				c.Equal(tc.tooltip, collectMarkdownText(link.Tooltip), "the tooltip")
			}
		})
	}
}

// TestDefaultMarkdownLinkHandler verifies which targets the default link handler opens: web and email addresses, and
// nothing else. A headless session records what is opened rather than opening it, so this does not call t.Parallel.
func TestDefaultMarkdownLinkHandler(t *testing.T) {
	c := check.New(t)
	screen := startHeadlessTest(t, HeadlessConfig{Width: 100, Height: 100})
	for _, target := range []string{
		"http://e.com", "https://e.com", "HTTPS://e.com", "mailto:foo@bar.com", "MAILTO:foo@bar.com", "docs/readme.md",
	} {
		c.True(screen.Do(func() { DefaultMarkdownLinkHandler(nil, target) }))
	}
	c.Equal([]string{"http://e.com", "https://e.com", "HTTPS://e.com", "mailto:foo@bar.com", "MAILTO:foo@bar.com"},
		screen.OpenedURLs())
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownImageSourceAndText verifies where an image is loaded from and the text it is given: its alt text, styled
// words included, becomes its name, its title its description, and both its tooltip, and a title with no alt text
// becomes its name. An <img> tag's attributes do the same. An image that fails to load, as missing-image-for-test.png
// does, is still given its text.
func TestMarkdownImageSourceAndText(t *testing.T) {
	saved := markdownPrimaryDisplay
	defer func() { markdownPrimaryDisplay = saved }()
	markdownPrimaryDisplay = func() *Display { return nil }
	loaded := &DrawableSVG{SVG: BrokenImageSVG, Size: geom.NewSize(30, 30)}
	target, err := ReviseTarget("", "image_for_test.png", nil)
	check.New(t).NoError(err)
	for _, tc := range []struct {
		name    string
		content string
		alt     string
		title   string
		loaded  bool
	}{
		{
			name:    "styled alt text",
			content: "![**styled alt text**](missing-image-for-test.png)\n",
			alt:     "styled alt text",
		},
		{name: "escapes in a destination", content: "![a](image\\_for\\_test.png)\n", alt: "a", loaded: true},
		{name: "br in alt text", content: "![one<br>two](image_for_test.png)\n", alt: "one two", loaded: true},
		{
			name:    "hard line break in alt text",
			content: "![one\\\ntwo](image_for_test.png)\n",
			alt:     "one two",
			loaded:  true,
		},
		{
			name:    "escapes and entities in a title",
			content: "![a](image_for_test.png \"T \\\" &amp; t\")\n",
			alt:     "a",
			title:   "T \" & t",
			loaded:  true,
		},
		{
			name:    "code spans in alt text as written",
			content: "![`a\\*b` &amp; `&lt;T&gt;`](image_for_test.png)\n",
			alt:     "a\\*b & &lt;T&gt;",
			loaded:  true,
		},
		{
			name:    "img tag in any case with entities",
			content: "<IMG Src=\"image_for_test.png\" ALT=\"A &amp; b\" Title='T &quot;t&quot;'>\n",
			alt:     "A & b",
			title:   "T \"t\"",
			loaded:  true,
		},
		{
			name:    "img tag within text with only a title",
			content: "Before <img src=\"image_for_test.png\" title=\"T\"> after.\n",
			alt:     "T",
			loaded:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			m := NewMarkdown(false)
			m.drawableCache[markdownImageKey{target: target}] = &drawableCacheEntry{drawable: loaded}
			m.SetContent(tc.content, 400)
			var image *DrawablePanel
			var walk func(p *Panel)
			walk = func(p *Panel) {
				if one, ok := p.Self.(*DrawablePanel); ok {
					image = one
				}
				for _, child := range p.Children() {
					walk(child)
				}
			}
			walk(m.AsPanel())
			c.NotNil(image)
			if image == nil {
				return
			}
			c.Equal(tc.loaded, image.Drawable == Drawable(loaded), "whether the image was loaded from image_for_test.png")
			c.Equal(tc.alt, image.Accessibility.Name, "the name")
			c.Equal(tc.title, image.Accessibility.Description, "the description")
			c.NotNil(image.Tooltip)
			if image.Tooltip != nil {
				c.Equal(strings.TrimSpace(tc.alt+" "+tc.title), collectMarkdownText(image.Tooltip), "the tooltip")
			}
		})
	}
}

// TestMarkdownImagesFromFileSystem verifies where the images of a Markdown given a FileSystem are loaded from, that
// an image's content rather than its name tells whether it is an SVG, that an SVG giving a size too large for a float32
// can still be scaled, that every image is held to PerImageByteLimit, that an image used twice on a page is shown both
// times, and when an image is read again. Each case shows its pages in turn, each with different text or width since
// showing unchanged content does nothing unless rebuilt, and checks the width of every image on the last, 0 standing
// for the broken-image placeholder.
func TestMarkdownImagesFromFileSystem(t *testing.T) {
	testenv.SkipTimingSensitive(t)
	saved := markdownPrimaryDisplay
	defer func() { markdownPrimaryDisplay = saved }()
	markdownPrimaryDisplay = func() *Display { return nil }
	codecs.Register() // normally done at app startup, which these tests skip
	setup := check.New(t)
	const sub = "Scripting"
	const fileA = "a.svg"
	svg := func(side int) []byte {
		return fmt.Appendf(nil, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %[1]d %[1]d"><rect width="%[1]d" height="%[1]d"/></svg>`, side)
	}
	small := svg(10)
	shot := distinctPNGForTest(setup, 6, 4, 131)
	fsys := fstest.MapFS{
		fileA:                  {Data: small},
		sub + "/a.svg":         {Data: svg(20)},
		"md:" + sub + "/a.svg": {Data: svg(20)},
		"images/shot.png":      {Data: shot},
		"images/shot.svg":      {Data: shot},
		// Too large for a float32, its width and height are infinite once read.
		"huge.svg": {
			Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1e39" height="1e39" viewBox="0 0 10 10"/>`),
		},
	}
	// The server's SVG has what may precede its root element: a byte order mark, an XML declaration, a comment and a
	// DOCTYPE.
	served := append([]byte("\xEF\xBB\xBF<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!-- c -->\n"+
		"<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd\">\n"),
		svg(30)...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write(served)
		setup.NoError(err)
	}))
	defer server.Close()
	diskDir := t.TempDir()
	setup.NoError(os.WriteFile(filepath.Join(diskDir, fileA), small, 0o600))
	type page struct {
		fsys         fstest.MapFS // Replaces the FileSystem when not nil.
		files        fstest.MapFS // Added to the case's own copy of fsys before the page is shown.
		dir, content string
		width        float32 // The width the content is set to, 0 standing for 400.
		rebuild      bool    // Rebuild rather than setting content.
	}
	const showA = "![a](a.svg)\n"
	changedA := fstest.MapFS{fileA: {Data: svg(20)}}
	for _, tc := range []struct {
		name   string
		pages  []page
		widths []float32
		alt    []string
		limit  int64
		onDisk bool // FileSystem is left nil.
	}{
		{
			name:   "relative to the working directory",
			pages:  []page{{dir: sub, content: showA}},
			widths: []float32{20},
		},
		{
			name:   "working directory beginning with an alternate link prefix",
			pages:  []page{{dir: "md:" + sub, content: showA}},
			widths: []float32{20},
			alt:    []string{"md:"},
		},
		{
			// The alternate link is read from the disk, where nothing has its name.
			name:   "file and alternate link spelled alike",
			pages:  []page{{dir: "md:" + sub, content: "![a](a.svg) ![b](md:" + sub + "/a.svg)\n"}},
			widths: []float32{20, 0},
			alt:    []string{"md:"},
		},
		{
			name: "same name in two directories",
			pages: []page{
				{content: "![root](a.svg)\n"},
				{dir: sub, content: "![scripting](a.svg)\n"},
			},
			widths: []float32{20},
		},
		{
			name: "file system replaced",
			pages: []page{
				{content: showA},
				{fsys: changedA, content: "![b](a.svg)\n"},
			},
			widths: []float32{20},
		},
		{
			name:   "changed and rebuilt",
			pages:  []page{{content: showA}, {files: changedA, rebuild: true}},
			widths: []float32{20},
		},
		{
			name: "changed while not shown",
			pages: []page{
				{content: showA},
				{files: changedA, content: "No image.\n"},
				{content: "![again](a.svg)\n"},
			},
			widths: []float32{20},
		},
		{
			name: "changed while still shown",
			pages: []page{
				{content: showA},
				{files: changedA, content: "![b](a.svg)\n"},
			},
			widths: []float32{10},
		},
		{
			name: "failed and laid out again",
			pages: []page{
				{content: "![late](late.svg)\n"},
				{files: fstest.MapFS{"late.svg": {Data: svg(20)}}, content: "![late](late.svg)\n", width: 300},
			},
			widths: []float32{0},
		},
		{
			name: "failed and shown by another page",
			pages: []page{
				{content: "![late](late.svg)\n"},
				{files: fstest.MapFS{"late.svg": {Data: svg(20)}}, content: "![again](late.svg)\n"},
			},
			widths: []float32{20},
		},
		{
			name:   "used twice on a page",
			pages:  []page{{content: "![one](a.svg) ![two](a.svg)\n"}},
			widths: []float32{10, 10},
		},
		{
			name:   "raster",
			pages:  []page{{dir: "images", content: "![shot](shot.png)\n"}},
			widths: []float32{6},
		},
		{
			name:   "raster named as an SVG",
			pages:  []page{{dir: "images", content: "![shot](shot.svg)\n"}},
			widths: []float32{6},
		},
		{
			name:   "URL",
			pages:  []page{{content: "![url](" + server.URL + "/c.svg)\n"}},
			widths: []float32{30},
		},
		{
			name:   "SVG URL without an extension",
			pages:  []page{{content: "![build](" + server.URL + "/badge/build-passing-green)\n"}},
			widths: []float32{30},
		},
		{
			name:   "SVG of a size too large for a float32 given a height",
			pages:  []page{{content: "<img src=\"huge.svg\" height=\"64\">\n"}},
			widths: []float32{64},
		},
		{
			name:   "at the byte limit",
			pages:  []page{{content: showA}},
			widths: []float32{10},
			limit:  int64(len(small)),
		},
		{
			name:   "over the byte limit",
			pages:  []page{{content: showA}},
			widths: []float32{0},
			limit:  int64(len(small)) - 1,
		},
		{
			name:   "SVG on disk over the byte limit",
			pages:  []page{{dir: diskDir, content: showA}},
			widths: []float32{0},
			limit:  int64(len(small)) - 1,
			onDisk: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			m := NewMarkdown(false)
			own := maps.Clone(fsys)
			if !tc.onDisk {
				m.FileSystem = own
			}
			m.PerImageByteLimit = tc.limit
			m.AltLinkPrefixes = tc.alt
			var dir string
			m.WorkingDirProvider = func(_ Paneler) string { return dir }
			// loading reports whether a placeholder is waiting on an image, as one is until its load is done with.
			loading := func() bool {
				m.drawableCacheLock.Lock()
				defer m.drawableCacheLock.Unlock()
				for _, entry := range m.drawableCache {
					if len(entry.targets) != 0 {
						return true
					}
				}
				return false
			}
			for _, one := range tc.pages {
				// The next page would otherwise find a load under way rather than its outcome.
				for deadline := time.Now().Add(10 * time.Second); loading(); time.Sleep(10 * time.Millisecond) {
					if time.Now().After(deadline) {
						t.Fatal("an image is still loading")
					}
				}
				if one.fsys != nil {
					m.FileSystem = one.fsys
				}
				maps.Copy(own, one.files)
				dir = one.dir
				switch {
				case one.rebuild:
					m.Rebuild()
				case one.width != 0:
					m.SetContent(one.content, one.width)
				default:
					m.SetContent(one.content, 400)
				}
			}
			var widths []float32
			var walk func(p *Panel)
			walk = func(p *Panel) {
				if image, ok := p.Self.(*DrawablePanel); ok {
					var width float32
					if broken, isSVG := image.Drawable.(*DrawableSVG); !isSVG || broken.SVG != BrokenImageSVG {
						width = image.Drawable.LogicalSize().Width
					}
					widths = append(widths, width)
				}
				for _, child := range p.Children() {
					walk(child)
				}
			}
			walk(m.AsPanel())
			c.Equal(tc.widths, widths, "the width of each image")
		})
	}
}

// TestMarkdownIsSVG verifies that data beginning as XML is read as an SVG when encoding/xml cannot read as far as its
// first element, so that the SVG parser reports what is wrong with it, but not when that element is something else.
func TestMarkdownIsSVG(t *testing.T) {
	const root = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"/>`
	const utf16Text = `<?xml version="1.0" encoding="UTF-16"?>` + root
	// Little-endian, after its byte order mark.
	utf16 := append(make([]byte, 0, 2+2*len(utf16Text)), 0xFF, 0xFE)
	for _, b := range []byte(utf16Text) {
		utf16 = append(utf16, b, 0)
	}
	for _, tc := range []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "entity declared in the DOCTYPE used in the root tag",
			data: []byte("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<!DOCTYPE svg [\n" +
				"\t<!ENTITY ns_extend \"http://ns.adobe.com/Extensibility/1.0/\">\n]>\n" +
				`<svg xmlns:x="&ns_extend;" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"/>`),
			want: true,
		},
		{name: "XML 1.1", data: []byte(`<?xml version="1.1"?>` + root), want: true},
		{name: "UTF-16", data: utf16, want: true},
		{name: "first element not svg", data: []byte(`<?xml version="1.0"?><html/>`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check.New(t).Equal(tc.want, markdownIsSVG(tc.data))
		})
	}
}

func TestMarkdownHeadingAnchors(t *testing.T) {
	const content = "# New\n\ntext\n\n## Second Heading\n\nmore\n\n### Custom {#my-id}\n\nend\n"
	c := check.New(t)
	m := NewMarkdown(false)
	m.SetContent(content, 400)

	// Headings automatically get GitHub-style slug anchors.
	c.NotNil(m.anchors["new"])
	c.NotNil(m.anchors["second-heading"])
	// An explicit {#id} overrides the auto-generated slug.
	c.NotNil(m.anchors["my-id"])

	// ScrollToAnchor accepts a bare slug, a '#'-prefixed slug, and is case-insensitive (so a link written as "#New"
	// still resolves to the lower-cased "new" slug).
	c.True(m.ScrollToAnchor("new"))
	c.True(m.ScrollToAnchor("#new"))
	c.True(m.ScrollToAnchor("#New"))
	c.True(m.ScrollToAnchor("#Second-Heading"))
	c.True(m.ScrollToAnchor("#my-id"))

	// A URL-escaped anchor is unescaped before matching.
	c.True(m.ScrollToAnchor("#second%2Dheading"))

	// An unknown anchor reports no match so the caller can fall back to normal link handling.
	c.False(m.ScrollToAnchor("#does-not-exist"))

	// With an AnchorLinkHandler, following a link to an anchor the document has hands the anchor to it in place of
	// scrolling, while a link to one it lacks still reaches the LinkHandler.
	scroll, m := markdownInScrollPanel(content, 20)
	var anchors, links []string
	m.AnchorLinkHandler = func(anchor string) { anchors = append(anchors, anchor) }
	m.LinkHandler = func(_ Paneler, target string) { links = append(links, target) }
	m.linkHandler(m, "#my-id")
	m.linkHandler(m, "#does-not-exist")
	m.linkHandler(m, "##new") // The anchor "#new", which no heading has.
	c.Equal([]string{"my-id"}, anchors, "the anchors handed to the AnchorLinkHandler")
	c.Equal([]string{"#does-not-exist", "##new"}, links, "the links handed to the LinkHandler")
	c.Equal(float32(0), scroll.Bar(false).Value(), "the AnchorLinkHandler is called in place of scrolling")

	// An anchor that matches several IDs only when case is ignored names the first of them in the document.
	m.SetContent("# One {#API}\n\n## Api\n", 400)
	first := m.anchors["API"]
	c.NotNil(first)
	c.NotNil(m.anchors["api"])
	same := true
	for range 100 {
		same = same && m.panelForAnchor("Api") == first
	}
	c.True(same, "the same heading every time")
}

// markdownInScrollPanel builds a Markdown from the given content inside a ScrollPanel with the given viewport height,
// lays it out, and returns both so tests can assert on scroll behavior. The root of the ScrollPanel sits at (0,0), so a
// panel's root coordinates are also its coordinates relative to the visible viewport (0..viewHeight).
func markdownInScrollPanel(content string, viewHeight float32) (*ScrollPanel, *Markdown) {
	m := NewMarkdown(false)
	m.SetContent(content, 400)
	scroll := NewScrollPanel()
	scroll.SetContent(m, behavior.Fill, behavior.Fill)
	scroll.SetFrameRect(geom.NewRect(0, 0, 420, viewHeight))
	scroll.ValidateLayout()
	return scroll, m
}

// TestMarkdownScrollToAnchorAlignsTop verifies that scrolling to an anchor brings the top of the heading to the top of
// the view (revealing the section that follows it) rather than leaving it pinned to the bottom of the view, even when
// the heading is taller than the view, so it isn't reduced to showing only its last "bit". A heading too near the end
// of the document to be pulled to the top is instead brought fully into view, the scroll clamped to the end.
func TestMarkdownScrollToAnchorAlignsTop(t *testing.T) {
	lines := func(n int, suffix string) string {
		var sb strings.Builder
		for i := range n {
			fmt.Fprintf(&sb, "line %d%s\n\n", i, suffix)
		}
		return sb.String()
	}
	for _, tc := range []struct {
		name       string
		content    string
		anchor     string // Given to ScrollToAnchor; slug is the key it resolves to.
		slug       string
		viewHeight float32
		taller     bool // The heading is taller than the view.
		nearEnd    bool // The content below the heading is too short for it to reach the top of the view.
	}{
		{
			name: "heading in the middle",
			content: "# Top\n\n" + lines(40, " of the top section") + "## Target Heading\n\n" +
				lines(40, " of the target section"),
			anchor:     "#Target-Heading",
			slug:       "target-heading",
			viewHeight: 300,
		},
		{
			name:       "heading taller than the view",
			content:    "# Top\n\n" + lines(40, "") + "## Target Heading\n\nbody\n",
			anchor:     "target-heading",
			slug:       "target-heading",
			viewHeight: 10,
			taller:     true,
		},
		{
			name:       "heading near the end",
			content:    "# Top\n\n" + lines(60, "") + "## Last\n\ntail\n",
			anchor:     "last",
			slug:       "last",
			viewHeight: 300,
			nearEnd:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			scroll, m := markdownInScrollPanel(tc.content, tc.viewHeight)
			p := m.anchors[tc.slug]
			c.NotNil(p)
			c.True(m.ScrollToAnchor(tc.anchor))
			rect := p.RectToRoot(p.ContentRect(true))
			if tc.nearEnd {
				c.True(rect.Y >= 0, "the heading is fully visible")
				bar := scroll.Bar(false)
				c.Equal(bar.Max(), bar.Value()+bar.Extent(), "the view is scrolled as far as the content allows")
			} else {
				c.Equal(float32(0), rect.Y, "the heading's top is brought to the top of the view")
			}
			if !tc.taller {
				c.True(rect.Bottom() <= tc.viewHeight, "the whole heading fits within the view")
			}
		})
	}
}

func TestMarkdownAnchorsResetOnNewContent(t *testing.T) {
	c := check.New(t)
	m := NewMarkdown(false)
	m.SetContent("# First\n", 400)
	c.True(m.ScrollToAnchor("#first"))
	// Replacing the content must clear stale anchors from the prior content.
	m.SetContent("# Second\n", 400)
	c.False(m.ScrollToAnchor("#first"))
	c.True(m.ScrollToAnchor("#second"))
}

func TestMarkdownEmptyContent(t *testing.T) {
	c := check.New(t)
	m := NewMarkdown(false)
	m.SetContent("", 400)
	// Empty content must not panic and must leave no rendered text.
	c.Equal("", collectMarkdownText(m.AsPanel()))
}

// TestMarkdownFitsMaxWidth verifies that content, including a list's or a quote's, is no wider than the width it was
// wrapped to, even a fractional one. The image wide.png is wider than any of those widths.
func TestMarkdownFitsMaxWidth(t *testing.T) {
	const words = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et " +
		"dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip.\n"
	wide := &DrawableSVG{SVG: BrokenImageSVG, Size: geom.NewSize(600, 40)}
	wideTarget, err := ReviseTarget("", "wide.png", nil)
	check.New(t).NoError(err)
	wideKey := markdownImageKey{target: wideTarget}
	for _, tc := range []struct {
		name    string
		content string
		// lateImage says wide.png arrives after the document has been built.
		lateImage bool
	}{
		{name: "paragraph", content: words},
		{name: "words two spaces apart", content: strings.ReplaceAll(words, " ", "  ")},
		{name: "words em spaces apart", content: strings.ReplaceAll(words, " ", "\u2003")},
		{name: "long word joined to a link", content: strings.ReplaceAll(words, "t, ", "t [link](u)longwordlongword ")},
		{name: "link wider than the line", content: "[This is a fairly long link title that will not fit in the width](u)\n"},
		{
			name:    "link wider than the line after text",
			content: "Some text [This is a fairly long link title that will not fit in the width](u)\n",
		},
		{name: "bare URL", content: "https://example.com/aaaaaaaaaaaaaaaaaaaa/bbbbbbbbbbbbbbbbbbbbbbb/cccccccccccccccc\n"},
		{name: "links between words", content: "x [y](u) zzz [link](u) and some more words to wrap [another](u) end.\n"},
		{name: "quote", content: "> " + words},
		{name: "item after a task", content: "- [ ] task\n- " + words},
		{name: "unordered", content: "- " + words},
		{name: "nested as the first thing in an item", content: "- - " + words},
		{name: "numbered from 1000", content: "1000. " + words},
		{name: "numbered from 100000", content: "100000. " + words},
		{name: "image in a list item", content: "- ![wide](wide.png)\n"},
		{name: "image in a quote", content: "> ![wide](wide.png)\n"},
		{name: "image arriving late in a list item", content: "- ![wide](wide.png)\n", lateImage: true},
		{
			name: "table with a word that cannot be broken",
			content: "| a | b |\n| - | - |\n" +
				"| [link in a cell](u) words and some more words to wrap | Supercalifragilistic |\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			for width := float32(200); width <= 400; width += 0.5 {
				m := NewMarkdown(false)
				if tc.lateImage {
					m.drawableCache[wideKey] = &drawableCacheEntry{}
				} else {
					m.drawableCache[wideKey] = &drawableCacheEntry{drawable: wide}
				}
				m.SetContent(tc.content, width)
				if tc.lateImage {
					m.updateDrawable(wideKey, wide)
				}
				_, prefSize, _ := m.Sizes(geom.Size{})
				c.True(prefSize.Width <= width, "wrapped to %v, the content prefers %v", width, prefSize.Width)
			}
		})
	}
}

// TestMarkdownImageKeepsMinimumWidth verifies that an image scaled down for a line with little or no room is kept as
// wide as the placeholder shown while it loads, and that one narrower than that is not enlarged.
func TestMarkdownImageKeepsMinimumWidth(t *testing.T) {
	const table = "| a | b |\n| - | - |\n| ![img](img.png) | Supercalifragilisticexpialidocious |\n"
	const nested = "- - - - - - - - - - - - ![img](img.png)\n"
	wide := &DrawableSVG{SVG: BrokenImageSVG, Size: geom.NewSize(600, 40)}
	target, err := ReviseTarget("", "img.png", nil)
	check.New(t).NoError(err)
	key := markdownImageKey{target: target}
	minimum := max(DefaultMarkdownTheme.Font.Size(), 24)
	for _, tc := range []struct {
		image   Drawable
		name    string
		content string
		width   float32
		want    float32
		// late says the image arrives after the document has been built.
		late bool
	}{
		{name: "in a table that cannot fit", content: table, image: wide, width: 150, want: minimum},
		{
			name:    "arriving late in a table that cannot fit",
			content: table,
			image:   wide,
			width:   150,
			want:    minimum,
			late:    true,
		},
		{name: "in a list item nested past the width", content: nested, image: wide, width: 200, want: minimum},
		{
			name:    "narrower than the minimum",
			content: nested,
			image:   &DrawableSVG{SVG: BrokenImageSVG, Size: geom.NewSize(10, 10)},
			width:   200,
			want:    10,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			m := NewMarkdown(false)
			if tc.late {
				m.drawableCache[key] = &drawableCacheEntry{}
			} else {
				m.drawableCache[key] = &drawableCacheEntry{drawable: tc.image}
			}
			m.SetContent(tc.content, tc.width)
			if tc.late {
				m.updateDrawable(key, tc.image)
			}
			var image *DrawablePanel
			var find func(p *Panel)
			find = func(p *Panel) {
				for _, child := range p.Children() {
					if one, ok := child.Self.(*DrawablePanel); ok && one.Accessibility.Name == "img" {
						image = one
					}
					find(child)
				}
			}
			find(m.AsPanel())
			c.NotNil(image)
			if image != nil {
				c.Equal(tc.want, image.Drawable.LogicalSize().Width, "the image's width")
			}
		})
	}
}

// TestMarkdownAlertMarkerMustBeginQuote verifies that a quote is an alert only when its marker begins it: one after
// anything else the quote shows, or within a block other than a paragraph, is shown as written. A quote that is an
// alert stays one whatever follows its marker.
func TestMarkdownAlertMarkerMustBeginQuote(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		alert   string // The name an alert is given, or "" for a quote that is not one.
	}{
		{name: "hard line break after the marker", content: "> [!NOTE]\n> one\\\n> two\n", alert: "Note"},
		{name: "image", content: "> ![x](missing-image-for-test.png)\n>\n> [!NOTE]\n> text\n"},
		{name: "img tag", content: "> <img src=\"missing-image-for-test.png\">\n>\n> [!NOTE]\n> text\n"},
		{name: "code block", content: "> ```\n> code\n> ```\n>\n> [!NOTE]\n> text\n"},
		{name: "thematic break", content: "> ***\n>\n> [!NOTE]\n> text\n"},
		{name: "nested quote", content: "> > quoted\n>\n> [!NOTE]\n> text\n"},
		{name: "heading beginning with the marker", content: "> # [!NOTE]\n> text\n"},
		{name: "list beginning with the marker", content: "> - [!NOTE]\n>   text\n"},
		{name: "table beginning with the marker", content: "> | [!NOTE] |\n> | --- |\n> | text |\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			m := NewMarkdown(false)
			m.SetContent(tc.content, 400)
			var quote *Panel
			var find func(p *Panel)
			find = func(p *Panel) {
				for _, child := range p.Children() {
					if quote == nil && child.Accessibility.Role == role.BlockQuote {
						quote = child
					}
					find(child)
				}
			}
			find(m.AsPanel())
			c.NotNil(quote)
			if quote != nil {
				c.Equal(tc.alert, quote.Accessibility.Name, "the name of the alert the quote is")
				if tc.alert == "" {
					c.Contains(collectMarkdownText(quote), "[!NOTE] text", "the marker is shown as written")
				}
			}
		})
	}
}

// TestMarkdownImgTagImages verifies which images <img> tags show and the size of each: the one their width and height
// give, the one given alone keeping the image's proportions, then scaled down to fit the line and to be no taller than
// markdownMaxImageSize. Each case is shown with a vector and a raster image, both 200 by 100, unless it has an image of
// its own, and with each to hand and arriving late.
func TestMarkdownImgTagImages(t *testing.T) {
	codecs.Register() // normally done at app startup, which these tests skip
	setup := check.New(t)
	raster, err := NewImageFromBytes(distinctPNGForTest(setup, 200, 100, 151), geom.NewPoint(1, 1))
	setup.NoError(err)
	defer raster.Dispose()
	vector := &DrawableSVG{SVG: BrokenImageSVG, Size: geom.NewSize(200, 100)}
	target, err := ReviseTarget("", "img.png", nil)
	setup.NoError(err)
	key := markdownImageKey{target: target}
	for _, tc := range []struct {
		image   Drawable
		name    string
		content string
		sizes   []geom.Size
	}{
		{name: "neither width nor height", content: `<img src="img.png">`, sizes: []geom.Size{geom.NewSize(200, 100)}},
		{name: "width", content: `<img src="img.png" width="50">`, sizes: []geom.Size{geom.NewSize(50, 25)}},
		{name: "height", content: `<img src="img.png" height="50">`, sizes: []geom.Size{geom.NewSize(100, 50)}},
		{
			name:    "width and height",
			content: `<img src="img.png" width="30" height="70">`,
			sizes:   []geom.Size{geom.NewSize(30, 70)},
		},
		{
			name:    "values that are not plain numbers",
			content: `<img src="img.png" width="50%" height="1e1">`,
			sizes:   []geom.Size{geom.NewSize(200, 100)},
		},
		{name: "wider than the line", content: `<img src="img.png" width="800">`, sizes: []geom.Size{geom.NewSize(400, 200)}},
		{
			// One height is the largest float32, the other past it.
			name: "heights of any size",
			content: "<img src=\"img.png\" height=\"340282350000000000000000000000000000000\">\n" +
				"<img src=\"img.png\" height=\"1" + strings.Repeat("0", 40) + "\">",
			sizes: []geom.Size{geom.NewSize(400, 200), geom.NewSize(400, 200)},
		},
		{
			name:    "taller than the most shown",
			content: `<img src="img.png">`,
			image:   &DrawableSVG{SVG: BrokenImageSVG, Size: geom.NewSize(100, 1<<25)},
			sizes:   []geom.Size{geom.NewSize(50, 1<<24)},
		},
		{
			// 50 / 1e-38 is past the largest float32.
			name:    "width of an image with no height and almost no width",
			content: `<img src="img.png" width="50">`,
			image:   &DrawableSVG{SVG: BrokenImageSVG, Size: geom.NewSize(1e-38, 0)},
			sizes:   []geom.Size{geom.NewSize(50, 0)},
		},
		{
			name:    "height of an image with no width and almost no height",
			content: `<img src="img.png" height="50">`,
			image:   &DrawableSVG{SVG: BrokenImageSVG, Size: geom.NewSize(0, 1e-38)},
			sizes:   []geom.Size{geom.NewSize(0, 50)},
		},
		{
			name:    "within text",
			content: `Before <img src="img.png" width="50"> after.`,
			sizes:   []geom.Size{geom.NewSize(50, 25)},
		},
		{
			name:    "block of the same image at two sizes",
			content: "<img src=\"img.png\" width=\"50\">\n<img src=\"img.png\" height=\"20\">",
			sizes:   []geom.Size{geom.NewSize(50, 25), geom.NewSize(40, 20)},
		},
		{
			name:    "no src or an empty one",
			content: "<img alt=\"x\">\n<img src=\"img.png\" width=\"50\">\n<img src=\"\" alt=\"y\">\n\nBefore <img alt=\"z\"> after.",
			sizes:   []geom.Size{geom.NewSize(50, 25)},
		},
		{name: "block with text", content: "<img src=\"img.png\">\ntext"},
		{name: "block with other markup", content: "<img src=\"img.png\">\n<br>"},
		{name: "block ending in an unfinished tag", content: "<img src=\"img.png\">\n<img src=\"img.png\""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			images := []Drawable{vector, raster}
			if tc.image != nil {
				images = []Drawable{tc.image}
			}
			for _, image := range images {
				for _, late := range []bool{false, true} {
					m := NewMarkdown(false)
					if late {
						m.drawableCache[key] = &drawableCacheEntry{}
					} else {
						m.drawableCache[key] = &drawableCacheEntry{drawable: image}
					}
					m.SetContent(tc.content+"\n", 400)
					if late {
						m.updateDrawable(key, image)
					}
					var sizes []geom.Size
					var walk func(p *Panel)
					walk = func(p *Panel) {
						if one, ok := p.Self.(*DrawablePanel); ok {
							sizes = append(sizes, one.Drawable.LogicalSize())
						}
						for _, child := range p.Children() {
							walk(child)
						}
					}
					walk(m.AsPanel())
					c.Equal(tc.sizes, sizes, "the size of each %T image, arriving late: %v", image, late)
				}
			}
		})
	}
}

// TestMarkdownBreaksTextIntoRows verifies what each row draws once text has been broken to fit, with an image drawn as
// U+FFFC, and the lines the blocks are read as, which hold every rune that was written, each row as one line.
func TestMarkdownBreaksTextIntoRows(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		rows    []string
		lines   []string
		width   float32
	}{
		{
			name:    "word wider than the line",
			content: "Supercalifragilisticexpialidocious\n",
			width:   100,
			rows:    []string{"Supercalifragilisticexpialidocious"},
			lines:   []string{"Supercalifragilisticexpialidocious"},
		},
		{
			name:    "only a trailing space past the width before a break",
			content: "aaa bbb <br>ddd\n",
			width:   52,
			rows:    []string{"aaa bbb", "ddd"},
			lines:   []string{"aaa bbb \n", "ddd"},
		},
		{
			name:    "only a trailing space past the width at the end",
			content: "Some words here <span></span>\n",
			width:   111,
			rows:    []string{"Some words here"},
			lines:   []string{"Some words here "},
		},
		{name: "line narrower than a W", content: "word\n", width: 10, rows: []string{"word"}, lines: []string{"word"}},
		{
			name:    "item nested deeper than a W leaves room for",
			content: "- - - - - - - - - - - - deep\n",
			width:   200,
			rows:    []string{"deep"},
			lines:   []string{"deep"},
		},
		{
			name:    "image wider than the line",
			content: "![x](missing-image-for-test.png)\n",
			width:   10,
			rows:    []string{"\uFFFC"},
			lines:   []string{"\uFFFC"},
		},
		{
			name:    "block of img tags apart and together",
			content: "<img src=\"missing-image-for-test.png\">\n<img src=\"a.png\"><img src=\"b.png\">\n",
			width:   400,
			rows:    []string{"￼ ￼￼"},
			lines:   []string{"￼ ￼￼"},
		},
		{
			name:    "block of img tags apart with one without src between",
			content: "<img src=\"missing-image-for-test.png\">\n<img alt=\"x\"><img src=\"b.png\">\n",
			width:   400,
			rows:    []string{"￼ ￼"},
			lines:   []string{"￼ ￼"},
		},
		{name: "block of img tags without src", content: "<img alt=\"x\">\n<img src=\"\">\n", width: 400},
		{
			name:    "link wider than the line",
			content: "[This is a fairly long link title that will not fit in the width](u)\n",
			width:   200,
			rows:    []string{"This is a fairly long link title that", "will not fit in the width"},
			lines:   []string{"This is a fairly long link title that ", "will not fit in the width"},
		},
		{
			name:    "words exactly as wide as a wrapped line",
			content: "take could world against\n",
			width:   84,
			rows:    []string{"take could", "world against"},
			lines:   []string{"take could ", "world against"},
		},
		{
			name:    "space after a break",
			content: "Line one<br> Line two\n",
			width:   400,
			rows:    []string{"Line one", "Line two"},
			lines:   []string{"Line one\n", " Line two"},
		},
		{
			name:    "spaces around a break",
			content: "a <br> b\n",
			width:   400,
			rows:    []string{"a", "b"},
			lines:   []string{"a \n", " b"},
		},
		{
			name:    "spaces after a break split between text and a link",
			content: "a<br> [ y](u) b\n",
			width:   400,
			rows:    []string{"a", "y b"},
			lines:   []string{"a\n", "  y b"},
		},
		{
			name:    "line endings directly after a break and after text following one",
			content: "a<br>\nb<br>c\nd\n",
			width:   400,
			rows:    []string{"a", "b", "c d"},
			lines:   []string{"a\n", "b\n", "c d"},
		},
		{
			name:    "break written over two lines",
			content: "a<br\n/>b\n",
			width:   400,
			rows:    []string{"a", "b"},
			lines:   []string{"a\n", "b"},
		},
		{
			name:    "rule written over two lines",
			content: "a<hr\n/>b\n",
			width:   400,
			rows:    []string{"a", "", "b"},
			lines:   []string{"a\n", "\n", "b"},
		},
		{
			name:    "space after a break on a narrow line",
			content: "Line one <br> Line ten\n",
			width:   25,
			rows:    []string{"Line", "one", "Line", "ten"},
			lines:   []string{"Line ", "one \n", " Line ", "ten"},
		},
		{
			name:    "text narrower than a W in what is left of the line",
			content: "Some words [link](u).\n",
			width:   110,
			rows:    []string{"Some words link."},
			lines:   []string{"Some words link."},
		},
		{
			name:    "text narrower than a W in what is left of a wrapped line",
			content: "Some words [link](u).\n",
			width:   73,
			rows:    []string{"Some", "words link."},
			lines:   []string{"Some ", "words link."},
		},
		{
			name:    "space between links before a link that does not fit",
			content: "[aaaa](u) [bbbbbbbbbbbbbbbbbb](u) [cc](u) [dddd dddd](u) e\n",
			width:   200,
			rows:    []string{"aaaa bbbbbbbbbbbbbbbbbb cc", "dddd dddd e"},
			lines:   []string{"aaaa bbbbbbbbbbbbbbbbbb cc ", "dddd dddd e"},
		},
		{
			name:    "space before a word wider than the line after a link",
			content: "Text [averylonglink](u) incomprehensibilities after.\n",
			width:   60,
			rows:    []string{"Text", "averylonglink", "incomprehensibilities", "after."},
			lines:   []string{"Text ", "averylonglink ", "incomprehensibilities ", "after."},
		},
		{
			name:    "space past the width before a break",
			content: "[aaaa](u) <br>b\n",
			width:   30,
			rows:    []string{"aaaa", "b"},
			lines:   []string{"aaaa \n", "b"},
		},
		{
			name:    "two breaks after a space past the width",
			content: "[aaaa](u) <br><br>b\n",
			width:   30,
			rows:    []string{"aaaa", "", "b"},
			lines:   []string{"aaaa \n", "\n", "b"},
		},
		{
			name:    "space after a link that fills the line",
			content: "Text [averylonglink](u) more words after.\n",
			width:   113,
			rows:    []string{"Text averylonglink", "more words after."},
			lines:   []string{"Text averylonglink ", "more words after."},
		},
		{
			name:    "space after an empty link beginning a line",
			content: "[](u) after\n",
			width:   400,
			rows:    []string{"after"},
			lines:   []string{" after"},
		},
		{
			name:    "image wider than the line after an empty link",
			content: "[](u)![x](missing-image-for-test.png)\n",
			width:   10,
			rows:    []string{"\uFFFC"},
			lines:   []string{"\uFFFC"},
		},
		{
			name:    "word wider than the line after an empty link",
			content: "[](u)Antidisestablishmentarianism\n",
			width:   100,
			rows:    []string{"Antidisestablishmentarianism"},
			lines:   []string{"Antidisestablishmentarianism"},
		},
		{
			name:    "space after a link of a space beginning a line",
			content: "a<br>[ ](u) b\n",
			width:   400,
			rows:    []string{"a", "b"},
			lines:   []string{"a\n", "  b"},
		},
		{
			name:    "code spans as written",
			content: "`C:\\\\dir` `\\.` `a\\*b` `&lt;T&gt;` `&#42;` [`\\*`](u)\n",
			width:   400,
			rows:    []string{"C:\\\\dir \\. a\\*b &lt;T&gt; &#42; \\*"},
			lines:   []string{"C:\\\\dir \\. a\\*b &lt;T&gt; &#42; \\*"},
		},
		{
			name:    "code span continued on the next line",
			content: "x `a\nb` y\n",
			width:   400,
			rows:    []string{"x a b y"},
			lines:   []string{"x a b y"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			m := NewMarkdown(false)
			m.SetContent(tc.content, tc.width)
			_, prefSize, _ := m.Sizes(geom.Size{})
			m.SetFrameRect(geom.NewRect(0, 0, max(tc.width, prefSize.Width), prefSize.Height))
			m.ValidateLayout()
			rows := make([]string, 0, len(tc.rows))
			for _, row := range axMarkdownRows(m.AsPanel()) {
				var drawn strings.Builder
				for _, child := range row.Children() {
					switch one := child.Self.(type) {
					case *Label:
						drawn.WriteString(one.String())
					case *DrawablePanel:
						drawn.WriteRune('\uFFFC')
					}
				}
				rows = append(rows, drawn.String())
			}
			c.Equal(fmt.Sprintf("%q", tc.rows), fmt.Sprintf("%q", rows))
			lines := make([]string, 0, len(tc.lines))
			for _, block := range axMarkdownBlocks(m.AsPanel()) {
				text := block.axText()
				for _, line := range text.lines {
					lines = append(lines, string(text.text[line.Start:line.End]))
				}
			}
			c.Equal(fmt.Sprintf("%q", tc.lines), fmt.Sprintf("%q", lines))
		})
	}
}

// TestMarkdownLinkWithNothingToDrawIsALink verifies that a link is added however little of it there is to draw, as with
// one of nothing but spaces where they begin a line and are not drawn, and that the text still reads as written.
func TestMarkdownLinkWithNothingToDrawIsALink(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		text    string
	}{
		{name: "no text", content: "[](u) after\n", text: " after"},
		{name: "space beginning a line", content: "[ ](u) after\n", text: "  after"},
		{name: "space within a line", content: "before [ ](u) after\n", text: "before   after"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			m := NewMarkdown(false)
			m.SetContent(tc.content, 400)
			_, prefSize, _ := m.Sizes(geom.Size{})
			m.SetFrameRect(geom.NewRect(0, 0, 400, prefSize.Height))
			m.ValidateLayout()
			links := 0
			var text strings.Builder
			for _, block := range axMarkdownBlocks(m.AsPanel()) {
				for _, row := range axMarkdownRows(block.AsPanel()) {
					for _, child := range row.Children() {
						if child.Accessibility.Role == role.Link {
							links++
						}
					}
				}
				text.WriteString(string(block.axText().text))
			}
			c.Equal(1, links, "the link should have been added")
			c.Equal(tc.text, text.String(), "the text should read as written")
		})
	}
}

// TestMarkdownImageInLinkIsPartOfIt verifies that an image within a link is drawn as a piece of the link: a click on it
// follows the link, and the link, which is the image itself when nothing comes before it, occupies the image and every
// word with it in the block's text.
func TestMarkdownImageInLinkIsPartOfIt(t *testing.T) {
	const target = "http://e.com"
	for _, tc := range []struct {
		name    string
		content string
		row     string
	}{
		{name: "image alone", content: "[![alt](missing-image-for-test.png)](" + target + ")\n", row: "\uFFFC"},
		{name: "image before text", content: "[![alt](missing-image-for-test.png) text](" + target + ")\n", row: "\uFFFC text"},
		{name: "image after text", content: "[text ![alt](missing-image-for-test.png)](" + target + ")\n", row: "text \uFFFC"},
		{name: "img tag", content: "[<img src=\"missing-image-for-test.png\" alt=\"alt\">](" + target + ")\n", row: "\uFFFC"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			m := NewMarkdown(false)
			var followed string
			m.LinkHandler = func(_ Paneler, to string) { followed = to }
			m.SetContent(tc.content, 400)
			_, prefSize, _ := m.Sizes(geom.Size{})
			m.SetFrameRect(geom.NewRect(0, 0, 400, prefSize.Height))
			m.ValidateLayout()
			var drawn strings.Builder
			var image *Panel
			var links []*Panel
			for _, row := range axMarkdownRows(m.AsPanel()) {
				for _, child := range row.Children() {
					if child.Accessibility.Role == role.Link {
						links = append(links, child)
					}
					switch one := child.Self.(type) {
					case *Label:
						drawn.WriteString(one.String())
					case *DrawablePanel:
						drawn.WriteRune('\uFFFC')
						image = child
					}
				}
			}
			c.Equal(tc.row, drawn.String(), "what is drawn")
			c.Equal(1, len(links), "the pieces of the link are one link")
			if image == nil || len(links) != 1 {
				return
			}
			c.Equal(target, links[0].Accessibility.URL)
			text := axMarkdownBlocks(m.AsPanel())[0].axText()
			var covered string
			for _, span := range text.spans {
				if span.panel == links[0] {
					covered = string(text.text[span.start:span.end])
				}
				c.True(span.panel.Accessibility.Role != role.None, "a span names only an element that is described")
			}
			c.Equal(tc.row, covered, "the link occupies the whole of itself")
			c.True(image.MouseDownCallback != nil && image.MouseUpCallback != nil, "the image can be clicked")
			if image.MouseDownCallback != nil && image.MouseUpCallback != nil {
				where := image.ContentRect(true).Center()
				image.MouseDownCallback(where, ButtonLeft, 1, 0)
				image.MouseUpCallback(where, ButtonLeft, 0)
			}
			c.Equal(target, followed, "a click on the image follows the link")
		})
	}
}

// TestMarkdownBreakingTextScalesLinearly verifies, by counting allocations, that breaking a paragraph into rows costs
// in proportion to its length rather than to its square.
func TestMarkdownBreakingTextScalesLinearly(t *testing.T) {
	allocs := func(words int) float64 {
		content := strings.Repeat("lorem ipsum dolor sit amet ", words/5) + "\n"
		return testing.AllocsPerRun(1, func() { NewMarkdown(false).SetContent(content, 100) })
	}
	short, long := allocs(1000), allocs(4000)
	check.New(t).True(long < 6*short, "four times the words took %v allocations against %v", long, short)
}

// TestMarkdownHeadingBeginningContainerHasNoMarginAbove verifies that the text of a heading that begins a list item or
// a quote is at the top of the container's content, where a list item's bullet is, rather than below a margin.
func TestMarkdownHeadingBeginningContainerHasNoMarginAbove(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "list item", content: "- # big\n"},
		{name: "quote", content: "> # Title\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			m := NewMarkdown(false)
			m.SetContent(tc.content, 400)
			_, prefSize, _ := m.Sizes(geom.Size{})
			m.SetFrameRect(geom.NewRect(0, 0, 400, prefSize.Height))
			m.ValidateLayout()
			rows := axMarkdownRows(m.AsPanel())
			c.Equal(1, len(rows))
			if len(rows) != 1 {
				return
			}
			container := rows[0].Parent()
			for container != nil {
				if _, ok := container.Self.(*markdownContainer); ok {
					break
				}
				container = container.Parent()
			}
			c.NotNil(container)
			if container == nil {
				return
			}
			c.Equal(container.RectTo(container.ContentRect(false), m.AsPanel()).Y,
				rows[0].RectTo(rows[0].ContentRect(true), m.AsPanel()).Y, "the heading's text is at the top")
		})
	}
}

// TestMarkdownRuleBeginningContainerPlacement verifies that a thematic break that begins the document or a quote has no
// margin above it, while one that begins a list item stays level with the middle of the item's bullet.
func TestMarkdownRuleBeginningContainerPlacement(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "document", content: "---\n"},
		{name: "quote", content: "> ---\n"},
		{name: "list item", content: "- ***\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			m := NewMarkdown(false)
			m.SetContent(tc.content, 400)
			_, prefSize, _ := m.Sizes(geom.Size{})
			m.SetFrameRect(geom.NewRect(0, 0, 400, prefSize.Height))
			m.ValidateLayout()
			var rule *Panel
			var find func(p *Panel)
			find = func(p *Panel) {
				for _, child := range p.Children() {
					if _, ok := child.Self.(*Separator); ok && rule == nil {
						rule = child
					}
					find(child)
				}
			}
			find(m.AsPanel())
			c.NotNil(rule)
			if rule == nil {
				return
			}
			ruleRect := rule.RectTo(rule.ContentRect(false), m.AsPanel())
			container := rule.Parent()
			if container.ClientData()[markdownListItemKey] == true {
				bullet := container.Parent().Children()[0]
				bulletRect := bullet.RectTo(bullet.ContentRect(true), m.AsPanel())
				c.True(ruleRect.Y > bulletRect.Y+bulletRect.Height/4 && ruleRect.Bottom() < bulletRect.Bottom()-
					bulletRect.Height/4, "the rule at %v is beside the middle of the bullet at %v", ruleRect, bulletRect)
				return
			}
			c.Equal(container.RectTo(container.ContentRect(false), m.AsPanel()).Y, ruleRect.Y, "the rule is at the top")
		})
	}
}

// TestMarkdownListItemSpacing verifies that the blocks of a loose list, which its source sets apart with blank lines,
// are as far apart as paragraphs are, that those of a tight list have no space between them, and that the last item of
// either adds no space below the list's own margin.
func TestMarkdownListItemSpacing(t *testing.T) {
	layout := func(content string) (tops []float32, rowHeight, height float32) {
		m := NewMarkdown(false)
		m.SetContent(content, 400)
		_, prefSize, _ := m.Sizes(geom.Size{})
		m.SetFrameRect(geom.NewRect(0, 0, 400, prefSize.Height))
		m.ValidateLayout()
		for _, row := range axMarkdownRows(m.AsPanel()) {
			r := row.RectTo(row.ContentRect(true), m.AsPanel())
			tops = append(tops, r.Y)
			rowHeight = r.Height
		}
		return tops, rowHeight, prefSize.Height
	}
	paragraphs, rowHeight, _ := layout("a\n\nb\n")
	apart := paragraphs[1] - paragraphs[0]
	margin := apart - rowHeight
	for _, tc := range []struct {
		name    string
		content string
		loose   bool
	}{
		{name: "tight", content: "- a\n- b\n- c\n"},
		{name: "loose", content: "- a\n\n- b\n\n- c\n", loose: true},
		{name: "tight with a nested list", content: "- a\n  - x\n- b\n"},
		{name: "loose with a nested list", content: "- a\n\n  - x\n\n- b\n", loose: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			tops, _, height := layout(tc.content)
			want := rowHeight
			if tc.loose {
				want = apart
			}
			for i := 1; i < len(tops); i++ {
				c.Equal(want, tops[i]-tops[i-1], "the distance from row %d to row %d", i-1, i)
			}
			c.Equal(margin, height-(tops[len(tops)-1]+rowHeight), "the space below the last row")
		})
	}
}

// TestMarkdownTableDoesNotWrapWhenItFits guards against regressing the table column-sizing logic so that a table whose
// natural width fits within the available width is left at full width rather than having its columns shrunk (which
// would force premature text wrapping). The column widths are only reduced below the available width when the table's
// natural width actually exceeds it.
func TestMarkdownTableDoesNotWrapWhenItFits(t *testing.T) {
	c := check.New(t)
	const content = "| one | two | three | four | five | six | seven | eight | nine | ten |\n" +
		"|-|-|-|-|-|-|-|-|-|-|\n" +
		"| some text | some more text | ccc | ddd | eee/eee | fff | ggg | hhh | iii | jjj |\n"

	// At a generous width the table fits easily, so no column should be shrunk below the available width.
	m := NewMarkdown(false)
	m.SetContent(content, 800)
	for i, w := range m.columnWidths {
		c.Equal(800, w, "column %d should not be shrunk when the table fits", i)
	}

	// At a width narrower than the table's natural width, columns must be shrunk so the table fits.
	m = NewMarkdown(false)
	m.SetContent(content, 200)
	total := 0
	for _, w := range m.columnWidths {
		c.True(w < 200, "columns should be shrunk when the table cannot fit")
		total += w
	}
	c.True(total <= 200, "shrunk columns must sum to no more than the available width")
}

// TestMarkdownRetrieveImageQueriesDisplayOnCallingGoroutine verifies that retrieveImage queries the primary display
// synchronously on the calling goroutine (the UI thread in production) rather than from the background image-loading
// goroutine it spawns. Querying displays off the UI thread is unsafe: on macOS the lookup calls into AppKit, which
// only permits access from the main thread, and the platform implementations rely on the UI thread for
// synchronization.
func TestMarkdownRetrieveImageQueriesDisplayOnCallingGoroutine(t *testing.T) {
	saved := markdownPrimaryDisplay
	defer func() { markdownPrimaryDisplay = saved }()
	entered := make(chan struct{})
	release := make(chan struct{})
	markdownPrimaryDisplay = func() *Display {
		close(entered)
		<-release
		return nil
	}
	m := NewMarkdown(false)
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.retrieveImage("missing-image-for-test.png",
			markdownImageTarget{panel: NewDrawablePanel(), maxWidth: DefaultMarkdownWidth})
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("retrieveImage never queried the display")
	}
	// retrieveImage waits at most one second for the image fetch before giving up and returning nil. If the display
	// query were still being made from the image-loading goroutine, retrieveImage would therefore return while the
	// blocked query keeps that goroutine stuck; when the query is made on the calling goroutine, retrieveImage cannot
	// return until the query has been released. Wait longer than that internal timeout to tell the two apart.
	select {
	case <-done:
		t.Fatal("retrieveImage returned while the display query was still blocked; it must query the display on the calling goroutine")
	case <-time.After(1500 * time.Millisecond):
	}
	close(release)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("retrieveImage did not return after the display query completed")
	}
}

// TestReviseTarget verifies how targets are turned into links or absolute paths. An already-absolute local target used
// to be joined onto the working directory anyway, turning "/tmp/img.png" into "<workingDir>/tmp/img.png", which then
// failed to load and rendered as the broken-image placeholder.
func TestReviseTarget(t *testing.T) {
	c := check.New(t)
	workingDir := t.TempDir()

	// URLs pass through untouched.
	revised, err := ReviseTarget(workingDir, "https://example.com/a.png", nil)
	c.NoError(err)
	c.Equal("https://example.com/a.png", revised)

	// So do targets matching one of the alternate link prefixes.
	revised, err = ReviseTarget(workingDir, "mailto:someone@example.com", []string{"mailto:"})
	c.NoError(err)
	c.Equal("mailto:someone@example.com", revised)

	// A relative target is resolved against the working directory, after URL unescaping.
	revised, err = ReviseTarget(workingDir, "sub/img%20one.png", nil)
	c.NoError(err)
	c.Equal(filepath.Join(workingDir, "sub", "img one.png"), revised)

	// An absolute target is left where it is rather than being buried under the working directory.
	abs := filepath.Join(workingDir, "abs.png")
	revised, err = ReviseTarget(filepath.Join(workingDir, "elsewhere"), abs, nil)
	c.NoError(err)
	c.Equal(abs, revised)

	// The same holds when the absolute target arrives URL-escaped.
	absWithSpace := filepath.Join(workingDir, "abs two.png")
	revised, err = ReviseTarget(workingDir, filepath.ToSlash(filepath.Join(workingDir, "abs%20two.png")), nil)
	c.NoError(err)
	c.Equal(absWithSpace, revised)
}

// TestReviseFSTarget verifies how targets are turned into links or paths within a file system.
func TestReviseFSTarget(t *testing.T) {
	const dir = "Scripting"
	for _, tc := range []struct {
		name       string
		workingDir string
		target     string
		want       string
		alt        []string
		invalid    bool
	}{
		{
			name:       "relative and unescaped",
			workingDir: dir,
			target:     "img%20one.png",
			want:       "Scripting/img one.png",
		},
		{
			name:       "up out of the working directory",
			workingDir: dir,
			target:     "../images/a.svg",
			want:       "images/a.svg",
		},
		{name: "dot is the root", workingDir: ".", target: "a.svg", want: "a.svg"},
		{name: "rooted", workingDir: dir, target: "/images/a.svg", want: "images/a.svg"},
		{name: "above the root", target: "../a.svg", invalid: true},
		{
			name:       "URL",
			workingDir: dir,
			target:     "https://example.com/a%20b.png",
			want:       "https://example.com/a%20b.png",
		},
		{name: "alt prefix", workingDir: dir, target: "md:Page%20One", want: "md:Page One", alt: []string{"md:"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			revised, err := ReviseFSTarget(tc.workingDir, tc.target, tc.alt)
			if tc.invalid {
				c.True(errors.Is(err, fs.ErrInvalid), "an error wrapping fs.ErrInvalid, not %v", err)
			} else {
				c.NoError(err)
				c.Equal(tc.want, revised)
			}
		})
	}
}

// These tests cover the keyboard focus a Markdown takes for an assistive technology's sake — see Panel.axTakesFocus:
// that it takes it only while one is being served, and only on the platforms whose screen readers start from the focus;
// that a window in which nothing held the focus hands it to the document as support starts, while a focus that was
// already held is left alone; and that a keyboard user nothing is listening to never finds the document in the tab
// order. A session owns most of the package's mutable globals while it runs, so none of these may call t.Parallel.

// markdownFocusContent is the document the tests below show. What it says does not matter; that it is a Markdown does.
const markdownFocusContent = "# Title\n\nSome body text.\n"

// TestMarkdownTakesFocusOnlyWhileAccessibilityIsActive shows a window holding nothing but a document, which is the
// window a screen reader was stranded at the edge of: with nothing inside it holding the focus, Narrator's cursor
// stayed on the window itself. Once an assistive technology is being served, the document takes the focus and the
// description reports it there; once none is, the document is no longer focusable and the next move of the focus
// leaves it.
func TestMarkdownTakesFocusOnlyWhileAccessibilityIsActive(t *testing.T) {
	c := check.New(t)
	var md *Markdown
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			md = NewMarkdown(false)
			md.SetContent(markdownFocusContent, 400)
			scroller := NewScrollPanel()
			scroller.SetContent(md, behavior.Fill, behavior.Fill)
			wnd = axNewTestWindow(t, "document focus", geom.NewRect(10, 10, 500, 500), scroller)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	focusable := func() bool {
		var f bool
		screen.Do(func() { f = md.Focusable() })
		return f
	}
	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}
	c.False(focusable(), "with no assistive technology, a document is not focusable")
	c.True(focus() == nil, "and a window holding nothing but one has no focus")

	screen.EnableAccessibility()
	c.True(IsAccessibilityActive())
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	node := screen.AccessibilityNodeFor(md)
	c.NotNil(node)
	if tree == nil || node == nil {
		return
	}
	if axReadersFollowFocus {
		c.True(focusable(), "while one is being served, the document takes the focus")
		c.True(focus().Is(md), "a window in which nothing held the focus hands it to the document as support starts")
		c.Equal(node.ID, tree.Focus, "and the description reports the focus there")
		c.True(node.Focusable, "the node says it can take the focus")
		c.True(node.Actions.Has(accessibility.Focus), "and offers to")
	} else {
		c.False(focusable(), "on a platform whose screen reader does not start from the focus, nothing changes")
		c.True(focus() == nil, "so the window still has no focus")
		c.Equal(accessibility.NodeID(0), tree.Focus, "and the description reports none")
		c.False(node.Focusable)
		c.False(node.Actions.Has(accessibility.Focus))
	}

	screen.Do(deactivateAccessibility)
	c.False(IsAccessibilityActive())
	c.False(focusable(), "once none is being served, the document is not focusable again")
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus() == nil, "and the next move of the focus leaves it, with nothing else to land on")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownJoinsTabOrderOnlyWhileAccessibilityIsActive shows a document between two text fields. With no assistive
// technology, Tab moves from one field to the other and never lands on the document, as it never has. Once one is being
// served, on the platforms where a document takes the focus, the document is a tab stop between the two — while the
// field that held the focus as support started keeps it, since only a window with no focus at all is given one.
func TestMarkdownJoinsTabOrderOnlyWhileAccessibilityIsActive(t *testing.T) {
	c := check.New(t)
	var first, second *Field
	var md *Markdown
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			first = NewField()
			md = NewMarkdown(false)
			md.SetContent(markdownFocusContent, 400)
			second = NewField()
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(first)
			column.AddChild(md)
			column.AddChild(second)
			wnd = axNewTestWindow(t, "document tab order", geom.NewRect(10, 10, 500, 500), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}
	focusFirst := func() {
		screen.Do(func() { first.RequestFocus() })
		c.True(focus().Is(first))
	}
	focusFirst()
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(second), "with no assistive technology, the tab order skips the document")

	focusFirst()
	screen.EnableAccessibility()
	c.True(IsAccessibilityActive())
	c.True(focus().Is(first), "support starting leaves a focus that was already held where it is")
	screen.KeyPress(KeyTab, mod.None)
	if axReadersFollowFocus {
		c.True(focus().Is(md), "while one is being served, the document is a tab stop")
		screen.KeyPress(KeyTab, mod.None)
	}
	c.True(focus().Is(second), "and the field after it is the next stop")

	screen.Do(deactivateAccessibility)
	c.False(IsAccessibilityActive())
	focusFirst()
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(second), "once none is being served, the tab order skips the document again")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
