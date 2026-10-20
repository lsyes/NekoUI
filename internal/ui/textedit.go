package ui

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"nekoui/internal/core"
)

type textEditor struct {
	caret    int
	anchor   int
	scrollX  float32
	dragging bool

	compose       string
	composeCursor int
}

func (e *textEditor) clamp(n int) {
	if e.caret > n {
		e.caret = n
	}
	if e.caret < 0 {
		e.caret = 0
	}
	if e.anchor > n {
		e.anchor = n
	}
	if e.anchor < 0 {
		e.anchor = e.caret
	}
}

func (e *textEditor) selection() (int, int, bool) {
	a, b := e.caret, e.anchor
	if a > b {
		a, b = b, a
	}
	return a, b, a != b
}

func (e *textEditor) setCaret(n int, extend bool) {
	e.caret = n
	if !extend {
		e.anchor = n
	}
}

func (e *textEditor) deleteSelection(r []rune) []rune {
	a, b, ok := e.selection()
	if !ok {
		return r
	}
	r = append(r[:a], r[b:]...)
	e.caret = a
	e.anchor = a
	return r
}

func (e *textEditor) insert(r []rune, text string) []rune {
	r = e.deleteSelection(r)
	ins := []rune(text)
	out := make([]rune, 0, len(r)+len(ins))
	out = append(out, r[:e.caret]...)
	out = append(out, ins...)
	out = append(out, r[e.caret:]...)
	e.caret += len(ins)
	e.anchor = e.caret
	return out
}

