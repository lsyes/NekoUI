// NekoUI Wayland client shim.
//
// The window/protocol plumbing lives in C because Wayland's object model is a
// large set of callback-based vtables; keeping it in one translation unit makes
// the cgo boundary small and lets the Go backend consume a flat event queue.
// The EGL surface/context and all rendering remain in Go.

#define _GNU_SOURCE

#include "shim.h"

#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <unistd.h>

#include <wayland-client.h>
#include <wayland-cursor.h>
#include <wayland-egl.h>

#include "xdg-shell-client-protocol.h"
#include "xdg-decoration-unstable-v1-client-protocol.h"
#include "viewporter-client-protocol.h"
#include "fractional-scale-v1-client-protocol.h"
#include "cursor-shape-v1-client-protocol.h"
#include "text-input-unstable-v3-client-protocol.h"

/* libxkbcommon is linked without development headers (the runtime library is a
 * hard dependency of every Wayland desktop). These declarations are the stable
 * public ABI we need. */
typedef uint32_t xkb_keysym_t;
typedef uint32_t xkb_keycode_t;
typedef uint32_t xkb_mod_index_t;
struct xkb_context;
struct xkb_keymap;
struct xkb_state;

#define XKB_KEYMAP_FORMAT_TEXT_V1 1
#define XKB_STATE_MODS_EFFECTIVE 8
#define XKB_MOD_INVALID 0xffffffffu

extern struct xkb_context *xkb_context_new(int flags);
extern void xkb_context_unref(struct xkb_context *context);
extern struct xkb_keymap *xkb_keymap_new_from_string(struct xkb_context *context,
                                                     const char *string, int format,
                                                     int flags);
extern void xkb_keymap_unref(struct xkb_keymap *keymap);
extern struct xkb_state *xkb_state_new(struct xkb_keymap *keymap);
extern void xkb_state_unref(struct xkb_state *state);
extern xkb_keysym_t xkb_state_key_get_one_sym(struct xkb_state *state, xkb_keycode_t key);
extern int xkb_state_key_get_utf8(struct xkb_state *state, xkb_keycode_t key,
                                  char *buffer, size_t size);
extern xkb_mod_index_t xkb_keymap_mod_get_index(struct xkb_keymap *keymap, const char *name);
extern int xkb_state_mod_index_is_active(struct xkb_state *state, xkb_mod_index_t idx,
                                         int component);
extern int xkb_state_update_mask(struct xkb_state *state, uint32_t depressed,
                                 uint32_t latched, uint32_t locked,
                                 uint32_t layout_depressed, uint32_t layout_latched,
                                 uint32_t layout_locked);

#define NW_QUEUE_CAP 512

struct nw_window {
    struct wl_display *display;
    struct wl_registry *registry;
    struct wl_compositor *compositor;
    struct wl_shm *shm;
    struct wl_surface *surface;
    struct xdg_wm_base *wm_base;
    struct xdg_surface *xdg_surface;
    struct xdg_toplevel *toplevel;
    struct wl_egl_window *egl_window;

    struct wl_seat *seat;
    struct wl_keyboard *keyboard;
    struct wl_pointer *pointer;
    struct wl_output *outputs[8];
    int output_count;

    struct wp_viewporter *viewporter;
    struct wp_viewport *viewport;
    struct wp_fractional_scale_manager_v1 *fractional_manager;
    struct wp_fractional_scale_v1 *fractional;
    struct wp_cursor_shape_manager_v1 *cursor_shape_manager;
    struct wp_cursor_shape_device_v1 *cursor_shape_device;
    struct zxdg_decoration_manager_v1 *decoration_manager;
    struct zxdg_toplevel_decoration_v1 *decoration;
    struct zwp_text_input_manager_v3 *text_input_manager;
    struct zwp_text_input_v3 *text_input;

    struct wl_cursor_theme *cursor_theme;
    struct wl_surface *cursor_surface;

    int logical_w, logical_h;
    int pixel_w, pixel_h;
    int pending_w, pending_h;
    int min_w, min_h;
    int resizable;
    int configured;
    int shown;
    float scale_override;
    float scale;
    int output_scale;
    int fractional_scale;
    int buffer_scale;
    int use_viewport;
    int failed;
    char err[512];

    uint32_t keyboard_serial;
    int keyboard_focus;
    int pointer_serial_valid;
    uint32_t pointer_serial;
    int pointer_entered;
    float pointer_x, pointer_y;
    int cursor;

    int text_input_wanted;
    int text_input_entered;
    int text_input_enabled;
    int text_input_dirty;
    int preedit_active;
    int ti_x, ti_y, ti_w, ti_h;
    int ti_last_x, ti_last_y, ti_last_w, ti_last_h;

    struct xkb_context *xkb_context;
    struct xkb_keymap *xkb_keymap;
    struct xkb_state *xkb_state;
    xkb_mod_index_t mod_shift, mod_ctrl, mod_alt, mod_super;
    int keys_down[256];

    nw_event queue[NW_QUEUE_CAP];
    int qhead, qtail;
};

static void set_errorf(nw_window *w, const char *fmt, ...) {
    if (w == NULL || w->failed) return;
    w->failed = 1;
    va_list ap;
    va_start(ap, fmt);
    vsnprintf(w->err, sizeof(w->err), fmt, ap);
    va_end(ap);
}

static nw_event *push_event(nw_window *w, int type) {
    int next = (w->qtail + 1) % NW_QUEUE_CAP;
    if (next == w->qhead) return NULL; /* queue full: drop */
    nw_event *e = &w->queue[w->qtail];
    memset(e, 0, sizeof(*e));
    e->type = type;
    w->qtail = next;
    return e;
}

static void copy_text(char *dst, size_t cap, const char *src) {
    if (cap == 0) return;
    if (src == NULL) {
        dst[0] = '\0';
        return;
    }
    size_t n = strlen(src);
    if (n >= cap) n = cap - 1;
    memcpy(dst, src, n);
    dst[n] = '\0';
}

static int rune_offset(const char *s, int byte_offset) {
    if (s == NULL || byte_offset <= 0) return 0;
    int runes = 0;
    int i = 0;
    while (s[i] != '\0' && i < byte_offset) {
        unsigned char c = (unsigned char)s[i];
        int len = 1;
        if ((c & 0xE0) == 0xC0) len = 2;
        else if ((c & 0xF0) == 0xE0) len = 3;
        else if ((c & 0xF8) == 0xF0) len = 4;
        i += len;
        runes++;
    }
    return runes;
}

