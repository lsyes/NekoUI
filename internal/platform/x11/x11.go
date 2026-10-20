//go:build linux && cgo

// Package x11 implements NekoUI's X11 windowing backend. It owns an Xlib
// connection, an XIM input context and an EGL window surface; layout, painting
// and input semantics remain in the UI layer.
package x11

/*
#cgo pkg-config: x11
#cgo LDFLAGS: -l:libEGL.so.1
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/Xatom.h>
#include <X11/Xresource.h>
#include <X11/keysym.h>
#include <X11/XKBlib.h>
#include <X11/cursorfont.h>
#include <locale.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <errno.h>
#include <poll.h>
#include <stdint.h>

static int ng_set_locale(void) { return setlocale(LC_ALL, "") != NULL; }
static void ng_locale_modifiers(void) { XSetLocaleModifiers(""); }

// ng_xim_server reads the XIM_SERVERS root property and extracts the advertised
// server name ("fcitx", "ibus", ...). Xlib silently falls back to a local input
// method when XMODIFIERS is unset, which makes CJK input appear broken; the Go
// side uses this to synthesize the conventional @im=<server> modifier.
static int ng_xim_server(Display *dpy, char *out, int cap) {
    if (out == NULL || cap <= 0) return 0;
    out[0] = '\0';
    Atom prop = XInternAtom(dpy, "XIM_SERVERS", False);
    if (prop == None) return 0;
    Atom type = None;
    int format = 0;
    unsigned long n = 0, after = 0;
    unsigned char *data = NULL;
    if (XGetWindowProperty(dpy, DefaultRootWindow(dpy), prop, 0, 64, False,
                           AnyPropertyType, &type, &format, &n, &after,
                           &data) != Success || data == NULL) {
        return 0;
    }
    int found = 0;
    if (format == 8) {
        unsigned long i = 0;
        while (i < n) {
            const char *s = (const char *)data + i;
            size_t len = strnlen(s, (size_t)(n - i));
            if (strncmp(s, "@server=", 8) == 0 && s[8] != '\0') {
                snprintf(out, (size_t)cap, "%s", s + 8);
                found = 1;
                break;
            }
            i += len + 1;
        }
    } else if (format == 32) {
        Atom *atoms = (Atom *)data;
        for (unsigned long i = 0; i < n; i++) {
            char *name = XGetAtomName(dpy, atoms[i]);
            if (name == NULL) continue;
            if (strncmp(name, "@server=", 8) == 0 && name[8] != '\0') {
                snprintf(out, (size_t)cap, "%s", name + 8);
                found = 1;
            }
            XFree(name);
            if (found) break;
        }
    }
    XFree(data);
    return found;
}

static Window ng_root(Display *dpy) { return DefaultRootWindow(dpy); }
static int ng_screen_w(Display *dpy) { return DisplayWidth(dpy, DefaultScreen(dpy)); }
static int ng_screen_h(Display *dpy) { return DisplayHeight(dpy, DefaultScreen(dpy)); }

static float ng_dpi(Display *dpy) {
    const char *v = XGetDefault(dpy, "Xft", "dpi");
    if (!v) return 0;
    return (float)atof(v);
}

static Window ng_create_window(Display *dpy, Window root, Visual *vis, int depth,
                               int x, int y, int w, int h, unsigned long bg,
                               long event_mask, Colormap *out_cmap) {
    XSetWindowAttributes a;
    memset(&a, 0, sizeof a);
    *out_cmap = XCreateColormap(dpy, root, vis, AllocNone);
    a.colormap = *out_cmap;
    a.border_pixel = 0;
    a.background_pixel = bg;
    a.event_mask = event_mask;
    return XCreateWindow(dpy, root, x, y, w, h, 0, depth, InputOutput, vis,
        CWColormap | CWBorderPixel | CWBackPixel | CWEventMask, &a);
}

static Visual *ng_visual(Display *dpy, unsigned long visualid, int *depth) {
    XVisualInfo tmpl;
    memset(&tmpl, 0, sizeof tmpl);
    tmpl.visualid = visualid;
    int n = 0;
    XVisualInfo *vi = XGetVisualInfo(dpy, VisualIDMask, &tmpl, &n);
    if (!vi) return NULL;
    Visual *v = vi->visual;
    *depth = vi->depth;
    XFree(vi);
    return v;
}

static void ng_set_title(Display *dpy, Window win, const char *title) {
    XStoreName(dpy, win, title);
    Atom utf8 = XInternAtom(dpy, "UTF8_STRING", False);
    Atom net = XInternAtom(dpy, "_NET_WM_NAME", False);
    XChangeProperty(dpy, win, net, utf8, 8, PropModeReplace,
                    (const unsigned char *)title, (int)strlen(title));
}

static void ng_set_class(Display *dpy, Window win, const char *name, const char *cls) {
    XClassHint h;
    h.res_name = (char *)name;
    h.res_class = (char *)cls;
    XSetClassHint(dpy, win, &h);
}

static void ng_set_size_hints(Display *dpy, Window win, int minw, int minh, int maxw, int maxh) {
    XSizeHints *h = XAllocSizeHints();
    if (!h) return;
    h->flags = 0;
    if (minw > 0 && minh > 0) {
        h->flags |= PMinSize;
        h->min_width = minw;
        h->min_height = minh;
    }
    if (maxw > 0 && maxh > 0) {
        h->flags |= PMaxSize;
        h->max_width = maxw;
        h->max_height = maxh;
    }
    if (h->flags) XSetWMNormalHints(dpy, win, h);
    XFree(h);
}

static XIC ng_create_ic(Display *dpy, XIM im, Window win) {
    return XCreateIC(im, XNInputStyle, XIMPreeditNothing | XIMStatusNothing,
                     XNClientWindow, win, XNFocusWindow, win, (void *)0);
}

static XIC ng_create_ic_none(Display *dpy, XIM im, Window win) {
    return XCreateIC(im, XNInputStyle, XIMPreeditNone | XIMStatusNone,
                     XNClientWindow, win, XNFocusWindow, win, (void *)0);
}

static XFontSet ng_create_fontset(Display *dpy) {
    char **missing = NULL;
    int nmissing = 0;
    char *def = NULL;
    XFontSet fs = XCreateFontSet(dpy,
        "-*-*-medium-r-normal--14-*-*-*-*-*-*-*,-*-*-*-*-*-*-14-*-*-*-*-*-*-*",
        &missing, &nmissing, &def);
    if (missing != NULL) XFreeStringList(missing);
    return fs;
}

// ng_pick_xim_style prefers over-the-spot (PreeditPosition) so fcitx can use
// XNSpotLocation for its candidate window. PreeditNothing is the fallback; it
// is universally supported but several input methods then anchor the window
// to the pointer instead of the caret.
static unsigned long ng_pick_xim_style(XIM im, int *position) {
    *position = 0;
    XIMStyles *styles = NULL;
    if (XGetIMValues(im, XNQueryInputStyle, &styles, NULL) != NULL || styles == NULL) {
        return 0;
    }
    unsigned long pos = XIMPreeditPosition | XIMStatusNothing;
    unsigned long nothing = XIMPreeditNothing | XIMStatusNothing;
    int has_pos = 0, has_nothing = 0;
    for (int i = 0; i < styles->count_styles; i++) {
        if (styles->supported_styles[i] == pos) has_pos = 1;
        if (styles->supported_styles[i] == nothing) has_nothing = 1;
    }
    XFree(styles);
    if (has_pos) {
        *position = 1;
        return pos;
    }
    if (has_nothing) return nothing;
    return 0;
}

static XIC ng_create_ic_position(Display *dpy, XIM im, Window win, XFontSet fs,
                                 int x, int y) {
    if (fs == NULL) return NULL;
    XPoint spot;
    spot.x = (short)x;
    spot.y = (short)y;
    XVaNestedList list = XVaCreateNestedList(0, XNFontSet, fs, XNSpotLocation, &spot, NULL);
    XIC ic = XCreateIC(im, XNInputStyle, XIMPreeditPosition | XIMStatusNothing,
                       XNClientWindow, win, XNFocusWindow, win,
                       XNPreeditAttributes, list, NULL);
    XFree(list);
    return ic;
}

static void ng_free_fontset(Display *dpy, XFontSet fs) {
    if (fs != NULL) XFreeFontSet(dpy, fs);
}

static unsigned long ng_ic_filter_events(XIC ic) {
    unsigned long filter = 0;
    if (ic != NULL) XGetICValues(ic, XNFilterEvents, &filter, NULL);
    return filter;
}

static void ng_select_input(Display *dpy, Window win, long mask) {
    XSelectInput(dpy, win, mask);
}

static int ng_set_spot(Display *dpy, XIC ic, int x, int y) {
    if (ic == NULL) return 0;
    if (x < -32768) x = -32768;
    if (x > 32767) x = 32767;
    if (y < -32768) y = -32768;
    if (y > 32767) y = 32767;
    XPoint spot;
    spot.x = (short)x;
    spot.y = (short)y;
    char *err = XSetICValues(ic, XNSpotLocation, &spot, NULL);
    if (err == NULL) return 1;
    if (getenv("NEKOUI_DEBUG") != NULL) {
        fprintf(stderr, "nekoui xim: direct spot failed: %s\n", err);
    }
    // Some input methods only read spot location from preeditAttributes.
    XVaNestedList list = XVaCreateNestedList(0, XNSpotLocation, &spot, NULL);
    err = XSetICValues(ic, XNPreeditAttributes, list, NULL);
    XFree(list);
    if (err == NULL) return 1;
    if (getenv("NEKOUI_DEBUG") != NULL) {
        fprintf(stderr, "nekoui xim: nested spot failed: %s\n", err);
    }
    return 0;
}

static void ng_ic_focus(XIC ic, int on) {
    if (ic == NULL) return;
    if (on) XSetICFocus(ic);
    else XUnsetICFocus(ic);
}

static int ng_lookup_utf8(XIC ic, XKeyEvent *ev, char *buf, int len, KeySym *ks, Status *status) {
    return Xutf8LookupString(ic, ev, buf, len, ks, status);
}

static int ng_lookup_latin1(XKeyEvent *ev, char *buf, int len, KeySym *ks) {
    XComposeStatus cs;
    memset(&cs, 0, sizeof cs);
    return XLookupString(ev, buf, len, ks, &cs);
}

static int ng_detectable_repeat(Display *dpy) {
    Bool supported = False;
    XkbSetDetectableAutoRepeat(dpy, True, &supported);
    return supported ? 1 : 0;
}

static int ng_pending(Display *dpy) { return XPending(dpy); }

static int ng_wait(Display *dpy, int timeout_ms) {
    struct pollfd p;
    p.fd = ConnectionNumber(dpy);
    p.events = POLLIN;
    p.revents = 0;
    XFlush(dpy);
    int r;
    do { r = poll(&p, 1, timeout_ms); } while (r < 0 && errno == EINTR);
    return r;
}

static int ng_ev_type(XEvent *e) { return e->type; }
static Window ng_ev_window(XEvent *e) { return e->xany.window; }
static unsigned int ng_ev_key_state(XEvent *e) { return e->xkey.state; }
static unsigned int ng_ev_key_code(XEvent *e) { return e->xkey.keycode; }
static KeySym ng_ev_keysym(XEvent *e) { return XLookupKeysym(&e->xkey, 0); }
static int ng_ev_button(XEvent *e) { return e->xbutton.button; }
static int ng_ev_button_x(XEvent *e) { return e->xbutton.x; }
static int ng_ev_button_y(XEvent *e) { return e->xbutton.y; }
static int ng_ev_motion_x(XEvent *e) { return e->xmotion.x; }
static int ng_ev_motion_y(XEvent *e) { return e->xmotion.y; }
static int ng_ev_configure_w(XEvent *e) { return e->xconfigure.width; }
static int ng_ev_configure_h(XEvent *e) { return e->xconfigure.height; }
static int ng_ev_focus(XEvent *e) { return e->xfocus.detail; }
static int ng_ev_focus_mode(XEvent *e) { return e->xfocus.mode; }
static int ng_ev_expose_count(XEvent *e) { return e->xexpose.count; }
static int ng_ev_synthetic(XEvent *e) { return e->xany.send_event ? 1 : 0; }
static void *ng_window_handle(Window w) { return (void *)(uintptr_t)w; }
static long ng_ev_client_data0(XEvent *e) { return e->xclient.data.l[0]; }
// XIM protocol messages (including XIM_COMMIT and forwarded key events) are
// ClientMessages. XFilterEvent must be called with window=None: passing the
// client window makes Xlib ignore _XIM_PROTOCOL messages and silently drops
// input-method commits.
static int ng_filter(Display *dpy, XEvent *e) { return XFilterEvent(e, None); }
*/
import "C"

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"nekoui/internal/core"
	"nekoui/internal/gpu/egl"
	"nekoui/internal/platform"
)

