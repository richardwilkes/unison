package main

import (
	_ "embed"

	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

//go:embed gopher.svg
var gopherSvg string

//go:embed tiger.svg
var tigerSvg string

//go:embed black-king.svg
var blackKingSvg string

func main() {
	unison.AttachConsole()
	unison.Start(unison.StartupFinishedCallback(func() {
		wnd, err := unison.NewWindow("example")
		if err != nil {
			panic(err)
		}

		unison.DefaultMenuFactory().BarForWindow(wnd, func(m unison.Menu) {
			unison.InsertStdMenus(m, func(_ unison.MenuItem) {}, nil, nil)
		})

		content := wnd.Content()
		content.SetBorder(unison.NewEmptyBorder(unison.NewUniformInsets(20)))
		content.SetLayout(&unison.FlexLayout{
			Columns:  1,
			HSpacing: unison.StdHSpacing,
			VSpacing: 10,
			// HAlign:   align.Middle,
			// VAlign:   align.Middle,
		})

		content.AddChild(newSVGs())

		wnd.Pack()
		wnd.ToFront()
	}))

}

type svgs struct {
	Drawable unison.Drawable
	unison.Panel
	bg unison.Color
}

func newSVGs() *svgs {
	ss := &svgs{
		bg: unison.MustColorDecode("#ffc0f0"),
	}
	ss.DrawCallback = ss.defaultDraw
	ss.SetSizer(ss.defaultSizes)
	return ss
}

func (ss *svgs) defaultSizes(_ unison.Size) (minSize, prefSize, maxSize unison.Size) {
	size := unison.Size{
		Width:  800,
		Height: 800,
	}
	return size, size, size
}

func (ss *svgs) defaultDraw(canvas *unison.Canvas, _ unison.Rect) {
	r := ss.ContentRect(false)
	unison.DrawRectBase(canvas, r, ss.bg, unison.Transparent)

	{
		svg := unison.DrawableSVG{
			SVG: unison.CircledChevronRightSVG,
		}
		svg.DrawInRect(canvas, unison.NewRect(0, 0, 100, 100), nil, nil)
	}

	{
		svg := unison.DrawableSVG{
			SVG: unison.CircledChevronRightSVG,
		}
		paint := unison.NewPaint()
		paint.SetColor(unison.RGB(0xff, 0x00, 0x00))
		paint.SetStyle(paintstyle.Fill)
		svg.DrawInRect(canvas, unison.NewRect(150, 0, 100, 100), nil, paint)
	}

	{

		svg := unison.DrawableSVG{
			SVG: unison.MustSVGFromContentString(blackKingSvg),
		}
		width := float32(120)
		height := width / svg.SVG.AspectRatio()

		bg := unison.NewPaint()
		bg.SetColor(unison.RGB(0x00, 0xff, 0x00))
		rect := unison.NewRect(300, 0, width, height)
		canvas.DrawRect(rect, bg)
		svg.DrawInRect(canvas, rect, nil, nil)
	}

	{
		svg := unison.DrawableSVG{
			SVG: unison.MustSVGFromContentString(gopherSvg),
		}
		width := float32(200)
		height := width / svg.SVG.AspectRatio()
		svg.DrawInRect(canvas, unison.NewRect(50, 150, width, height), nil, nil)
	}

	{
		svg := unison.DrawableSVG{
			SVG: unison.MustSVGFromContentString(tigerSvg),
		}
		width := float32(400)
		height := width / svg.SVG.AspectRatio()
		svg.DrawInRect(canvas, unison.NewRect(300, 150, width, height), nil, nil)
	}
}
