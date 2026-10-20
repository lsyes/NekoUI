package ui

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"nekoui/internal/render"
)

// ImageFit controls how a source image fills its widget box.
type ImageFit uint8

const (
	// ImageStretch fills the box, ignoring the source aspect ratio.
	ImageStretch ImageFit = iota
	// ImageContain letterboxes the source inside the box.
	ImageContain
	// ImageCover fills the box and crops the overflow.
	ImageCover
	// ImageNone draws the source at its natural size, centered and cropped.
	ImageNone
)

// Image is a decoded, GPU-ready RGBA texture. Create it once (NewImage or
// LoadImage) and draw it every frame with Ctx.Image. The zero value is empty.
type Image struct {
	data *render.Image
}

// NewImage converts any image.Image into a reusable NekoUI texture. The source
// is copied and alpha-premultiplied, so callers may mutate it afterwards.
func NewImage(src image.Image) *Image {
	if src == nil {
		return &Image{}
	}
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return &Image{}
	}
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return &Image{data: &render.Image{
		Width:   w,
		Height:  h,
		Pix:     dst.Pix,
		Version: 1,
	}}
}

// LoadImage decodes a PNG, JPEG or GIF file into a reusable texture.
func LoadImage(path string) (*Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("nekoui: open image %s: %w", path, err)
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("nekoui: decode image %s: %w", path, err)
	}
	return NewImage(src), nil
}

// Size is the source size in pixels.
func (im *Image) Size() Size {
	if im == nil || im.data == nil {
		return Size{}
	}
	return Size{W: float32(im.data.Width), H: float32(im.data.Height)}
}

// Valid reports whether the texture has pixels.
func (im *Image) Valid() bool {
	return im != nil && im.data != nil && im.data.Width > 0 && im.data.Height > 0
}

// Update uploads new pixels for an existing image, preserving its identity so
// GPU textures and layout stay stable. It is intended for video/animated
// sources; normal images only need NewImage once.
func (im *Image) Update(src image.Image) {
	if im == nil {
		return
	}
	next := NewImage(src)
	im.data = next.data
}

// Image draws an image with optional rounded corners, fit mode and tint. The
// natural size is the source pixel size; set Width/Height to resize. If only
// one dimension is set the aspect ratio is preserved.
func (c *Ctx) Image(img *Image, opts ...Option) {
	o := c.options(opts)
	radius := float32(0)
	if o.radius >= 0 {
		radius = o.radius
	}
	if !img.Valid() {
		c.imagePlaceholder(Rect{}, 0, 0, o, "no image")
		return
	}
	sw := float32(img.data.Width)
	sh := float32(img.data.Height)
	w, h := o.width, o.height
	switch {
	case w <= 0 && h <= 0:
		w, h = sw, sh
	case w <= 0:
		w = h * sw / sh
	case h <= 0:
		h = w * sh / sw
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	// Source rectangle inside the widget plus normalized UV coordinates.
	box := Rect{W: w, H: h}
	drawRect := box
	uv := Rect{X: 0, Y: 0, W: 1, H: 1}
	fit := ImageCover
	if o.fitSet {
		fit = o.fit
	}
	switch fit {
	case ImageStretch:
		// full box, full source
	case ImageContain:
		scale := min32(w/sw, h/sh)
		dw, dh := sw*scale, sh*scale
		drawRect = Rect{X: (w - dw) / 2, Y: (h - dh) / 2, W: dw, H: dh}
	case ImageNone:
		drawRect = Rect{X: (w - sw) / 2, Y: (h - sh) / 2, W: sw, H: sh}
		if sw > w || sh > h {
			// Crop the centered natural-size image.
			ux := (sw - w) / 2 / sw
			uy := (sh - h) / 2 / sh
			uv = Rect{X: ux, Y: uy, W: w / sw, H: h / sh}
			drawRect = box
		}
	case ImageCover:
		scale := max32(w/sw, h/sh)
		dw, dh := sw*scale, sh*scale
		ux := (dw - w) / 2 / dw
		uy := (dh - h) / 2 / dh
		uv = Rect{X: ux, Y: uy, W: w / dw, H: h / dh}
	}
	tint := White
	if o.tintSet {
		tint = o.tint
	}

	size := Size{W: w, H: h}
	l := c.leaf(size, func(p *Painter) {
		if o.bgSet {
			p.RoundRect(box, radius, o.bg)
		}
		p.Image(img.data, drawRect, uv, radius, tint)
		if o.borderSet && o.borderW > 0 {
			p.StrokeRoundRect(box.InsetAll(o.borderW/2), radius, o.borderW, o.border)
		}
	})
	c.place(l, size)
}

// ImageFile draws an image from disk, caching decoded textures by path. On a
// load error it draws a visible placeholder instead of failing the frame.
func (c *Ctx) ImageFile(path string, opts ...Option) {
	o := c.options(opts)
	if c.engine == nil {
		img, err := LoadImage(path)
		if err != nil {
			c.imagePlaceholder(Rect{}, o.width, o.height, o, err.Error())
			return
		}
		c.Image(img, opts...)
		return
	}
	if c.engine.images == nil {
		c.engine.images = make(map[string]*Image)
	}
	if c.engine.imageErrs == nil {
		c.engine.imageErrs = make(map[string]error)
	}
	if img, ok := c.engine.images[path]; ok {
		c.Image(img, opts...)
		return
	}
	if err, ok := c.engine.imageErrs[path]; ok {
		c.imagePlaceholder(Rect{}, o.width, o.height, o, err.Error())
		return
	}
	img, err := LoadImage(path)
	if err != nil {
		c.engine.imageErrs[path] = err
		c.imagePlaceholder(Rect{}, o.width, o.height, o, err.Error())
		return
	}
	c.engine.images[path] = img
	c.Image(img, opts...)
}

// imagePlaceholder draws a themed box with a short message so a missing asset
// is obvious but never breaks the layout.
func (c *Ctx) imagePlaceholder(_ Rect, w, h float32, o *options, msg string) {
	if w <= 0 {
		w = 160
	}
	if h <= 0 {
		h = 90
	}
	fs := o.fontSize
	if fs <= 0 {
		fs = c.Theme.SmallSize
	}
	size := Size{W: w, H: h}
	l := c.leaf(size, func(p *Painter) {
		p.RoundRect(Rect{W: w, H: h}, 8, c.Theme.PanelAlt)
		p.StrokeRoundRect(Rect{W: w, H: h}.InsetAll(0.5), 8, 1, c.Theme.Border)
		p.PushClip(Rect{W: w, H: h})
		p.TextIn(Rect{X: 8, Y: 0, W: w - 16, H: h}, p.Ellipsize(msg, fs, w-16),
			fs, c.Theme.TextFaint, Center)
		p.PopClip()
	})
	c.place(l, size)
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