const (
	xKeyPress        = 2
	xKeyRelease      = 3
	xButtonPress     = 4
	xButtonRelease   = 5
	xMotionNotify    = 6
	xEnterNotify     = 7
	xLeaveNotify     = 8
	xFocusIn         = 9
	xFocusOut        = 10
	xExpose          = 12
	xDestroyNotify   = 17
	xMapNotify       = 19
	xConfigureNotify = 22
	xClientMessage   = 33
)

type window struct {
	dpy        *C.Display
	win        C.Window
	root       C.Window
	cmap       C.Colormap
	im         C.XIM
	ic         C.XIC
	fontSet    C.XFontSet
	preeditPos bool
	atomDelete C.Atom
	eventMask  C.long

	eglDpy  *egl.Display
	cfg     *egl.Config
	surface *egl.Surface
	ctx     *egl.Context

	logicalW, logicalH int
	pixelW, pixelH     int
	scale              float32
	title              string
	vsync              bool
	info               string
	imName             string

	focused   bool
	icFocused bool
	textInput core.TextInput
	spotSet   bool
	spotPX    int
	spotPY    int

	cursors   map[core.Cursor]C.Cursor
	curCursor core.Cursor
	keysDown  map[C.KeyCode]bool
	shown     bool
	destroyed bool
	debug     bool
}

