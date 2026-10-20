package ui

import (
	"fmt"
	"image"
	"image/color"
	"testing"
	"time"

	"nekoui/internal/core"
	"nekoui/internal/render"
	"nekoui/internal/text"
)

func newTestCtx(t testing.TB, w, h int) (*Ctx, *Engine) {
	t.Helper()
	e, err := text.Load(text.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(e, DefaultTheme(), nil, false, false)
	eng.SetPixelSize(w*2, h*2)
	c := eng.ctx
	c.W = w
	c.H = h
	c.Scale = 2
	return c, eng
}

func TestScrollableWheelHitsAndDrag(t *testing.T) {
	c, _ := newTestCtx(t, 400, 300)
	scrollID := hashParts(nil, "scroll")
	buttonID := hashParts(nil, "pick")
	build := func() *render.Frame {
		c.begin()
		c.Scrollable(func() {
			c.Button("pick", ID("pick"))
			for i := 0; i < 20; i++ {
				c.Label(fmt.Sprintf("row %02d", i), Height(30), ID(fmt.Sprintf("row-%02d", i)))
			}
		}, ID("scroll"), Height(100), Padding(0), Gap(0))
		return c.finish()
	}

	build()
	st := c.ui.scroll[scrollID]
	if st == nil {
		t.Fatal("scroll state missing")
	}
	if st.contentH <= 100 {
		t.Fatalf("content not taller than viewport: %.1f", st.contentH)
	}
	if st.y != 0 {
		t.Fatalf("initial scroll = %v, want 0", st.y)
	}
	found := false
	for _, hit := range c.ui.hits {
		if hit.ID == buttonID {
			found = true
			if hit.ScrollID != scrollID {
				t.Fatalf("button ScrollID=%d, want %d", hit.ScrollID, scrollID)
			}
			if hit.Clip.H > 100 || hit.Clip.Bottom() > 100.001 {
				t.Fatalf("button hit not clipped to viewport: %+v", hit.Clip)
			}
		}
	}
	if !found {
		t.Fatal("button hit missing from scroll content")
	}

	c.ui.wheel[scrollID] = Point{Y: -1} // wheel down
	f2 := build()
	if st.y <= 0 {
		t.Fatalf("wheel did not scroll down: y=%v", st.y)
	}
	if st.y > st.contentH-100 {
		t.Fatalf("scroll exceeds max: y=%v content=%v", st.y, st.contentH)
	}
	clipped := false
	for _, op := range f2.Ops {
		if op.Clip.H <= 100.001 && op.Clip.Y >= -0.001 {
			clipped = true
			break
		}
	}
	if !clipped {
		t.Fatal("no display-list op clipped to scroll viewport")
	}

	// Dragging the scrollbar to the middle should move the offset.
	sid := scrollbarID(scrollID)
	c.ui.activeID = sid
	c.ui.pointerLocal[sid] = Point{Y: 40}
	before := st.y
	build()
	if st.y == before {
		t.Fatalf("scrollbar drag did not change offset (%v)", st.y)
	}
}

func TestImageWidgetDisplayList(t *testing.T) {
	c, _ := newTestCtx(t, 300, 200)
	src := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			src.SetRGBA(x, y, color.RGBA{R: 40, G: 120, B: 220, A: 255})
		}
	}
	img := NewImage(src)
	if img.Size() != (Size{W: 40, H: 20}) {
		t.Fatalf("image size = %+v", img.Size())
	}
	c.begin()
	c.Image(img, Width(120), Height(60), Fit(ImageCover), Radius(9))
	f := c.finish()
	if len(f.Images) != 1 || f.Images[0] != img.data {
		t.Fatalf("frame image list = %+v", f.Images)
	}
	var op *render.Op
	for i := range f.Ops {
		if f.Ops[i].Kind == render.ImageQuad {
			op = &f.Ops[i]
			break
		}
	}
	if op == nil {
		t.Fatal("image op missing")
	}
	if op.Rect.W != 120 || op.Rect.H != 60 || op.Radius != 9 {
		t.Fatalf("image op = %+v", *op)
	}
	if op.UV.W <= 0 || op.UV.W > 1 || op.UV.H <= 0 || op.UV.H > 1 {
		t.Fatalf("invalid cover UV: %+v", op.UV)
	}
	if op.Image == nil || op.Image.Width != 40 || op.Image.Height != 20 {
		t.Fatalf("image texture = %+v", op.Image)
	}
}

