package render

import "nekoui/internal/core"

// Atlas is an R8 glyph atlas shared by all text runs in a frame. The text
// engine owns packing; the renderer only uploads dirty regions.
type Atlas struct {
	Width, Height int
	Pix           []byte
	dirty         []core.Rect
}

// NewAtlas allocates a cleared glyph atlas.
func NewAtlas(w, h int) *Atlas {
	return &Atlas{Width: w, Height: h, Pix: make([]byte, w*h)}
}

// MarkDirty records a pixel region that must be uploaded before the next draw.
func (a *Atlas) MarkDirty(r core.Rect) {
	if a == nil || r.IsEmpty() {
		return
	}
	a.dirty = append(a.dirty, r)
}

// Dirty returns the pending upload regions. The slice is owned by the atlas and
// is only valid until ClearDirty.
func (a *Atlas) Dirty() []core.Rect { return a.dirty }

// ClearDirty marks all pending uploads as consumed.
func (a *Atlas) ClearDirty() { a.dirty = a.dirty[:0] }
