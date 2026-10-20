// Package core contains the small, dependency-free vocabulary shared by the
// UI layer, the renderer and the platform backends.
package core

import "math"

// Point is a position in logical UI units.
type Point struct {
	X, Y float32
}

// Size is a width/height pair in logical UI units.
type Size struct {
	W, H float32
}

// Rect is an axis-aligned rectangle in logical UI units. X/Y is the top-left
// corner, matching windowing and paint coordinates.
type Rect struct {
	X, Y, W, H float32
}

// Insets is a four-sided spacing value.
type Insets struct {
	L, T, R, B float32
}

// All returns insets with the same value on every side.
func All(v float32) Insets { return Insets{v, v, v, v} }

// XY returns horizontal/vertical insets.
func XY(h, v float32) Insets { return Insets{h, v, h, v} }

// Horizontal is the total left+right inset.
func (i Insets) Horizontal() float32 { return i.L + i.R }

// Vertical is the total top+bottom inset.
func (i Insets) Vertical() float32 { return i.T + i.B }

// Apply shrinks r by the insets.
func (i Insets) Apply(r Rect) Rect { return r.Inset(i.L, i.T, i.R, i.B) }

// Point returns a point offset by a horizontal/vertical amount.
func (p Point) Add(x, y float32) Point { return Point{p.X + x, p.Y + y} }

// Sub returns p-q.
func (p Point) Sub(q Point) Point { return Point{p.X - q.X, p.Y - q.Y} }

// Distance returns the Euclidean distance between two points.
func (p Point) Distance(q Point) float32 {
	dx, dy := p.X-q.X, p.Y-q.Y
	return float32(math.Hypot(float64(dx), float64(dy)))
}

// Inside reports whether p is within the rect. The top-left edge is included,
// the bottom-right edge is excluded.
func (r Rect) Inside(p Point) bool {
	return p.X >= r.X && p.Y >= r.Y && p.X < r.X+r.W && p.Y < r.Y+r.H
}

// Right returns x+w.
func (r Rect) Right() float32 { return r.X + r.W }

// Bottom returns y+h.
func (r Rect) Bottom() float32 { return r.Y + r.H }

// Center returns the center point.
func (r Rect) Center() Point { return Point{r.X + r.W/2, r.Y + r.H/2} }

// Size returns the rectangle's size.
func (r Rect) Size() Size { return Size{r.W, r.H} }

// Inset shrinks the rectangle by the given sides.
func (r Rect) Inset(l, t, rr, b float32) Rect {
	return Rect{r.X + l, r.Y + t, r.W - l - rr, r.H - t - b}
}

// InsetAll shrinks the rectangle uniformly.
func (r Rect) InsetAll(v float32) Rect { return r.Inset(v, v, v, v) }

// Translate moves the rect.
func (r Rect) Translate(dx, dy float32) Rect {
	return Rect{r.X + dx, r.Y + dy, r.W, r.H}
}

// Intersect returns the overlap of two rectangles. The result is empty if they
// do not overlap; callers usually check W<=0 || H<=0.
func (r Rect) Intersect(o Rect) Rect {
	x0 := maxf(r.X, o.X)
	y0 := maxf(r.Y, o.Y)
	x1 := minf(r.Right(), o.Right())
	y1 := minf(r.Bottom(), o.Bottom())
	return Rect{x0, y0, maxf(0, x1-x0), maxf(0, y1-y0)}
}

// IsEmpty reports whether the rect has no area.
func (r Rect) IsEmpty() bool { return r.W <= 0 || r.H <= 0 }

// Contains reports whether o is fully inside r.
func (r Rect) Contains(o Rect) bool {
	return o.X >= r.X && o.Y >= r.Y && o.Right() <= r.Right() && o.Bottom() <= r.Bottom()
}

func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// Align is cross-axis alignment inside a container.
type Align uint8

const (
	AlignStart Align = iota
	AlignCenter
	AlignEnd
	AlignStretch
)

// AlignOffset returns the offset needed to align a child of size child inside
// an available span of size span.
func AlignOffset(a Align, span, child float32) float32 {
	switch a {
	case AlignCenter:
		return (span - child) / 2
	case AlignEnd:
		return span - child
	default:
		return 0
	}
}
