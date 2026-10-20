package ui

import (
	"fmt"
	"os"
	"time"

	"nekoui/internal/core"
	"nekoui/internal/render"
	"nekoui/internal/text"
)

// Engine owns the retained frame metadata (hit tree, focus, editors, scroll
// offsets, caches) and drives the user's view function. It is independent of
// the native window and GL: the app loop feeds normalized events in and feeds
// the produced display list to a renderer.
type Engine struct {
	theme *Theme
	view  func(*Ctx)
	ctx   *Ctx
	ui    *uiState

	input  Input
	events []core.Event
	used   []bool

	pixelW, pixelH  int
	stats           render.Stats
	clipboard       string
	images          map[string]*Image
	imageErrs       map[string]error
	animating       bool
	quit            bool
	title           string
	debug, debugOps bool
}

// NewEngine creates the frame engine for one view function. debug/debugOps
// enable the same NEKOUI_DEBUG logs as the native runner.
func NewEngine(font *text.Engine, theme *Theme, view func(*Ctx), debug, debugOps bool) *Engine {
	if theme == nil {
		theme = DefaultTheme()
	}
	e := &Engine{
		theme:     theme,
		view:      view,
		ui:        newUIState(),
		input:     Input{keys: make(map[Key]bool)},
		clipboard: "",
		images:    make(map[string]*Image),
		imageErrs: make(map[string]error),
		debug:     debug,
		debugOps:  debugOps,
	}
	e.ctx = &Ctx{
		engine: e,
		font:   font,
		ui:     e.ui,
		Theme:  theme,
		Input:  &e.input,
	}
	return e
}

// Ctx returns the public UI context. Info fields (Backend, FontInfo, ...) may
// be set before the first Build.
func (e *Engine) Ctx() *Ctx { return e.ctx }

// SetPixelSize records the framebuffer size used in the next frame.
func (e *Engine) SetPixelSize(w, h int) { e.pixelW, e.pixelH = w, h }

// SetStats publishes renderer counters to the view.
func (e *Engine) SetStats(s render.Stats) { e.stats = s }

// TextInput returns the input-method state published by the last frame.
func (e *Engine) TextInput() core.TextInput { return e.ctx.textInput }

// Animating reports whether the view requested continuous frames.
func (e *Engine) Animating() bool { return e.animating }

// Quit reports whether the view or the platform asked to close the window.
func (e *Engine) Quit() bool { return e.quit }

// Title returns the most recent title requested by the view.
func (e *Engine) Title() string { return e.title }

// Build runs one view pass and returns its display list.
func (e *Engine) Build(w, h int, scale float32, dt, elapsed time.Duration, frame int, fps float32) *render.Frame {
	if scale <= 0 {
		scale = 1
	}
	c := e.ctx
	c.W, c.H = w, h
	c.Scale = scale
	c.DT = dt
	c.Elapsed = elapsed
	c.Frame = frame
	c.FPS = fps
	c.Input = &e.input
	c.Theme = e.theme
	c.Stats = e.stats
	c.events = e.events
	c.used = e.used

	c.begin()
	if e.view != nil {
		e.view(c)
	}
	f := c.finish()
	e.animating = c.animating
	if c.quit {
		e.quit = true
	}
	if c.title != "" {
		e.title = c.title
	}
	return f
}