func TestCheckboxRadioSlider(t *testing.T) {
	c, _ := newTestCtx(t, 400, 300)
	checked := false
	choice := -1
	value := float32(0)

	c.begin()
	c.Checkbox(&checked, "check", ID("check"))
	c.Radio(&choice, 2, "radio", ID("radio"))
	c.Slider(&value, 0, 100, Width(200), ID("slider"))
	c.ProgressBar(0.5, Width(200))
	c.finish()

	c.ui.clickedID = hashParts(nil, "check")
	c.begin()
	checkboxChanged := c.Checkbox(&checked, "check", ID("check"))
	c.ui.clickedID = hashParts(nil, "radio")
	radioSelected := c.Radio(&choice, 2, "radio", ID("radio"))
	c.finish()
	if !checkboxChanged || !checked {
		t.Fatalf("checkbox did not toggle (changed=%v value=%v)", checkboxChanged, checked)
	}
	if !radioSelected || choice != 2 {
		t.Fatalf("radio did not select (changed=%v value=%d)", radioSelected, choice)
	}

	// Click at 75% of the slider track.
	c.ui.activeID = hashParts(nil, "slider")
	c.ui.pointerLocal[c.ui.activeID] = Point{X: 150, Y: 15}
	c.begin()
	sliderChanged := c.Slider(&value, 0, 100, Width(200), ID("slider"))
	c.finish()
	if !sliderChanged || value <= 50 || value > 100 {
		t.Fatalf("slider did not follow pointer (changed=%v value=%v)", sliderChanged, value)
	}
}

func TestDeleteSurrounding(t *testing.T) {
	c, _ := newTestCtx(t, 300, 120)
	value := "hello"
	id := hashParts(nil, "edit")

	c.begin()
	c.InputText(&value, ID("edit"))
	c.finish()
	ed := c.ui.editors[id]
	if ed == nil {
		t.Fatal("editor state missing")
	}
	ed.caret, ed.anchor = 5, 5
	c.ui.focusID = id
	c.events = []core.Event{{Type: core.EventDeleteSurrounding, Before: 1}}
	c.used = []bool{false}
	c.begin()
	c.InputText(&value, ID("edit"))
	c.finish()
	if value != "hell" {
		t.Fatalf("delete surrounding value = %q, want %q", value, "hell")
	}
}

func TestTextInputStatePublished(t *testing.T) {
	c, _ := newTestCtx(t, 300, 200)
	value := ""
	c.ui.focusID = hashParts(nil, "edit")
	c.begin()
	c.InputText(&value, ID("edit"), Autofocus())
	f := c.finish()
	if f == nil {
		t.Fatal("nil frame")
	}
	if !c.textInput.Enabled {
		t.Fatal("text input not enabled while editor focused")
	}
	if c.textInput.Caret.W <= 0 || c.textInput.Caret.H <= 0 {
		t.Fatalf("bad caret rect: %+v", c.textInput.Caret)
	}
	if c.textInput.Caret.X < 0 || c.textInput.Caret.Y < 0 ||
		c.textInput.Caret.X+c.textInput.Caret.W > 300 ||
		c.textInput.Caret.Y+c.textInput.Caret.H > 200 {
		t.Fatalf("caret outside window: %+v", c.textInput.Caret)
	}
}