// InputText draws a single-line text editor bound to value. It returns true when
// the value changed. The editor keeps caret/selection/scroll state by widget ID,
// so value remains the single source of truth in application state.
func (c *Ctx) InputText(value *string, opts ...Option) bool {
	o := c.options(opts)
	id := c.nextID(o)
	fs := o.fontSize
	if fs <= 0 {
		fs = c.Theme.FontSize
	}
	pad := XY(12, 10)
	if o.padSet {
		pad = o.pad
	}
	a := c.avail()
	w := float32(300)
	if o.width > 0 {
		w = o.width
	} else if a.W > 0 && a.W < w {
		w = a.W
	}
	if w < 120 {
		w = 120
	}
	lineH := c.font.LineHeight(pxFor(fs, c.Scale)) / c.Scale
	h := lineH + pad.Vertical() + 8
	if o.height > 0 {
		h = o.height
	}
	if h < 34 {
		h = 34
	}
	if value == nil {
		value = new(string)
	}
	ed := c.ui.editors[id]
	if ed == nil {
		ed = &textEditor{}
		c.ui.editors[id] = ed
	}
	if o.autofocus && !o.disabled && c.ui.focusID == 0 {
		c.ui.focusID = id
	}
	focused := c.ui.focusID == id && !o.disabled
	changed := false
	runes := []rune(*value)
	ed.clamp(len(runes))

	inner := Rect{X: pad.L, Y: pad.T, W: w - pad.Horizontal(), H: h - pad.Vertical()}
	textTop := inner.Y + (inner.H-lineH)/2

	if focused {
		if c.ui.focusClickID == id {
			n := c.indexAtX(runes, c.ui.focusClick.X-pad.L+ed.scrollX, fs)
			if c.ui.doubleClickedID == id {
				// Double click selects the word under the pointer.
				ed.anchor, ed.caret = wordBounds(runes, n)
				ed.dragging = false
			} else {
				ed.caret = n
				ed.anchor = n
				ed.dragging = true
			}
			c.ui.focusClickID = 0
		}
		// Left-button drag selects text. pointerLocal is kept up to date for the
		// active widget even when the pointer leaves the field, so dragging past
		// either edge extends the selection to the end of the line.
		if c.ui.activeID == id && c.Input != nil && c.Input.MouseDown(MouseLeft) {
			if p, ok := c.ui.pointerLocal[id]; ok {
				ed.caret = c.indexAtX(runes, p.X-pad.L+ed.scrollX, fs)
				ed.dragging = true
			}
		} else if ed.dragging && (c.Input == nil || !c.Input.MouseDown(MouseLeft)) {
			// The press and release may land in the same event batch (fast
			// drags, remote input). Finish the selection at the release point.
			if c.ui.clickedID == id {
				if p, ok := c.ui.clickLocal[id]; ok {
					ed.caret = c.indexAtX(runes, p.X-pad.L+ed.scrollX, fs)
				}
			}
			ed.dragging = false
		}
		for i := range c.events {
			if c.used[i] {
				continue
			}
			e := &c.events[i]
			switch e.Type {
			case core.EventText:
				if e.Text != "" {
					txt := strings.ReplaceAll(strings.ReplaceAll(e.Text, "\r", ""), "\n", " ")
					if txt != "" {
						runes = ed.insert(runes, txt)
						changed = true
						ed.compose = ""
					}
				}
				c.used[i] = true
			case core.EventPreedit:
				ed.compose = e.Text
				ed.composeCursor = e.Cursor
				c.used[i] = true
			case core.EventDeleteSurrounding:
				r := runes
				if a, b, ok := ed.selection(); ok {
					r = append(r[:a], r[b:]...)
					ed.caret, ed.anchor = a, a
				}
				r = deleteSurrounding(r, ed.caret, e.Before, e.After)
				runes = r
				ed.clamp(len(runes))
				changed = true
				c.used[i] = true
			case core.EventKeyDown:
				if c.handleEditorKey(ed, &runes, e, o, &changed) {
					c.used[i] = true
				}
			}
		}
		ed.clamp(len(runes))
	}
	if !focused {
		ed.compose = ""
		ed.dragging = false
	}

	*value = string(runes)

	// Keep the caret visible with a small horizontal scroll margin.
	caretX := c.measureRunes(runes[:ed.caret], fs)
	viewW := inner.W
	maxScroll := max32(0, c.measureRunes(runes, fs)+4-viewW)
	if caretX-ed.scrollX > viewW-2 {
		ed.scrollX = caretX - viewW + 8
	}
	if caretX-ed.scrollX < 0 {
		ed.scrollX = caretX - 4
	}
	if ed.scrollX < 0 {
		ed.scrollX = 0
	}
	if ed.scrollX > maxScroll {
		ed.scrollX = maxScroll
	}

	hovered := c.ui.hoverID == id
	size := Size{W: w, H: h}
	l := c.leaf(size, func(p *Painter) {
		radius := float32(10)
		if o.radius >= 0 {
			radius = o.radius
		}
		bg := c.Theme.PanelAlt
		if o.bgSet {
			bg = o.bg
		}
		border := c.Theme.Border
		bw := float32(1)
		if hovered {
			border = c.Theme.BorderStrong
		}
		if focused {
			border = c.Theme.Accent
			bw = 1.5
		}
		if o.borderSet {
			border = o.border
		}
		p.RoundRect(Rect{W: w, H: h}, radius, bg)
		p.StrokeRoundRect(Rect{W: w, H: h}.InsetAll(0.75), radius, bw, border)

		txtCol := c.Theme.Text
		if o.fgSet {
			txtCol = o.fg
		}
		p.PushClip(inner)
		x := pad.L - ed.scrollX
		if len(runes) == 0 && ed.compose == "" && o.placeholder != "" {
			p.Text(o.placeholder, Point{X: x, Y: textTop}, fs, c.Theme.TextFaint)
		}
		if a, b, ok := ed.selection(); ok && focused {
			x0 := pad.L - ed.scrollX + c.measureRunes(runes[:a], fs)
			x1 := pad.L - ed.scrollX + c.measureRunes(runes[:b], fs)
			p.Rect(Rect{X: x0 - 2, Y: textTop - 1, W: x1 - x0 + 4, H: lineH + 2},
				c.Theme.Accent.WithAlpha(52))
		}
		if len(runes) > 0 {
			p.Text(string(runes), Point{X: x, Y: textTop}, fs, txtCol)
		}
		caretDraw := pad.L - ed.scrollX + caretX
		if ed.compose != "" {
			cw := p.MeasureText(ed.compose, fs).W
			p.Text(ed.compose, Point{X: caretDraw, Y: textTop}, fs, c.Theme.Accent)
			p.Rect(Rect{X: caretDraw, Y: textTop + lineH + 1, W: cw, H: 1.5}, c.Theme.Accent)
			caretDraw += cw
		}
		if focused {
			// ~1.9 Hz blink, but still animate at display rate while focused.
			on := (c.Elapsed/(530*time.Millisecond))%2 == 0
			if on {
				p.Rect(Rect{X: caretDraw, Y: textTop - 1, W: 1.5, H: lineH + 2}, c.Theme.Accent)
			}
			c.Animate()
		}
		p.PopClip()
	})
	if !o.disabled {
		c.addHit(l, id, Rect{W: w, H: h}, CursorIBeam, true, false)
	}
	if focused && !o.disabled {
		spotX := pad.L - ed.scrollX + caretX
		c.textInputFor(l, Rect{X: spotX, Y: textTop - 1, W: 1.5, H: lineH + 2}, ed.caret, ed.anchor)
	}
	c.place(l, size)
	return changed
}

// deleteSurrounding removes whole runes around the caret. The IME byte counts
// may not line up with our copy of the text (or may cut a rune); we delete the
// largest whole-rune range that fits.
func deleteSurrounding(r []rune, caret, before, after int) []rune {
	if caret < 0 || caret > len(r) || (before <= 0 && after <= 0) {
		return r
	}
	start := caret
	for n := 0; start > 0; {
		size := utf8.RuneLen(r[start-1])
		if size < 0 || n+size > before {
			break
		}
		n += size
		start--
	}
	end := caret
	for n := 0; end < len(r); {
		size := utf8.RuneLen(r[end])
		if size < 0 || n+size > after {
			break
		}
		n += size
		end++
	}
	if start == end {
		return r
	}
	out := make([]rune, 0, len(r)-(end-start))
	out = append(out, r[:start]...)
	out = append(out, r[end:]...)
	return out
}

