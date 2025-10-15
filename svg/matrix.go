package svg

import (
	"math"

	"golang.org/x/image/math/fixed"
)

// Matrix2D represents 2D matrix
type Matrix2D struct {
	ScaleX float64
	SkwX   float64
	TransX float64
	SkwY   float64
	ScaleY float64
	TransY float64
}

type matrix3 [9]float64

func otherPair(i int) (a, b int) {
	switch i {
	case 0:
		a, b = 1, 2
	case 1:
		a, b = 0, 2
	case 2:
		a, b = 0, 1
	}
	return a, b
}

func (m *matrix3) coFact(i, j int) float64 {
	ai, bi := otherPair(i)
	aj, bj := otherPair(j)
	a, b, c, d := m[ai+aj*3], m[bi+bj*3], m[ai+bj*3], m[bi+aj*3]
	return a*b - c*d
}

func (m *matrix3) invert() *matrix3 {
	var cofact matrix3
	for i := range 3 {
		for j := range 3 {
			cofact[i+j*3] = m.coFact(i, j) * float64(1-(i+j%2)%2*2)
		}
	}
	deteriminate := m[0]*cofact[0] + m[1]*cofact[1] + m[2]*cofact[2]
	for i := range 2 {
		for j := i + 1; j < 3; j++ {
			cofact[i+j*3], cofact[j+i*3] = cofact[j+i*3], cofact[i+j*3]
		}
	}
	for i := range 9 {
		cofact[i] /= deteriminate
	}
	return &cofact
}

// Invert returns the inverse matrix.
func (m Matrix2D) Invert() Matrix2D {
	n := &matrix3{
		m.ScaleX,
		m.SkwX,
		m.TransX,
		m.SkwY,
		m.ScaleY,
		m.TransY,
		0,
		0,
		1,
	}
	n = n.invert()
	return Matrix2D{
		ScaleX: n[0],
		SkwX:   n[1],
		TransX: n[2],
		SkwY:   n[3],
		ScaleY: n[4],
		TransY: n[5],
	}
}

// Mult returns a*b.
func (m Matrix2D) Mult(b Matrix2D) Matrix2D {
	return Matrix2D{
		ScaleX: m.ScaleX*b.ScaleX + m.SkwX*b.SkwY,
		SkwX:   m.ScaleX*b.SkwX + m.SkwX*b.ScaleY,
		TransX: m.ScaleX*b.TransX + m.SkwX*b.TransY + m.TransX,
		SkwY:   m.SkwY*b.ScaleX + m.ScaleY*b.SkwY,
		ScaleY: m.SkwY*b.SkwX + m.ScaleY*b.ScaleY,
		TransY: m.SkwY*b.TransX + m.ScaleY*b.TransY + m.TransY,
	}
}

// Identity is the identity matrix.
var Identity = Matrix2D{
	ScaleX: 1,
	ScaleY: 1,
}

// TFixed transforms a fixed.Point26_6 by the matrix.
func (m Matrix2D) TFixed(x fixed.Point26_6) (y fixed.Point26_6) {
	return fixed.Point26_6{
		X: fixed.Int26_6((float64(x.X)*m.ScaleX + float64(x.Y)*m.SkwX) + m.TransX*64),
		Y: fixed.Int26_6((float64(x.X)*m.SkwY + float64(x.Y)*m.ScaleY) + m.TransY*64),
	}
}

// Transform multiples the input vector by matrix m and outputs the results vector components.
func (m Matrix2D) Transform(x1, y1 float64) (x2, y2 float64) {
	return x1*m.ScaleX + y1*m.SkwX + m.TransX, x1*m.SkwY + y1*m.ScaleY + m.TransY
}

// TransformVector is a modified version of Transform that ignores the translation components.
func (m Matrix2D) TransformVector(x1, y1 float64) (x2, y2 float64) {
	return x1*m.ScaleX + y1*m.SkwX, x1*m.SkwY + y1*m.ScaleY
}

// Scale matrix in x and y dimensions.
func (m Matrix2D) Scale(x, y float64) Matrix2D {
	return Matrix2D{
		ScaleX: m.ScaleX * x,
		SkwX:   m.SkwX * x,
		TransX: m.TransX * x,
		SkwY:   m.SkwY * y,
		ScaleY: m.ScaleY * y,
		TransY: m.TransY * y,
	}
}

// SkewY skews the matrix in the Y dimension.
func (m Matrix2D) SkewY(theta float64) Matrix2D {
	return m.Mult(Matrix2D{
		ScaleX: 1,
		SkwY:   math.Tan(theta),
		ScaleY: 1,
	})
}

// SkewX skews the matrix in the X dimension.
func (m Matrix2D) SkewX(theta float64) Matrix2D {
	return m.Mult(Matrix2D{
		ScaleX: 1,
		SkwX:   math.Tan(theta),
		ScaleY: 1,
	})
}

// Translate translates the matrix to the x, y point.
func (m Matrix2D) Translate(x, y float64) Matrix2D {
	return Matrix2D{
		ScaleX: m.ScaleX,
		SkwX:   m.SkwX,
		TransX: m.TransX + x,
		SkwY:   m.SkwY,
		ScaleY: m.ScaleY,
		TransY: m.TransY + y,
	}
}

// Rotate rotate the matrix by theta (in radians).
func (m Matrix2D) Rotate(theta float64) Matrix2D {
	return m.Mult(Matrix2D{
		ScaleX: math.Cos(theta),
		SkwX:   -math.Sin(theta),
		SkwY:   math.Sin(theta),
		ScaleY: math.Cos(theta),
	})
}

// matrixAdder add points to path after applying a matrix M to all points
type matrixAdder struct {
	path *Path
	M    Matrix2D
}

// Start starts a new path.
func (m *matrixAdder) Start(a fixed.Point26_6) {
	m.path.Start(m.M.TFixed(a))
}

// Line adds a linear segment to the current curve.
func (m *matrixAdder) Line(b fixed.Point26_6) {
	m.path.Line(m.M.TFixed(b))
}

// CubeBezier adds a cubic segment to the current curve.
func (m *matrixAdder) CubeBezier(b, c, d fixed.Point26_6) {
	m.path.CubeBezier(m.M.TFixed(b), m.M.TFixed(c), m.M.TFixed(d))
}
