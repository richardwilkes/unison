package svg

import (
	"math"

	"github.com/richardwilkes/toolbox/v2/xmath"
	"golang.org/x/image/math/fixed"
)

const (
	cubicsPerHalfCircle = 8 // Number of cubic beziers to approx half a circle

	// fixed point t parameterization shift factor;
	// (2^this)/64 is the max length of t for fixed.Int26_6
	tStrokeShift = 14

	// maxDx is the maximum radians a cubic splice is allowed to span
	// in ellipse parametric when approximating an off-axis ellipse.
	maxDx float32 = math.Pi / 8
)

func toFixedPt(x, y float32) (p fixed.Point26_6) {
	return fixed.Point26_6{
		X: fixed.Int26_6(x * 64),
		Y: fixed.Int26_6(y * 64),
	}
}

func (p *Path) addRect(minX, minY, maxX, maxY, rot float32) {
	rot *= math.Pi / 180
	cx := (minX + maxX) / 2
	cy := (minY + maxY) / 2
	m := Identity.Translate(cx, cy).Rotate(rot).Translate(-cx, -cy)
	q := &matrixAdder{M: m, path: p}
	q.Start(toFixedPt(minX, minY))
	q.Line(toFixedPt(maxX, minY))
	q.Line(toFixedPt(maxX, maxY))
	q.Line(toFixedPt(minX, maxY))
	q.path.Stop(true)
}

// length is the distance from the origin of the point
func length(v fixed.Point26_6) fixed.Int26_6 {
	vx := float32(v.X)
	vy := float32(v.Y)
	return fixed.Int26_6(xmath.Sqrt(vx*vx + vy*vy))
}

// addArc strokes a circular arc by approximation with bezier curves
func addArc(p *matrixAdder, a, s1, s2 fixed.Point26_6, clockwise bool, trimStart, trimEnd fixed.Int26_6, firstPoint func(p fixed.Point26_6)) (ps1, ds1, ps2, ds2 fixed.Point26_6) {
	// Approximate the circular arc using a set of cubic bezier curves by the method of L. Maisonobe, "Drawing an
	// elliptical arc using polylines, quadratic or cubic Bezier curves", 2003
	// https://www.spaceroots.org/documents/elllipse/elliptical-arc.pdf The method was simplified for circles.
	theta1 := xmath.Atan2(float32(s1.Y-a.Y), float32(s1.X-a.X))
	theta2 := xmath.Atan2(float32(s2.Y-a.Y), float32(s2.X-a.X))
	if !clockwise {
		for theta1 < theta2 {
			theta1 += math.Pi * 2
		}
	} else {
		for theta2 < theta1 {
			theta2 += math.Pi * 2
		}
	}
	deltaTheta := theta2 - theta1
	if trimStart > 0 {
		ds := (deltaTheta * float32(trimStart)) / float32(1<<tStrokeShift)
		deltaTheta -= ds
		theta1 += ds
	}
	if trimEnd > 0 {
		ds := (deltaTheta * float32(trimEnd)) / float32(1<<tStrokeShift)
		deltaTheta -= ds
	}
	segs := int(xmath.Abs(deltaTheta)/(math.Pi/cubicsPerHalfCircle)) + 1
	dTheta := deltaTheta / float32(segs)
	tde := xmath.Tan(dTheta / 2)
	alpha := fixed.Int26_6(xmath.Sin(dTheta) * (xmath.Sqrt(4+3*tde*tde) - 1) * (64.0 / 3.0)) // Math is fun!
	r := float32(length(s1.Sub(a)))                                                          // Note r is *64
	ldp := fixed.Point26_6{X: -fixed.Int26_6(r * xmath.Sin(theta1)), Y: fixed.Int26_6(r * xmath.Cos(theta1))}
	ds1 = ldp
	ps1 = fixed.Point26_6{X: a.X + ldp.Y, Y: a.Y - ldp.X}
	firstPoint(ps1)
	s1 = ps1
	for i := 1; i <= segs; i++ {
		eta := theta1 + dTheta*float32(i)
		ds2 = fixed.Point26_6{X: -fixed.Int26_6(r * xmath.Sin(eta)), Y: fixed.Int26_6(r * xmath.Cos(eta))}
		ps2 = fixed.Point26_6{X: a.X + ds2.Y, Y: a.Y - ds2.X} // Using deriviative to calc new pt, because circle
		p1 := s1.Add(ldp.Mul(alpha))
		p2 := ps2.Sub(ds2.Mul(alpha))
		p.CubeBezier(p1, p2, ps2)
		s1, ldp = ps2, ds2
	}
	return ps1, ds1, ps2, ds2
}