func TestTextDragSelection(t *testing.T) {
	c, _ := newTestCtx(t, 500, 120)
	value := "hello world"
	id := hashParts(nil, "edit")
	c.ui.focusID = id

	c.begin()
	c.InputText(&value, ID("edit"))
	c.finish()
	ed := c.ui.editors[id]
	if ed == nil {
		t.Fatal("editor state missing")
	}

	// Press at "hel|lo" and drag to "hello wo|rld".
	pad := float32(12)
	clickX := pad + c.measureRunes([]rune("hel"), c.Theme.FontSize)
	dragX := pad + c.measureRunes([]rune("hello wo"), c.Theme.FontSize)
	c.ui.focusClickID = id
	c.ui.focusClick = Point{X: clickX}
	c.ui.pointerLocal[id] = Point{X: clickX}
	c.ui.activeID = id
	c.Input.buttons[MouseLeft] = true
	c.begin()
	c.InputText(&value, ID("edit"))
	c.finish()
	if a, b, _ := ed.selection(); a != 3 || b != 3 {
		t.Fatalf("initial click selection = (%d,%d), want caret 3", a, b)
	}

	c.ui.pointerLocal[id] = Point{X: dragX}
	c.begin()
	c.InputText(&value, ID("edit"))
	c.finish()
	a, b, ok := ed.selection()
	if !ok || a != 3 || b != 8 {
		t.Fatalf("drag selection = (%d,%d,%v), want (3,8,true)", a, b, ok)
	}
	if got := string([]rune(value)[a:b]); got != "lo wo" {
		t.Fatalf("selected text = %q", got)
	}

	// A press+release coalesced into one frame must still select the drag range.
	c.Input.buttons[MouseLeft] = false
	c.ui.activeID = 0
	c.ui.focusClickID = id
	c.ui.focusClick = Point{X: pad}
	c.ui.clickedID = id
	c.ui.clickLocal[id] = Point{X: pad + c.measureRunes([]rune("hello"), c.Theme.FontSize)}
	c.begin()
	c.InputText(&value, ID("edit"))
	c.finish()
	a, b, ok = ed.selection()
	if !ok || string([]rune(value)[a:b]) != "hello" {
		t.Fatalf("batched drag selection = (%d,%d,%v) %q", a, b, ok, string([]rune(value)[a:b]))
	}

	// Double click selects the word under the pointer.
	c.ui.clickedID = 0
	c.ui.doubleClickedID = id
	c.Input.buttons[MouseLeft] = false
	c.ui.activeID = 0
	c.ui.doubleClickedID = id
	c.ui.focusClickID = id
	c.ui.focusClick = Point{X: pad + c.measureRunes([]rune("world"), c.Theme.FontSize)}
	c.begin()
	c.InputText(&value, ID("edit"))
	c.finish()
	a, b, ok = ed.selection()
	if !ok || string([]rune(value)[a:b]) != "world" {
		t.Fatalf("double-click selection = (%d,%d,%v) %q", a, b, ok, string([]rune(value)[a:b]))
	}
}

// TestWidgetGeometryStable guards against the loong64 aggregate-argument
// miscompilation described next to Ctx.place: a lost Size field used to make
// Checkbox/Radio zero-width (so they were not hit-testable) and Slider 128px
// wide regardless of Width.
func TestWidgetGeometryStable(t *testing.T) {
	c, _ := newTestCtx(t, 400, 300)
	checked, choice := false, 0
	value := float32(20)

	c.begin()
	c.Checkbox(&checked, "启用通知", TextSize(12), ID("geom-check"))
	c.Radio(&choice, 0, "深色", TextSize(12), ID("geom-radio-0"))
	c.Radio(&choice, 1, "浅色", TextSize(12), ID("geom-radio-1"))
	c.Slider(&value, 0, 100, Width(200), ID("geom-slider"))
	c.finish()

	hits := map[uint64]Rect{}
	for _, h := range c.ui.hits {
		hits[h.ID] = h.Rect
	}
	check, ok := hits[hashParts(nil, "geom-check")]
	if !ok || check.W <= 20 || check.H < 20 {
		t.Fatalf("checkbox geometry = %+v (found=%v), want width > 20 and height >= 20", check, ok)
	}
	r0, ok0 := hits[hashParts(nil, "geom-radio-0")]
	r1, ok1 := hits[hashParts(nil, "geom-radio-1")]
	if !ok0 || !ok1 || r0.W <= 20 || r0.H < 20 || r1.W <= 20 || r1.H < 20 {
		t.Fatalf("radio geometry = %+v / %+v (found=%v/%v)", r0, r1, ok0, ok1)
	}
	if r1.Y-r0.Y < 20 {
		t.Fatalf("radio rows overlap: y0=%.1f y1=%.1f", r0.Y, r1.Y)
	}
	slider, ok := hits[hashParts(nil, "geom-slider")]
	if !ok || slider.W != 200 || slider.H != 30 {
		t.Fatalf("slider geometry = %+v (found=%v), want 200x30", slider, ok)
	}
}

