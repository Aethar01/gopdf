package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Length is a coordinate of a shape's path: Frac of the box's width or
// height plus Px logical pixels, as "100%-6" is the whole width less 6.
type Length struct {
	Frac, Px float64
}

// Of is the length in a box size long.
func (l Length) Of(size float64) float64 { return l.Frac*size + l.Px }

func (l Length) plus(m Length) Length { return Length{l.Frac + m.Frac, l.Px + m.Px} }

// PathPoint is a point of a shape's path.
type PathPoint struct{ X, Y Length }

// PathOp is one step of a shape's path: Op is 'M' to start a subpath at
// Pts[0], 'L' a line to it, 'Q' a quadratic curve through Pts[0] to
// Pts[1], 'C' a cubic curve through Pts[0] and Pts[1] to Pts[2], and 'Z'
// closes the subpath.
type PathOp struct {
	Op  byte
	Pts [3]PathPoint
}

// ParsePath reads a shape written as SVG path data: the commands M, L, H,
// V, Q, C and Z, lowercase for relative coordinates. A coordinate is in
// logical pixels from the box's top left, or a percentage of its width or
// height, which can be offset by pixels written straight after it, as in
// "100%-6".
func ParsePath(s string) ([]PathOp, error) {
	p := pathParser{s: s}
	var ops []PathOp
	var cmd byte
	var current, start PathPoint
	for {
		p.skipSeparators()
		if p.done() {
			break
		}
		if c := p.s[p.i]; isPathCommand(c) {
			cmd = c
			p.i++
		} else if cmd == 0 {
			return nil, fmt.Errorf("path must start with a command, not %q", p.rest())
		} else if cmd == 'Z' || cmd == 'z' {
			return nil, fmt.Errorf("unexpected %q after Z", p.rest())
		}
		relative := cmd >= 'a'
		point := func() (PathPoint, error) {
			x, err := p.length()
			if err != nil {
				return PathPoint{}, err
			}
			y, err := p.length()
			if err != nil {
				return PathPoint{}, err
			}
			pt := PathPoint{x, y}
			if relative {
				pt = PathPoint{current.X.plus(x), current.Y.plus(y)}
			}
			return pt, nil
		}
		var op PathOp
		switch cmd {
		case 'M', 'm', 'L', 'l':
			pt, err := point()
			if err != nil {
				return nil, err
			}
			op = PathOp{Op: 'L', Pts: [3]PathPoint{pt}}
			if cmd == 'M' || cmd == 'm' {
				op.Op, start = 'M', pt
				// Coordinates after a move are lines.
				if cmd == 'M' {
					cmd = 'L'
				} else {
					cmd = 'l'
				}
			}
		case 'H', 'h', 'V', 'v':
			v, err := p.length()
			if err != nil {
				return nil, err
			}
			pt := current
			horizontal := cmd == 'H' || cmd == 'h'
			switch {
			case horizontal && relative:
				pt.X = current.X.plus(v)
			case horizontal:
				pt.X = v
			case relative:
				pt.Y = current.Y.plus(v)
			default:
				pt.Y = v
			}
			op = PathOp{Op: 'L', Pts: [3]PathPoint{pt}}
		case 'Q', 'q', 'C', 'c':
			op.Op = 'Q'
			n := 2
			if cmd == 'C' || cmd == 'c' {
				op.Op, n = 'C', 3
			}
			for i := range n {
				pt, err := point()
				if err != nil {
					return nil, err
				}
				op.Pts[i] = pt
			}
		case 'Z', 'z':
			op = PathOp{Op: 'Z'}
		}
		if op.Op != 'M' && len(ops) == 0 {
			return nil, fmt.Errorf("path must start with M")
		}
		ops = append(ops, op)
		switch op.Op {
		case 'Z':
			current = start
		case 'Q':
			current = op.Pts[1]
		case 'C':
			current = op.Pts[2]
		default:
			current = op.Pts[0]
		}
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("empty path")
	}
	return ops, nil
}

func isPathCommand(c byte) bool { return strings.IndexByte("MmLlHhVvQqCcZz", c) >= 0 }

type pathParser struct {
	s string
	i int
}

func (p *pathParser) done() bool   { return p.i >= len(p.s) }
func (p *pathParser) rest() string { return p.s[p.i:] }

func (p *pathParser) skipSeparators() {
	for !p.done() && strings.IndexByte(" \t\r\n,", p.s[p.i]) >= 0 {
		p.i++
	}
}

// number reads a number at the cursor, without skipping anything first.
func (p *pathParser) number() (float64, bool) {
	start := p.i
	if !p.done() && (p.s[p.i] == '+' || p.s[p.i] == '-') {
		p.i++
	}
	digits := false
	for !p.done() && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
		p.i, digits = p.i+1, true
	}
	if !p.done() && p.s[p.i] == '.' {
		p.i++
		for !p.done() && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i, digits = p.i+1, true
		}
	}
	if !digits {
		p.i = start
		return 0, false
	}
	v, err := strconv.ParseFloat(p.s[start:p.i], 64)
	return v, err == nil
}