// Open creates an X11 window, an EGL surface and an OpenGL 3.3 core context.
func Open(opts platform.Options) (platform.Window, error) {
	C.ng_set_locale()

	dpy := C.XOpenDisplay(nil)
	if dpy == nil {
		return nil, fmt.Errorf("x11: cannot open display %q", os.Getenv("DISPLAY"))
	}
	w := &window{
		dpy:      dpy,
		root:     C.ng_root(dpy),
		scale:    1,
		title:    opts.Title,
		vsync:    opts.VSync,
		cursors:  make(map[core.Cursor]C.Cursor),
		keysDown: make(map[C.KeyCode]bool),
		debug:    os.Getenv("NEKOUI_DEBUG") != "",
	}
	// Xlib falls back to a local IM when XMODIFIERS is unset. KDE/GNOME sessions
	// usually export it, but terminals and CI shells often do not; synthesize it
	// from the XIM_SERVERS advertisement so fcitx5/ibus actually get used.
	if os.Getenv("XMODIFIERS") == "" {
		var name [128]C.char
		if C.ng_xim_server(dpy, &name[0], C.int(len(name))) != 0 {
			w.imName = C.GoString(&name[0])
			_ = os.Setenv("XMODIFIERS", "@im="+w.imName)
		}
	}
	C.ng_locale_modifiers()

	w.logicalW, w.logicalH = opts.Width, opts.Height
	if w.logicalW <= 0 {
		w.logicalW = 960
	}
	if w.logicalH <= 0 {
		w.logicalH = 640
	}

	w.scale = opts.Scale
	if w.scale <= 0 {
		dpi := float64(C.ng_dpi(dpy))
		if dpi <= 0 {
			dpi = 96
		}
		w.scale = float32(dpi / 96.0)
	}
	if w.scale < 1 {
		w.scale = 1
	}
	if w.scale > 4 {
		w.scale = 4
	}
	if v := os.Getenv("NEKOUI_SCALE"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil && f > 0 {
			w.scale = float32(f)
		}
	}
	w.pixelW = int(math.Round(float64(float32(w.logicalW) * w.scale)))
	w.pixelH = int(math.Round(float64(float32(w.logicalH) * w.scale)))

	ed, err := egl.OpenX11(unsafe.Pointer(dpy))
	if err != nil {
		w.Destroy()
		return nil, fmt.Errorf("x11: %w", err)
	}
	w.eglDpy = ed
	cfg, err := ed.ChooseConfig(egl.WindowSurface, 4)
	if err != nil {
		w.Destroy()
		return nil, fmt.Errorf("x11: %w", err)
	}
	w.cfg = cfg

	var depth C.int
	vis := C.ng_visual(dpy, C.ulong(cfg.VisualID()), &depth)
	if vis == nil {
		w.Destroy()
		return nil, fmt.Errorf("x11: no X visual for EGL config visual 0x%x", cfg.VisualID())
	}

	const mask = C.long(C.ExposureMask | C.StructureNotifyMask | C.KeyPressMask |
		C.KeyReleaseMask | C.ButtonPressMask | C.ButtonReleaseMask |
		C.PointerMotionMask | C.EnterWindowMask | C.LeaveWindowMask |
		C.FocusChangeMask | C.PropertyChangeMask)
	w.eventMask = mask
	sw := int(C.ng_screen_w(dpy))
	sh := int(C.ng_screen_h(dpy))
	x := (sw - w.pixelW) / 2
	y := (sh - w.pixelH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	w.win = C.ng_create_window(dpy, w.root, vis, depth, C.int(x), C.int(y),
		C.int(w.pixelW), C.int(w.pixelH), 0x0b0d12, mask, &w.cmap)
	if w.win == 0 {
		w.Destroy()
		return nil, fmt.Errorf("x11: XCreateWindow failed")
	}

	w.atomDelete = C.XInternAtom(dpy, C.CString("WM_DELETE_WINDOW"), C.False)
	// The string above is intentionally not freed: XInternAtom copies it and
	// this path is process-lifetime. Keep the allocation tiny and explicit.
	C.XSetWMProtocols(dpy, w.win, &w.atomDelete, 1)
	className := C.CString("nekoui")
	classClass := C.CString("NekoUI")
	C.ng_set_class(dpy, w.win, className, classClass)
	C.free(unsafe.Pointer(className))
	C.free(unsafe.Pointer(classClass))
	w.setTitle(opts.Title)

	if opts.Resizable {
		minW := int(float32(opts.MinWidth) * w.scale)
		minH := int(float32(opts.MinHeight) * w.scale)
		C.ng_set_size_hints(dpy, w.win, C.int(maxInt(minW, 1)), C.int(maxInt(minH, 1)), 0, 0)
	} else {
		C.ng_set_size_hints(dpy, w.win, C.int(w.pixelW), C.int(w.pixelH), C.int(w.pixelW), C.int(w.pixelH))
	}

	surface, err := ed.CreateWindowSurface(cfg, C.ng_window_handle(w.win))
	if err != nil {
		w.Destroy()
		return nil, fmt.Errorf("x11: %w", err)
	}
	w.surface = surface
	ctx, err := ed.CreateContext(cfg)
	if err != nil {
		w.Destroy()
		return nil, fmt.Errorf("x11: %w", err)
	}
	w.ctx = ctx
	if err := ctx.MakeCurrent(surface); err != nil {
		w.Destroy()
		return nil, fmt.Errorf("x11: %w", err)
	}
	if opts.VSync {
		ed.SetSwapInterval(1)
	} else {
		ed.SetSwapInterval(0)
	}

	// Input method setup is best-effort: plain ASCII typing must keep working
	// when no XIM server is available.
	if w.imName == "" {
		if mods := os.Getenv("XMODIFIERS"); mods != "" {
			if i := strings.Index(mods, "@im="); i >= 0 {
				w.imName = strings.TrimSpace(strings.SplitN(mods[i+4:], ",", 2)[0])
			}
		}
	}
	w.im = C.XOpenIM(dpy, nil, nil, nil)
	if w.im != nil {
		w.fontSet = C.ng_create_fontset(dpy)
		var position C.int
		style := C.ng_pick_xim_style(w.im, &position)
		if style != 0 {
			if position != 0 {
				w.ic = C.ng_create_ic_position(dpy, w.im, w.win, w.fontSet, 0, 0)
				w.preeditPos = w.ic != nil
			}
			if w.ic == nil && style&C.XIMPreeditNothing != 0 {
				w.ic = C.ng_create_ic(dpy, w.im, w.win)
			}
		}
		if w.ic == nil {
			// A few minimal IM servers only advertise the None fallback style.
			w.ic = C.ng_create_ic_none(dpy, w.im, w.win)
		}
		if w.ic != nil {
			// XIM asks for extra events (usually KeyReleaseMask) through
			// XNFilterEvents; without them some servers never see key releases.
			filter := C.ulong(C.ng_ic_filter_events(w.ic))
			C.ng_select_input(dpy, w.win, w.eventMask|C.long(filter))
		}
	}
	C.ng_detectable_repeat(dpy)

	xim := " · no XIM"
	if w.ic != nil {
		xim = " · XIM"
		if w.imName != "" {
			xim += "(" + w.imName + ")"
		}
		if w.preeditPos {
			xim += "/over-the-spot"
		}
	}
	w.info = fmt.Sprintf("X11 · EGL %s · %d× MSAA%s", ed.Version(), cfg.Samples(), xim)
	w.SetCursor(core.CursorArrow)
	return w, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (w *window) Name() string { return "X11" }

func (w *window) Info() string { return w.info }

func (w *window) Size() (int, int) { return w.logicalW, w.logicalH }

func (w *window) PixelSize() (int, int) { return w.pixelW, w.pixelH }

func (w *window) Scale() float32 { return w.scale }

func (w *window) Show() {
	if w.shown || w.destroyed {
		return
	}
	C.XMapWindow(w.dpy, w.win)
	C.XFlush(w.dpy)
	w.shown = true
}

func (w *window) setTitle(title string) {
	if title == "" {
		return
	}
	w.title = title
	cs := C.CString(title)
	defer C.free(unsafe.Pointer(cs))
	C.ng_set_title(w.dpy, w.win, cs)
}

func (w *window) SetTitle(title string) {
	if w.destroyed || title == w.title {
		return
	}
	w.setTitle(title)
	C.XFlush(w.dpy)
}

func (w *window) SetCursor(c core.Cursor) {
	if w.destroyed || c == w.curCursor {
		return
	}
	cur, ok := w.cursors[c]
	if !ok {
		shape := C.uint(C.XC_left_ptr)
		switch c {
		case core.CursorIBeam:
			shape = C.uint(C.XC_xterm)
		case core.CursorHand:
			shape = C.uint(C.XC_hand2)
		case core.CursorCrosshair:
			shape = C.uint(C.XC_crosshair)
		case core.CursorResizeH:
			shape = C.uint(C.XC_sb_h_double_arrow)
		case core.CursorResizeV:
			shape = C.uint(C.XC_sb_v_double_arrow)
		case core.CursorNotAllowed:
			shape = C.uint(C.XC_X_cursor)
		}
		cur = C.XCreateFontCursor(w.dpy, shape)
		w.cursors[c] = cur
	}
	C.XDefineCursor(w.dpy, w.win, cur)
	C.XFlush(w.dpy)
	w.curCursor = c
}

// SetTextInput publishes the focused editor's caret so XIM can place the
// candidate/preedit window over the caret instead of the window origin.
func (w *window) SetTextInput(state core.TextInput) {
	if w.destroyed {
		return
	}
	w.textInput = state
	w.applyTextInput()
}

func (w *window) applyTextInput() {
	if w.ic == nil || w.destroyed {
		return
	}
	wanted := w.focused && w.textInput.Enabled
	if !wanted {
		if w.icFocused {
			C.ng_ic_focus(w.ic, 0)
			w.icFocused = false
		}
		w.spotSet = false
		return
	}
	caret := w.textInput.Caret
	// XNSpotLocation is client-window-relative (input methods translate it to
	// root coordinates themselves). Sending root coordinates makes fcitx add
	// the window origin a second time, which is the classic "candidate window
	// is offset by the window position" bug.
	px := int(math.Round(float64(caret.X * w.scale)))
	py := int(math.Round(float64((caret.Y + caret.H) * w.scale)))
	if !w.spotSet || px != w.spotPX || py != w.spotPY {
		if C.ng_set_spot(w.dpy, w.ic, C.int(px), C.int(py)) != 0 {
			w.spotSet = true
			w.spotPX, w.spotPY = px, py
			if w.debug {
				fmt.Fprintf(os.Stderr, "nekoui ime: caret=%+v local=(%d,%d) enabled=%v\n",
					caret, px, py, w.textInput.Enabled)
			}
		}
	}
	if !w.icFocused {
		C.ng_ic_focus(w.ic, 1)
		w.icFocused = true
	}
	C.XFlush(w.dpy)
}

func (w *window) MakeCurrent() error {
	if w.destroyed {
		return fmt.Errorf("x11: window destroyed")
	}
	return w.ctx.MakeCurrent(w.surface)
}

func (w *window) Swap() error {
	if w.destroyed {
		return fmt.Errorf("x11: window destroyed")
	}
	return w.surface.Swap()
}

func (w *window) PollEvents(timeout time.Duration) ([]core.Event, error) {
	if w.destroyed {
		return nil, nil
	}
	var out []core.Event
	if timeout < 0 {
		var ev C.XEvent
		C.XNextEvent(w.dpy, &ev)
		out = append(out, w.handle(&ev)...)
	} else {
		ms := int(timeout / time.Millisecond)
		if C.ng_pending(w.dpy) == 0 && C.ng_wait(w.dpy, C.int(ms)) == 0 {
			return nil, nil
		}
	}
	for C.ng_pending(w.dpy) > 0 {
		var ev C.XEvent
		C.XNextEvent(w.dpy, &ev)
		out = append(out, w.handle(&ev)...)
	}
	return out, nil
}

func (w *window) handle(ev *C.XEvent) []core.Event {
	typ := int(C.ng_ev_type(ev))
	// XIM is driven by _XIM_PROTOCOL ClientMessages (commit strings and
	// forwarded key events), not just key events. Every event must pass through
	// XFilterEvent with window=None before dispatch; Xlib injects forwarded key
	// events back into the queue, which the PollEvents loop above drains.
	// Synthetic key events (tests/automation) deliberately bypass XIM.
	syntheticKey := (typ == xKeyPress || typ == xKeyRelease) && C.ng_ev_synthetic(ev) != 0
	if !syntheticKey && C.ng_filter(w.dpy, ev) != 0 {
		return nil
	}
	switch typ {
	case xKeyPress:
		return w.key(ev, true)
	case xKeyRelease:
		return w.key(ev, false)
	case xMotionNotify:
		return []core.Event{{
			Type: core.EventMouseMove,
			X:    float32(C.ng_ev_motion_x(ev)) / w.scale,
			Y:    float32(C.ng_ev_motion_y(ev)) / w.scale,
		}}
	case xButtonPress:
		btn := int(C.ng_ev_button(ev))
		if btn >= 4 && btn <= 7 {
			dy := float32(0)
			dx := float32(0)
			switch btn {
			case 4:
				dy = 1
			case 5:
				dy = -1
			case 6:
				dx = -1
			case 7:
				dx = 1
			}
			return []core.Event{{
				Type: core.EventScroll,
				X:    float32(C.ng_ev_button_x(ev)) / w.scale,
				Y:    float32(C.ng_ev_button_y(ev)) / w.scale,
				DX:   dx, DY: dy,
			}}
		}
		return []core.Event{{
			Type:   core.EventMouseDown,
			Button: core.MouseButton(btn),
			X:      float32(C.ng_ev_button_x(ev)) / w.scale,
			Y:      float32(C.ng_ev_button_y(ev)) / w.scale,
		}}
	case xButtonRelease:
		btn := int(C.ng_ev_button(ev))
		if btn >= 4 && btn <= 7 {
			return nil
		}
		return []core.Event{{
			Type:   core.EventMouseUp,
			Button: core.MouseButton(btn),
			X:      float32(C.ng_ev_button_x(ev)) / w.scale,
			Y:      float32(C.ng_ev_button_y(ev)) / w.scale,
		}}
	case xEnterNotify:
		return []core.Event{{Type: core.EventMouseEnter}}
	case xLeaveNotify:
		return []core.Event{{Type: core.EventMouseLeave}}
	case xFocusIn:
		// Keyboard grabs (XIM servers, menus, XTEST helpers) generate
		// synthetic FocusIn/FocusOut pairs. Treating those as real focus
		// changes would disable the IME in the middle of a composition.
		if mode := int(C.ng_ev_focus_mode(ev)); mode != C.NotifyNormal {
			if w.debug {
				fmt.Fprintf(os.Stderr, "nekoui focus-in ignored mode=%d detail=%d\n", mode, int(C.ng_ev_focus(ev)))
			}
			return nil
		}
		w.focused = true
		w.applyTextInput()
		return []core.Event{{Type: core.EventFocus, Focused: true}}
	case xFocusOut:
		if mode := int(C.ng_ev_focus_mode(ev)); mode != C.NotifyNormal {
			if w.debug {
				fmt.Fprintf(os.Stderr, "nekoui focus-out ignored mode=%d detail=%d\n", mode, int(C.ng_ev_focus(ev)))
			}
			return nil
		}
		w.focused = false
		w.applyTextInput()
		clear(w.keysDown)
		return []core.Event{{Type: core.EventFocus, Focused: false}}
	case xConfigureNotify:
		// The window may have moved without resizing; the XIM spot is in root
		// coordinates, so it must be refreshed even when the size is unchanged.
		w.spotSet = false
		w.applyTextInput()
		pw := int(C.ng_ev_configure_w(ev))
		ph := int(C.ng_ev_configure_h(ev))
		if pw == w.pixelW && ph == w.pixelH {
			return nil
		}
		w.pixelW, w.pixelH = pw, ph
		w.logicalW = int(math.Round(float64(float32(pw) / w.scale)))
		w.logicalH = int(math.Round(float64(float32(ph) / w.scale)))
		return []core.Event{{
			Type:        core.EventResize,
			Width:       w.logicalW,
			Height:      w.logicalH,
			PixelWidth:  w.pixelW,
			PixelHeight: w.pixelH,
			Scale:       w.scale,
		}}
	case xExpose:
		if int(C.ng_ev_expose_count(ev)) == 0 {
			return []core.Event{{Type: core.EventExpose}}
		}
	case xClientMessage:
		if C.Atom(C.ng_ev_client_data0(ev)) == w.atomDelete {
			return []core.Event{{Type: core.EventClose}}
		}
	case xDestroyNotify:
		return []core.Event{{Type: core.EventClose}}
	}
	return nil
}

func (w *window) key(ev *C.XEvent, down bool) []core.Event {
	keycode := C.KeyCode(C.ng_ev_key_code(ev))
	ks := C.ng_ev_keysym(ev)
	mods := modsFromState(C.ng_ev_key_state(ev))
	key := mapKeysym(ks)
	var out []core.Event

	if !down {
		delete(w.keysDown, keycode)
		out = append(out, core.Event{
			Type: core.EventKeyUp, Key: key, Mods: mods,
		})
		return out
	}

	repeat := w.keysDown[keycode]
	w.keysDown[keycode] = true
	textOK := mods&(core.ModCtrl|core.ModAlt|core.ModSuper) == 0
	synthetic := C.ng_ev_synthetic(ev) != 0

	var text string
	switch {
	case textOK && w.ic != nil && !synthetic:
		var buf [256]byte
		var sym C.KeySym
		var status C.Status
		n := C.ng_lookup_utf8(w.ic, (*C.XKeyEvent)(unsafe.Pointer(ev)),
			(*C.char)(unsafe.Pointer(&buf[0])), C.int(len(buf)), &sym, &status)
		if status&(C.XLookupChars|C.XLookupBoth) != 0 && n > 0 {
			text = cleanText(string(buf[:int(n)]))
		}
		if status&(C.XLookupKeySym|C.XLookupBoth) != 0 && sym != 0 {
			key = mapKeysym(sym)
		}
	case textOK:
		// Synthetic events (tests/remote control) bypass XIM, and systems
		// without an input method still get printable Latin-1 text.
		var buf [64]byte
		var sym C.KeySym
		n := C.ng_lookup_latin1((*C.XKeyEvent)(unsafe.Pointer(ev)),
			(*C.char)(unsafe.Pointer(&buf[0])), C.int(len(buf)), &sym)
		if n > 0 {
			text = cleanText(string(buf[:int(n)]))
		}
		if sym != 0 {
			key = mapKeysym(sym)
		}
	}

	out = append(out, core.Event{
		Type: core.EventKeyDown, Key: key, Mods: mods, Repeat: repeat,
	})
	if text != "" {
		out = append(out, core.Event{Type: core.EventText, Text: text})
	}
	return out
}

func cleanText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func modsFromState(state C.uint) core.Modifiers {
	var m core.Modifiers
	if state&C.ShiftMask != 0 {
		m |= core.ModShift
	}
	if state&C.ControlMask != 0 {
		m |= core.ModCtrl
	}
	if state&C.Mod1Mask != 0 {
		m |= core.ModAlt
	}
	if state&C.Mod4Mask != 0 {
		m |= core.ModSuper
	}
	return m
}

const (
	keysymBackspace = 0xff08
	keysymTab       = 0xff09
	keysymReturn    = 0xff0d
	keysymEscape    = 0xff1b
	keysymDelete    = 0xffff
	keysymHome      = 0xff50
	keysymLeft      = 0xff51
	keysymUp        = 0xff52
	keysymRight     = 0xff53
	keysymDown      = 0xff54
	keysymPageUp    = 0xff55
	keysymPageDown  = 0xff56
	keysymEnd       = 0xff57
	keysymInsert    = 0xff63
	keysymF1        = 0xffbe
	keysymShiftL    = 0xffe1
	keysymShiftR    = 0xffe2
	keysymControlL  = 0xffe3
	keysymControlR  = 0xffe4
	keysymCapsLock  = 0xffe5
	keysymAltL      = 0xffe9
	keysymAltR      = 0xffea
	keysymSuperL    = 0xffeb
	keysymSuperR    = 0xffec
)

func mapKeysym(ks C.KeySym) core.Key {
	return core.KeyFromKeysym(uint32(ks))
}

func (w *window) Destroy() {
	if w.destroyed {
		return
	}
	w.destroyed = true
	if w.ic != nil {
		if w.icFocused {
			C.ng_ic_focus(w.ic, 0)
			w.icFocused = false
		}
		C.XDestroyIC(w.ic)
		w.ic = nil
	}
	if w.im != nil {
		C.XCloseIM(w.im)
		w.im = nil
	}
	if w.fontSet != nil {
		C.ng_free_fontset(w.dpy, w.fontSet)
		w.fontSet = nil
	}
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
	if w.dpy != nil {
		for _, c := range w.cursors {
			C.XFreeCursor(w.dpy, c)
		}
		w.cursors = nil
		if w.win != 0 {
			C.XDestroyWindow(w.dpy, w.win)
			w.win = 0
		}
		if w.cmap != 0 {
			C.XFreeColormap(w.dpy, w.cmap)
			w.cmap = 0
		}
		C.XCloseDisplay(w.dpy)
		w.dpy = nil
	}
}
