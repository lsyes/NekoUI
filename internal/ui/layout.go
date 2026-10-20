package ui

import (
	"math"

	"nekoui/internal/core"
	"nekoui/internal/render"
)

type flow uint8

const (
	flowVertical flow = iota
	flowHorizontal
)

type layer struct {
	flow flow
	root bool

	parent *layer
	x, y   float32

	avail  Size
	fixedW float32
	fixedH float32
	fillW  bool
	fillH  bool

	pad   Insets
	gap   float32
	align Align

	clip       Rect
	clipBounds bool
	scrollX    float32
	scrollY    float32

	bg        Color
	bgSet     bool
	border    Color
	borderSet bool
	borderW   float32
	radius    float32

	preOps   []render.Op
	postOps  []render.Op
	children []child
	ops      []render.Op // leaf primitives only; composed at the end of the frame
	hits     []core.Hit  // this layer's own hits, emitted below its children
	postHits []core.Hit  // hits above children, e.g. a scrollbar
	size     Size
}

type child struct {
	l *layer
	w float32
	h float32

	weight float32
	body   func()

	x, y float32
}

var hugeRect = core.Rect{X: -1e6, Y: -1e6, W: 2e6, H: 2e6}

// Layers are recycled between frames. Their slice capacities (ops, hits,
// children) are kept, which removes the per-widget/per-container allocations
// that dominated frame cost. A layer is never reused within the same frame.
func (c *Ctx) allocLayer() *layer {
	n := len(c.layerFree)
	if n == 0 {
		l := new(layer)
		c.layerUsed = append(c.layerUsed, l)
		return l
	}
	l := c.layerFree[n-1]
	c.layerFree = c.layerFree[:n-1]
	*l = layer{
		preOps:   l.preOps[:0],
		postOps:  l.postOps[:0],
		ops:      l.ops[:0],
		hits:     l.hits[:0],
		postHits: l.postHits[:0],
		children: l.children[:0],
	}
	c.layerUsed = append(c.layerUsed, l)
	return l
}

func (c *Ctx) recycleLayers() {
	c.layerFree = append(c.layerFree, c.layerUsed...)
	c.layerUsed = c.layerUsed[:0]
}

// place and leaf are deliberately not inlined. Go 1.27 on loong64 miscompiles
// inlined calls that take a Size aggregate while surrounding closures capture
// stack values: one of the float32 fields is silently dropped (observed as
// Checkbox width 0, Radio height 8 and Slider width 128). Keeping the aggregate
// argument behind a real call boundary materializes it correctly. These calls
// happen once per widget; the cost is noise next to text shaping.
//
//go:noinline
func (c *Ctx) place(l *layer, size Size) {
	l.size = size
	c.current.children = append(c.current.children, child{l: l, w: size.W, h: size.H})
}

//go:noinline
func (c *Ctx) leaf(size Size, draw func(*Painter)) *layer {
	l := c.allocLayer()
	l.size = size
	l.clip = core.Rect{W: size.W, H: size.H}
	if draw != nil {
		draw(c.takePainter(l, l.clip))
	}
	return l
}

func (c *Ctx) currentScrollID() uint64 {
	if n := len(c.scrollStack); n > 0 {
		return c.scrollStack[n-1]
	}
	return 0
}

func (c *Ctx) addHit(l *layer, id uint64, rect Rect, cursor Cursor, focusable, disabled bool) {
	if rect.IsEmpty() {
		return
	}
	if cursor == CursorArrow && focusable {
		cursor = CursorIBeam
	}
	l.hits = append(l.hits, core.Hit{
		ID:        id,
		Rect:      rect,
		Clip:      l.clip,
		Cursor:    cursor,
		Focusable: focusable,
		Disabled:  disabled,
		ScrollID:  c.currentScrollID(),
	})
}