// Dispatch applies a batch of platform events to the retained hit tree and
// returns the cursor shape the native window should show.
func (e *Engine) Dispatch(evs []core.Event) Cursor {
	e.input.beginFrame()
	e.events = e.events[:0]
	e.used = e.used[:0]
	e.ui.clickedID = 0
	e.ui.doubleClickedID = 0
	clear(e.ui.wheel)
	clear(e.ui.clickLocal)
	clear(e.ui.pointerLocal)
	clear(e.ui.drag)

	for _, ev := range evs {
		if e.debug {
			fmt.Fprintf(os.Stderr, "nekoui event: %+v\n", ev)
		}
		e.events = append(e.events, ev)
		e.used = append(e.used, false)
		idx := len(e.events) - 1

		switch ev.Type {
		case core.EventKeyDown:
			e.input.keys[ev.Key] = true
			e.input.downKeys = append(e.input.downKeys, ev.Key)
			e.input.Mods = ev.Mods
			if ev.Key == KeyTab && ev.Mods&(ModCtrl|ModAlt|ModSuper) == 0 {
				e.focusNext(ev.Mods&ModShift != 0)
				e.used[idx] = true
			}
		case core.EventKeyUp:
			delete(e.input.keys, ev.Key)
			e.input.upKeys = append(e.input.upKeys, ev.Key)
			e.input.Mods = ev.Mods
		case core.EventText, core.EventPreedit:
			// Consumed by the focused editor during Build.
		case core.EventMouseMove:
			e.pointerTo(ev.X, ev.Y)
		case core.EventMouseDown:
			e.pointerTo(ev.X, ev.Y)
			if int(ev.Button) < len(e.input.buttons) {
				e.input.buttons[ev.Button] = true
				e.input.downMouse[ev.Button] = true
			}
			if ev.Button == MouseLeft {
				e.pointerDown(ev.X, ev.Y)
			}
		case core.EventMouseUp:
			e.pointerTo(ev.X, ev.Y)
			if int(ev.Button) < len(e.input.buttons) {
				e.input.buttons[ev.Button] = false
				e.input.upMouse[ev.Button] = true
			}
			e.pointerUp(ev.X, ev.Y, ev.Button)
		case core.EventScroll:
			e.input.Wheel = e.input.Wheel.Add(ev.DX, ev.DY)
			e.wheelAt(ev.X, ev.Y, ev.DX, ev.DY)
		case core.EventMouseEnter:
			e.input.MouseInside = true
			e.pointerTo(ev.X, ev.Y)
		case core.EventMouseLeave:
			e.input.MouseInside = false
			e.ui.hoverID = 0
			e.ui.cursor = CursorArrow
		case core.EventFocus:
			if !ev.Focused {
				e.input.clearAll()
				e.ui.activeID = 0
				e.ui.hoverID = 0
				e.ui.cursor = CursorArrow
			}
		case core.EventResize:
			e.pixelW, e.pixelH = ev.PixelWidth, ev.PixelHeight
		case core.EventClose:
			e.quit = true
		}
	}

	// Keep drag deltas for the active widget and make the local pointer
	// available to its Response.
	if e.ui.activeID != 0 {
		if h := e.hitByID(e.ui.activeID); h != nil {
			p := e.input.Mouse
			lp := Point{X: p.X - h.Rect.X, Y: p.Y - h.Rect.Y}
			prev := e.ui.pointerLocal[h.ID]
			if prev != (Point{}) {
				e.ui.drag[h.ID] = lp.Sub(prev)
			}
			e.ui.pointerLocal[h.ID] = lp
		}
	}
	return e.ui.cursor
}

func (e *Engine) focusNext(back bool) {
	list := e.ui.focusables
	if len(list) == 0 {
		return
	}
	idx := -1
	for i, id := range list {
		if id == e.ui.focusID {
			idx = i
			break
		}
	}
	if idx == -1 {
		if back {
			e.ui.focusID = list[len(list)-1]
		} else {
			e.ui.focusID = list[0]
		}
		return
	}
	if back {
		idx = (idx - 1 + len(list)) % len(list)
	} else {
		idx = (idx + 1) % len(list)
	}
	e.ui.focusID = list[idx]
	e.ui.focusClickID = 0
}

func (e *Engine) pointerTo(x, y float32) {
	e.input.MousePrev = e.input.Mouse
	e.input.Mouse = Point{X: x, Y: y}
	e.input.MouseInside = true
	if h := e.hitAt(x, y); h != nil {
		e.ui.hoverID = h.ID
		e.ui.cursor = h.Cursor
		e.ui.pointerLocal[h.ID] = Point{X: x - h.Rect.X, Y: y - h.Rect.Y}
	} else {
		e.ui.hoverID = 0
		e.ui.cursor = CursorArrow
	}
}

func (e *Engine) pointerDown(x, y float32) {
	h := e.hitAt(x, y)
	if h == nil {
		e.ui.activeID = 0
		e.ui.focusID = 0
		return
	}
	lp := Point{X: x - h.Rect.X, Y: y - h.Rect.Y}
	e.ui.activeID = h.ID
	e.ui.activeButton = MouseLeft
	e.ui.focusClickID = 0
	if h.Focusable {
		e.ui.focusID = h.ID
		e.ui.focusClickID = h.ID
		e.ui.focusClick = lp
	} else {
		e.ui.focusID = 0
	}
	e.ui.pointerLocal[h.ID] = lp
	e.ui.clickLocal[h.ID] = lp
	if e.debug {
		fmt.Fprintf(os.Stderr, "nekoui hit down: id=%d rect=%+v local=%+v\n", h.ID, h.Rect, lp)
	}
}

