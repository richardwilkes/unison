package svg

import (
	"fmt"
	"strings"

	"golang.org/x/image/math/fixed"
)

// Operation groups the different SVG commands
type Operation interface {
	// SVG text representation of the command
	fmt.Stringer
}

// OpMoveTo moves the current point.
type OpMoveTo fixed.Point26_6

// OpLineTo draws a line from the current point,
// and updates it.
type OpLineTo fixed.Point26_6

// OpQuadTo draws a quadratic Bezier curve from the current point,
// and updates it.
type OpQuadTo [2]fixed.Point26_6

// OpCubicTo draws a cubic Bezier curve from the current point,
// and updates it.
type OpCubicTo [3]fixed.Point26_6

// OpClose close the current path.
type OpClose struct{}

func (op OpMoveTo) String() string {
	return fmt.Sprintf("M%4.3f,%4.3f", float32(op.X)/64, float32(op.Y)/64)
}

func (op OpLineTo) String() string {
	return fmt.Sprintf("L%4.3f,%4.3f", float32(op.X)/64, float32(op.Y)/64)
}

func (op OpQuadTo) String() string {
	return fmt.Sprintf("Q%4.3f,%4.3f,%4.3f,%4.3f", float32(op[0].X)/64, float32(op[0].Y)/64,
		float32(op[1].X)/64, float32(op[1].Y)/64)
}

func (op OpCubicTo) String() string {
	return "C" + fmt.Sprintf("C%4.3f,%4.3f,%4.3f,%4.3f,%4.3f,%4.3f", float32(op[0].X)/64, float32(op[0].Y)/64,
		float32(op[1].X)/64, float32(op[1].Y)/64, float32(op[2].X)/64, float32(op[2].Y)/64)
}

func (op OpClose) String() string {
	return "Z"
}

// Path describes a sequence of basic SVG operations, which should not be nil
// Higher-level shapes may be reduced to a path.
type Path []Operation

// ToSVGPath returns a string representation of the path
func (p Path) ToSVGPath() string {
	chunks := make([]string, len(p))
	for i, op := range p {
		chunks[i] = op.String()
	}
	return strings.Join(chunks, " ")
}

// String returns a readable representation of a Path.
func (p Path) String() string {
	return p.ToSVGPath()
}

// Clear zeros the path slice
func (p *Path) Clear() {
	*p = (*p)[:0]
}

// Start starts a new curve at the given point.
func (p *Path) Start(a fixed.Point26_6) {
	*p = append(*p, OpMoveTo{a.X, a.Y})
}

// Line adds a linear segment to the current curve.
func (p *Path) Line(b fixed.Point26_6) {
	*p = append(*p, OpLineTo{b.X, b.Y})
}

// QuadBezier adds a quadratic segment to the current curve.
func (p *Path) QuadBezier(b, c fixed.Point26_6) {
	*p = append(*p, OpQuadTo{b, c})
}

// CubeBezier adds a cubic segment to the current curve.
func (p *Path) CubeBezier(b, c, d fixed.Point26_6) {
	*p = append(*p, OpCubicTo{b, c, d})
}

// Stop joins the ends of the path
func (p *Path) Stop(closeLoop bool) {
	if closeLoop {
		*p = append(*p, OpClose{})
	}
}
