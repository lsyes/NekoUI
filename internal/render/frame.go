package render

import "nekoui/internal/core"

// Frame is one fully constructed UI frame, in logical coordinates.
type Frame struct {
	Width, Height           int // logical window size
	PixelWidth, PixelHeight int // framebuffer size
	Scale                   float32
	Clear                   core.Color
	Ops                     []Op
	Atlas                   *Atlas
	// Images lists every texture referenced by Ops this frame. Renderers use it
	// to evict GPU textures for images that are no longer drawn.
	Images []*Image
}

// Stats describes the cost of the most recent GPU render.
type Stats struct {
	DrawCalls int
	Vertices  int
	CPUTime   int64 // nanoseconds spent generating/uploading vertices
}
