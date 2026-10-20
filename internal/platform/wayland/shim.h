#ifndef NEKOUI_WAYLAND_SHIM_H
#define NEKOUI_WAYLAND_SHIM_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct nw_window nw_window;

enum nw_event_type {
    NW_EVENT_NONE = 0,
    NW_EVENT_RESIZE,
    NW_EVENT_CLOSE,
    NW_EVENT_FOCUS,
    NW_EVENT_MOUSE_MOVE,
    NW_EVENT_MOUSE_DOWN,
    NW_EVENT_MOUSE_UP,
    NW_EVENT_SCROLL,
    NW_EVENT_MOUSE_ENTER,
    NW_EVENT_MOUSE_LEAVE,
    NW_EVENT_KEY_DOWN,
    NW_EVENT_KEY_UP,
    NW_EVENT_TEXT,
    NW_EVENT_PREEDIT,
    NW_EVENT_DELETE_SURROUNDING
};

enum nw_modifier {
    NW_MOD_SHIFT = 1 << 0,
    NW_MOD_CTRL = 1 << 1,
    NW_MOD_ALT = 1 << 2,
    NW_MOD_SUPER = 1 << 3
};

typedef struct nw_event {
    int type;
    uint32_t key; /* XKB keysym */
    uint32_t mods; /* NW_MOD_* */
    int repeat;
    char text[512]; /* UTF-8 commit or preedit */
    int cursor;     /* rune offset inside preedit text */
    int before;     /* delete_surrounding_text byte counts */
    int after;
    int button;
    float x, y;
    float dx, dy;
    int width, height;
    int pixel_width, pixel_height;
    float scale;
    int focused;
} nw_event;

nw_window *nw_open(int width, int height, int min_width, int min_height,
                   int resizable, float scale_override, int vsync,
                   char *err, int errcap);
void nw_close(nw_window *w);

void *nw_display(nw_window *w);
void *nw_egl_window(nw_window *w);
int nw_width(nw_window *w);
int nw_height(nw_window *w);
int nw_pixel_width(nw_window *w);
int nw_pixel_height(nw_window *w);
float nw_scale(nw_window *w);
int nw_has_text_input(nw_window *w);
int nw_has_cursor_shape(nw_window *w);

void nw_show(nw_window *w);
void nw_set_title(nw_window *w, const char *title);
void nw_set_cursor(nw_window *w, int cursor);
void nw_set_text_input(nw_window *w, int enabled,
                       int x, int y, int width, int height,
                       int cursor, int anchor);

/* Fills up to max_out queued events. timeout_ms < 0 blocks forever, 0 polls.
 * Returns the number of events, or -1 on a fatal protocol error. */
int nw_poll(nw_window *w, int timeout_ms, nw_event *out, int max_out);
int nw_flush(nw_window *w);
const char *nw_error(nw_window *w);

#ifdef __cplusplus
}
#endif

#endif
