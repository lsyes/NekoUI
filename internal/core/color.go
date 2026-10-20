package core

// Color is a straight-alpha 8-bit RGBA color.
type Color struct {
	R, G, B, A uint8
}

// RGBA builds a color from components.
func RGBA(r, g, b, a uint8) Color { return Color{r, g, b, a} }

// Hex builds an opaque color from 0xRRGGBB.
func Hex(v uint32) Color {
	return Color{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}

// HexA builds a color from 0xRRGGBBAA.
func HexA(v uint32) Color {
	return Color{uint8(v >> 24), uint8(v >> 16), uint8(v >> 8), uint8(v)}
}

// WithAlpha returns the color with a replaced alpha.
func (c Color) WithAlpha(a uint8) Color {
	c.A = a
	return c
}

// ScaleAlpha multiplies alpha by f (0..1).
func (c Color) ScaleAlpha(f float32) Color {
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	c.A = uint8(float32(c.A) * f)
	return c
}

// Mix linearly interpolates between two colors, t in [0,1].
func (c Color) Mix(o Color, t float32) Color {
	if t <= 0 {
		return c
	}
	if t >= 1 {
		return o
	}
	l := func(a, b uint8) uint8 { return uint8(float32(a) + (float32(b)-float32(a))*t) }
	return Color{l(c.R, o.R), l(c.G, o.G), l(c.B, o.B), l(c.A, o.A)}
}

// Premultiplied returns linear-ish premultiplied components in [0,1] for the
// renderer. NekoUI uses straight alpha at the API boundary and premultiplies
// in the shader to keep edge antialiasing correct.
func (c Color) Premultiplied() (r, g, b, a float32) {
	a = float32(c.A) / 255
	return float32(c.R) / 255 * a, float32(c.G) / 255 * a, float32(c.B) / 255 * a, a
}

// Common colors.
var (
	Transparent = Color{}
	White       = Color{255, 255, 255, 255}
	Black       = Color{0, 0, 0, 255}
)