static int clean_utf8(const char *src, char *dst, size_t cap) {
    size_t out = 0;
    if (cap == 0) return 0;
    for (size_t i = 0; src[i] != '\0' && out + 4 < cap;) {
        unsigned char c = (unsigned char)src[i];
        int len = 1;
        if (c < 0x80) {
            if (c < 0x20 || c == 0x7f) {
                i++;
                continue;
            }
        } else if ((c & 0xE0) == 0xC0) {
            len = 2;
        } else if ((c & 0xF0) == 0xE0) {
            len = 3;
        } else if ((c & 0xF8) == 0xF0) {
            len = 4;
        }
        for (int k = 0; k < len; k++) {
            if (src[i + k] == '\0') {
                dst[out] = '\0';
                return (int)out;
            }
        }
        memcpy(dst + out, src + i, (size_t)len);
        out += (size_t)len;
        i += (size_t)len;
    }
    dst[out] = '\0';
    return (int)out;
}

static void queue_resize(nw_window *w) {
    if (!w->configured) return;
    nw_event *e = push_event(w, NW_EVENT_RESIZE);
    if (e == NULL) return;
    e->width = w->logical_w;
    e->height = w->logical_h;
    e->pixel_width = w->pixel_w;
    e->pixel_height = w->pixel_h;
    e->scale = w->scale;
}

static void update_layout(nw_window *w) {
    if (w->logical_w <= 0) w->logical_w = 1;
    if (w->logical_h <= 0) w->logical_h = 1;

    float s = 1.0f;
    if (w->scale_override > 0) {
        s = w->scale_override;
    } else if (w->fractional != NULL && w->fractional_scale > 0) {
        s = (float)w->fractional_scale / 120.0f;
    } else if (w->output_scale > 0) {
        s = (float)w->output_scale;
    }
    if (s < 0.5f) s = 0.5f;
    if (s > 4.0f) s = 4.0f;

    w->use_viewport = (w->viewporter != NULL && w->viewport != NULL);
    int iscale = (int)(s + 0.5f);
    if (iscale < 1) iscale = 1;
    if (w->use_viewport) {
        w->buffer_scale = 1;
        w->scale = s;
        w->pixel_w = (int)((float)w->logical_w * s + 0.5f);
        w->pixel_h = (int)((float)w->logical_h * s + 0.5f);
    } else {
        w->buffer_scale = iscale;
        w->scale = (float)iscale;
        w->pixel_w = w->logical_w * iscale;
        w->pixel_h = w->logical_h * iscale;
    }
    if (w->pixel_w < 1) w->pixel_w = 1;
    if (w->pixel_h < 1) w->pixel_h = 1;
    if (w->surface != NULL) wl_surface_set_buffer_scale(w->surface, w->buffer_scale);
    if (w->viewport != NULL) wp_viewport_set_destination(w->viewport, w->logical_w, w->logical_h);
    if (w->egl_window != NULL) wl_egl_window_resize(w->egl_window, w->pixel_w, w->pixel_h, 0, 0);
    queue_resize(w);
}

/* ------------------------------------------------------------------ */
/* Cursor                                                              */

static const char *cursor_name_for(int cursor) {
    switch (cursor) {
    case 1: return "text";
    case 2: return "pointer";
    case 3: return "crosshair";
    case 4: return "sb_h_double_arrow";
    case 5: return "sb_v_double_arrow";
    case 6: return "not-allowed";
    default: return "left_ptr";
    }
}

static uint32_t cursor_shape_for(int cursor) {
    switch (cursor) {
    case 1: return WP_CURSOR_SHAPE_DEVICE_V1_SHAPE_TEXT;
    case 2: return WP_CURSOR_SHAPE_DEVICE_V1_SHAPE_POINTER;
    case 3: return WP_CURSOR_SHAPE_DEVICE_V1_SHAPE_CROSSHAIR;
    case 4: return WP_CURSOR_SHAPE_DEVICE_V1_SHAPE_EW_RESIZE;
    case 5: return WP_CURSOR_SHAPE_DEVICE_V1_SHAPE_NS_RESIZE;
    case 6: return WP_CURSOR_SHAPE_DEVICE_V1_SHAPE_NOT_ALLOWED;
    default: return WP_CURSOR_SHAPE_DEVICE_V1_SHAPE_DEFAULT;
    }
}

static void apply_cursor(nw_window *w) {
    if (w->pointer == NULL || !w->pointer_serial_valid) return;
    if (w->cursor_shape_device != NULL) {
        wp_cursor_shape_device_v1_set_shape(w->cursor_shape_device, w->pointer_serial,
                                            cursor_shape_for(w->cursor));
        wl_display_flush(w->display);
        return;
    }
    if (w->compositor == NULL || w->shm == NULL) return;
    if (w->cursor_theme == NULL) {
        int size = 24 * (w->buffer_scale > 0 ? w->buffer_scale : 1);
        w->cursor_theme = wl_cursor_theme_load(NULL, size, w->shm);
        if (w->cursor_theme == NULL) return;
    }
    struct wl_cursor *cursor = wl_cursor_theme_get_cursor(w->cursor_theme,
                                                          cursor_name_for(w->cursor));
    if (cursor == NULL || cursor->image_count == 0) return;
    struct wl_cursor_image *image = cursor->images[0];
    struct wl_buffer *buffer = wl_cursor_image_get_buffer(image);
    if (buffer == NULL) return;
    if (w->cursor_surface == NULL) w->cursor_surface = wl_compositor_create_surface(w->compositor);
    wl_surface_attach(w->cursor_surface, buffer, 0, 0);
    wl_surface_damage(w->cursor_surface, 0, 0, (int32_t)image->width, (int32_t)image->height);
    wl_surface_commit(w->cursor_surface);
    wl_pointer_set_cursor(w->pointer, w->pointer_serial, w->cursor_surface,
                          (int32_t)image->hotspot_x, (int32_t)image->hotspot_y);
}

/* ------------------------------------------------------------------ */
/* Text input                                                          */

