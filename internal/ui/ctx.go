package ui

import (
	"fmt"
	"runtime"
	"time"

	"nekoui/internal/core"
	"nekoui/internal/render"
	"nekoui/internal/text"
)

// Ctx is the per-frame UI context. It is only valid inside the view callback;
// do not retain it after Run returns or between frames.
type Ctx struct {
	// Logical window size.
	W, H int
	// DPI scale: physical pixels per logical pixel.
	Scale float32
	// Time since the previous frame.
	DT time.Duration
	// Time since the application started.
	Elapsed time.Duration
	// Monotonic frame counter.
	Frame int
	// Smoothed frames per second.
	FPS float32

	// Input is this frame's normalized input summary.
	Input *Input
	// Theme is the active theme. Mutate it or call UseTheme to switch.
	Theme *Theme

	// Backend/backendInfo describe the native windowing layer.
	Backend     string
	BackendInfo string
	// FontInfo identifies the loaded UI font.
	FontInfo string
	// GLInfo identifies the OpenGL implementation.
	GLInfo string
	// Stats is the cost of the previous GPU frame.
	Stats render.Stats

	engine *Engine
	font   *text.Engine
	ui     *uiState
	events []core.Event
	used   []bool

	root        *layer
	current     *layer
	layers      []*layer
	occ         map[uintptr]int
	path        []uint64
	pathHash    uint64
	scrollStack []uint64
	opsBuf      []render.Op
	hitsBuf     []core.Hit
	imageSeen   map[*render.Image]struct{}
	layerFree   []*layer
	layerUsed   []*layer
	optSlab     []options
	optN        int

	glyphChunks   [][]render.Glyph
	glyphChunk    int
	textRuns      map[textRunKey][]render.Glyph
	textRunGlyphs int
	painterSlab   []Painter
	painterN      int
	canvasSlab    []Canvas
	canvasN       int

	frame     *render.Frame
	animating bool
	quit      bool
	title     string

	// Input method state published to the backend after layout.
	textInput core.TextInput
	imeLayer  *layer
	imeCaret  Rect
	imeCursor int
	imeAnchor int
}

type uiState struct {
	hits       []core.Hit
	focusables []uint64

	hoverID         uint64
	activeID        uint64
	focusID         uint64
	clickedID       uint64
	doubleClickedID uint64
	activeButton    MouseButton
	focusClickID    uint64
	focusClick      Point
	lastClick       time.Time
	lastClickID     uint64
	lastClickPos    Point
	cursor          Cursor

	pointerLocal map[uint64]Point
	clickLocal   map[uint64]Point
	wheel        map[uint64]Point
	drag         map[uint64]Point
	scroll       map[uint64]*scrollState

	editors map[uint64]*textEditor
}

func newUIState() *uiState {
	return &uiState{
		pointerLocal: make(map[uint64]Point),
		clickLocal:   make(map[uint64]Point),
		wheel:        make(map[uint64]Point),
		drag:         make(map[uint64]Point),
		scroll:       make(map[uint64]*scrollState),
		editors:      make(map[uint64]*textEditor),
	}
}

// Response is the interaction state of a Canvas for the current frame.
type Response struct {
	Hovered       bool
	Pressed       bool
	Clicked       bool
	DoubleClicked bool
	Down          bool
	Focused       bool
	Mouse         Point
	Wheel         Point
	Drag          Point
}

func (c *Ctx) response(id uint64) Response {
	r := Response{
		Hovered:       c.ui.hoverID == id || c.ui.activeID == id,
		Pressed:       c.ui.activeID == id,
		Clicked:       c.ui.clickedID == id,
		DoubleClicked: c.ui.doubleClickedID == id,
		Focused:       c.ui.focusID == id,
		Mouse:         c.ui.pointerLocal[id],
		Wheel:         c.ui.wheel[id],
		Drag:          c.ui.drag[id],
	}
	r.Down = r.Pressed && c.Input != nil && c.Input.MouseDown(c.ui.activeButton)
	return r
}

func (c *Ctx) begin() {
	if c.occ == nil {
		c.occ = make(map[uintptr]int, 64)
	} else {
		clear(c.occ)
	}
	c.path = c.path[:0]
	c.pathHash = 0
	c.layers = c.layers[:0]
	c.scrollStack = c.scrollStack[:0]
	c.optN = 0
	for i := range c.glyphChunks {
		c.glyphChunks[i] = c.glyphChunks[i][:0]
	}
	c.glyphChunk = 0
	c.painterN = 0
	c.canvasN = 0
	c.root = c.allocLayer()
	c.root.flow = flowVertical
	c.root.avail = Size{W: float32(c.W), H: float32(c.H)}
	c.root.fixedW = float32(c.W)
	c.root.fixedH = float32(c.H)
	c.root.fillW = true
	c.root.fillH = true
	c.root.clip = Rect{W: float32(c.W), H: float32(c.H)}
	c.root.size = Size{W: float32(c.W), H: float32(c.H)}
	c.root.align = Start
	c.root.root = true
	c.current = c.root
	c.layers = append(c.layers, c.root)
	c.animating = false
	c.quit = false
	c.frame = nil
	c.textInput = core.TextInput{}
	c.imeLayer = nil
	c.imeCaret = Rect{}
	c.imeCursor, c.imeAnchor = 0, 0
}

