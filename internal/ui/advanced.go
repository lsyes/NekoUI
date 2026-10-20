package ui

// Checkbox draws a themed check box with a label and flips value on click. It
// returns true when the value changed this frame.
func (c *Ctx) Checkbox(value *bool, label string, opts ...Option) bool {
	o := c.options(opts)
	id := c.nextID(o)
	fs := o.fontSize
	if fs <= 0 {
		fs = c.Theme.FontSize
	}
	box := float32(20)
	lineH := c.font.LineHeight(pxFor(fs, c.Scale)) / c.Scale
	labelW := float32(0)
	if label != "" {
		labelW = c.font.Measure(label, pxFor(fs, c.Scale)).W/c.Scale + 9
	}
	h := max32(box, lineH)
	w := box + labelW
	if o.width > 0 {
		w = o.width
	}
	if o.height > 0 {
		h = o.height
	}
	on := value != nil && *value
	disabled := o.disabled || value == nil
	hovered := c.ui.hoverID == id && !disabled

	size := Size{W: w, H: h}
	l := c.leaf(size, func(p *Painter) {
		by := (h - box) / 2
		r := Rect{X: 0, Y: by, W: box, H: box}
		fill := c.Theme.PanelAlt
		border := c.Theme.BorderStrong
		if hovered {
			border = c.Theme.TextFaint
		}
		if on {
			fill, border = c.Theme.Accent, c.Theme.Accent
		}
		if disabled {
			fill = fill.ScaleAlpha(0.45)
			border = border.ScaleAlpha(0.45)
		}
		p.RoundRect(r, 5, fill)
		p.StrokeRoundRect(r.InsetAll(0.5), 5, 1, border)
		if on {
			col := c.Theme.OnAccent
			if disabled {
				col = col.ScaleAlpha(0.5)
			}
			p.Line(Point{X: 5, Y: by + 10}, Point{X: 8.5, Y: by + 14}, 2, col)
			p.Line(Point{X: 8.5, Y: by + 14}, Point{X: 15, Y: by + 6}, 2, col)
		}
		if label != "" {
			col := c.Theme.Text
			if o.fgSet {
				col = o.fg
			}
			if disabled {
				col = col.ScaleAlpha(0.5)
			}
			p.TextIn(Rect{X: box + 9, Y: 0, W: labelW - 9, H: h}, label, fs, col, Start)
		}
	})
	if !disabled {
		c.addHit(l, id, Rect{W: w, H: h}, CursorHand, false, false)
	}
	c.place(l, size)
	if !disabled && c.ui.clickedID == id {
		*value = !*value
		return true
	}
	return false
}

// Radio draws a radio dot with a label. The caller owns the selected index and
// usually passes the same pointer to a group of Radio calls. It returns true
// when this item was selected by the click.
func (c *Ctx) Radio(value *int, index int, label string, opts ...Option) bool {
	o := c.options(opts)
	id := c.nextID(o)
	fs := o.fontSize
	if fs <= 0 {
		fs = c.Theme.FontSize
	}
	box := float32(20)
	lineH := c.font.LineHeight(pxFor(fs, c.Scale)) / c.Scale
	labelW := float32(0)
	if label != "" {
		labelW = c.font.Measure(label, pxFor(fs, c.Scale)).W/c.Scale + 9
	}
	h := max32(box, lineH)
	w := box + labelW
	if o.width > 0 {
		w = o.width
	}
	if o.height > 0 {
		h = o.height
	}
	selected := value != nil && *value == index
	disabled := o.disabled || value == nil
	hovered := c.ui.hoverID == id && !disabled
	size := Size{W: w, H: h}
	l := c.leaf(size, func(p *Painter) {
		by := (h - box) / 2
		r := Rect{X: 0, Y: by, W: box, H: box}
		fill := c.Theme.PanelAlt
		border := c.Theme.BorderStrong
		if hovered {
			border = c.Theme.TextFaint
		}
		if selected {
			border = c.Theme.Accent
		}
		if disabled {
			fill = fill.ScaleAlpha(0.45)
			border = border.ScaleAlpha(0.45)
		}
		p.circlePtr(Point{X: box / 2, Y: by + box/2}, box/2, &fill)
		p.StrokeRoundRect(r.InsetAll(0.75), box/2, 1.5, border)
		if selected {
			col := c.Theme.Accent
			if disabled {
				col = col.ScaleAlpha(0.5)
			}
			p.Circle(Point{X: box / 2, Y: by + box/2}, 5.5, col)
		}
		if label != "" {
			col := c.Theme.Text
			if o.fgSet {
				col = o.fg
			}
			if disabled {
				col = col.ScaleAlpha(0.5)
			}
			p.TextIn(Rect{X: box + 9, Y: 0, W: labelW - 9, H: h}, label, fs, col, Start)
		}
	})
	if !disabled {
		c.addHit(l, id, Rect{W: w, H: h}, CursorHand, false, false)
	}
	c.place(l, size)
	if !disabled && c.ui.clickedID == id {
		*value = index
		return true
	}
	return false
}