static void sync_text_input(nw_window *w) {
    if (w->text_input == NULL) return;
    int want = w->text_input_wanted && w->keyboard_focus && w->text_input_entered;
    if (want && !w->text_input_enabled) {
        zwp_text_input_v3_enable(w->text_input);
        w->text_input_enabled = 1;
        w->text_input_dirty = 1;
    } else if (!want && w->text_input_enabled) {
        zwp_text_input_v3_disable(w->text_input);
        w->text_input_enabled = 0;
        w->preedit_active = 0;
        w->text_input_dirty = 1;
    }
    if (want) {
        if (w->ti_x != w->ti_last_x || w->ti_y != w->ti_last_y ||
            w->ti_w != w->ti_last_w || w->ti_h != w->ti_last_h) {
            zwp_text_input_v3_set_cursor_rectangle(w->text_input, w->ti_x, w->ti_y,
                                                   w->ti_w, w->ti_h);
            w->ti_last_x = w->ti_x;
            w->ti_last_y = w->ti_y;
            w->ti_last_w = w->ti_w;
            w->ti_last_h = w->ti_h;
            w->text_input_dirty = 1;
        }
    }
    if (w->text_input_dirty) {
        zwp_text_input_v3_commit(w->text_input);
        w->text_input_dirty = 0;
    }
    wl_display_flush(w->display);
}

/* ------------------------------------------------------------------ */
/* Text input listener                                                 */

static void ti_enter(void *data, struct zwp_text_input_v3 *ti, struct wl_surface *surface) {
    (void)ti;
    (void)surface;
    nw_window *w = data;
    w->text_input_entered = 1;
    sync_text_input(w);
}

static void ti_leave(void *data, struct zwp_text_input_v3 *ti, struct wl_surface *surface) {
    (void)ti;
    (void)surface;
    nw_window *w = data;
    w->text_input_entered = 0;
    w->preedit_active = 0;
    if (w->text_input_enabled) {
        zwp_text_input_v3_disable(w->text_input);
        zwp_text_input_v3_commit(w->text_input);
        w->text_input_enabled = 0;
    }
    nw_event *e = push_event(w, NW_EVENT_PREEDIT);
    if (e != NULL) e->text[0] = '\0';
}

static void ti_preedit(void *data, struct zwp_text_input_v3 *ti, const char *text,
                       int32_t cursor_begin, int32_t cursor_end) {
    (void)ti;
    (void)cursor_end;
    nw_window *w = data;
    nw_event *e = push_event(w, NW_EVENT_PREEDIT);
    if (e != NULL) {
        copy_text(e->text, sizeof(e->text), text);
        e->cursor = rune_offset(text, cursor_begin);
    }
    w->preedit_active = (text != NULL && text[0] != '\0');
}

static void ti_commit(void *data, struct zwp_text_input_v3 *ti, const char *text) {
    (void)ti;
    nw_window *w = data;
    nw_event *e = push_event(w, NW_EVENT_TEXT);
    if (e != NULL) copy_text(e->text, sizeof(e->text), text);
    w->preedit_active = 0;
}

static void ti_delete(void *data, struct zwp_text_input_v3 *ti, uint32_t before,
                      uint32_t after) {
    (void)ti;
    nw_window *w = data;
    nw_event *e = push_event(w, NW_EVENT_DELETE_SURROUNDING);
    if (e != NULL) {
        e->before = (int)before;
        e->after = (int)after;
    }
}

static void ti_done(void *data, struct zwp_text_input_v3 *ti, uint32_t serial) {
    (void)data;
    (void)ti;
    (void)serial;
}

static void ti_action(void *data, struct zwp_text_input_v3 *ti, uint32_t action,
                      uint32_t serial) {
    (void)data;
    (void)ti;
    (void)action;
    (void)serial;
}

static void ti_language(void *data, struct zwp_text_input_v3 *ti, const char *language) {
    (void)data;
    (void)ti;
    (void)language;
}

static void ti_preedit_hint(void *data, struct zwp_text_input_v3 *ti, uint32_t start,
                            uint32_t end, uint32_t hint) {
    (void)data;
    (void)ti;
    (void)start;
    (void)end;
    (void)hint;
}

static const struct zwp_text_input_v3_listener text_input_listener = {
    .enter = ti_enter,
    .leave = ti_leave,
    .preedit_string = ti_preedit,
    .commit_string = ti_commit,
    .delete_surrounding_text = ti_delete,
    .done = ti_done,
    .action = ti_action,
    .language = ti_language,
    .preedit_hint = ti_preedit_hint,
};

/* ------------------------------------------------------------------ */
/* Keyboard                                                            */

static uint32_t current_mods(nw_window *w) {
    uint32_t mods = 0;
    if (w->xkb_state != NULL) {
        if (w->mod_shift != XKB_MOD_INVALID &&
            xkb_state_mod_index_is_active(w->xkb_state, w->mod_shift, XKB_STATE_MODS_EFFECTIVE))
            mods |= NW_MOD_SHIFT;
        if (w->mod_ctrl != XKB_MOD_INVALID &&
            xkb_state_mod_index_is_active(w->xkb_state, w->mod_ctrl, XKB_STATE_MODS_EFFECTIVE))
            mods |= NW_MOD_CTRL;
        if (w->mod_alt != XKB_MOD_INVALID &&
            xkb_state_mod_index_is_active(w->xkb_state, w->mod_alt, XKB_STATE_MODS_EFFECTIVE))
            mods |= NW_MOD_ALT;
        if (w->mod_super != XKB_MOD_INVALID &&
            xkb_state_mod_index_is_active(w->xkb_state, w->mod_super, XKB_STATE_MODS_EFFECTIVE))
            mods |= NW_MOD_SUPER;
    }
    return mods;
}

static void kb_keymap(void *data, struct wl_keyboard *kb, uint32_t format, int32_t fd,
                      uint32_t size) {
    (void)kb;
    nw_window *w = data;
    if (format != WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1 || size == 0 || w->xkb_context == NULL) {
        close(fd);
        return;
    }
    char *map = mmap(NULL, size, PROT_READ, MAP_PRIVATE, fd, 0);
    if (map == MAP_FAILED) {
        close(fd);
        return;
    }
    struct xkb_keymap *keymap = xkb_keymap_new_from_string(w->xkb_context, map,
                                                           XKB_KEYMAP_FORMAT_TEXT_V1, 0);
    munmap(map, size);
    close(fd);
    if (keymap == NULL) return;
    struct xkb_state *state = xkb_state_new(keymap);
    if (state == NULL) {
        xkb_keymap_unref(keymap);
        return;
    }
    if (w->xkb_state != NULL) xkb_state_unref(w->xkb_state);
    if (w->xkb_keymap != NULL) xkb_keymap_unref(w->xkb_keymap);
    w->xkb_keymap = keymap;
    w->xkb_state = state;
    w->mod_shift = xkb_keymap_mod_get_index(keymap, "Shift");
    w->mod_ctrl = xkb_keymap_mod_get_index(keymap, "Control");
    w->mod_alt = xkb_keymap_mod_get_index(keymap, "Mod1");
    w->mod_super = xkb_keymap_mod_get_index(keymap, "Mod4");
}

