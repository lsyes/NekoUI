package ui

import (
	"math"

	"nekoui/internal/render"
)

// Painter records primitives into the current display list. Coordinates are
// local to the widget being painted; custom art uses a Canvas.
type Painter struct {
	c     *Ctx
	layer *layer
	clip  Rect
	stack []Rect
}

// takePainter and takeCanvas hand out per-frame scratch objects. Each slot is
// used by one widget invocation per frame and reset in begin, so painting does
// not allocate a Painter/Canvas for every label and custom canvas.
func (c *Ctx) takePainter(l *layer, clip Rect) *Painter {
	if c.painterN == len(c.painterSlab) {
		c.painterSlab = append(c.painterSlab, Painter{})
	}
	p := &c.painterSlab[c.painterN]
	c.painterN++
	p.c = c
	p.layer = l
	p.clip = clip
	p.stack = p.stack[:0]
	return p
}

func (c *Ctx) takeCanvas(p *Painter) *Canvas {
	if c.canvasN == len(c.canvasSlab) {
		c.canvasSlab = append(c.canvasSlab, Canvas{})
	}
	cv := &c.canvasSlab[c.canvasN]
	c.canvasN++
	*cv = Canvas{Painter: p, Theme: c.Theme}
	return cv
}

// PushClip intersects the current clip with r.
func (p *Painter) PushClip(r Rect) {
	p.stack = append(p.stack, p.clip)
	p.clip = p.clip.Intersect(r)
}

// PopClip restores the previous clip.
func (p *Painter) PopClip() {
	if n := len(p.stack); n > 0 {
		p.clip = p.stack[n-1]
		p.stack = p.stack[:n-1]
	}
}

func (p *Painter) add(op render.Op) {
	if op.Clip.IsEmpty() {
		return
	}
	op.ClampClip(p.layer.clip)
	if op.Clip.IsEmpty() {
		return
	}
	p.layer.ops = append(p.layer.ops, op)
}

// Rect fills an axis-aligned rectangle.
func (p *Painter) Rect(r Rect, col Color) {
	p.RoundRect(r, 0, col)
}

// RoundRect fills a rounded rectangle.
func (p *Painter) RoundRect(r Rect, radius float32, col Color) {
	if r.IsEmpty() || col.A == 0 {
		return
	}
	p.add(render.Op{
		Kind:   render.RectFill,
		Clip:   p.clip,
		Rect:   r,
		Radius: radius,
		Color:  col,
	})
}

// circlePtr is the internal circle path. Some built-in controls call it with an
// addressable color because Go 1.27/loong64 with -race can corrupt a Color
// aggregate passed directly from a paint closure into the inlined Circle path.
//
//go:noinline
func (p *Painter) circlePtr(center Point, radius float32, col *Color) {
	if radius <= 0 || col.A == 0 {
		return
	}
	r := Rect{X: center.X - radius, Y: center.Y - radius, W: radius * 2, H: radius * 2}
	p.add(render.Op{
		Kind:   render.RectFill,
		Clip:   p.clip,
		Rect:   r,
		Radius: radius,
		Color:  *col,
	})
}

// StrokeRoundRect outlines a rounded rectangle.
func (p *Painter) StrokeRoundRect(r Rect, radius, width float32, col Color) {
	if r.IsEmpty() || width <= 0 || col.A == 0 {
		return
	}
	p.add(render.Op{
		Kind:   render.RectStroke,
		Clip:   p.clip,
		Rect:   r,
		Radius: radius,
		Stroke: width,
		Color:  col,
	})
}

// Circle fills a circle.
func (p *Painter) Circle(center Point, radius float32, col Color) {
	if radius <= 0 || col.A == 0 {
		return
	}
	r := Rect{X: center.X - radius, Y: center.Y - radius, W: radius * 2, H: radius * 2}
	p.RoundRect(r, radius, col)
}

// Line draws a solid line with square caps.
func (p *Painter) Line(a, b Point, width float32, col Color) {
	if width <= 0 || col.A == 0 {
		return
	}
	dx, dy := b.X-a.X, b.Y-a.Y
	l := float32(math.Hypot(float64(dx), float64(dy)))
	if l < 0.0001 {
		return
	}
	ux, uy := dx/l, dy/l
	px, py := -uy*width/2, ux*width/2
	ex, ey := ux*width/2, uy*width/2
	a1 := Point{X: a.X - ex + px, Y: a.Y - ey + py}
	a2 := Point{X: a.X - ex - px, Y: a.Y - ey - py}
	b1 := Point{X: b.X + ex - px, Y: b.Y + ey - py}
	b2 := Point{X: b.X + ex + px, Y: b.Y + ey + py}
	p.quad(a1, a2, b1, b2, col)
}

// Polygon fills a convex polygon with a triangle fan.
func (p *Painter) Polygon(points []Point, col Color) {
	if len(points) < 3 || col.A == 0 {
		return
	}
	tris := make([]render.Triangle, 0, len(points)-2)
	for i := 1; i < len(points)-1; i++ {
		tris = append(tris, render.Triangle{
			A: render.Vertex{P: points[0], C: col},
			B: render.Vertex{P: points[i], C: col},
			C: render.Vertex{P: points[i+1], C: col},
		})
	}
	p.add(render.Op{Kind: render.Triangles, Clip: p.clip, Tris: tris})
}

// GradientTriangle draws a triangle with per-corner colors.
func (p *Painter) GradientTriangle(a, b, c Point, ca, cb, cc Color) {
	p.add(render.Op{
		Kind: render.Triangles,
		Clip: p.clip,
		Tris: []render.Triangle{{
			A: render.Vertex{P: a, C: ca},
			B: render.Vertex{P: b, C: cb},
			C: render.Vertex{P: c, C: cc},
		}},
	})
}