func TestWheelRoutingNestedScrollables(t *testing.T) {
	c, eng := newTestCtx(t, 400, 300)
	outerID := hashParts(nil, "wheel-outer")
	innerID := hashParts(nil, "wheel-inner")
	build := func() {
		c.begin()
		c.Scrollable(func() {
			c.Scrollable(func() {
				for i := 0; i < 30; i++ {
					c.Label(fmt.Sprintf("inner %02d", i), Height(20), ID(fmt.Sprintf("inner-row-%02d", i)))
				}
			}, ID("wheel-inner"), Height(100), Padding(0), Gap(0), NoScrollbar())
			for i := 0; i < 30; i++ {
				c.Label(fmt.Sprintf("outer %02d", i), Height(20), ID(fmt.Sprintf("outer-row-%02d", i)))
			}
		}, ID("wheel-outer"), Height(200), Padding(0), Gap(0), NoScrollbar())
		c.finish()
	}
	build()
	if st := c.ui.scroll[innerID]; st == nil || st.parentID != outerID || !st.vertical {
		t.Fatalf("inner scroll state = %+v, want parent %d", st, outerID)
	}

	// One notch down over the inner viewport moves only the inner viewport.
	eng.wheelAt(200, 50, 0, -1)
	build()
	inner := c.ui.scroll[innerID]
	outer := c.ui.scroll[outerID]
	if inner.y != scrollWheelStep {
		t.Fatalf("inner wheel offset = %.1f, want %d", inner.y, scrollWheelStep)
	}
	if outer.y != 0 {
		t.Fatalf("outer scrolled while inner had room: %.1f", outer.y)
	}

	// Keep scrolling: once the inner view reaches its end the remainder must
	// chain to the outer viewport.
	for i := 0; i < 20; i++ {
		eng.wheelAt(200, 50, 0, -1)
	}
	build()
	if inner.y < inner.contentH-inner.viewH-0.01 {
		t.Fatalf("inner did not reach its end: %.1f / %.1f", inner.y, inner.contentH-inner.viewH)
	}
	if outer.y <= 0 {
		t.Fatalf("wheel did not chain to outer viewport at inner edge: %.1f", outer.y)
	}

	// Wheel over outer-only space targets the outer viewport directly.
	outerBefore := outer.y
	eng.wheelAt(200, 150, 0, 1)
	build()
	if outer.y >= outerBefore {
		t.Fatalf("outer did not scroll up: before=%.1f after=%.1f", outerBefore, outer.y)
	}
}

func TestWheelRoutedToHorizontalScrollableX(t *testing.T) {
	c, eng := newTestCtx(t, 300, 200)
	id := hashParts(nil, "hscroll")
	build := func() {
		c.begin()
		c.ScrollableX(func() {
			for i := 0; i < 10; i++ {
				c.Canvas(120, 40, nil, ID(fmt.Sprintf("hcard-%02d", i)))
			}
		}, ID("hscroll"), Height(60), Padding(0), Gap(0), NoScrollbar())
		c.finish()
	}
	build()
	eng.wheelAt(150, 30, 1, 0)
	build()
	st := c.ui.scroll[id]
	if st == nil || st.x != scrollWheelStep {
		t.Fatalf("horizontal wheel offset = %+v, want %d", st, scrollWheelStep)
	}
}