static void kb_enter(void *data, struct wl_keyboard *kb, uint32_t serial,
                     struct wl_surface *surface, struct wl_array *keys) {
    (void)kb;
    (void)surface;
    nw_window *w = data;
    w->keyboard_focus = 1;
    w->keyboard_serial = serial;
    memset(w->keys_down, 0, sizeof(w->keys_down));
    if (keys != NULL) {
        uint32_t *key;
        wl_array_for_each(key, keys) {
            uint32_t kc = *key + 8;
            if (kc < 256) w->keys_down[kc] = 1;
        }
    }
    nw_event *e = push_event(w, NW_EVENT_FOCUS);
    if (e != NULL) e->focused = 1;
    sync_text_input(w);
}

static void kb_leave(void *data, struct wl_keyboard *kb, uint32_t serial,
                     struct wl_surface *surface) {
    (void)kb;
    (void)serial;
    (void)surface;
    nw_window *w = data;
    w->keyboard_focus = 0;
    memset(w->keys_down, 0, sizeof(w->keys_down));
    nw_event *e = push_event(w, NW_EVENT_FOCUS);
    if (e != NULL) e->focused = 0;
    sync_text_input(w);
}

static void kb_key(void *data, struct wl_keyboard *kb, uint32_t serial, uint32_t time,
                   uint32_t key, uint32_t state) {
    (void)kb;
    (void)serial;
    (void)time;
    nw_window *w = data;
    uint32_t keycode = key + 8;
    if (keycode >= 256) return;
    xkb_keysym_t sym = 0;
    uint32_t mods = current_mods(w);
    if (w->xkb_state != NULL) sym = xkb_state_key_get_one_sym(w->xkb_state, keycode);
    if (state == WL_KEYBOARD_KEY_STATE_PRESSED) {
        int repeat = w->keys_down[keycode] != 0;
        w->keys_down[keycode] = 1;
        nw_event *e = push_event(w, NW_EVENT_KEY_DOWN);
        if (e != NULL) {
            e->key = sym;
            e->mods = mods;
            e->repeat = repeat;
        }
        if (w->xkb_state != NULL && !w->preedit_active &&
            (mods & (NW_MOD_CTRL | NW_MOD_ALT | NW_MOD_SUPER)) == 0) {
            char raw[64];
            char clean[64];
            int n = xkb_state_key_get_utf8(w->xkb_state, keycode, raw, sizeof(raw));
            if (n > 0) {
                raw[sizeof(raw) - 1] = '\0';
                if (clean_utf8(raw, clean, sizeof(clean)) > 0) {
                    nw_event *t = push_event(w, NW_EVENT_TEXT);
                    if (t != NULL) copy_text(t->text, sizeof(t->text), clean);
                }
            }
        }
    } else if (state == WL_KEYBOARD_KEY_STATE_RELEASED) {
        w->keys_down[keycode] = 0;
        nw_event *e = push_event(w, NW_EVENT_KEY_UP);
        if (e != NULL) {
            e->key = sym;
            e->mods = mods;
        }
    }
}

static void kb_modifiers(void *data, struct wl_keyboard *kb, uint32_t serial,
                         uint32_t depressed, uint32_t latched, uint32_t locked,
                         uint32_t group) {
    (void)kb;
    (void)serial;
    nw_window *w = data;
    if (w->xkb_state != NULL) {
        xkb_state_update_mask(w->xkb_state, depressed, latched, locked, 0, 0, group);
    }
}

static void kb_repeat_info(void *data, struct wl_keyboard *kb, int32_t rate,
                           int32_t delay) {
    (void)data;
    (void)kb;
    (void)rate;
    (void)delay;
}

static const struct wl_keyboard_listener keyboard_listener = {
    .keymap = kb_keymap,
    .enter = kb_enter,
    .leave = kb_leave,
    .key = kb_key,
    .modifiers = kb_modifiers,
    .repeat_info = kb_repeat_info,
};

/* ------------------------------------------------------------------ */
/* Pointer                                                             */

static void pointer_enter(void *data, struct wl_pointer *pointer, uint32_t serial,
                          struct wl_surface *surface, wl_fixed_t sx, wl_fixed_t sy) {
    (void)surface;
    nw_window *w = data;
    w->pointer = pointer;
    w->pointer_entered = 1;
    w->pointer_serial = serial;
    w->pointer_serial_valid = 1;
    w->pointer_x = (float)wl_fixed_to_double(sx);
    w->pointer_y = (float)wl_fixed_to_double(sy);
    nw_event *e = push_event(w, NW_EVENT_MOUSE_ENTER);
    if (e != NULL) {
        e->x = (float)wl_fixed_to_double(sx);
        e->y = (float)wl_fixed_to_double(sy);
    }
    nw_event *m = push_event(w, NW_EVENT_MOUSE_MOVE);
    if (m != NULL) {
        m->x = (float)wl_fixed_to_double(sx);
        m->y = (float)wl_fixed_to_double(sy);
    }
    apply_cursor(w);
}

static void pointer_leave(void *data, struct wl_pointer *pointer, uint32_t serial,
                          struct wl_surface *surface) {
    (void)pointer;
    (void)serial;
    (void)surface;
    nw_window *w = data;
    w->pointer_entered = 0;
    w->pointer_serial_valid = 0;
    push_event(w, NW_EVENT_MOUSE_LEAVE);
}

static void pointer_motion(void *data, struct wl_pointer *pointer, uint32_t time,
                           wl_fixed_t sx, wl_fixed_t sy) {
    (void)pointer;
    (void)time;
    nw_window *w = data;
    w->pointer_x = (float)wl_fixed_to_double(sx);
    w->pointer_y = (float)wl_fixed_to_double(sy);
    nw_event *e = push_event(w, NW_EVENT_MOUSE_MOVE);
    if (e != NULL) {
        e->x = w->pointer_x;
        e->y = w->pointer_y;
    }
}

static int button_code(uint32_t button) {
    switch (button) {
    case 0x110: return 1; /* BTN_LEFT */
    case 0x112: return 2; /* BTN_MIDDLE */
    case 0x111: return 3; /* BTN_RIGHT */
    case 0x113: return 8; /* BTN_SIDE */
    case 0x114: return 9; /* BTN_EXTRA */
    default: return 0;
    }
}