func (c *Ctx) handleEditorKey(ed *textEditor, runes *[]rune, e *core.Event, o *options, changed *bool) bool {
	r := *runes
	shift := e.Mods&ModShift != 0
	ctrl := e.Mods&ModCtrl != 0
	super := e.Mods&ModSuper != 0
	alt := e.Mods&ModAlt != 0
	if super || alt {
		return false
	}
	switch e.Key {
	case KeyEnter:
		ed.compose = ""
		if o.onSubmit != nil {
			o.onSubmit(string(r))
		}
		return true
	case KeyEscape:
		c.ui.focusID = 0
		return true
	case KeyBackspace:
		if a, b, ok := ed.selection(); ok {
			r = append(r[:a], r[b:]...)
			ed.caret, ed.anchor = a, a
		} else if ed.caret > 0 {
			start := ed.caret - 1
			if ctrl {
				start = wordStart(r, ed.caret)
			}
			r = append(r[:start], r[ed.caret:]...)
			ed.caret, ed.anchor = start, start
		}
		*changed = true
	case KeyDelete:
		if a, b, ok := ed.selection(); ok {
			r = append(r[:a], r[b:]...)
			ed.caret, ed.anchor = a, a
		} else if ed.caret < len(r) {
			end := ed.caret + 1
			if ctrl {
				end = wordEnd(r, ed.caret)
			}
			r = append(r[:ed.caret], r[end:]...)
		}
		*changed = true
	case KeyLeft:
		n := ed.caret
		if ctrl {
			n = wordStart(r, n)
		} else if n > 0 {
			n--
		}
		ed.setCaret(n, shift)
	case KeyRight:
		n := ed.caret
		if ctrl {
			n = wordEnd(r, n)
		} else if n < len(r) {
			n++
		}
		ed.setCaret(n, shift)
	case KeyHome:
		ed.setCaret(0, shift)
	case KeyEnd:
		ed.setCaret(len(r), shift)
	case KeyA:
		if ctrl {
			ed.anchor, ed.caret = 0, len(r)
			return true
		}
		return false
	case KeyC:
		if ctrl {
			if a, b, ok := ed.selection(); ok {
				c.engine.clipboard = string(r[a:b])
			}
			return true
		}
		return false
	case KeyX:
		if ctrl {
			if a, b, ok := ed.selection(); ok {
				c.engine.clipboard = string(r[a:b])
				r = append(r[:a], r[b:]...)
				ed.caret, ed.anchor = a, a
				*changed = true
			}
			return true
		}
		return false
	case KeyV:
		if ctrl {
			if c.engine.clipboard != "" {
				r = ed.insert(r, c.engine.clipboard)
				*changed = true
			}
			return true
		}
		return false
	default:
		return false
	}
	*runes = r
	return true
}

func (c *Ctx) measureRunes(r []rune, size float32) float32 {
	if len(r) == 0 {
		return 0
	}
	return c.font.Measure(string(r), pxFor(size, c.Scale)).W / c.Scale
}

func (c *Ctx) indexAtX(runes []rune, x, size float32) int {
	if x <= 0 {
		return 0
	}
	var w float32
	for i, r := range runes {
		g := c.font.Measure(string(r), pxFor(size, c.Scale)).W / c.Scale
		if x < w+g/2 {
			return i
		}
		w += g
	}
	return len(runes)
}

// wordBounds returns the rune range of the word around n. Clicking on
// whitespace selects the whitespace run; clicking past the end selects the last
// word.
func wordBounds(r []rune, n int) (int, int) {
	if len(r) == 0 {
		return 0, 0
	}
	if n < 0 {
		n = 0
	}
	if n >= len(r) {
		n = len(r) - 1
	}
	if unicode.IsSpace(r[n]) {
		start := n
		for start > 0 && unicode.IsSpace(r[start-1]) {
			start--
		}
		end := n + 1
		for end < len(r) && unicode.IsSpace(r[end]) {
			end++
		}
		return start, end
	}
	start := n
	for start > 0 && !unicode.IsSpace(r[start-1]) {
		start--
	}
	end := n + 1
	for end < len(r) && !unicode.IsSpace(r[end]) {
		end++
	}
	return start, end
}

func wordStart(r []rune, n int) int {
	for n > 0 && unicode.IsSpace(r[n-1]) {
		n--
	}
	for n > 0 && !unicode.IsSpace(r[n-1]) {
		n--
	}
	return n
}

func wordEnd(r []rune, n int) int {
	for n < len(r) && unicode.IsSpace(r[n]) {
		n++
	}
	for n < len(r) && !unicode.IsSpace(r[n]) {
		n++
	}
	return n
}
