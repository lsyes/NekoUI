//go:build !linux || !cgo

package nekoui

import "errors"

// Run reports that the current build does not have a native backend. NekoUI
// targets Linux with cgo (Xlib/Wayland/EGL/OpenGL); the frame engine and public
// API still build and unit-test without it.
func Run(cfg Config, view func(*Ctx)) error {
	return errors.New("nekoui: this build has no platform backend; use linux/amd64, arm64 or loong64 with CGO_ENABLED=1")
}
