// Package platform defines NekoUI's windowing contract. Backends own the
// native window, the EGL surface/context and normalized events; they do not
// own widgets, frames or GL draw state.
package platform

import (
	"time"

	"nekoui/internal/core"
)

// Options configures a native window.
type Options struct {
	Backend   string // "auto", "x11", "wayland", "headless"
	Title     string
	Width     int // logical
	Height    int // logical
	MinWidth  int
	MinHeight int
	Resizable bool
	Scale     float32 // 0 = backend default
	VSync     bool
}

// Window is a native surface plus a normalized event source.
type Window interface {
	// Size is the logical client size (what layout uses).
	Size() (w, h int)
	// PixelSize is the framebuffer size, including DPI scale.
	PixelSize() (w, h int)
	Scale() float32
	// Name identifies the platform backend ("X11", "Wayland", "Headless").
	Name() string
	// Info returns implementation details for a diagnostics HUD.
	Info() string

	Show()
	SetTitle(title string)
	SetCursor(c core.Cursor)
	// SetTextInput publishes the focused editor's caret to the input method for
	// candidate/preedit placement. Call it once per frame; Enabled=false
	// releases the input method so it does not swallow shortcut keys.
	SetTextInput(state core.TextInput)
	MakeCurrent() error
	Swap() error
	// PollEvents blocks for up to timeout (negative = forever) and returns
	// normalized events. Backends must not call into the UI layer.
	PollEvents(timeout time.Duration) ([]core.Event, error)
	Destroy()
}
