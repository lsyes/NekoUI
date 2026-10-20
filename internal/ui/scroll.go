package ui

import (
	"nekoui/internal/core"
	"nekoui/internal/render"
)

// scrollState is retained per scrollable widget between frames. Geometry is
// cached so scrollbar drags can be resolved before the new content is measured.
type scrollState struct {
	x, y               float32
	contentW, contentH float32
	viewW, viewH       float32

	// vertical selects the wheel axis. parentID is the enclosing scrollable, so
	// wheel input can chain from an inner viewport to an outer one at its edge.
	vertical bool
	parentID uint64

	trackH, thumbH float32
	trackW, thumbW float32
}

const scrollWheelStep = 54

func scrollbarID(id uint64) uint64 { return id ^ 0x9e3779b97f4a7c15 }

// Scrollable draws a vertically scrolling viewport. The body is laid out with
// its natural height; wheel input, scrollbar dragging and keyboard-focused
// children all keep working while content is clipped to the viewport.
func (c *Ctx) Scrollable(body func(), opts ...Option) {
	c.scrollable(flowVertical, body, c.options(opts))
}

// ScrollableX is the horizontal counterpart of Scrollable.
func (c *Ctx) ScrollableX(body func(), opts ...Option) {
	c.scrollable(flowHorizontal, body, c.options(opts))
}