// length reads a coordinate: a number, or a percentage with an optional
// pixel offset straight after it.
func (p *pathParser) length() (Length, error) {
	p.skipSeparators()
	v, ok := p.number()
	if !ok {
		if p.done() {
			return Length{}, fmt.Errorf("path ends where a coordinate was expected")
		}
		return Length{}, fmt.Errorf("expected a coordinate at %q", p.rest())
	}
	if p.done() || p.s[p.i] != '%' {
		return Length{Px: v}, nil
	}
	p.i++
	l := Length{Frac: v / 100}
	if !p.done() && (p.s[p.i] == '+' || p.s[p.i] == '-') {
		offset, ok := p.number()
		if !ok {
			return Length{}, fmt.Errorf("expected an offset at %q", p.rest())
		}
		l.Px = offset
	}
	return l, nil
}

// kappa places the control points of a cubic curve tracing a quarter
// circle.
const kappa = 0.5522847498

func pxPoint(x, y float64) PathPoint { return PathPoint{Length{Px: x}, Length{Px: y}} }

// RoundedRectPath is the path of a rect at x, y, w by h, its corners
// rounded clockwise from the top left by r, which must fit.
func RoundedRectPath(x, y, w, h float64, r Corners) []PathOp {
	ops := make([]PathOp, 1, 10)
	ops[0] = PathOp{Op: 'M', Pts: [3]PathPoint{pxPoint(x+r[0], y)}}
	// Each corner: its point on the edge before it, its point on the
	// edge after it, and the direction from the first to the corner.
	type corner struct{ cx, cy, r, inX, inY, outX, outY float64 }
	for _, c := range []corner{
		{x + w, y, r[1], -1, 0, 0, 1},
		{x + w, y + h, r[2], 0, -1, -1, 0},
		{x, y + h, r[3], 1, 0, 0, -1},
		{x, y, r[0], 0, 1, 1, 0},
	} {
		start := pxPoint(c.cx+c.inX*c.r, c.cy+c.inY*c.r)
		end := pxPoint(c.cx+c.outX*c.r, c.cy+c.outY*c.r)
		ops = append(ops, PathOp{Op: 'L', Pts: [3]PathPoint{start}})
		if c.r > 0 {
			k := c.r * (1 - kappa)
			ops = append(ops, PathOp{Op: 'C', Pts: [3]PathPoint{
				pxPoint(c.cx+c.inX*k, c.cy+c.inY*k),
				pxPoint(c.cx+c.outX*k, c.cy+c.outY*k),
				end,
			}})
		}
	}
	return append(ops, PathOp{Op: 'Z'})
}

// ellipsePath is the path of the ellipse centred on cx, cy with radii rx
// and ry.
func ellipsePath(cx, cy, rx, ry float64) []PathOp {
	kx, ky := rx*kappa, ry*kappa
	return []PathOp{
		{Op: 'M', Pts: [3]PathPoint{pxPoint(cx+rx, cy)}},
		{Op: 'C', Pts: [3]PathPoint{pxPoint(cx+rx, cy+ky), pxPoint(cx+kx, cy+ry), pxPoint(cx, cy+ry)}},
		{Op: 'C', Pts: [3]PathPoint{pxPoint(cx-kx, cy+ry), pxPoint(cx-rx, cy+ky), pxPoint(cx-rx, cy)}},
		{Op: 'C', Pts: [3]PathPoint{pxPoint(cx-rx, cy-ky), pxPoint(cx-kx, cy-ry), pxPoint(cx, cy-ry)}},
		{Op: 'C', Pts: [3]PathPoint{pxPoint(cx+kx, cy-ry), pxPoint(cx+rx, cy-ky), pxPoint(cx+rx, cy)}},
		{Op: 'Z'},
	}
}

// FormatPath writes ops as path data that ParsePath reads back.
func FormatPath(ops []PathOp) string {
	var b strings.Builder
	length := func(l Length) {
		switch {
		case l.Frac == 0:
			b.WriteString(strconv.FormatFloat(l.Px, 'f', -1, 64))
		default:
			b.WriteString(strconv.FormatFloat(l.Frac*100, 'f', -1, 64) + "%")
			if l.Px > 0 {
				b.WriteByte('+')
			}
			if l.Px != 0 {
				b.WriteString(strconv.FormatFloat(l.Px, 'f', -1, 64))
			}
		}
	}
	points := map[byte]int{'M': 1, 'L': 1, 'Q': 2, 'C': 3}
	for i, op := range ops {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte(op.Op)
		for _, p := range op.Pts[:points[op.Op]] {
			b.WriteByte(' ')
			length(p.X)
			b.WriteByte(',')
			length(p.Y)
		}
	}
	return b.String()
}