func (c *Ctx) finish() *render.Frame {
	c.finishLayer(c.root)
	ops := c.opsBuf[:0]
	hits := c.hitsBuf[:0]
	c.flattenLayer(c.root, 0, 0, Rect{W: float32(c.W), H: float32(c.H)}, &ops, &hits)
	c.opsBuf, c.hitsBuf = ops, hits
	if c.imeLayer != nil {
		o := c.absOrigin(c.imeLayer)
		c.textInput = core.TextInput{
			Enabled: true,
			Caret:   c.imeCaret.Translate(o.X, o.Y),
			Cursor:  c.imeCursor,
			Anchor:  c.imeAnchor,
		}
	}
	clear := c.Theme.Bg
	f := &render.Frame{
		Width:       c.W,
		Height:      c.H,
		PixelWidth:  c.engine.pixelW,
		PixelHeight: c.engine.pixelH,
		Scale:       c.Scale,
		Clear:       clear,
		Ops:         ops,
		Atlas:       c.font.Atlas(),
	}
	if c.imageSeen == nil {
		c.imageSeen = make(map[*render.Image]struct{}, 8)
	}
	for k := range c.imageSeen {
		delete(c.imageSeen, k)
	}
	f.Images = f.Images[:0]
	for i := range f.Ops {
		if img := f.Ops[i].Image; img != nil {
			if _, ok := c.imageSeen[img]; !ok {
				c.imageSeen[img] = struct{}{}
				f.Images = append(f.Images, img)
			}
		}
	}
	c.frame = f
	if c.engine != nil && c.engine.debugOps {
		for i, op := range f.Ops {
			if i > 24 {
				break
			}
			fmt.Printf("op %d kind=%d rect=%+v clip=%+v color=%+v glyphs=%d\n", i, op.Kind, op.Rect, op.Clip, op.Color, len(op.Glyphs))
		}
	}
	c.ui.hits = hits
	c.ui.focusables = c.ui.focusables[:0]
	for _, h := range c.ui.hits {
		if h.Focusable && !h.Disabled {
			c.ui.focusables = append(c.ui.focusables, h.ID)
		}
	}
	c.recycleLayers()
	return f
}

// textInputFor publishes an editor's caret to the platform input method. The
// caret is local to l; finish resolves it to window coordinates after the layer
// tree has been composed (which is also when scroll offsets are known).
func (c *Ctx) textInputFor(l *layer, caret Rect, cursor, anchor int) {
	if l == nil {
		return
	}
	c.imeLayer = l
	c.imeCaret = caret
	c.imeCursor = cursor
	c.imeAnchor = anchor
}

func (c *Ctx) absOrigin(l *layer) Point {
	var p Point
	for l != nil {
		p.X += l.x
		p.Y += l.y
		l = l.parent
	}
	return p
}

// Scope gives nested widgets a stable identity namespace. It is the immediate
// mode equivalent of a keyed list item.
func (c *Ctx) Scope(key string, body func()) {
	if key == "" {
		body()
		return
	}
	oldHash := c.pathHash
	c.path = append(c.path, hashString(key))
	c.pathHash = mixID(c.pathHash, c.path[len(c.path)-1])
	defer func() {
		c.path = c.path[:len(c.path)-1]
		c.pathHash = oldHash
	}()
	body()
}

// nextID hashes the call site once per occurrence. Explicit IDs keep the
// path-aware hash so two loops with the same ID name under different Scopes stay
// distinct; automatic IDs use the call PC directly and avoid allocating a
// file:line string every frame.
func (c *Ctx) nextID(o *options) uint64 {
	if o != nil && o.id != "" {
		return hashParts(c.path, o.id)
	}
	var pcs [1]uintptr
	pc := uintptr(0)
	if runtime.Callers(3, pcs[:]) > 0 {
		pc = pcs[0]
	}
	key := pc
	n := c.occ[key]
	c.occ[key] = n + 1
	return mixID(mixID(c.pathHash, uint64(pc)), uint64(n)+1)
}

const (
	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211
)

type textRunKey struct {
	s     string
	px    int
	scale float32
	gen   uint64
}

