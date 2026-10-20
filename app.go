//go:build linux && cgo

package nekoui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"nekoui/internal/gpu"
	"nekoui/internal/platform"
	"nekoui/internal/platform/headless"
	"nekoui/internal/platform/wayland"
	"nekoui/internal/platform/x11"
	"nekoui/internal/render"
	"nekoui/internal/text"
	"nekoui/internal/ui"
)

// Run creates a window, enters the UI loop and calls view once per frame. The
// goroutine is locked to its OS thread because both Xlib and OpenGL require it.
func Run(cfg Config, view func(*Ctx)) error {
	if view == nil {
		return fmt.Errorf("nekoui: nil view")
	}
	if cfg.Width <= 0 {
		cfg.Width = 1100
	}
	if cfg.Height <= 0 {
		cfg.Height = 720
	}
	if cfg.MinWidth <= 0 {
		cfg.MinWidth = 640
	}
	if cfg.MinHeight <= 0 {
		cfg.MinHeight = 420
	}
	if cfg.ScreenshotFrames <= 0 {
		cfg.ScreenshotFrames = 3
	}
	if cfg.Theme == nil {
		cfg.Theme = DefaultTheme()
	}
	if cfg.Scale == 0 && (cfg.Headless || cfg.Screenshot != "") {
		cfg.Scale = 2
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	backend := selectBackend(cfg)
	opts := platform.Options{
		Backend:   backend,
		Title:     cfg.Title,
		Width:     cfg.Width,
		Height:    cfg.Height,
		MinWidth:  cfg.MinWidth,
		MinHeight: cfg.MinHeight,
		Resizable: cfg.Resizable,
		Scale:     cfg.Scale,
		VSync:     !cfg.DisableVSync,
	}
	win, err := openWindow(backend, opts)
	if err != nil {
		// In an XWayland session the Wayland socket may be absent/broken while
		// DISPLAY still works. Automatic selection falls back, explicit choices
		// keep their error so failures are visible.
		if (cfg.Backend == "" || cfg.Backend == "auto") && backend == "wayland" &&
			os.Getenv("DISPLAY") != "" {
			backend = "x11"
			win, err = openWindow(backend, opts)
		}
		if err != nil {
			return err
		}
	}
	defer win.Destroy()

	if err := win.MakeCurrent(); err != nil {
		return fmt.Errorf("nekoui: make GL context current: %w", err)
	}
	renderer, err := gpu.NewRenderer()
	if err != nil {
		return fmt.Errorf("nekoui: create renderer: %w", err)
	}
	defer func() {
		_ = win.MakeCurrent()
		renderer.Destroy()
	}()

	font, err := text.Load(text.LoadOptions{})
	if err != nil {
		return fmt.Errorf("nekoui: load font: %w", err)
	}

	debug := os.Getenv("NEKOUI_DEBUG") != ""
	debugOps := os.Getenv("NEKOUI_DEBUG_OPS") != ""
	eng := ui.NewEngine(font, cfg.Theme, view, debug, debugOps)
	a := &nativeApp{
		cfg:        cfg,
		win:        win,
		renderer:   renderer,
		eng:        eng,
		start:      time.Now(),
		last:       time.Now(),
		screenshot: cfg.Screenshot,
		debug:      debug,
	}
	eng.SetPixelSize(win.PixelSize())

	c := eng.Ctx()
	c.Backend = win.Name()
	c.BackendInfo = win.Info()
	c.FontInfo = font.Family() + " · " + filepath.Base(font.Path())
	c.GLInfo = renderer.Info()
	if a.debug {
		fmt.Fprintf(os.Stderr, "nekoui: backend=%s\n  window: %s\n  font: %s (%s)\n  renderer: %s\n",
			win.Name(), win.Info(), font.Family(), font.Path(), renderer.Info())
	}
	win.Show()
	win.SetTitle(cfg.Title)
	return a.loop()
}

func selectBackend(cfg Config) string {
	b := cfg.Backend
	if b != "" && b != "auto" {
		return b
	}
	if env := os.Getenv("NEKOUI_BACKEND"); env != "" && env != "auto" {
		return env
	}
	if cfg.Headless || cfg.Screenshot != "" {
		return "headless"
	}
	// A Wayland session also exposes DISPLAY through XWayland. Prefer the
	// native socket; X11 remains available via -backend x11 / NEKOUI_BACKEND.
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return "wayland"
	}
	if os.Getenv("DISPLAY") != "" {
		return "x11"
	}
	return "headless"
}