func (p *Painter) quad(a, b, c, d Point, col Color) {
	tris := []render.Triangle{
		{A: render.Vertex{P: a, C: col}, B: render.Vertex{P: b, C: col}, C: render.Vertex{P: c, C: col}},
		{A: render.Vertex{P: a, C: col}, B: render.Vertex{P: c, C: col}, C: render.Vertex{P: d, C: col}},
	}
	p.add(render.Op{Kind: render.Triangles, Clip: p.clip, Tris: tris})
}

// Image draws an already-decoded texture with rounded corners and a tint.
// Most callers use Ctx.Image, which computes fit/crop and layout for them.
func (p *Painter) Image(img *render.Image, r Rect, uv Rect, radius float32, tint Color) {
	if img == nil || img.Width <= 0 || img.Height <= 0 || r.IsEmpty() || tint.A == 0 {
		return
	}
	p.add(render.Op{
		Kind:   render.ImageQuad,
		Clip:   p.clip,
		Rect:   r,
		Radius: radius,
		Color:  tint,
		Image:  img,
		UV:     uv,
	})
}

// Text draws a single line with its top-left at pos. It returns the advance
// width in logical pixels.
func (p *Painter) Text(s string, pos Point, size float32, col Color) float32 {
	if s == "" || col.A == 0 || p.clip.IsEmpty() {
		return 0
	}
	if size <= 0 {
		size = p.c.Theme.FontSize
	}
	px := int(math.Round(float64(size * p.c.Scale)))
	if px < 4 {
		px = 4
	}
	adv := p.c.font.Measure(s, px)
	run := p.c.shapeRun(s, px, p.c.Scale)
	if len(run) > 0 {
		glyphs := p.c.glyphSpace(len(run))
		for i := range run {
			g := run[i]
			g.X += pos.X
			g.Y += pos.Y
			g.Color = col
			glyphs[i] = g
		}
		p.add(render.Op{Kind: render.Text, Clip: p.clip, Glyphs: glyphs})
	}
	return adv.W / p.c.Scale
}

// TextIn draws text centered vertically in r and aligned on the horizontal axis.
func (p *Painter) TextIn(r Rect, s string, size float32, col Color, align Align) float32 {
	if size <= 0 {
		size = p.c.Theme.FontSize
	}
	w := p.MeasureText(s, size).W
	lh := p.TextHeight(size)
	x := r.X
	switch align {
	case Center:
		x = r.X + (r.W-w)/2
	case End:
		x = r.Right() - w
	}
	y := r.Y + (r.H-lh)/2
	return p.Text(s, Point{X: x, Y: y}, size, col)
}

// TextHeight is the natural line height for a font size, in logical pixels.
func (p *Painter) TextHeight(size float32) float32 {
	if size <= 0 {
		size = p.c.Theme.FontSize
	}
	px := int(math.Round(float64(size * p.c.Scale)))
	return p.c.font.LineHeight(px) / p.c.Scale
}

// MeasureText measures a single line in logical pixels.
func (p *Painter) MeasureText(s string, size float32) Size {
	if size <= 0 {
		size = p.c.Theme.FontSize
	}
	px := int(math.Round(float64(size * p.c.Scale)))
	m := p.c.font.Measure(s, px)
	return Size{W: m.W / p.c.Scale, H: m.H / p.c.Scale}
}

// Ellipsize truncates s with an ellipsis so that it fits maxW.
func (p *Painter) Ellipsize(s string, size, maxW float32) string {
	if maxW <= 0 || p.MeasureText(s, size).W <= maxW {
		return s
	}
	ell := "…"
	r := []rune(s)
	lo, hi := 0, len(r)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if p.MeasureText(string(r[:mid])+ell, size).W <= maxW {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo <= 0 {
		return ell
	}
	return string(r[:lo]) + ell
}

// Canvas is a self-drawn, interactive rectangle. It gives custom widgets a
// Painter plus a Response resolved from the retained input frame.
type Canvas struct {
	*Painter
	Bounds Rect
	Response
	Cursor Cursor
	Theme  *Theme
}

// SetCursor changes the pointer shape while hovering this canvas.
func (cv *Canvas) SetCursor(c Cursor) { cv.Cursor = c }

// Canvas creates a self-drawn rectangle of size w×h. The draw callback runs once
// per frame. Use the embedded Response for hover/click/drag/wheel and the
// Painter for antialiased primitives and text.
func (c *Ctx) Canvas(w, h float32, draw func(*Canvas), opts ...Option) {
	o := c.options(opts)
	if o.width > 0 {
		w = o.width
	}
	if o.height > 0 {
		h = o.height
	}
	if w <= 0 || h <= 0 {
		return
	}
	id := c.nextID(o)
	size := Size{W: w, H: h}
	l := c.allocLayer()
	l.size = size
	l.clip = Rect{W: w, H: h}
	cv := c.takeCanvas(c.takePainter(l, l.clip))
	cv.Bounds = Rect{W: w, H: h}
	cv.Response = c.response(id)
	cv.Cursor = CursorArrow
	if o.cursorSet {
		cv.Cursor = o.cursor
	}
	if o.disabled {
		cv.Response = Response{}
	}
	if draw != nil {
		draw(cv)
	}
	if !o.disabled {
		c.addHit(l, id, Rect{W: w, H: h}, cv.Cursor, o.focusable, false)
	}
	c.place(l, size)
}

// Empty returns whether the painter's current clip is empty.
func (p *Painter) Empty() bool { return p.clip.IsEmpty() }