// Slider draws a horizontal value slider. Dragging the knob or clicking the
// track updates value; it returns true when the value changed this frame.
func (c *Ctx) Slider(value *float32, min, max float32, opts ...Option) bool {
	o := c.options(opts)
	id := c.nextID(o)
	if max < min {
		min, max = max, min
	}
	w := float32(220)
	if o.width > 0 {
		w = o.width
	}
	h := float32(30)
	if o.height > 0 {
		h = o.height
	}
	disabled := o.disabled || value == nil
	hovered := c.ui.hoverID == id && !disabled
	active := c.ui.activeID == id && !disabled
	changed := false

	// Track geometry is fixed so hover growth of the knob does not shift the
	// value mapping under the pointer.
	const baseKnobR = float32(8)
	trackX := baseKnobR + 1
	trackW := max32(1, w-trackX*2)

	// Resolve interaction before painting: the knob must reflect a click or
	// drag in the same frame instead of lagging one frame behind.
	if !disabled {
		r := c.response(id)
		if r.Pressed || (r.Down && active) {
			nx := clampF32(r.Mouse.X-trackX, 0, trackW)
			nv := min + nx/trackW*(max-min)
			if o.step > 0 && nx < trackW {
				nv = min + float32(roundTo(float64((nv-min)/o.step)))*o.step
			}
			nv = clampF32(nv, min, max)
			if nv != *value {
				*value = nv
				changed = true
			}
		}
	}

	t := float32(0)
	if value != nil && max > min {
		t = clamp01((*value - min) / (max - min))
	}
	knobR := baseKnobR
	if active {
		knobR = 9
	} else if hovered {
		knobR = 8.5
	}
	knobX := trackX + t*trackW

	size := Size{W: w, H: h}
	l := c.leaf(size, func(p *Painter) {
		cy := h / 2
		track := Rect{X: trackX, Y: cy - 2, W: trackW, H: 4}
		bg := c.Theme.BorderStrong
		if disabled {
			bg = bg.ScaleAlpha(0.35)
		}
		p.RoundRect(track, 2, bg)
		fill := c.Theme.Accent
		if disabled {
			fill = fill.ScaleAlpha(0.35)
		}
		if knobX > trackX {
			p.RoundRect(Rect{X: trackX, Y: cy - 2, W: knobX - trackX, H: 4}, 2, fill)
		}
		knobCol := c.Theme.Panel
		border := c.Theme.BorderStrong
		if hovered || active {
			knobCol = c.Theme.Accent
			border = c.Theme.Accent
		}
		if disabled {
			knobCol = knobCol.ScaleAlpha(0.4)
			border = border.ScaleAlpha(0.4)
		}
		p.Circle(Point{X: knobX, Y: cy}, knobR, knobCol)
		p.StrokeRoundRect(Rect{X: knobX - knobR, Y: cy - knobR, W: knobR * 2, H: knobR * 2},
			knobR, 1.5, border)
	})
	if !disabled {
		c.addHit(l, id, Rect{W: w, H: h}, CursorHand, false, false)
	}
	c.place(l, size)
	return changed
}

// ProgressBar draws a 0..1 progress bar. Set Width/Height to size it; without
// Width it fills the available container width.
func (c *Ctx) ProgressBar(value float32, opts ...Option) {
	o := c.options(opts)
	w := o.width
	if w <= 0 {
		if a := c.avail(); a.W > 0 {
			w = a.W
		} else {
			w = 200
		}
	}
	h := float32(8)
	if o.height > 0 {
		h = o.height
	}
	t := clamp01(value)
	size := Size{W: w, H: h}
	l := c.leaf(size, func(p *Painter) {
		trackCol := c.Theme.PanelAlt
		if o.bgSet {
			trackCol = o.bg
		}
		fillCol := c.Theme.Accent
		if o.fgSet {
			fillCol = o.fg
		}
		radius := h / 2
		p.RoundRect(Rect{W: w, H: h}, radius, trackCol)
		if t > 0 {
			p.RoundRect(Rect{W: max32(h, w*t), H: h}, radius, fillCol)
		}
		if o.borderSet && o.borderW > 0 {
			p.StrokeRoundRect(Rect{W: w, H: h}.InsetAll(o.borderW/2), radius, o.borderW, o.border)
		}
	})
	c.place(l, size)
}

func roundTo(v float64) float64 {
	if v < 0 {
		return float64(int64(v - 0.5))
	}
	return float64(int64(v + 0.5))
}
