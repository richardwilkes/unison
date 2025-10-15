// Provides parsing and rendering of SVG images.
// SVG files are parsed into an abstract representation,
// which can then be consumed by painting drivers.
package svg

import (
	"encoding/xml"
	"errors"
	"io"

	"github.com/richardwilkes/toolbox/v2/errs"
	"golang.org/x/net/html/charset"
)

// SVG holds data from parsed SVGs.
type SVG struct {
	SvgMasks  map[string]*Mask
	grads     map[string]*Gradient
	defs      map[string][]definition
	Width     string
	Height    string
	SvgPaths  []StyledPath
	ViewBox   Bounds
	Transform Matrix2D
}

// PathStyle holds the state of the style.
type PathStyle struct {
	Masks             []string
	FillerColor       Pattern // either PlainColor or Gradient
	LinerColor        Pattern // either PlainColor or Gradient
	Dash              DashOptions
	Join              JoinOptions
	FillOpacity       float64
	LineOpacity       float64
	LineWidth         float64
	Transform         Matrix2D // current transform
	UseNonZeroWinding bool
}

// StyledPath binds a PathStyle to a Path.
type StyledPath struct {
	Path  Path
	Style PathStyle
}

// Mask is the element that defines a mask for the referenced elements.
type Mask struct {
	ID        string
	SvgPaths  []StyledPath
	X         float64
	Y         float64
	W         float64
	H         float64
	Transform Matrix2D
}

// Parse reads the Icon from the given io.Reader
// This only supports a sub-set of SVG, but
// is enough to draw many svgs. errMode determines if the svg ignores, errors out, or logs a warning
// if it does not handle an element found in the svg file.
func Parse(stream io.Reader) (*SVG, error) {
	svg := &SVG{
		defs:      make(map[string][]definition),
		grads:     make(map[string]*Gradient),
		SvgMasks:  make(map[string]*Mask),
		Transform: Identity,
	}
	pos := &cursor{
		styleStack: []PathStyle{DefaultStyle},
		svg:        svg,
	}
	decoder := xml.NewDecoder(stream)
	decoder.CharsetReader = charset.NewReaderLabel
	seenTag := false
	for {
		t, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if !seenTag {
					return nil, errs.New("invalid svg data")
				}
				return svg, nil
			}
			return svg, err
		}
		switch se := t.(type) {
		case xml.StartElement:
			seenTag = true
			if err = pos.pushStyle(se.Attr); err != nil {
				return svg, err
			}
			if err = pos.readStartElement(se); err != nil {
				return svg, err
			}
		case xml.EndElement:
			pos.styleStack = pos.styleStack[:len(pos.styleStack)-1]
			switch se.Name.Local {
			case "g":
				if pos.inDefs {
					pos.currentDef = append(pos.currentDef, definition{Tag: "endg"})
				}
			case "mask":
				if pos.mask != nil {
					pos.svg.SvgMasks[pos.mask.ID] = pos.mask
					pos.mask = nil
				}
				pos.inMask = false
			case "defs":
				if len(pos.currentDef) > 0 {
					pos.svg.defs[pos.currentDef[0].ID] = pos.currentDef
					pos.currentDef = make([]definition, 0)
				}
				pos.inDefs = false
			case "radialGradient", "linearGradient":
				pos.inGrad = false
			}
		}
	}
}
