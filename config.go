package nekoui

// Config configures the application window and bootstrap.
type Config struct {
	Title  string
	Width  int
	Height int

	MinWidth  int
	MinHeight int
	Resizable bool

	// Backend is "auto", "x11", "wayland" or "headless". Empty means auto:
	// Wayland when WAYLAND_DISPLAY is present, then X11, then headless.
	// NEKOUI_BACKEND can override the automatic choice.
	Backend string
	// Scale overrides DPI scale. Zero lets the backend decide (2 for offline
	// screenshots).
	Scale float32
	// DisableVSync trades tearing control for latency.
	DisableVSync bool

	// Theme may be nil, in which case DefaultTheme is used.
	Theme *Theme

	// Headless forces the EGL surfaceless backend. Screenshot implies headless
	// unless Backend (or NEKOUI_BACKEND) is set explicitly.
	Headless bool
	// Screenshot renders a PNG and exits. It is intended for docs and CI.
	Screenshot string
	// ScreenshotFrames is the warm-up frame count before capture (default 3).
	ScreenshotFrames int
}
