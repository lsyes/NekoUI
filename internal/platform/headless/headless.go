//go:build linux && cgo

// Package headless provides an EGL surfaceless pbuffer window. It exists so the
// renderer and the UI can be exercised deterministically in CI and to produce
// documentation screenshots without opening a visible window.
package headless

import (
	"fmt"
	"time"

	"nekoui/internal/core"
	"nekoui/internal/gpu/egl"
	"nekoui/internal/platform"
)

type window struct {
	eglDpy  *egl.Display
	cfg     *egl.Config
	surface *egl.Surface
	ctx     *egl.Context

	logicalW, logicalH int
	pixelW, pixelH     int
	scale              float32
	info               string
	title              string
	destroyed          bool
}

// Open creates a surfaceless EGL context backed by a pbuffer.
func Open(opts platform.Options) (platform.Window, error) {
	w := &window{scale: opts.Scale, title: opts.Title}
	if w.scale <= 0 {
		w.scale = 1
	}
	w.logicalW, w.logicalH = opts.Width, opts.Height
	if w.logicalW <= 0 {
		w.logicalW = 960
	}
	if w.logicalH <= 0 {
		w.logicalH = 640
	}
	w.pixelW = int(float32(w.logicalW)*w.scale + 0.5)
	w.pixelH = int(float32(w.logicalH)*w.scale + 0.5)

	d, err := egl.OpenSurfaceless()
	if err != nil {
		return nil, fmt.Errorf("headless: %w", err)
	}
	w.eglDpy = d
	cfg, err := d.ChooseConfig(egl.PbufferSurface, 4)
	if err != nil {
		w.Destroy()
		return nil, fmt.Errorf("headless: %w", err)
	}
	w.cfg = cfg
	s, err := d.CreatePbufferSurface(cfg, w.pixelW, w.pixelH)
	if err != nil {
		w.Destroy()
		return nil, fmt.Errorf("headless: %w", err)
	}
	w.surface = s
	ctx, err := d.CreateContext(cfg)
	if err != nil {
		w.Destroy()
		return nil, fmt.Errorf("headless: %w", err)
	}
	w.ctx = ctx
	if err := ctx.MakeCurrent(s); err != nil {
		w.Destroy()
		return nil, fmt.Errorf("headless: %w", err)
	}
	d.SetSwapInterval(0)
	w.info = fmt.Sprintf("Headless · EGL %s · %d× MSAA", d.Version(), cfg.Samples())
	return w, nil
}

func (w *window) Name() string                { return "Headless" }
func (w *window) Info() string                { return w.info }
func (w *window) Size() (int, int)            { return w.logicalW, w.logicalH }
func (w *window) PixelSize() (int, int)       { return w.pixelW, w.pixelH }
func (w *window) Scale() float32              { return w.scale }
func (w *window) Show()                       {}
func (w *window) SetTitle(title string)       { w.title = title }
func (w *window) SetCursor(core.Cursor)       {}
func (w *window) SetTextInput(core.TextInput) {}
func (w *window) Swap() error                 { return nil }
func (w *window) MakeCurrent() error {
	if w.destroyed {
		return fmt.Errorf("headless: window destroyed")
	}
	return w.ctx.MakeCurrent(w.surface)
}

func (w *window) PollEvents(timeout time.Duration) ([]core.Event, error) {
	if timeout > 0 {
		time.Sleep(timeout)
	}
	return nil, nil
}

func (w *window) Destroy() {
	if w.destroyed {
		return
	}
	w.destroyed = true
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