// roundGap bridges miter-limit gaps with a circular arc
func roundGap(p *matrixAdder, a, tNorm, lNorm fixed.Point26_6) {
	addArc(p, a, a.Add(tNorm), a.Add(lNorm), true, 0, 0, p.Line)
	p.Line(a.Add(lNorm)) // just to be sure line joins cleanly,
	// last pt in stoke arc may not be precisely s2
}

// addRoundRect adds a rectangle of the indicated size, rotated around the center by rot degrees with rounded corners of
// radius rx in the x axis and ry in the y axis.
func (p *Path) addRoundRect(minX, minY, maxX, maxY, rx, ry, rot float32) {
	if rx <= 0 || ry <= 0 {
		p.addRect(minX, minY, maxX, maxY, rot)
		return
	}
	rot *= math.Pi / 180

	w := maxX - minX
	if w < rx*2 {
		rx = w / 2
	}
	h := maxY - minY
	if h < ry*2 {
		ry = h / 2
	}
	stretch := rx / ry
	midY := minY + h/2
	m := Identity.Translate(minX+w/2, midY).Rotate(rot).Scale(1, 1/stretch).Translate(-minX-w/2, -minY-h/2)
	maxY = midY + h/2*stretch
	minY = midY - h/2*stretch

	q := &matrixAdder{M: m, path: p}

	q.Start(toFixedPt(minX+rx, minY))
	q.Line(toFixedPt(maxX-rx, minY))
	roundGap(q, toFixedPt(maxX-rx, minY+rx), toFixedPt(0, -rx), toFixedPt(rx, 0))
	q.Line(toFixedPt(maxX, maxY-rx))
	roundGap(q, toFixedPt(maxX-rx, maxY-rx), toFixedPt(rx, 0), toFixedPt(0, rx))
	q.Line(toFixedPt(minX+rx, maxY))
	roundGap(q, toFixedPt(minX+rx, maxY-rx), toFixedPt(0, rx), toFixedPt(-rx, 0))
	q.Line(toFixedPt(minX, minY+rx))
	roundGap(q, toFixedPt(minX+rx, minY+rx), toFixedPt(-rx, 0), toFixedPt(0, -rx))
	q.path.Stop(true)
}

// addArc adds an arc to the adder p
func (p *Path) addArc(points []float32, cx, cy, px, py float32) (lx, ly float32) {
	rotX := points[2] * math.Pi / 180 // Convert degress to radians
	largeArc := points[3] != 0
	sweep := points[4] != 0
	startAngle := xmath.Atan2(py-cy, px-cx) - rotX
	endAngle := xmath.Atan2(points[6]-cy, points[5]-cx) - rotX
	deltaTheta := endAngle - startAngle
	arcBig := xmath.Abs(deltaTheta) > math.Pi

	// Approximate ellipse using cubic bezeir splines
	etaStart := xmath.Atan2(xmath.Sin(startAngle)/points[1], xmath.Cos(startAngle)/points[0])
	etaEnd := xmath.Atan2(xmath.Sin(endAngle)/points[1], xmath.Cos(endAngle)/points[0])
	deltaEta := etaEnd - etaStart
	if (arcBig && !largeArc) || (!arcBig && largeArc) { // Go has no boolean XOR
		if deltaEta < 0 {
			deltaEta += math.Pi * 2
		} else {
			deltaEta -= math.Pi * 2
		}
	}
	// This check might be needed if the center point of the elipse is
	// at the midpoint of the start and end lines.
	if deltaEta < 0 && sweep {
		deltaEta += math.Pi * 2
	} else if deltaEta >= 0 && !sweep {
		deltaEta -= math.Pi * 2
	}

	// Round up to determine number of cubic splines to approximate bezier curve
	segs := int(xmath.Abs(deltaEta)/maxDx) + 1
	dEta := deltaEta / float32(segs) // span of each segment
	// Approximate the ellipse using a set of cubic bezier curves by the method of
	// L. Maisonobe, "Drawing an elliptical arc using polylines, quadratic
	// or cubic Bezier curves", 2003
	// https://www.spaceroots.org/documents/elllipse/elliptical-arc.pdf
	tde := xmath.Tan(dEta / 2)
	alpha := xmath.Sin(dEta) * (xmath.Sqrt(4+3*tde*tde) - 1) / 3 // Math is fun!
	lx, ly = px, py
	sinTheta, cosTheta := xmath.Sin(rotX), xmath.Cos(rotX)
	ldx, ldy := ellipsePrime(points[0], points[1], sinTheta, cosTheta, etaStart)
	for i := 1; i <= segs; i++ {
		eta := etaStart + dEta*float32(i)
		if i == segs {
			px, py = points[5], points[6] // Just makes the end point exact; no roundoff error
		} else {
			px, py = ellipsePointAt(points[0], points[1], sinTheta, cosTheta, eta, cx, cy)
		}
		dx, dy := ellipsePrime(points[0], points[1], sinTheta, cosTheta, eta)
		p.CubeBezier(toFixedPt(lx+alpha*ldx, ly+alpha*ldy),
			toFixedPt(px-alpha*dx, py-alpha*dy), toFixedPt(px, py))
		lx, ly, ldx, ldy = px, py, dx, dy
	}
	return lx, ly
}

