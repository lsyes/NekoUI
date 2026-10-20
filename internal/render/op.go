// Package render is NekoUI's backend-independent display list. The UI layer
// builds a Frame; any renderer (currently OpenGL) consumes it.
package render

import "nekoui/internal/core"

// Kind is a display-list primitive.
type Kind uint8

const (
	// RectFill is an antialiased rounded rectangle. Radius 0 is a plain rect.
	RectFill Kind = iota
	// RectStroke is an antialiased rounded-rect outline.
	RectStroke
	// Text is a run of pre-shaped glyph quads sharing one atlas.
	Text
	// Triangles is a set of solid, optionally gradient, triangles.
	Triangles
	// ImageQuad is an RGBA texture quad. UV is the normalized source rectangle.
	ImageQuad
)

// Glyph is a textured quad in logical UI coordinates.
type Glyph struct {
	X, Y, W, H float32
	U0, V0     float32
	U1, V1     float32
	Color      core.Color
}

// Vertex is a position/color pair used by solid triangles. Colors interpolate
// across the triangle, which gives cheap gradients for custom art.
type Vertex struct {
	P core.Point
	C core.Color
}

// Triangle is a solid primitive in logical UI coordinates.
type Triangle struct {
	A, B, C Vertex
}

// Op is one display-list command. Clip is the intersection of every clipping
// container the primitive was recorded in.
type Op struct {
	Kind   Kind
	Clip   core.Rect
	Rect   core.Rect
	Radius float32
	Stroke float32
	Color  core.Color
	Glyphs []Glyph
	Tris   []Triangle

	// Image/texture quad fields. UV is normalized (0..1) with V growing
	// downward, matching the top-down pixel layout of Image.
	Image *Image
	UV    core.Rect
}

// Translate moves an op and its clip into a parent's coordinate space.
func (o *Op) Translate(dx, dy float32) {
	o.Clip.X += dx
	o.Clip.Y += dy
	o.Rect.X += dx
	o.Rect.Y += dy
	for i := range o.Glyphs {
		o.Glyphs[i].X += dx
		o.Glyphs[i].Y += dy
	}
	for i := range o.Tris {
		o.Tris[i].A.P.X += dx
		o.Tris[i].A.P.Y += dy
		o.Tris[i].B.P.X += dx
		o.Tris[i].B.P.Y += dy
		o.Tris[i].C.P.X += dx
		o.Tris[i].C.P.Y += dy
	}
}

// ClampClip intersects the op's clip with r.
func (o *Op) ClampClip(r core.Rect) {
	o.Clip = o.Clip.Intersect(r)
}
