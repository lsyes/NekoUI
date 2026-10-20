//go:build linux && cgo

// Package wayland implements NekoUI's Wayland window backend on top of
// libwayland-client, xdg-shell, EGL and xkbcommon. The protocol state machine
// lives in shim.c; this file adapts its event queue to platform.Window and
// keeps OpenGL ownership in Go.
package wayland

/*
#cgo pkg-config: wayland-client wayland-egl wayland-cursor
#cgo LDFLAGS: -l:libxkbcommon.so.0
#include <stdlib.h>
#include "shim.h"
*/
import "C"

import (
	"fmt"
	"math"
	"time"
	"unsafe"

	"nekoui/internal/core"
	"nekoui/internal/gpu/egl"
	"nekoui/internal/platform"
)

type window struct {
	w *C.nw_window

	eglDpy  *egl.Display
	cfg     *egl.Config
	surface *egl.Surface
	ctx     *egl.Context

	logicalW, logicalH int
	pixelW, pixelH     int
	scale              float32
	info               string

	cursor    core.Cursor
	destroyed bool
}

// Open creates an xdg-shell toplevel, an EGL surface and an OpenGL 3.3 core
// context. It waits for the compositor's initial configure before returning.
func Open(opts platform.Options) (platform.Window, error) {
	var errbuf [512]C.char
	native := C.nw_open(
		C.int(opts.Width), C.int(opts.Height),
		C.int(opts.MinWidth), C.int(opts.MinHeight),
		boolInt(opts.Resizable),
		C.float(opts.Scale),
		boolInt(opts.VSync),
		&errbuf[0], C.int(len(errbuf)),
	)
	if native == nil {
		msg := C.GoString(&errbuf[0])
		if msg == "" {
			msg = "wayland: cannot create window"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	w := &window{
		w:        native,
		logicalW: int(C.nw_width(native)),
		logicalH: int(C.nw_height(native)),
		pixelW:   int(C.nw_pixel_width(native)),
		pixelH:   int(C.nw_pixel_height(native)),
		scale:    float32(C.nw_scale(native)),
		cursor:   core.CursorArrow,
	}

	dpy, err := egl.OpenWayland(C.nw_display(native))
	if err != nil {
		w.closeNative()
		return nil, fmt.Errorf("wayland: %w", err)
	}
	w.eglDpy = dpy
	cfg, err := dpy.ChooseConfig(egl.WindowSurface, 4)
	if err != nil {
		w.cleanupEGL()
		w.closeNative()
		return nil, fmt.Errorf("wayland: %w", err)
	}
	w.cfg = cfg
	surface, err := dpy.CreateWindowSurface(cfg, C.nw_egl_window(native))
	if err != nil {
		w.cleanupEGL()
		w.closeNative()
		return nil, fmt.Errorf("wayland: %w", err)
	}
	w.surface = surface
	ctx, err := dpy.CreateContext(cfg)
	if err != nil {
		w.cleanupEGL()
		w.closeNative()
		return nil, fmt.Errorf("wayland: %w", err)
	}
	w.ctx = ctx
	if err := ctx.MakeCurrent(surface); err != nil {
		w.cleanupEGL()
		w.closeNative()
		return nil, fmt.Errorf("wayland: %w", err)
	}
	if opts.VSync {
		dpy.SetSwapInterval(1)
	} else {
		dpy.SetSwapInterval(0)
	}

	extra := "xkb"
	if C.nw_has_text_input(native) != 0 {
		extra += " · text-input-v3"
	}
	if C.nw_has_cursor_shape(native) != 0 {
		extra += " · cursor-shape"
	}
	w.info = fmt.Sprintf("Wayland · EGL %s · %d× MSAA · xdg-shell · %s",
		dpy.Version(), cfg.Samples(), extra)
	w.SetTitle(opts.Title)
	w.SetCursor(core.CursorArrow)
	return w, nil
}

func boolInt(v bool) C.int {
	if v {
		return 1
	}
	return 0
}

func (w *window) Name() string { return "Wayland" }

func (w *window) Info() string { return w.info }

func (w *window) Size() (int, int) { return w.logicalW, w.logicalH }

func (w *window) PixelSize() (int, int) { return w.pixelW, w.pixelH }

func (w *window) Scale() float32 { return w.scale }

func (w *window) Show() {
	if w.destroyed {
		return
	}
	C.nw_show(w.w)
}

func (w *window) SetTitle(title string) {
	if w.destroyed || title == "" {
		return
	}
	cs := C.CString(title)
	defer C.free(unsafe.Pointer(cs))
	C.nw_set_title(w.w, cs)
}

func (w *window) SetCursor(c core.Cursor) {
	if w.destroyed || c == w.cursor {
		return
	}
	w.cursor = c
	C.nw_set_cursor(w.w, C.int(c))
}

func (w *window) SetTextInput(state core.TextInput) {
	if w.destroyed {
		return
	}
	if !state.Enabled {
		C.nw_set_text_input(w.w, 0, 0, 0, 1, 1, 0, 0)
		return
	}
	x := int(math.Round(float64(state.Caret.X)))
	y := int(math.Round(float64(state.Caret.Y)))
	ww := int(math.Round(float64(state.Caret.W)))
	h := int(math.Round(float64(state.Caret.H)))
	if ww < 1 {
		ww = 1
	}
	if h < 1 {
		h = 1
	}
	C.nw_set_text_input(w.w, 1, C.int(x), C.int(y), C.int(ww), C.int(h),
		C.int(state.Cursor), C.int(state.Anchor))
}

func (w *window) MakeCurrent() error {
	if w.destroyed {
		return fmt.Errorf("wayland: window destroyed")
	}
	return w.ctx.MakeCurrent(w.surface)
}

func (w *window) Swap() error {
	if w.destroyed {
		return fmt.Errorf("wayland: window destroyed")
	}
	if err := w.surface.Swap(); err != nil {
		return err
	}
	if C.nw_flush(w.w) != 0 {
		return fmt.Errorf("wayland: flush: %s", w.lastError())
	}
	return nil
}

func (w *window) PollEvents(timeout time.Duration) ([]core.Event, error) {
	if w.destroyed {
		return nil, nil
	}
	ms := -1
	if timeout >= 0 {
		ms = int(timeout / time.Millisecond)
	}
	var raw [64]C.nw_event
	n := C.nw_poll(w.w, C.int(ms), &raw[0], C.int(len(raw)))
	if n < 0 {
		return nil, fmt.Errorf("wayland: %s", w.lastError())
	}
	if n == 0 {
		return nil, nil
	}
	out := make([]core.Event, 0, int(n))
	for i := 0; i < int(n); i++ {
		e := &raw[i]
		switch e._type {
		case C.NW_EVENT_RESIZE:
			w.logicalW = int(e.width)
			w.logicalH = int(e.height)
			w.pixelW = int(e.pixel_width)
			w.pixelH = int(e.pixel_height)
			w.scale = float32(e.scale)
			out = append(out, core.Event{
				Type:        core.EventResize,
				Width:       w.logicalW,
				Height:      w.logicalH,
				PixelWidth:  w.pixelW,
				PixelHeight: w.pixelH,
				Scale:       w.scale,
			})
		case C.NW_EVENT_CLOSE:
			out = append(out, core.Event{Type: core.EventClose})
		case C.NW_EVENT_FOCUS:
			out = append(out, core.Event{Type: core.EventFocus, Focused: e.focused != 0})
		case C.NW_EVENT_MOUSE_MOVE:
			out = append(out, core.Event{Type: core.EventMouseMove, X: float32(e.x), Y: float32(e.y)})
		case C.NW_EVENT_MOUSE_DOWN:
			out = append(out, core.Event{
				Type:   core.EventMouseDown,
				Button: core.MouseButton(e.button),
				X:      float32(e.x), Y: float32(e.y),
			})
		case C.NW_EVENT_MOUSE_UP:
			out = append(out, core.Event{
				Type:   core.EventMouseUp,
				Button: core.MouseButton(e.button),
				X:      float32(e.x), Y: float32(e.y),
			})
		case C.NW_EVENT_SCROLL:
			out = append(out, core.Event{
				Type: core.EventScroll,
				X:    float32(e.x), Y: float32(e.y),
				DX: float32(e.dx), DY: float32(e.dy),
			})
		case C.NW_EVENT_MOUSE_ENTER:
			out = append(out, core.Event{Type: core.EventMouseEnter})
		case C.NW_EVENT_MOUSE_LEAVE:
			out = append(out, core.Event{Type: core.EventMouseLeave})
		case C.NW_EVENT_KEY_DOWN:
			out = append(out, core.Event{
				Type:   core.EventKeyDown,
				Key:    core.KeyFromKeysym(uint32(e.key)),
				Mods:   mods(uint32(e.mods)),
				Repeat: e.repeat != 0,
			})
		case C.NW_EVENT_KEY_UP:
			out = append(out, core.Event{
				Type: core.EventKeyUp,
				Key:  core.KeyFromKeysym(uint32(e.key)),
				Mods: mods(uint32(e.mods)),
			})
		case C.NW_EVENT_TEXT:
			if txt := C.GoString(&e.text[0]); txt != "" {
				out = append(out, core.Event{Type: core.EventText, Text: txt})
			}
		case C.NW_EVENT_PREEDIT:
			out = append(out, core.Event{
				Type:   core.EventPreedit,
				Text:   C.GoString(&e.text[0]),
				Cursor: int(e.cursor),
			})
		case C.NW_EVENT_DELETE_SURROUNDING:
			out = append(out, core.Event{
				Type:   core.EventDeleteSurrounding,
				Before: int(e.before),
				After:  int(e.after),
			})
		}
	}
	return out, nil
}

func mods(v uint32) core.Modifiers {
	var m core.Modifiers
	if v&C.NW_MOD_SHIFT != 0 {
		m |= core.ModShift
	}
	if v&C.NW_MOD_CTRL != 0 {
		m |= core.ModCtrl
	}
	if v&C.NW_MOD_ALT != 0 {
		m |= core.ModAlt
	}
	if v&C.NW_MOD_SUPER != 0 {
		m |= core.ModSuper
	}
	return m
}

func (w *window) lastError() string {
	if w.w == nil {
		return "unknown error"
	}
	return C.GoString(C.nw_error(w.w))
}

func (w *window) cleanupEGL() {
	if w.ctx != nil {
		w.ctx.Destroy()
		w.ctx = nil
	}
	if w.surface != nil {
		w.surface.Destroy()
		w.surface = nil
	}
	if w.eglDpy != nil {
		w.eglDpy.Destroy()
		w.eglDpy = nil
	}
}

func (w *window) closeNative() {
	if w.w != nil {
		C.nw_close(w.w)
		w.w = nil
	}
}

func (w *window) Destroy() {
	if w.destroyed {
		return
	}
	w.destroyed = true
	// EGL surface/context must go before wl_egl_window.
	w.cleanupEGL()
	w.closeNative()
}
