package ui

import (
	"math"
	"strings"
	"unicode"
)

// Label draws single- or multi-line text. Newlines are honored; Wrap() breaks
// at the available width, otherwise each line is ellipsized to fit.
func (c *Ctx) Label(text string, opts ...Option) {
	o := c.options(opts)
	fs := o.fontSize
	if fs <= 0 {
		fs = c.Theme.FontSize
	}
	a := c.avail()
	innerW := float32(0)
	if o.width > 0 {
		innerW = o.width - o.pad.Horizontal()
	} else if o.wrap && a.W > 0 {
		innerW = a.W - o.pad.Horizontal()
	}
	// The overwhelmingly common case is one line. Avoid splitLines/slice
	// capture so static labels do not allocate at all.
	if !o.wrap && strings.IndexByte(text, '\n') < 0 {
		size := c.labelSizeOne(text, fs, a, o)
		l := c.leaf(size, func(p *Painter) {
			col := c.Theme.Text
			if o.fgSet {
				col = o.fg
			}
			c.paintBox(p, size.W, size.H, o)
			c.paintLabelLine(p, text, fs, col, size, o)
		})
		c.place(l, size)
		return
	}
	lines := splitLines(text)
	if o.wrap && innerW > 0 {
		lines = c.wrapText(text, fs, innerW)
	}
	size := c.labelSize(lines, fs, a, o)
	l := c.leaf(size, func(p *Painter) {
		col := c.Theme.Text
		if o.fgSet {
			col = o.fg
		}
		c.paintBox(p, size.W, size.H, o)
		c.paintLabelLines(p, lines, fs, col, size, o)
	})
	c.place(l, size)
}

func (c *Ctx) labelSizeOne(text string, fs float32, a Size, o *options) Size {
	lineH := c.font.LineHeight(pxFor(fs, c.Scale)) / c.Scale
	w := c.font.Measure(text, pxFor(fs, c.Scale)).W/c.Scale + o.pad.Horizontal()
	h := lineH + o.pad.Vertical()
	if o.width > 0 {
		w = o.width
	} else if o.fillW && a.W > 0 {
		w = a.W
	}
	if o.height > 0 {
		h = o.height
	}
	if h < lineH+o.pad.Vertical() {
		h = lineH + o.pad.Vertical()
	}
	return Size{W: max32(0, w), H: max32(0, h)}
}

func (c *Ctx) paintLabelLine(p *Painter, text string, fs float32, col Color, size Size, o *options) {
	lineH := p.TextHeight(fs)
	innerW := size.W - o.pad.Horizontal()
	y := o.pad.T
	if size.H > lineH {
		y = (size.H - lineH) / 2
	}
	s := p.Ellipsize(text, fs, innerW)
	w := p.MeasureText(s, fs).W
	x := o.pad.L
	switch o.align {
	case Center:
		x = o.pad.L + (innerW-w)/2
	case End:
		x = o.pad.L + innerW - w
	}
	p.Text(s, Point{X: x, Y: y}, fs, col)
}

func (c *Ctx) labelSize(lines []string, fs float32, a Size, o *options) Size {
	lineH := c.font.LineHeight(pxFor(fs, c.Scale)) / c.Scale
	w := float32(0)
	for _, line := range lines {
		m := c.font.Measure(line, pxFor(fs, c.Scale))
		w = max32(w, m.W/c.Scale)
	}
	w += o.pad.Horizontal()
	h := float32(len(lines))*lineH + o.pad.Vertical()
	if o.width > 0 {
		w = o.width
	} else if o.fillW && a.W > 0 {
		w = a.W
	} else if o.wrap && a.W > 0 && w > a.W {
		w = a.W
	}
	if o.height > 0 {
		h = o.height
	}
	if h < lineH+o.pad.Vertical() {
		h = lineH + o.pad.Vertical()
	}
	return Size{W: max32(0, w), H: max32(0, h)}
}

