//go:build linux && cgo

// Package egl is a deliberately thin EGL binding. NekoUI only needs the
// platform/context/window-surface flow; everything above it (batching, shaders,
// text, frame pacing) is implemented in this repository.
package egl

/*
#cgo LDFLAGS: -l:libEGL.so.1
#include "egl_api.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

var errUnsupported = errors.New("egl: unsupported request")

// Display is an initialized EGL display.
type Display struct {
	p       C.EGLDisplay
	version string
}

// Config is a framebuffer configuration. VisualID is used by the X11 backend to
// create a window whose visual matches the EGL surface.
type Config struct {
	d        *Display
	p        C.EGLConfig
	visualID int
	samples  int
}

// Surface is a window or pbuffer surface.
type Surface struct {
	d *Display
	p C.EGLSurface
}

// Context is an OpenGL 3.3 core context.
type Context struct {
	d *Display
	p C.EGLContext
}

// OpenX11 creates an EGL display backed by an Xlib Display*.
func OpenX11(native unsafe.Pointer) (*Display, error) {
	return open(C.EGL_PLATFORM_X11_KHR, native, "x11")
}

// OpenWayland creates an EGL display backed by a wl_display*.
func OpenWayland(native unsafe.Pointer) (*Display, error) {
	return open(C.EGL_PLATFORM_WAYLAND_KHR, native, "wayland")
}

// OpenSurfaceless creates an EGL display that renders to pbuffers without a
// window system. It is used for screenshots and headless verification.
func OpenSurfaceless() (*Display, error) {
	return open(C.EGL_PLATFORM_SURFACELESS_MESA, nil, "surfaceless")
}

func open(platform C.EGLenum, native unsafe.Pointer, kind string) (*Display, error) {
	var p C.EGLDisplay
	if platform == C.EGL_PLATFORM_SURFACELESS_MESA {
		p = C.eglGetPlatformDisplay(platform, nil, nil)
	} else if native != nil {
		p = C.eglGetPlatformDisplay(platform, native, nil)
	}
	if p == 0 {
		p = C.eglGetDisplay(C.EGLNativeDisplayType(native))
	}
	if p == 0 {
		return nil, fmt.Errorf("egl: no display for %s platform", kind)
	}
	var major, minor C.EGLint
	if C.eglInitialize(p, &major, &minor) == C.EGL_FALSE {
		return nil, fmt.Errorf("egl: initialize %s display: %s", kind, errorString())
	}
	d := &Display{p: p, version: fmt.Sprintf("%d.%d", int(major), int(minor))}
	if C.eglBindAPI(C.EGL_OPENGL_API) == C.EGL_FALSE {
		e := errorString()
		d.Destroy()
		return nil, fmt.Errorf("egl: bind OpenGL API: %s", e)
	}
	return d, nil
}

// Version is the EGL major.minor string.
func (d *Display) Version() string { return d.version }

// Vendor returns the EGL vendor string.
func (d *Display) Vendor() string { return C.GoString(C.eglQueryString(d.p, C.EGL_VENDOR)) }

// SurfaceKind controls which EGL surface types the config must support.
type SurfaceKind int

const (
	WindowSurface SurfaceKind = iota
	PbufferSurface
)

// ChooseConfig selects an RGBA8 config with OpenGL support and, when possible,
// 4x MSAA. It transparently retries without multisampling.
func (d *Display) ChooseConfig(kind SurfaceKind, samples int) (*Config, error) {
	for _, s := range sampleOptions(samples) {
		cfg, n, err := d.choose(kind, s)
		if err != nil {
			return nil, err
		}
		if n > 0 && cfg != 0 {
			visual, err := d.configAttrib(cfg, C.EGL_NATIVE_VISUAL_ID)
			if err != nil {
				return nil, err
			}
			return &Config{d: d, p: cfg, visualID: int(visual), samples: s}, nil
		}
	}
	return nil, fmt.Errorf("egl: no matching OpenGL RGBA8 configuration")
}

func sampleOptions(samples int) []int {
	if samples > 0 {
		return []int{samples, 0}
	}
	return []int{0}
}

func (d *Display) choose(kind SurfaceKind, samples int) (C.EGLConfig, C.EGLint, error) {
	surface := C.EGLint(C.EGL_WINDOW_BIT)
	if kind == PbufferSurface {
		surface = C.EGLint(C.EGL_PBUFFER_BIT)
	}
	attrs := []C.EGLint{
		C.EGL_SURFACE_TYPE, surface,
		C.EGL_RENDERABLE_TYPE, C.EGL_OPENGL_BIT,
		C.EGL_RED_SIZE, 8,
		C.EGL_GREEN_SIZE, 8,
		C.EGL_BLUE_SIZE, 8,
		C.EGL_ALPHA_SIZE, 8,
		C.EGL_DEPTH_SIZE, 0,
		C.EGL_STENCIL_SIZE, 0,
	}
	if samples > 0 {
		attrs = append(attrs, C.EGL_SAMPLE_BUFFERS, 1, C.EGL_SAMPLES, C.EGLint(samples))
	}
	attrs = append(attrs, C.EGL_NONE)

	var cfg C.EGLConfig
	var n C.EGLint
	if C.eglChooseConfig(d.p, &attrs[0], &cfg, 1, &n) == C.EGL_FALSE {
		return 0, 0, fmt.Errorf("egl: choose config: %s", errorString())
	}
	if n == 0 {
		return 0, 0, nil
	}
	return cfg, n, nil
}

func (d *Display) configAttrib(cfg C.EGLConfig, attr C.EGLint) (C.EGLint, error) {
	var v C.EGLint
	if C.eglGetConfigAttrib(d.p, cfg, attr, &v) == C.EGL_FALSE {
		return 0, fmt.Errorf("egl: get config attribute 0x%x: %s", uint32(attr), errorString())
	}
	return v, nil
}

// VisualID is the X11 visual that must be used with this config.
func (c *Config) VisualID() int { return c.visualID }

// Samples is the chosen MSAA sample count (0 if none).
func (c *Config) Samples() int { return c.samples }

// CreateWindowSurface wraps a native window (X11 Window or wl_surface) in an EGL
// window surface.
func (d *Display) CreateWindowSurface(cfg *Config, native unsafe.Pointer) (*Surface, error) {
	if native == nil {
		return nil, errUnsupported
	}
	p := C.eglCreateWindowSurface(d.p, cfg.p, C.EGLNativeWindowType(native), nil)
	if p == nil {
		return nil, fmt.Errorf("egl: create window surface: %s", errorString())
	}
	return &Surface{d: d, p: p}, nil
}

// CreatePbufferSurface creates an offscreen color buffer of the given physical
// size.
func (d *Display) CreatePbufferSurface(cfg *Config, w, h int) (*Surface, error) {
	attrs := []C.EGLint{
		C.EGL_WIDTH, C.EGLint(w),
		C.EGL_HEIGHT, C.EGLint(h),
		C.EGL_NONE,
	}
	p := C.eglCreatePbufferSurface(d.p, cfg.p, &attrs[0])
	if p == nil {
		return nil, fmt.Errorf("egl: create pbuffer surface: %s", errorString())
	}
	return &Surface{d: d, p: p}, nil
}

// CreateContext creates an OpenGL 3.3 core context.
func (d *Display) CreateContext(cfg *Config) (*Context, error) {
	attrs := []C.EGLint{
		C.EGL_CONTEXT_MAJOR_VERSION, 3,
		C.EGL_CONTEXT_MINOR_VERSION, 3,
		C.EGL_CONTEXT_OPENGL_PROFILE_MASK, C.EGL_CONTEXT_OPENGL_CORE_PROFILE_BIT,
		C.EGL_NONE,
	}
	p := C.eglCreateContext(d.p, cfg.p, nil, &attrs[0])
	if p == nil {
		return nil, fmt.Errorf("egl: create OpenGL 3.3 core context: %s", errorString())
	}
	return &Context{d: d, p: p}, nil
}

// MakeCurrent binds the context to a surface.
func (c *Context) MakeCurrent(s *Surface) error {
	if C.eglMakeCurrent(c.d.p, s.p, s.p, c.p) == C.EGL_FALSE {
		return fmt.Errorf("egl: make current: %s", errorString())
	}
	return nil
}

// Swap presents the surface.
func (s *Surface) Swap() error {
	if C.eglSwapBuffers(s.d.p, s.p) == C.EGL_FALSE {
		return fmt.Errorf("egl: swap buffers: %s", errorString())
	}
	return nil
}

// SetSwapInterval enables or disables vsync for the current context.
func (d *Display) SetSwapInterval(n int) {
	C.eglSwapInterval(d.p, C.EGLint(n))
}

// Destroy releases the surface.
func (s *Surface) Destroy() {
	if s != nil && s.p != nil {
		C.eglDestroySurface(s.d.p, s.p)
		s.p = nil
	}
}

// Destroy releases the context.
func (c *Context) Destroy() {
	if c != nil && c.p != nil {
		C.eglDestroyContext(c.d.p, c.p)
		c.p = nil
	}
}

// Destroy terminates the display.
func (d *Display) Destroy() {
	if d != nil && d.p != 0 {
		C.eglMakeCurrent(d.p, nil, nil, nil)
		C.eglTerminate(d.p)
		d.p = 0
	}
}

func errorString() string {
	code := C.eglGetError()
	name := map[C.EGLint]string{
		C.EGL_NOT_INITIALIZED:     "EGL_NOT_INITIALIZED",
		C.EGL_BAD_ACCESS:          "EGL_BAD_ACCESS",
		C.EGL_BAD_ALLOC:           "EGL_BAD_ALLOC",
		C.EGL_BAD_ATTRIBUTE:       "EGL_BAD_ATTRIBUTE",
		C.EGL_BAD_CONFIG:          "EGL_BAD_CONFIG",
		C.EGL_BAD_CONTEXT:         "EGL_BAD_CONTEXT",
		C.EGL_BAD_CURRENT_SURFACE: "EGL_BAD_CURRENT_SURFACE",
		C.EGL_BAD_DISPLAY:         "EGL_BAD_DISPLAY",
		C.EGL_BAD_MATCH:           "EGL_BAD_MATCH",
		C.EGL_BAD_NATIVE_WINDOW:   "EGL_BAD_NATIVE_WINDOW",
		C.EGL_BAD_PARAMETER:       "EGL_BAD_PARAMETER",
		C.EGL_BAD_SURFACE:         "EGL_BAD_SURFACE",
		C.EGL_CONTEXT_LOST:        "EGL_CONTEXT_LOST",
	}
	if s, ok := name[code]; ok {
		return fmt.Sprintf("%s (0x%x)", s, uint32(code))
	}
	return fmt.Sprintf("EGL error 0x%x", uint32(code))
}