func (e *Engine) pointerUp(x, y float32, b MouseButton) {
	if b != e.ui.activeButton || e.ui.activeID == 0 {
		return
	}
	h := e.hitAt(x, y)
	id := e.ui.activeID
	if h != nil && h.ID == id {
		lp := Point{X: x - h.Rect.X, Y: y - h.Rect.Y}
		e.ui.clickedID = id
		e.ui.clickLocal[id] = lp
		if e.debug {
			fmt.Fprintf(os.Stderr, "nekoui click: id=%d rect=%+v local=%+v\n", id, h.Rect, lp)
		}
		now := time.Now()
		last := e.ui.lastClick
		if e.ui.lastClickID == id && now.Sub(last) < 420*time.Millisecond &&
			e.ui.lastClickPos.Distance(Point{X: x, Y: y}) < 6 {
			e.ui.doubleClickedID = id
		}
		e.ui.lastClick = now
		e.ui.lastClickID = id
		e.ui.lastClickPos = Point{X: x, Y: y}
	}
	e.ui.activeID = 0
}

func (e *Engine) hitAt(x, y float32) *core.Hit {
	p := Point{X: x, Y: y}
	for i := len(e.ui.hits) - 1; i >= 0; i-- {
		h := &e.ui.hits[i]
		if h.Disabled {
			continue
		}
		if !h.Rect.Inside(p) || !h.Clip.Inside(p) {
			continue
		}
		return h
	}
	return nil
}

func (e *Engine) hitByID(id uint64) *core.Hit {
	for i := range e.ui.hits {
		if e.ui.hits[i].ID == id {
			return &e.ui.hits[i]
		}
	}
	return nil
}

// wheelAt routes a wheel event to the widget under (x, y) and then walks the
// scrollable ancestor chain, giving each viewport only the movement it can
// still absorb. Nested Scrollables therefore scroll the innermost view first
// and hand the remainder to the parent at the edge.
func (e *Engine) wheelAt(x, y, dx, dy float32) {
	h := e.hitAt(x, y)
	if h == nil {
		return
	}
	lp := Point{X: x - h.Rect.X, Y: y - h.Rect.Y}
	e.ui.pointerLocal[h.ID] = lp

	// Custom canvases see the wheel even when an ancestor consumes it. The
	// viewport hit is the scroll container itself and is handled by the chain.
	if h.ID != h.ScrollID {
		e.ui.wheel[h.ID] = e.ui.wheel[h.ID].Add(dx, dy)
	}

	remX, remY := dx, dy
	for sid := h.ScrollID; sid != 0 && (remX != 0 || remY != 0); {
		st := e.ui.scroll[sid]
		if st == nil {
			break
		}
		e.consumeWheel(st, sid, &remX, &remY)
		sid = st.parentID
	}
}

// consumeWheel assigns as much of the remaining wheel delta to st as its
// current scroll range allows. Wheel units are notch fractions; Scrollable
// multiplies them by scrollWheelStep when it rebuilds.
func (e *Engine) consumeWheel(st *scrollState, id uint64, remX, remY *float32) {
	if st.vertical {
		if *remY == 0 {
			return
		}
		maxY := max32(0, st.contentH-st.viewH)
		if maxY <= 0 {
			return
		}
		pending := e.ui.wheel[id].Y
		eff := clampF32(st.y-pending*scrollWheelStep, 0, maxY)
		delta := -*remY * scrollWheelStep
		take := float32(0)
		if delta > 0 {
			take = min32(delta, maxY-eff)
		} else {
			take = max32(delta, -eff)
		}
		if take == 0 {
			return
		}
		e.ui.wheel[id] = e.ui.wheel[id].Add(0, -take/scrollWheelStep)
		*remY += take / scrollWheelStep
		return
	}
	// Horizontal viewports consume the horizontal axis only. A vertical wheel
	// over a horizontal-only viewport is intentionally left for an ancestor.
	if *remX == 0 {
		return
	}
	maxX := max32(0, st.contentW-st.viewW)
	if maxX <= 0 {
		return
	}
	pending := e.ui.wheel[id].X
	eff := clampF32(st.x-pending*scrollWheelStep, 0, maxX)
	delta := *remX * scrollWheelStep
	take := float32(0)
	if delta > 0 {
		take = min32(delta, maxX-eff)
	} else {
		take = max32(delta, -eff)
	}
	if take == 0 {
		return
	}
	e.ui.wheel[id] = e.ui.wheel[id].Add(take/scrollWheelStep, 0)
	*remX -= take / scrollWheelStep
}