func (c *Ctx) scrollable(fl flow, body func(), o *options) {
	if c.current == nil {
		if body != nil {
			body()
		}
		return
	}
	id := c.nextID(o)
	st := c.ui.scroll[id]
	if st == nil {
		st = &scrollState{}
		c.ui.scroll[id] = st
	}
	vertical := fl == flowVertical

	a := c.avail()
	vw, vh := o.width, o.height
	if vw <= 0 {
		vw = a.W
	}
	if vh <= 0 {
		vh = a.H
	}
	if vw <= 0 {
		vw = 320
	}
	if vh <= 0 {
		vh = 240
	}

	sid := scrollbarID(id)
	// Scrollbar drag uses last frame's thumb geometry; it is stable while the
	// track does not move, and it lets us offset content before measuring it.
	if c.ui.activeID == sid && st.thumbH > 0 {
		if p, ok := c.ui.pointerLocal[sid]; ok {
			if vertical {
				travel := st.trackH - st.thumbH
				if travel > 0 {
					t := clamp01((p.Y - st.thumbH/2) / travel)
					st.y = t * max32(0, st.contentH-st.viewH)
				}
			} else {
				travel := st.trackW - st.thumbW
				if travel > 0 {
					t := clamp01((p.X - st.thumbW/2) / travel)
					st.x = t * max32(0, st.contentW-st.viewW)
				}
			}
		}
	}
	st.vertical = vertical
	st.parentID = 0
	if n := len(c.scrollStack); n > 0 {
		st.parentID = c.scrollStack[n-1]
	}
	// Wheel deltas were routed here by App.wheelAt using last frame's geometry.
	// Consume them now; nested viewports have already consumed their share.
	if w := c.ui.wheel[id]; w != (Point{}) {
		if vertical {
			st.y -= w.Y * scrollWheelStep
		} else {
			st.x += w.X * scrollWheelStep
		}
		delete(c.ui.wheel, id)
	}

	// Measure content with an unbounded main axis.
	parent := c.current
	content := c.allocLayer()
	content.flow = fl
	content.avail = Size{W: vw, H: vh}
	content.pad = o.pad
	content.gap = o.gap
	content.align = o.align
	content.clip = hugeRect
	if vertical {
		content.fixedW = vw
		content.avail.H = 0
	} else {
		content.fixedH = vh
		content.avail.W = 0
	}
	c.current = content
	c.layers = append(c.layers, content)
	c.scrollStack = append(c.scrollStack, id)
	if body != nil {
		body()
	}
	c.scrollStack = c.scrollStack[:len(c.scrollStack)-1]
	c.finishLayer(content)
	c.layers = c.layers[:len(c.layers)-1]
	c.current = parent

	st.contentW, st.contentH = content.size.W, content.size.H
	st.viewW, st.viewH = vw, vh
	maxX := max32(0, st.contentW-vw)
	maxY := max32(0, st.contentH-vh)
	st.x = clampF32(st.x, 0, maxX)
	st.y = clampF32(st.y, 0, maxY)

	vp := c.allocLayer()
	vp.flow = fl
	vp.fixedW = vw
	vp.fixedH = vh
	vp.clip = hugeRect
	vp.clipBounds = true
	vp.scrollX = st.x
	vp.scrollY = st.y
	vp.children = []child{{l: content, w: content.size.W, h: content.size.H}}
	c.finishLayer(vp)

	// The viewport itself accepts wheel input in empty areas. Own hits are
	// flattened below child hits, so widgets keep receiving pointer events.
	vp.hits = append(vp.hits, core.Hit{
		ID:       id,
		Rect:     Rect{W: vw, H: vh},
		Clip:     Rect{W: vw, H: vh},
		ScrollID: id,
		Cursor:   CursorArrow,
	})

	margin := float32(4)
	barW := float32(8)
	near := c.ui.hoverID == sid || c.ui.activeID == sid
	trackCol := c.Theme.Border.WithAlpha(90)
	if near {
		trackCol = c.Theme.BorderStrong.WithAlpha(140)
	}
	thumbCol := c.Theme.TextFaint.WithAlpha(170)
	if near {
		thumbCol = c.Theme.Accent.WithAlpha(210)
	}

	if !o.hideScrollbar {
		if vertical && st.contentH > vh {
			track := Rect{X: vw - margin - barW - 2, Y: margin, W: barW + 2, H: vh - margin*2}
			st.trackH = track.H
			st.thumbH = clampF32(track.H*float32(vh)/st.contentH, 24, track.H)
			ty := track.Y
			if maxY > 0 {
				ty += (st.y / maxY) * (track.H - st.thumbH)
			}
			thumb := Rect{X: track.X + 2, Y: ty, W: barW, H: st.thumbH}
			vp.postOps = append(vp.postOps,
				render.Op{Kind: render.RectFill, Clip: Rect{W: vw, H: vh}, Rect: track, Radius: track.W / 2, Color: trackCol},
				render.Op{Kind: render.RectFill, Clip: Rect{W: vw, H: vh}, Rect: thumb, Radius: barW / 2, Color: thumbCol},
			)
			vp.postHits = append(vp.postHits, core.Hit{
				ID: sid, Rect: track, Clip: Rect{W: vw, H: vh}, Cursor: CursorArrow, ScrollID: id,
			})
		} else if !vertical && st.contentW > vw {
			track := Rect{X: margin, Y: vh - margin - barW - 2, W: vw - margin*2, H: barW + 2}
			st.trackW = track.W
			st.thumbW = clampF32(track.W*float32(vw)/st.contentW, 24, track.W)
			tx := track.X
			if maxX > 0 {
				tx += (st.x / maxX) * (track.W - st.thumbW)
			}
			thumb := Rect{X: tx, Y: track.Y + 2, W: st.thumbW, H: barW}
			vp.postOps = append(vp.postOps,
				render.Op{Kind: render.RectFill, Clip: Rect{W: vw, H: vh}, Rect: track, Radius: track.H / 2, Color: trackCol},
				render.Op{Kind: render.RectFill, Clip: Rect{W: vw, H: vh}, Rect: thumb, Radius: barW / 2, Color: thumbCol},
			)
			vp.postHits = append(vp.postHits, core.Hit{
				ID: sid, Rect: track, Clip: Rect{W: vw, H: vh}, Cursor: CursorArrow, ScrollID: id,
			})
		}
	}

	c.place(vp, Size{W: vw, H: vh})
}

func clampF32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clamp01(v float32) float32 { return clampF32(v, 0, 1) }