// avail returns the usable content area of the current container.
func (c *Ctx) avail() Size {
	l := c.current
	if l == nil {
		return Size{W: float32(c.W), H: float32(c.H)}
	}
	w, h := l.avail.W, l.avail.H
	if l.fixedW > 0 {
		w = l.fixedW
	}
	if l.fixedH > 0 {
		h = l.fixedH
	}
	if w > 0 {
		w -= l.pad.Horizontal()
	}
	if h > 0 {
		h -= l.pad.Vertical()
	}
	return Size{W: max32(0, w), H: max32(0, h)}
}

// Available reports the usable content area of the current container. Custom
// canvases use it to size themselves to the surrounding card.
func (c *Ctx) Available() Size { return c.avail() }

func (c *Ctx) container(fl flow, body func(), o *options) {
	parent := c.current
	a := c.avail()
	l := c.allocLayer()
	l.flow = fl
	l.avail = a
	l.pad = o.pad
	l.gap = o.gap
	l.align = o.align
	l.clip = hugeRect
	l.radius = o.radius
	if o.width > 0 {
		l.fixedW = o.width
	}
	if o.height > 0 {
		l.fixedH = o.height
	}
	l.fillW, l.fillH = o.fillW, o.fillH
	l.clipBounds = o.clip
	if o.bgSet {
		l.bg, l.bgSet = o.bg, true
	}
	if o.borderSet {
		l.border, l.borderSet = o.border, true
	}
	if o.borderW > 0 {
		l.borderW = o.borderW
	}
	if o.radius < 0 {
		l.radius = 0
	}

	c.current = l
	c.layers = append(c.layers, l)
	if body != nil {
		body()
	}
	c.finishLayer(l)
	c.layers = c.layers[:len(c.layers)-1]
	c.current = parent
	c.place(l, l.size)
}

// Column stacks children vertically.
func (c *Ctx) Column(body func(), opts ...Option) {
	c.container(flowVertical, body, c.options(opts))
}

// Row lays children out horizontally.
func (c *Ctx) Row(body func(), opts ...Option) {
	c.container(flowHorizontal, body, c.options(opts))
}

// Card is a themed panel: fill width, rounded background, border, padding and
// vertical child flow. Its height hugs its content unless Height is set.
func (c *Ctx) Card(body func(), opts ...Option) {
	o := c.takeOptions()
	o.fillW = c.current != nil && c.current.flow == flowVertical
	o.pad = All(c.Theme.Pad)
	o.gap = 10
	o.radius = c.Theme.Radius
	o.bg = c.Theme.Panel
	o.bgSet = true
	o.border = c.Theme.Border
	o.borderSet = true
	o.borderW = 1
	o.clip = true
	c.applyOptions(o, opts)
	c.container(flowVertical, body, o)
}

// Flex reserves a weighted share of the remaining main-axis space. Its body is
// executed once, after fixed siblings have been measured, with the allocated
// size known. A nil body is a flexible spacer.
func (c *Ctx) Flex(weight float32, body func()) {
	if weight <= 0 {
		weight = 1
	}
	if c.current == nil {
		if body != nil {
			body()
		}
		return
	}
	c.current.children = append(c.current.children, child{weight: weight, body: body})
}

// Spacer consumes the remaining space in a Row or Column. Multiple spacers split
// it by weight.
func (c *Ctx) Spacer() { c.Flex(1, nil) }