static void pointer_button(void *data, struct wl_pointer *pointer, uint32_t serial,
                           uint32_t time, uint32_t button, uint32_t state) {
    (void)pointer;
    (void)serial;
    (void)time;
    nw_window *w = data;
    int code = button_code(button);
    if (code == 0) return;
    int type = state == WL_POINTER_BUTTON_STATE_PRESSED ? NW_EVENT_MOUSE_DOWN
                                                         : NW_EVENT_MOUSE_UP;
    nw_event *e = push_event(w, type);
    if (e != NULL) {
        e->button = code;
        e->x = w->pointer_x;
        e->y = w->pointer_y;
    }
}

static void pointer_axis(void *data, struct wl_pointer *pointer, uint32_t time,
                         uint32_t axis, wl_fixed_t value) {
    (void)pointer;
    (void)time;
    nw_window *w = data;
    double v = wl_fixed_to_double(value);
    if (v == 0) return;
    nw_event *e = push_event(w, NW_EVENT_SCROLL);
    if (e == NULL) return;
    e->x = w->pointer_x;
    e->y = w->pointer_y;
    if (axis == WL_POINTER_AXIS_VERTICAL_SCROLL) {
        e->dy = v > 0 ? -1.0f : 1.0f; /* positive Wayland = down */
    } else if (axis == WL_POINTER_AXIS_HORIZONTAL_SCROLL) {
        e->dx = v > 0 ? 1.0f : -1.0f; /* positive = right */
    }
}

static void pointer_frame(void *data, struct wl_pointer *pointer) {
    (void)data;
    (void)pointer;
}

static void pointer_axis_source(void *data, struct wl_pointer *pointer, uint32_t source) {
    (void)data;
    (void)pointer;
    (void)source;
}

static void pointer_axis_stop(void *data, struct wl_pointer *pointer, uint32_t time,
                              uint32_t axis) {
    (void)data;
    (void)pointer;
    (void)time;
    (void)axis;
}

static void pointer_axis_discrete(void *data, struct wl_pointer *pointer, uint32_t axis,
                                  int32_t discrete) {
    (void)data;
    (void)pointer;
    (void)axis;
    (void)discrete;
}

static const struct wl_pointer_listener pointer_listener = {
    .enter = pointer_enter,
    .leave = pointer_leave,
    .motion = pointer_motion,
    .button = pointer_button,
    .axis = pointer_axis,
    .frame = pointer_frame,
    .axis_source = pointer_axis_source,
    .axis_stop = pointer_axis_stop,
    .axis_discrete = pointer_axis_discrete,
};

/* ------------------------------------------------------------------ */
/* Seat                                                                */

static void create_text_input(nw_window *w) {
    if (w->text_input_manager == NULL || w->seat == NULL || w->text_input != NULL) return;
    w->text_input = zwp_text_input_manager_v3_get_text_input(w->text_input_manager, w->seat);
    if (w->text_input != NULL) {
        zwp_text_input_v3_add_listener(w->text_input, &text_input_listener, w);
    }
}

static void create_cursor_shape(nw_window *w) {
    if (w->cursor_shape_manager == NULL || w->pointer == NULL ||
        w->cursor_shape_device != NULL)
        return;
    w->cursor_shape_device =
        wp_cursor_shape_manager_v1_get_pointer(w->cursor_shape_manager, w->pointer);
}

static void seat_capabilities(void *data, struct wl_seat *seat, uint32_t caps) {
    nw_window *w = data;
    if ((caps & WL_SEAT_CAPABILITY_KEYBOARD) && w->keyboard == NULL) {
        w->keyboard = wl_seat_get_keyboard(seat);
        wl_keyboard_add_listener(w->keyboard, &keyboard_listener, w);
    } else if (!(caps & WL_SEAT_CAPABILITY_KEYBOARD) && w->keyboard != NULL) {
        wl_keyboard_release(w->keyboard);
        w->keyboard = NULL;
        w->keyboard_focus = 0;
    }
    if ((caps & WL_SEAT_CAPABILITY_POINTER) && w->pointer == NULL) {
        w->pointer = wl_seat_get_pointer(seat);
        wl_pointer_add_listener(w->pointer, &pointer_listener, w);
        create_cursor_shape(w);
    } else if (!(caps & WL_SEAT_CAPABILITY_POINTER) && w->pointer != NULL) {
        if (w->cursor_shape_device != NULL) {
            wp_cursor_shape_device_v1_destroy(w->cursor_shape_device);
            w->cursor_shape_device = NULL;
        }
        wl_pointer_release(w->pointer);
        w->pointer = NULL;
        w->pointer_entered = 0;
        w->pointer_serial_valid = 0;
    }
    create_text_input(w);
}

static void seat_name(void *data, struct wl_seat *seat, const char *name) {
    (void)data;
    (void)seat;
    (void)name;
}

static const struct wl_seat_listener seat_listener = {
    .capabilities = seat_capabilities,
    .name = seat_name,
};

/* ------------------------------------------------------------------ */
/* Output                                                              */

static void output_scale(void *data, struct wl_output *output, int32_t factor) {
    (void)output;
    nw_window *w = data;
    if (factor > w->output_scale) {
        w->output_scale = factor;
        update_layout(w);
    }
}

static void output_geometry(void *data, struct wl_output *output, int32_t x, int32_t y,
                            int32_t physical_width, int32_t physical_height, int32_t subpixel,
                            const char *make, const char *model, int32_t transform) {
    (void)data;
    (void)output;
    (void)x;
    (void)y;
    (void)physical_width;
    (void)physical_height;
    (void)subpixel;
    (void)make;
    (void)model;
    (void)transform;
}

static void output_mode(void *data, struct wl_output *output, uint32_t flags,
                        int32_t width, int32_t height, int32_t refresh) {
    (void)data;
    (void)output;
    (void)flags;
    (void)width;
    (void)height;
    (void)refresh;
}

static void output_done(void *data, struct wl_output *output) {
    (void)data;
    (void)output;
}

static const struct wl_output_listener output_listener = {
    .geometry = output_geometry,
    .mode = output_mode,
    .done = output_done,
    .scale = output_scale,
};

/* ------------------------------------------------------------------ */
/* xdg-shell                                                           */

static void wm_ping(void *data, struct xdg_wm_base *wm, uint32_t serial) {
    (void)data;
    xdg_wm_base_pong(wm, serial);
}

static const struct xdg_wm_base_listener wm_listener = {
    .ping = wm_ping,
};

static void surface_configure(void *data, struct xdg_surface *surface, uint32_t serial) {
    nw_window *w = data;
    if (w->pending_w > 0) w->logical_w = w->pending_w;
    if (w->pending_h > 0) w->logical_h = w->pending_h;
    w->configured = 1;
    update_layout(w);
    xdg_surface_ack_configure(surface, serial);
    wl_surface_commit(w->surface);
    wl_display_flush(w->display);
}

