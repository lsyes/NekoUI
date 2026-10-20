package render

// Image is an RGBA8 texture source. Pixels are stored top-down and
// alpha-premultiplied so the renderer can use the same premultiplied blend
// function as rectangles and glyphs. The UI layer owns the pixel buffer and
// bumps Version when it changes.
type Image struct {
	Width, Height int
	Pix           []byte // len(Pix) == Width*Height*4
	Version       uint64
}