func (c *Ctx) finishLayer(l *layer) {
	c.executeFlex(l)

	// Content extents.
	var contentW, contentH float32
	if l.flow == flowVertical {
		for i, ch := range l.children {
			if ch.l != nil {
				contentW = max32(contentW, ch.l.size.W)
			}
			contentH += ch.h
			if i > 0 {
				contentH += l.gap
			}
		}
	} else {
		for i, ch := range l.children {
			if ch.l != nil {
				contentH = max32(contentH, ch.l.size.H)
			}
			contentW += ch.w
			if i > 0 {
				contentW += l.gap
			}
		}
	}

	w, h := contentW+l.pad.Horizontal(), contentH+l.pad.Vertical()
	if l.fixedW > 0 {
		w = l.fixedW
	} else if l.fillW && l.avail.W > 0 {
		w = l.avail.W
	}
	if l.fixedH > 0 {
		h = l.fixedH
	} else if l.fillH && l.avail.H > 0 {
		h = l.avail.H
	}
	if l.root {
		w, h = float32(c.W), float32(c.H)
	}
	if w < l.pad.Horizontal() {
		w = l.pad.Horizontal()
	}
	if h < l.pad.Vertical() {
		h = l.pad.Vertical()
	}
	l.size = Size{W: w, H: h}

	innerW := w - l.pad.Horizontal()
	innerH := h - l.pad.Vertical()

	// Place children along the main axis and align on the cross axis. Scroll
	// offsets are applied to each child once here so flattening is pure
	// translation.
	if l.flow == flowVertical {
		y := l.pad.T
		for i := range l.children {
			ch := &l.children[i]
			ch.x = l.pad.L + core.AlignOffset(l.align, innerW, ch.w)
			ch.y = y
			y += ch.h + l.gap
			if ch.l != nil {
				ch.l.parent = l
				ch.l.x = ch.x - l.scrollX
				ch.l.y = ch.y - l.scrollY
			}
		}
	} else {
		x := l.pad.L
		for i := range l.children {
			ch := &l.children[i]
			ch.x = x
			ch.y = l.pad.T + core.AlignOffset(l.align, innerH, ch.h)
			x += ch.w + l.gap
			if ch.l != nil {
				ch.l.parent = l
				ch.l.x = ch.x - l.scrollX
				ch.l.y = ch.y - l.scrollY
			}
		}
	}
}

// flattenLayer produces the final display list and hit tree in one pass. Layout
// deliberately does not compose per-layer op/hit slices: that used to copy every
// descendant primitive and hit once per ancestor, dominating frame allocation.
func (c *Ctx) flattenLayer(l *layer, dx, dy float32, clip Rect, ops *[]render.Op, hits *[]core.Hit) {
	ownClip := clip
	if l.clipBounds || l.root {
		bounds := Rect{W: l.size.W, H: l.size.H}
		if l.root {
			bounds = Rect{W: float32(c.W), H: float32(c.H)}
		}
		bounds.X += dx
		bounds.Y += dy
		ownClip = ownClip.Intersect(bounds)
	}

	emit := func(op render.Op) {
		op.Translate(dx, dy)
		op.ClampClip(ownClip)
		if !op.Clip.IsEmpty() {
			*ops = append(*ops, op)
		}
	}
	emitHit := func(h core.Hit) {
		h.Rect = h.Rect.Translate(dx, dy)
		h.Clip = h.Clip.Translate(dx, dy).Intersect(ownClip)
		if !h.Rect.IsEmpty() && !h.Clip.IsEmpty() {
			*hits = append(*hits, h)
		}
	}

	if l.bgSet && l.bg.A > 0 {
		emit(render.Op{
			Kind:   render.RectFill,
			Clip:   core.Rect{W: l.size.W, H: l.size.H},
			Rect:   core.Rect{W: l.size.W, H: l.size.H},
			Radius: l.radius,
			Color:  l.bg,
		})
	}
	for _, op := range l.preOps {
		emit(op)
	}
	for _, op := range l.ops {
		emit(op)
	}
	// Own hits sit below child hits; postHits (scrollbars) sit above them.
	for _, h := range l.hits {
		emitHit(h)
	}
	for i := range l.children {
		ch := &l.children[i]
		if ch.l == nil {
			continue
		}
		c.flattenLayer(ch.l, dx+ch.l.x, dy+ch.l.y, ownClip, ops, hits)
	}
	for _, op := range l.postOps {
		emit(op)
	}
	if l.borderSet && l.borderW > 0 {
		emit(render.Op{
			Kind:   render.RectStroke,
			Clip:   core.Rect{W: l.size.W, H: l.size.H},
			Rect:   core.Rect{W: l.size.W, H: l.size.H},
			Radius: l.radius,
			Stroke: l.borderW,
			Color:  l.border,
		})
	}
	for _, h := range l.postHits {
		emitHit(h)
	}
}