// shapeRun returns a cached glyph layout relative to the text origin. Glyph
// positions and sizes are in logical pixels; color is applied when the run is
// copied into the frame arena, so theme changes do not invalidate geometry.
func (c *Ctx) shapeRun(s string, px int, scale float32) []render.Glyph {
	if s == "" || c.font == nil {
		return nil
	}
	key := textRunKey{s: s, px: px, scale: scale, gen: c.font.Generation()}
	if run, ok := c.textRuns[key]; ok {
		return run
	}
	glyphs := c.font.Shape(s, px, 0, 0, White)
	if len(glyphs) == 0 {
		return nil
	}
	inv := 1 / scale
	for i := range glyphs {
		g := &glyphs[i]
		g.X *= inv
		g.Y *= inv
		g.W *= inv
		g.H *= inv
	}
	if c.textRuns == nil {
		c.textRuns = make(map[textRunKey][]render.Glyph, 128)
	}
	if len(c.textRuns) > 4096 || c.textRunGlyphs+len(glyphs) > 200000 {
		clear(c.textRuns)
		c.textRunGlyphs = 0
	}
	c.textRuns[key] = glyphs
	c.textRunGlyphs += len(glyphs)
	return glyphs
}

// glyphSpace carves n glyph slots out of monotonically growing per-frame
// blocks. Ops keep subslices until the renderer consumes the frame; blocks are
// reset in begin, so a steady-state UI allocates no glyph storage per frame.
func (c *Ctx) glyphSpace(n int) []render.Glyph {
	if n <= 0 {
		return nil
	}
	for {
		if c.glyphChunk == len(c.glyphChunks) {
			size := 4096
			if n > size {
				size = n
			}
			c.glyphChunks = append(c.glyphChunks, make([]render.Glyph, 0, size))
		}
		buf := c.glyphChunks[c.glyphChunk]
		if cap(buf)-len(buf) >= n {
			start := len(buf)
			buf = buf[:start+n]
			c.glyphChunks[c.glyphChunk] = buf
			return buf[start : start+n]
		}
		c.glyphChunk++
	}
}

func hashString(s string) uint64 {
	h := uint64(fnvOffset64)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	return h
}

func hashParts(parts []uint64, more ...string) uint64 {
	h := uint64(fnvOffset64)
	mix := func(b byte) {
		h ^= uint64(b)
		h *= fnvPrime64
	}
	for _, p := range parts {
		for i := 0; i < 8; i++ {
			mix(byte(p >> (8 * i)))
		}
	}
	for _, s := range more {
		for i := 0; i < len(s); i++ {
			mix(s[i])
		}
		mix(0)
	}
	return h
}

// mixID is a splitmix64-style finalizer used to fold path, call site and
// occurrence number into a stable widget ID.
func mixID(h, v uint64) uint64 {
	x := h ^ v + 0x9e3779b97f4a7c15 + (h << 6) + (h >> 2)
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// Animate requests continuous frames while the UI is in this state. Animations
// and caret blink call this automatically.
func (c *Ctx) Animate() { c.animating = true }

// Quit closes the application after this frame.
func (c *Ctx) Quit() { c.quit = true }

// SetTitle updates the native window title.
func (c *Ctx) SetTitle(title string) { c.title = title }

// UseTheme switches the active theme for subsequent frames and for widgets
// drawn after this call in the current frame.
func (c *Ctx) UseTheme(t *Theme) {
	if t == nil {
		return
	}
	c.Theme = t
	c.engine.theme = t
}

// Background paints the window background. It is drawn before all content.
func (c *Ctx) Background(col Color) {
	c.root.bg = col
	c.root.bgSet = true
}

// Backdrop records absolute-coordinate decoration behind the layout tree. Use it
// for gradients/glows/dot grids that should not participate in layout.
func (c *Ctx) Backdrop(draw func(p *Painter)) {
	if draw == nil || c.root == nil {
		return
	}
	l := c.allocLayer()
	l.size = Size{W: float32(c.W), H: float32(c.H)}
	l.clip = Rect{W: float32(c.W), H: float32(c.H)}
	draw(c.takePainter(l, l.clip))
	c.root.preOps = append(c.root.preOps, l.ops...)
}

// Shortcut returns true once when the given chord was pressed and no earlier
// widget consumed it. Text fields consume their editing chords before a global
// shortcut can see them; place global shortcuts after inputs if they overlap.
func (c *Ctx) Shortcut(mods Modifiers, key Key) bool {
	for i := range c.events {
		if c.used[i] {
			continue
		}
		e := &c.events[i]
		if e.Type == core.EventKeyDown && e.Key == key && e.Mods == mods {
			c.used[i] = true
			return true
		}
	}
	return false
}

// ShortcutString formats a chord for labels.
func ShortcutString(mods Modifiers, key Key) string {
	return core.ShortcutString(mods, key)
}