static const struct xdg_surface_listener surface_listener = {
    .configure = surface_configure,
};

static void toplevel_configure(void *data, struct xdg_toplevel *toplevel, int32_t width,
                               int32_t height, struct wl_array *states) {
    (void)toplevel;
    (void)states;
    nw_window *w = data;
    w->pending_w = width;
    w->pending_h = height;
}

static void toplevel_close(void *data, struct xdg_toplevel *toplevel) {
    (void)toplevel;
    nw_window *w = data;
    push_event(w, NW_EVENT_CLOSE);
}

static void toplevel_configure_bounds(void *data, struct xdg_toplevel *toplevel,
                                      int32_t width, int32_t height) {
    (void)data;
    (void)toplevel;
    (void)width;
    (void)height;
}

static void toplevel_wm_capabilities(void *data, struct xdg_toplevel *toplevel,
                                     struct wl_array *capabilities) {
    (void)data;
    (void)toplevel;
    (void)capabilities;
}

static const struct xdg_toplevel_listener toplevel_listener = {
    .configure = toplevel_configure,
    .close = toplevel_close,
    .configure_bounds = toplevel_configure_bounds,
    .wm_capabilities = toplevel_wm_capabilities,
};

static void decoration_configure(void *data, struct zxdg_toplevel_decoration_v1 *deco,
                                 uint32_t mode) {
    (void)data;
    (void)deco;
    (void)mode;
}

static const struct zxdg_toplevel_decoration_v1_listener decoration_listener = {
    .configure = decoration_configure,
};

static void fractional_scale(void *data, struct wp_fractional_scale_v1 *frac,
                             uint32_t scale) {
    (void)frac;
    nw_window *w = data;
    if (scale == 0) return;
    w->fractional_scale = (int)scale;
    update_layout(w);
}

static const struct wp_fractional_scale_v1_listener fractional_listener = {
    .preferred_scale = fractional_scale,
};

/* ------------------------------------------------------------------ */
/* Registry                                                            */

static void registry_global(void *data, struct wl_registry *registry, uint32_t name,
                            const char *interface, uint32_t version) {
    nw_window *w = data;
    if (strcmp(interface, wl_compositor_interface.name) == 0) {
        w->compositor = wl_registry_bind(registry, name, &wl_compositor_interface,
                                         version < 4 ? version : 4);
    } else if (strcmp(interface, wl_shm_interface.name) == 0) {
        w->shm = wl_registry_bind(registry, name, &wl_shm_interface, 1);
    } else if (strcmp(interface, wl_seat_interface.name) == 0 && w->seat == NULL) {
        w->seat = wl_registry_bind(registry, name, &wl_seat_interface,
                                   version < 5 ? version : 5);
        wl_seat_add_listener(w->seat, &seat_listener, w);
    } else if (strcmp(interface, wl_output_interface.name) == 0 &&
               w->output_count < (int)(sizeof(w->outputs) / sizeof(w->outputs[0]))) {
        struct wl_output *out = wl_registry_bind(registry, name, &wl_output_interface,
                                                 version < 2 ? version : 2);
        w->outputs[w->output_count++] = out;
        wl_output_add_listener(out, &output_listener, w);
    } else if (strcmp(interface, xdg_wm_base_interface.name) == 0 && w->wm_base == NULL) {
        w->wm_base = wl_registry_bind(registry, name, &xdg_wm_base_interface,
                                      version < 3 ? version : 3);
        xdg_wm_base_add_listener(w->wm_base, &wm_listener, w);
    } else if (strcmp(interface, zxdg_decoration_manager_v1_interface.name) == 0) {
        w->decoration_manager = wl_registry_bind(
            registry, name, &zxdg_decoration_manager_v1_interface, 1);
    } else if (strcmp(interface, wp_viewporter_interface.name) == 0) {
        w->viewporter = wl_registry_bind(registry, name, &wp_viewporter_interface, 1);
    } else if (strcmp(interface, wp_fractional_scale_manager_v1_interface.name) == 0) {
        w->fractional_manager = wl_registry_bind(
            registry, name, &wp_fractional_scale_manager_v1_interface, 1);
    } else if (strcmp(interface, wp_cursor_shape_manager_v1_interface.name) == 0) {
        w->cursor_shape_manager = wl_registry_bind(
            registry, name, &wp_cursor_shape_manager_v1_interface, 1);
        create_cursor_shape(w);
    } else if (strcmp(interface, zwp_text_input_manager_v3_interface.name) == 0) {
        w->text_input_manager = wl_registry_bind(
            registry, name, &zwp_text_input_manager_v3_interface, 1);
        create_text_input(w);
    }
}

static void registry_global_remove(void *data, struct wl_registry *registry, uint32_t name) {
    (void)data;
    (void)registry;
    (void)name;
}

static const struct wl_registry_listener registry_listener = {
    .global = registry_global,
    .global_remove = registry_global_remove,
};

/* ------------------------------------------------------------------ */
/* Display pump                                                        */

static int pump(nw_window *w, int timeout_ms, int ignore_queue) {
    if (w->failed) return -1;
    struct wl_display *display = w->display;
    for (;;) {
        if (wl_display_dispatch_pending(display) < 0) {
            set_errorf(w, "wayland: dispatch failed: %s", strerror(errno));
            return -1;
        }
        if ((!ignore_queue && w->qhead != w->qtail) || timeout_ms == 0) break;
        if (wl_display_prepare_read(display) != 0) {
            if (errno == EAGAIN) continue;
            set_errorf(w, "wayland: prepare_read failed: %s", strerror(errno));
            return -1;
        }
        int flush = wl_display_flush(display);
        if (flush < 0 && errno != EAGAIN) {
            wl_display_cancel_read(display);
            set_errorf(w, "wayland: flush failed: %s", strerror(errno));
            return -1;
        }
        struct pollfd pfd;
        pfd.fd = wl_display_get_fd(display);
        pfd.events = POLLIN;
        pfd.revents = 0;
        if (flush < 0) pfd.events |= POLLOUT;
        int r;
        do {
            r = poll(&pfd, 1, timeout_ms);
        } while (r < 0 && errno == EINTR);
        if (r <= 0) {
            wl_display_cancel_read(display);
            break;
        }
        if ((pfd.revents & POLLOUT) && wl_display_flush(display) < 0 && errno != EAGAIN) {
            wl_display_cancel_read(display);
            set_errorf(w, "wayland: flush failed: %s", strerror(errno));
            return -1;
        }
        if (pfd.revents & POLLIN) {
            if (wl_display_read_events(display) < 0) {
                set_errorf(w, "wayland: read failed: %s", strerror(errno));
                return -1;
            }
            if (wl_display_dispatch_pending(display) < 0) {
                set_errorf(w, "wayland: dispatch failed: %s", strerror(errno));
                return -1;
            }
            break;
        }
        wl_display_cancel_read(display);
        break;
    }
    if (wl_display_flush(display) < 0 && errno != EAGAIN) {
        set_errorf(w, "wayland: flush failed: %s", strerror(errno));
        return -1;
    }
    return 0;
}