func TestSliderUpdatesPaintingSameFrame(t *testing.T) {
	c, _ := newTestCtx(t, 300, 120)
	value := float32(0)
	c.ui.activeID = hashParts(nil, "paint-slider")
	// 75% of the track: trackX=9, trackW=200-18=182.
	c.ui.pointerLocal[c.ui.activeID] = Point{X: 9 + 0.75*182, Y: 15}
	c.begin()
	changed := c.Slider(&value, 0, 100, Width(200), Step(1), ID("paint-slider"))
	f := c.finish()
	if !changed || value < 74.9 || value > 75.1 {
		t.Fatalf("slider value = %v (changed=%v), want 75", value, changed)
	}
	// The filled part of the track must already show 75% in this frame.
	fillW := float32(0)
	for _, op := range f.Ops {
		if op.Kind == render.RectFill && op.Color == c.Theme.Accent && op.Rect.H == 4 {
			fillW = op.Rect.W
		}
	}
	if fillW < 0.7*182 {
		t.Fatalf("slider fill width in update frame = %.1f, want >= %.1f", fillW, 0.7*182)
	}
}

func TestSliderStepAndClamp(t *testing.T) {
	c, _ := newTestCtx(t, 300, 120)
	value := float32(0)
	id := hashParts(nil, "step-slider")
	c.begin()
	c.Slider(&value, -50, 50, Width(200), Step(10), ID("step-slider"))
	c.finish()

	c.ui.activeID = id
	c.ui.pointerLocal[id] = Point{X: 9 + 0.57*182, Y: 15}
	c.begin()
	c.Slider(&value, -50, 50, Width(200), Step(10), ID("step-slider"))
	c.finish()
	if value != 10 {
		t.Fatalf("stepped slider value = %v, want 10", value)
	}

	// Dragging beyond the right edge clamps to max, including when max is not
	// on the step grid.
	c.ui.pointerLocal[id] = Point{X: 1000, Y: 15}
	value = 0
	c.begin()
	c.Slider(&value, 0, 95, Width(200), Step(30), ID("step-slider"))
	c.finish()
	if value != 95 {
		t.Fatalf("slider right-edge value = %v, want 95", value)
	}
}

// TestAutoIDsStable pins the auto-ID contract: widgets at different source
// call sites get distinct IDs, and the same call sites get the same IDs on the
// next frame. This must hold with the runtime.Callers fast path.
func TestAutoIDsStable(t *testing.T) {
	c, _ := newTestCtx(t, 400, 300)
	build := func() []uint64 {
		c.begin()
		c.Button("one")
		c.Button("two")
		c.Button("three")
		c.finish()
		ids := make([]uint64, 0, len(c.ui.hits))
		for _, h := range c.ui.hits {
			ids = append(ids, h.ID)
		}
		return ids
	}
	first := build()
	second := build()
	if len(first) != 3 {
		t.Fatalf("hits = %v, want 3 buttons", first)
	}
	if first[0] == first[1] || first[1] == first[2] || first[0] == first[2] {
		t.Fatalf("auto IDs collide: %v", first)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("auto ID %d changed between frames: %d -> %d", i, first[i], second[i])
		}
	}
}

// TestDispatchWheelUsesEventCoordinates exercises the real dispatch path. A
// backend that omits X/Y (the Wayland bug fixed alongside this test) would
// resolve the hit at 0,0 and leave the viewport unscrolled.
func TestDispatchWheelUsesEventCoordinates(t *testing.T) {
	c, eng := newTestCtx(t, 400, 300)
	id := hashParts(nil, "dispatch-scroll")
	eng.view = func(c *Ctx) {
		c.Scrollable(func() {
			for i := 0; i < 30; i++ {
				c.Label(fmt.Sprintf("row %02d", i), Height(20), ID(fmt.Sprintf("dispatch-row-%02d", i)))
			}
		}, ID("dispatch-scroll"), Height(100), Padding(0), Gap(0), NoScrollbar())
	}
	build := func() {
		eng.Build(400, 300, 1, time.Millisecond, time.Millisecond, 0, 60)
	}
	build()
	eng.Dispatch([]core.Event{{Type: core.EventScroll, X: 200, Y: 50, DY: -1}})
	build()
	st := c.ui.scroll[id]
	if st == nil || st.y == 0 {
		t.Fatalf("wheel event at (200,50) did not scroll: %+v", st)
	}
}