// ellipsePrime gives tangent vectors for parameterized elipse; a, b, radii, eta parameter
func ellipsePrime(a, b, sinTheta, cosTheta, eta float32) (px, py float32) {
	bCosEta := b * xmath.Cos(eta)
	aSinEta := a * xmath.Sin(eta)
	return -aSinEta*cosTheta - bCosEta*sinTheta, -aSinEta*sinTheta + bCosEta*cosTheta
}

// ellipsePointAt gives points for parameterized elipse; a, b, radii, eta parameter, center cx, cy
func ellipsePointAt(a, b, sinTheta, cosTheta, eta, cx, cy float32) (px, py float32) {
	aCosEta := a * xmath.Cos(eta)
	bSinEta := b * xmath.Sin(eta)
	return cx + aCosEta*cosTheta - bSinEta*sinTheta, cy + aCosEta*sinTheta + bSinEta*cosTheta
}

// findEllipseCenter locates the center of the Ellipse if it exists. If it does not exist,
// the radius values will be increased minimally for a solution to be possible
// while preserving the ra to rb ratio.  ra and rb arguments are pointers that can be
// checked after the call to see if the values changed. This method uses coordinate transformations
// to reduce the problem to finding the center of a circle that includes the origin
// and an arbitrary point. The center of the circle is then transformed
// back to the original coordinates and returned.
func findEllipseCenter(ra, rb *float32, rotX, startX, startY, endX, endY float32, sweep, smallArc bool) (cx, cy float32) {
	cos, sin := xmath.Cos(rotX), xmath.Sin(rotX)

	// Move origin to start point
	nx, ny := endX-startX, endY-startY

	// Rotate ellipse x-axis to coordinate x-axis
	nx, ny = nx*cos+ny*sin, -nx*sin+ny*cos
	// Scale X dimension so that ra = rb
	nx *= *rb / *ra // Now the ellipse is a circle radius rb; therefore foci and center coincide

	midX, midY := nx/2, ny/2
	midlenSq := midX*midX + midY*midY

	var hr float32
	if *rb**rb < midlenSq {
		// Requested ellipse does not exist; scale ra, rb to fit. Length of
		// span is greater than max width of ellipse, must scale *ra, *rb
		nrb := xmath.Sqrt(midlenSq)
		if *ra == *rb {
			*ra = nrb // prevents roundoff
		} else {
			*ra = *ra * nrb / *rb
		}
		*rb = nrb
	} else {
		hr = xmath.Sqrt(*rb**rb-midlenSq) / xmath.Sqrt(midlenSq)
	}
	// Notice that if hr is zero, both answers are the same.
	if (sweep && smallArc) || (!sweep && !smallArc) {
		cx = midX + midY*hr
		cy = midY - midX*hr
	} else {
		cx = midX - midY*hr
		cy = midY + midX*hr
	}

	// reverse scale
	cx *= *ra / *rb
	// Reverse rotate and translate back to original coordinates
	return cx*cos - cy*sin + startX, cx*sin + cy*cos + startY
}