func (c *Ctx) executeFlex(l *layer) {
	hasFlex := false
	for _, ch := range l.children {
		if ch.weight > 0 {
			hasFlex = true
			break
		}
	}
	if !hasFlex {
		return
	}

	mainAvail := float32(0)
	innerW := float32(0)
	innerH := float32(0)
	if l.fixedW > 0 {
		innerW = l.fixedW - l.pad.Horizontal()
	} else if l.avail.W > 0 {
		innerW = l.avail.W - l.pad.Horizontal()
	}
	if l.fixedH > 0 {
		innerH = l.fixedH - l.pad.Vertical()
	} else if l.avail.H > 0 {
		innerH = l.avail.H - l.pad.Vertical()
	}
	if l.flow == flowHorizontal {
		mainAvail = innerW
	} else {
		mainAvail = innerH
	}

	var fixedMain, totalWeight float32
	for i, ch := range l.children {
		if ch.weight > 0 {
			totalWeight += ch.weight
		} else if l.flow == flowHorizontal {
			fixedMain += ch.w
		} else {
			fixedMain += ch.h
		}
		if i > 0 {
			fixedMain += l.gap
		}
	}
	remaining := mainAvail - fixedMain
	if remaining < 0 {
		remaining = 0
	}
	if totalWeight <= 0 {
		return
	}

	for i := range l.children {
		ch := &l.children[i]
		if ch.weight <= 0 {
			continue
		}
		alloc := remaining * ch.weight / totalWeight
		if ch.body == nil {
			if l.flow == flowHorizontal {
				ch.w, ch.h = alloc, 0
			} else {
				ch.w, ch.h = 0, alloc
			}
			continue
		}
		fl := c.allocLayer()
		fl.flow = flowVertical
		fl.clip = hugeRect
		fl.avail = Size{W: max32(0, innerW), H: max32(0, innerH)}
		fl.align = Start
		if l.flow == flowHorizontal {
			fl.fixedW = max32(0, alloc)
			fl.avail.W = max32(0, alloc)
		} else {
			fl.fixedH = max32(0, alloc)
			fl.avail.H = max32(0, alloc)
			fl.fillW = innerW > 0
			fl.avail.W = max32(0, innerW)
		}
		c.current = fl
		c.layers = append(c.layers, fl)
		ch.body()
		c.finishLayer(fl)
		c.layers = c.layers[:len(c.layers)-1]
		c.current = l
		ch.l = fl
		ch.w, ch.h = fl.size.W, fl.size.H
		if l.flow == flowHorizontal {
			ch.w = alloc
			fl.size.W = alloc
		} else {
			ch.h = alloc
			fl.size.H = alloc
		}
	}
}

// Fixed returns a fixed-sized clipping container. It is useful for custom
// layout islands and for wrapping a Row/Column in an exact slot.
func (c *Ctx) Fixed(w, h float32, body func(), opts ...Option) {
	o := c.options(opts)
	o.width, o.height = w, h
	o.clip = true
	c.container(flowVertical, body, o)
}

// Divider draws a one-logical-pixel horizontal rule across the available width.
func (c *Ctx) Divider(opts ...Option) {
	o := c.options(opts)
	a := c.avail()
	w := o.width
	if w <= 0 {
		w = a.W
	}
	h := o.height
	if h <= 0 {
		h = 1
	}
	l := c.leaf(Size{W: w, H: h}, func(p *Painter) {
		col := c.Theme.Border
		if o.fgSet {
			col = o.fg
		}
		p.Rect(Rect{W: w, H: h}, col)
	})
	c.place(l, Size{W: w, H: h})
}

func max32(a, b float32) float32 {
	return float32(math.Max(float64(a), float64(b)))
}
