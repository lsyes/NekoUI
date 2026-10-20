# Wayland backend

This directory is a thin, self-contained Wayland client backend. It has three
layers:

- `shim.c` / `shim.h` — owns the libwayland objects, xdg-shell lifecycle, input
  devices, xkbcommon state, text-input-v3 and a small normalized event queue.
  Keeping the protocol vtables in one C translation unit makes the cgo boundary
  a flat `nw_*` API and keeps the Go side free of callback plumbing.
- `wayland.go` — adapts the shim to `platform.Window`, owns the EGL display,
  surface and GL context through `internal/gpu/egl`.
- `*-client-protocol.h` / `*-protocol.c` — generated Wayland protocol bindings.

## Generated protocols

The generated files are committed so a normal Go build only needs the runtime
libraries (`libwayland-client`, `libwayland-egl`, `libwayland-cursor`,
`libxkbcommon`) and the small set of Wayland headers, not `wayland-scanner` or
the `wayland-protocols` package. Regenerate them with:

```bash
XML=/path/to/wayland-protocols/protocols
gen() {
  wayland-scanner client-header "$1" "$2-client-protocol.h"
  wayland-scanner private-code "$1" "$2-protocol.c"
}
gen "$XML/stable/xdg-shell/xdg-shell.xml" xdg-shell
gen "$XML/stable/viewporter/viewporter.xml" viewporter
gen "$XML/stable/tablet/tablet-v2.xml" tablet-v2
gen "$XML/staging/cursor-shape/cursor-shape-v1.xml" cursor-shape-v1
gen "$XML/staging/fractional-scale/fractional-scale-v1.xml" fractional-scale-v1
gen "$XML/unstable/text-input/text-input-unstable-v3.xml" text-input-unstable-v3
gen "$XML/unstable/xdg-decoration/xdg-decoration-unstable-v1.xml" xdg-decoration-unstable-v1
```

`tablet-v2` is generated only because the cursor-shape interface table refers to
`zwp_tablet_tool_v2_interface`; NekoUI does not use tablets yet.

The protocol XML is MIT-licensed by the wayland-protocols project; the generated
files retain their copyright headers.

## Design notes

- **Scale**: logical size comes from `xdg_toplevel.configure`; physical pixels
  are `logical × scale`. `fractional-scale-v1` + `viewporter` are used when
  available, otherwise an integer `wl_surface.set_buffer_scale` is used.
- **Input**: pointer/keyboard events carry surface-local logical coordinates.
  `xkbcommon` turns keycodes into keysyms/UTF-8 and modifier indices.
- **Text input**: `text-input-v3` publishes preedit/commit/delete-surrounding
  events to the UI. The cursor rectangle is sent in surface-local logical
  coordinates, which is what compositors use to place the candidate window.
- **Pacing**: EGL swap interval (vsync by default). `wl_surface.frame` callbacks
  are not used explicitly yet.
- **Not implemented yet**: `wl_data_device` clipboard, drag-and-drop, client-side
  decorations fallback when the compositor rejects server-side decorations.