/* ------------------------------------------------------------------ */
/* Public API                                                          */

static void destroy_proxies(nw_window *w) {
    if (w->text_input != NULL) {
        zwp_text_input_v3_destroy(w->text_input);
        w->text_input = NULL;
    }
    if (w->cursor_shape_device != NULL) {
        wp_cursor_shape_device_v1_destroy(w->cursor_shape_device);
        w->cursor_shape_device = NULL;
    }
    if (w->keyboard != NULL) {
        wl_keyboard_release(w->keyboard);
        w->keyboard = NULL;
    }
    if (w->pointer != NULL) {
        wl_pointer_release(w->pointer);
        w->pointer = NULL;
    }
    if (w->seat != NULL) {
        wl_seat_release(w->seat);
        w->seat = NULL;
    }
    if (w->decoration != NULL) {
        zxdg_toplevel_decoration_v1_destroy(w->decoration);
        w->decoration = NULL;
    }
    if (w->fractional != NULL) {
        wp_fractional_scale_v1_destroy(w->fractional);
        w->fractional = NULL;
    }
    if (w->viewport != NULL) {
        wp_viewport_destroy(w->viewport);
        w->viewport = NULL;
    }
    if (w->egl_window != NULL) {
        wl_egl_window_destroy(w->egl_window);
        w->egl_window = NULL;
    }
    if (w->toplevel != NULL) {
        xdg_toplevel_destroy(w->toplevel);
        w->toplevel = NULL;
    }
    if (w->xdg_surface != NULL) {
        xdg_surface_destroy(w->xdg_surface);
        w->xdg_surface = NULL;
    }
    if (w->surface != NULL) {
        wl_surface_destroy(w->surface);
        w->surface = NULL;
    }
    for (int i = 0; i < w->output_count; i++) {
        if (w->outputs[i] != NULL) wl_output_destroy(w->outputs[i]);
        w->outputs[i] = NULL;
    }
    w->output_count = 0;
    if (w->text_input_manager != NULL) {
        zwp_text_input_manager_v3_destroy(w->text_input_manager);
        w->text_input_manager = NULL;
    }
    if (w->cursor_shape_manager != NULL) {
        wp_cursor_shape_manager_v1_destroy(w->cursor_shape_manager);
        w->cursor_shape_manager = NULL;
    }
    if (w->fractional_manager != NULL) {
        wp_fractional_scale_manager_v1_destroy(w->fractional_manager);
        w->fractional_manager = NULL;
    }
    if (w->viewporter != NULL) {
        wp_viewporter_destroy(w->viewporter);
        w->viewporter = NULL;
    }
    if (w->decoration_manager != NULL) {
        zxdg_decoration_manager_v1_destroy(w->decoration_manager);
        w->decoration_manager = NULL;
    }
    if (w->wm_base != NULL) {
        xdg_wm_base_destroy(w->wm_base);
        w->wm_base = NULL;
    }
    if (w->compositor != NULL) {
        wl_compositor_destroy(w->compositor);
        w->compositor = NULL;
    }
    if (w->shm != NULL) {
        wl_shm_destroy(w->shm);
        w->shm = NULL;
    }
    if (w->registry != NULL) {
        wl_registry_destroy(w->registry);
        w->registry = NULL;
    }
}