func (c *Ctx) paintLabelLines(p *Painter, lines []string, fs float32, col Color, size Size, o *options) {
	lineH := p.TextHeight(fs)
	innerW := size.W - o.pad.Horizontal()
	y := o.pad.T
	if len(lines) == 1 && size.H > lineH {
		y = (size.H - lineH) / 2
	}
	for _, line := range lines {
		s := line
		if !o.wrap {
			s = p.Ellipsize(line, fs, innerW)
		}
		w := p.MeasureText(s, fs).W
		x := o.pad.L
		switch o.align {
		case Center:
			x = o.pad.L + (innerW-w)/2
		case End:
			x = o.pad.L + innerW - w
		}
		p.Text(s, Point{X: x, Y: y}, fs, col)
		y += lineH
	}
}

func (c *Ctx) paintBox(p *Painter, w, h float32, o *options) {
	r := Rect{W: w, H: h}
	radius := o.radius
	if radius < 0 {
		radius = 0
	}
	if o.bgSet {
		p.RoundRect(r, radius, o.bg)
	}
	if o.borderSet && o.borderW > 0 {
		p.StrokeRoundRect(r.InsetAll(o.borderW/2), radius, o.borderW, o.border)
	}
}

func (c *Ctx) wrapText(text string, fs, maxW float32) []string {
	px := pxFor(fs, c.Scale)
	var out []string
	for _, para := range strings.Split(text, "\n") {
		para = strings.TrimRight(para, " \t")
		if para == "" {
			out = append(out, "")
			continue
		}
		r := []rune(para)
		for len(r) > 0 {
			for len(r) > 0 && unicode.IsSpace(r[0]) {
				r = r[1:]
			}
			if len(r) == 0 {
				break
			}
			var w float32
			end, lastSpace := 0, -1
			for end < len(r) {
				aw := c.font.Measure(string(r[end]), px).W / c.Scale
				if w+aw > maxW && end > 0 {
					break
				}
				w += aw
				if unicode.IsSpace(r[end]) {
					lastSpace = end
				}
				end++
			}
			if end >= len(r) {
				out = append(out, string(r[:end]))
				break
			}
			cut := end
			if lastSpace > 0 {
				cut = lastSpace
			}
			out = append(out, strings.TrimRight(string(r[:cut]), " "))
			if cut < len(r) && unicode.IsSpace(r[cut]) {
				cut++
			}
			r = r[cut:]
		}
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

// Button draws a themed push button and reports a completed click.
func (c *Ctx) Button(label string, opts ...Option) bool {
	o := c.options(opts)
	id := c.nextID(o)
	fs := o.fontSize
	if fs <= 0 {
		fs = c.Theme.FontSize
	}
	pad := XY(15, 9)
	if o.padSet {
		pad = o.pad
	}
	m := c.font.Measure(label, pxFor(fs, c.Scale))
	w := m.W/c.Scale + pad.Horizontal()
	h := m.H/c.Scale + pad.Vertical()
	if w < 64 {
		w = 64
	}
	if h < 36 {
		h = 36
	}
	if o.width > 0 {
		w = o.width
	}
	if o.height > 0 {
		h = o.height
	}
	hovered := c.ui.hoverID == id
	pressed := c.ui.activeID == id && c.Input.MouseDown(c.ui.activeButton)
	size := Size{W: w, H: h}
	l := c.leaf(size, func(p *Painter) {
		radius := float32(8)
		if o.radius >= 0 {
			radius = o.radius
		}
		var bg, fg, border Color
		c.buttonColors(o, hovered, pressed, &bg, &fg, &border)
		if o.disabled {
			bg = bg.ScaleAlpha(0.45)
			fg = fg.ScaleAlpha(0.55)
			border = border.ScaleAlpha(0.4)
		}
		if bg.A > 0 {
			p.RoundRect(Rect{W: w, H: h}, radius, bg)
		}
		if border.A > 0 {
			p.StrokeRoundRect(Rect{W: w, H: h}.InsetAll(0.5), radius, 1, border)
		}
		p.TextIn(Rect{W: w, H: h}, label, fs, fg, Center)
	})
	if !o.disabled {
		c.addHit(l, id, Rect{W: w, H: h}, CursorHand, false, false)
	}
	c.place(l, size)
	return !o.disabled && c.ui.clickedID == id
}

// buttonColors writes through pointers instead of returning a Color tuple.
// Go 1.27/loong64 with -race miscompiles the multi-value Color result inside
// Button's paint closure (fields get shifted); pointer writes keep the RGBA
// bytes intact.
func (c *Ctx) buttonColors(o *options, hovered, pressed bool, bg, fg, border *Color) {
	t := c.Theme
	*bg, *fg, *border = t.PanelAlt, t.Text, t.Border
	switch o.variant {
	case variantPrimary:
		*bg, *fg, *border = t.Accent, t.OnAccent, Transparent
		if hovered {
			*bg = t.AccentHover
		}
	case variantGhost:
		*bg, *fg, *border = Transparent, t.TextMuted, Transparent
		if hovered {
			*bg, *fg = t.PanelAlt, t.Text
		}
	case variantDanger:
		*bg, *fg, *border = t.Danger, t.OnDanger, Transparent
		if hovered {
			*bg = t.Danger.Mix(White, 0.12)
		}
	}
	if pressed {
		*bg = bg.Mix(Black, 0.16)
	}
	if o.bgSet {
		*bg = o.bg
	}
	if o.fgSet {
		*fg = o.fg
	}
	if o.borderSet {
		*border = o.border
	}
}

// Toggle draws a switch plus an optional label and flips value on click.
func (c *Ctx) Toggle(value *bool, label string, opts ...Option) bool {
	o := c.options(opts)
	id := c.nextID(o)
	fs := o.fontSize
	if fs <= 0 {
		fs = c.Theme.SmallSize
	}
	trackW, trackH := float32(42), float32(24)
	labelW := float32(0)
	if label != "" {
		labelW = c.font.Measure(label, pxFor(fs, c.Scale)).W/c.Scale + 10
	}
	h := trackH
	if label != "" {
		h = max32(h, c.font.LineHeight(pxFor(fs, c.Scale))/c.Scale)
	}
	w := trackW + labelW
	if o.width > 0 {
		w = o.width
	}
	if o.height > 0 {
		h = o.height
	}
	on := value != nil && *value
	hovered := c.ui.hoverID == id

	size := Size{W: w, H: h}
	l := c.leaf(size, func(p *Painter) {
		ty := (h - trackH) / 2
		track := Rect{X: 0, Y: ty, W: trackW, H: trackH}
		off := c.Theme.BorderStrong
		if hovered {
			off = c.Theme.TextFaint
		}
		trackCol := off
		if on {
			trackCol = c.Theme.Accent
		}
		if o.disabled {
			trackCol = trackCol.ScaleAlpha(0.4)
		}
		p.RoundRect(track, trackH/2, trackCol)
		kr := trackH/2 - 3
		kx := trackH / 2
		if on {
			kx = trackW - trackH/2
		}
		p.Circle(Point{X: kx, Y: ty + trackH/2}, kr, c.Theme.Panel)
		if label != "" {
			col := c.Theme.TextMuted
			if o.fgSet {
				col = o.fg
			}
			p.TextIn(Rect{X: trackW + 10, Y: 0, W: labelW - 10, H: h}, label, fs, col, Start)
		}
	})
	if !o.disabled && value != nil {
		c.addHit(l, id, Rect{W: w, H: h}, CursorHand, false, false)
	}
	c.place(l, size)
	if !o.disabled && value != nil && c.ui.clickedID == id {
		*value = !*value
		return true
	}
	return false
}

func splitLines(s string) []string {
	if s == "" {
		return []string{""}
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func pxFor(size, scale float32) int {
	px := int(math.Round(float64(size * scale)))
	if px < 4 {
		return 4
	}
	return px
}