func openWindow(backend string, opts platform.Options) (platform.Window, error) {
	switch backend {
	case "x11":
		return x11.Open(opts)
	case "headless":
		return headless.Open(opts)
	case "wayland":
		return wayland.Open(opts)
	default:
		return nil, fmt.Errorf("nekoui: unknown backend %q", backend)
	}
}

// nativeApp is the platform/GL runner. All widget state lives in ui.Engine.
type nativeApp struct {
	cfg      Config
	win      platform.Window
	renderer *gpu.Renderer
	eng      *ui.Engine

	frame           int
	start, last     time.Time
	fps             float32
	animating       bool
	quit            bool
	stats           render.Stats
	screenshot      string
	screenshotTaken bool
	debug           bool
}

func (a *nativeApp) loop() error {
	for !a.quit {
		timeout := time.Duration(-1)
		if a.frame == 0 || a.animating || (a.screenshot != "" && !a.screenshotTaken) {
			timeout = 16 * time.Millisecond
		}
		evs, err := a.win.PollEvents(timeout)
		if err != nil {
			return err
		}
		cursor := a.eng.Dispatch(evs)
		a.win.SetCursor(cursor)
		if a.eng.Quit() {
			a.quit = true
		}
		if a.quit {
			break
		}
		if err := a.renderFrame(); err != nil {
			return err
		}
	}
	return nil
}

func (a *nativeApp) renderFrame() error {
	now := time.Now()
	dt := now.Sub(a.last)
	if dt <= 0 {
		dt = time.Microsecond
	}
	a.last = now
	inst := float32(time.Second / dt)
	if a.fps == 0 {
		a.fps = inst
	} else {
		a.fps = a.fps*0.9 + inst*0.1
	}

	w, h := a.win.Size()
	pw, ph := a.win.PixelSize()
	a.eng.SetPixelSize(pw, ph)
	a.eng.SetStats(a.stats)
	scale := a.win.Scale()
	frame := a.eng.Build(w, h, scale, dt, now.Sub(a.start), a.frame, a.fps)
	a.win.SetTextInput(a.eng.TextInput())
	a.animating = a.eng.Animating()
	if title := a.eng.Title(); title != "" {
		a.win.SetTitle(title)
	}
	if a.eng.Quit() {
		a.quit = true
	}

	if err := a.win.MakeCurrent(); err != nil {
		return err
	}
	if err := a.renderer.Render(frame); err != nil {
		return err
	}

	if a.screenshot != "" && !a.screenshotTaken {
		if a.frame+1 >= a.cfg.ScreenshotFrames {
			if err := a.takeScreenshot(pw, ph); err != nil {
				return err
			}
			a.screenshotTaken = true
			a.quit = true
			return nil
		}
	}
	if err := a.win.Swap(); err != nil {
		return err
	}
	a.frame++
	a.stats = a.renderer.Stats()
	return nil
}

func (a *nativeApp) takeScreenshot(w, h int) error {
	rgba, err := a.renderer.Capture(w, h)
	if err != nil {
		return err
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, rgba)
	f, err := os.Create(a.screenshot)
	if err != nil {
		return fmt.Errorf("nekoui: create screenshot: %w", err)
	}
	err = png.Encode(f, img)
	cerr := f.Close()
	if err != nil {
		return fmt.Errorf("nekoui: encode screenshot: %w", err)
	}
	if cerr != nil {
		return fmt.Errorf("nekoui: close screenshot: %w", cerr)
	}
	fmt.Fprintf(os.Stderr, "nekoui: screenshot %s (%dx%d)\n", a.screenshot, w, h)
	return nil
}