nw_window *nw_open(int width, int height, int min_width, int min_height, int resizable,
                   float scale_override, int vsync, char *err, int errcap) {
    (void)vsync;
    nw_window *w = calloc(1, sizeof(*w));
    if (w == NULL) {
        if (err) snprintf(err, (size_t)errcap, "wayland: out of memory");
        return NULL;
    }
    w->logical_w = width > 0 ? width : 960;
    w->logical_h = height > 0 ? height : 640;
    w->min_w = min_width;
    w->min_h = min_height;
    w->resizable = resizable;
    w->scale_override = scale_override;
    w->scale = 1;
    w->output_scale = 1;
    w->buffer_scale = 1;
    w->cursor = 0;
    w->display = wl_display_connect(NULL);
    if (w->display == NULL) {
        set_errorf(w, "wayland: cannot connect (WAYLAND_DISPLAY=%s)",
                   getenv("WAYLAND_DISPLAY") ? getenv("WAYLAND_DISPLAY") : "unset");
        goto fail;
    }
    w->xkb_context = xkb_context_new(0);
    w->registry = wl_display_get_registry(w->display);
    if (w->registry == NULL) {
        set_errorf(w, "wayland: get registry failed");
        goto fail;
    }
    wl_registry_add_listener(w->registry, &registry_listener, w);
    if (wl_display_roundtrip(w->display) < 0) {
        set_errorf(w, "wayland: registry roundtrip failed");
        goto fail;
    }
    if (wl_display_roundtrip(w->display) < 0) {
        set_errorf(w, "wayland: initial roundtrip failed");
        goto fail;
    }
    if (w->compositor == NULL || w->wm_base == NULL) {
        set_errorf(w, "wayland: compositor does not provide wl_compositor + xdg_wm_base");
        goto fail;
    }
    update_layout(w);
    w->surface = wl_compositor_create_surface(w->compositor);
    if (w->surface == NULL) {
        set_errorf(w, "wayland: create surface failed");
        goto fail;
    }
    if (w->viewporter != NULL) {
        w->viewport = wp_viewporter_get_viewport(w->viewporter, w->surface);
        update_layout(w);
    }
    if (w->fractional_manager != NULL) {
        w->fractional = wp_fractional_scale_manager_v1_get_fractional_scale(
            w->fractional_manager, w->surface);
        if (w->fractional != NULL) {
            wp_fractional_scale_v1_add_listener(w->fractional, &fractional_listener, w);
        }
    }
    w->xdg_surface = xdg_wm_base_get_xdg_surface(w->wm_base, w->surface);
    if (w->xdg_surface == NULL) {
        set_errorf(w, "wayland: create xdg_surface failed");
        goto fail;
    }
    xdg_surface_add_listener(w->xdg_surface, &surface_listener, w);
    w->toplevel = xdg_surface_get_toplevel(w->xdg_surface);
    if (w->toplevel == NULL) {
        set_errorf(w, "wayland: create xdg_toplevel failed");
        goto fail;
    }
    xdg_toplevel_add_listener(w->toplevel, &toplevel_listener, w);
    xdg_toplevel_set_app_id(w->toplevel, "nekoui");
    xdg_toplevel_set_title(w->toplevel, "NekoUI");
    if (w->min_w > 0 && w->min_h > 0) {
        xdg_toplevel_set_min_size(w->toplevel, w->min_w, w->min_h);
    }
    if (!w->resizable) {
        xdg_toplevel_set_max_size(w->toplevel, w->logical_w, w->logical_h);
    }
    if (w->decoration_manager != NULL) {
        w->decoration = zxdg_decoration_manager_v1_get_toplevel_decoration(
            w->decoration_manager, w->toplevel);
        if (w->decoration != NULL) {
            zxdg_toplevel_decoration_v1_add_listener(w->decoration, &decoration_listener, w);
            zxdg_toplevel_decoration_v1_set_mode(
                w->decoration, ZXDG_TOPLEVEL_DECORATION_V1_MODE_SERVER_SIDE);
        }
    }
    create_text_input(w);
    w->egl_window = wl_egl_window_create(w->surface, w->pixel_w, w->pixel_h);
    if (w->egl_window == NULL) {
        set_errorf(w, "wayland: create wl_egl_window failed");
        goto fail;
    }
    wl_surface_commit(w->surface);
    wl_display_flush(w->display);
    for (int i = 0; i < 300 && !w->configured && !w->failed; i++) {
        if (pump(w, 10, 1) < 0) break;
    }
    if (w->failed) goto fail;
    if (!w->configured) {
        set_errorf(w, "wayland: timed out waiting for the initial configure");
        goto fail;
    }
    return w;

fail:
    if (w->display != NULL) {
        destroy_proxies(w);
        if (w->cursor_surface != NULL) wl_surface_destroy(w->cursor_surface);
        if (w->cursor_theme != NULL) wl_cursor_theme_destroy(w->cursor_theme);
        if (w->xkb_state != NULL) xkb_state_unref(w->xkb_state);
        if (w->xkb_keymap != NULL) xkb_keymap_unref(w->xkb_keymap);
        if (w->xkb_context != NULL) xkb_context_unref(w->xkb_context);
        wl_display_disconnect(w->display);
    } else {
        if (w->xkb_state != NULL) xkb_state_unref(w->xkb_state);
        if (w->xkb_keymap != NULL) xkb_keymap_unref(w->xkb_keymap);
        if (w->xkb_context != NULL) xkb_context_unref(w->xkb_context);
    }
    if (err != NULL && errcap > 0) {
        snprintf(err, (size_t)errcap, "%s", w->err[0] ? w->err : "wayland: open failed");
    }
    free(w);
    return NULL;
}

void nw_close(nw_window *w) {
    if (w == NULL) return;
    destroy_proxies(w);
    if (w->cursor_surface != NULL) wl_surface_destroy(w->cursor_surface);
    if (w->cursor_theme != NULL) wl_cursor_theme_destroy(w->cursor_theme);
    if (w->xkb_state != NULL) xkb_state_unref(w->xkb_state);
    if (w->xkb_keymap != NULL) xkb_keymap_unref(w->xkb_keymap);
    if (w->xkb_context != NULL) xkb_context_unref(w->xkb_context);
    if (w->display != NULL) {
        wl_display_flush(w->display);
        wl_display_disconnect(w->display);
    }
    free(w);
}

void *nw_display(nw_window *w) { return w != NULL ? w->display : NULL; }
void *nw_egl_window(nw_window *w) { return w != NULL ? w->egl_window : NULL; }
int nw_width(nw_window *w) { return w != NULL ? w->logical_w : 0; }
int nw_height(nw_window *w) { return w != NULL ? w->logical_h : 0; }
int nw_pixel_width(nw_window *w) { return w != NULL ? w->pixel_w : 0; }
int nw_pixel_height(nw_window *w) { return w != NULL ? w->pixel_h : 0; }
float nw_scale(nw_window *w) { return w != NULL ? w->scale : 1; }
int nw_has_text_input(nw_window *w) { return w != NULL && w->text_input != NULL; }
int nw_has_cursor_shape(nw_window *w) {
    return w != NULL && w->cursor_shape_device != NULL;
}

void nw_show(nw_window *w) {
    if (w == NULL || w->failed) return;
    w->shown = 1;
    wl_surface_commit(w->surface);
    wl_display_flush(w->display);
}

void nw_set_title(nw_window *w, const char *title) {
    if (w == NULL || w->toplevel == NULL || title == NULL) return;
    xdg_toplevel_set_title(w->toplevel, title);
    wl_display_flush(w->display);
}

void nw_set_cursor(nw_window *w, int cursor) {
    if (w == NULL || w->failed) return;
    w->cursor = cursor;
    apply_cursor(w);
}

void nw_set_text_input(nw_window *w, int enabled, int x, int y, int width, int height,
                       int cursor, int anchor) {
    (void)cursor;
    (void)anchor;
    if (w == NULL || w->failed) return;
    if (width < 1) width = 1;
    if (height < 1) height = 1;
    if (w->text_input_wanted != enabled || w->ti_x != x || w->ti_y != y ||
        w->ti_w != width || w->ti_h != height) {
        w->text_input_wanted = enabled;
        w->ti_x = x;
        w->ti_y = y;
        w->ti_w = width;
        w->ti_h = height;
        w->text_input_dirty = 1;
    }
    sync_text_input(w);
}

int nw_poll(nw_window *w, int timeout_ms, nw_event *out, int max_out) {
    if (w == NULL) return -1;
    if (pump(w, timeout_ms, 0) < 0) return -1;
    int n = 0;
    while (w->qhead != w->qtail && n < max_out) {
        out[n++] = w->queue[w->qhead];
        w->qhead = (w->qhead + 1) % NW_QUEUE_CAP;
    }
    return n;
}

const char *nw_error(nw_window *w) {
    if (w == NULL) return "wayland: null window";
    return w->err[0] ? w->err : "wayland: unknown error";
}

int nw_flush(nw_window *w) {
    if (w == NULL || w->display == NULL) return -1;
    if (wl_display_flush(w->display) < 0 && errno != EAGAIN) return -1;
    return 0;
}
