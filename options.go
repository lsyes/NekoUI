package nekoui

import "nekoui/internal/ui"

// ID gives a widget a stable identity independent of its source location. Use it
// for widgets rendered in loops or helper functions.
func ID(name string) Option { return ui.ID(name) }

// Width fixes the outer width of a widget or container.
func Width(w float32) Option { return ui.Width(w) }

// Height fixes the outer height of a widget or container.
func Height(h float32) Option { return ui.Height(h) }

// SizeOf fixes both dimensions.
func SizeOf(w, h float32) Option { return ui.SizeOf(w, h) }

// FillWidth makes a container expand to the available width.
func FillWidth() Option { return ui.FillWidth() }

// FillHeight makes a container expand to the available height.
func FillHeight() Option { return ui.FillHeight() }

// Fill expands in both axes.
func Fill() Option { return ui.Fill() }

// Padding sets inner spacing on all sides.
func Padding(v float32) Option { return ui.Padding(v) }

// PaddingXY sets horizontal and vertical padding.
func PaddingXY(h, v float32) Option { return ui.PaddingXY(h, v) }

// PaddingInsets sets each side independently.
func PaddingInsets(v Insets) Option { return ui.PaddingInsets(v) }

// Gap sets spacing between a container's children.
func Gap(v float32) Option { return ui.Gap(v) }

// AlignTo sets cross-axis alignment inside a container.
func AlignTo(a Align) Option { return ui.AlignTo(a) }

// Clip clips children to the container bounds.
func Clip() Option { return ui.Clip() }

// Radius overrides the corner radius.
func Radius(v float32) Option { return ui.Radius(v) }

// Background sets a fill color.
func Background(c Color) Option { return ui.Background(c) }

// Border sets a border color.
func Border(c Color) Option { return ui.Border(c) }

// BorderWidth sets the border thickness.
func BorderWidth(w float32) Option { return ui.BorderWidth(w) }

// TextColor overrides the foreground color.
func TextColor(c Color) Option { return ui.TextColor(c) }

// TextSize overrides the font size in logical pixels.
func TextSize(px float32) Option { return ui.TextSize(px) }

// Placeholder sets single-line input placeholder text.
func Placeholder(s string) Option { return ui.Placeholder(s) }

// Primary selects the accent-filled button style.
func Primary() Option { return ui.Primary() }

// Ghost selects the borderless button style.
func Ghost() Option { return ui.Ghost() }

// Danger selects the destructive button style.
func Danger() Option { return ui.Danger() }

// Wrap breaks text at the available width instead of clipping it. Combined with
// FillWidth it produces paragraph-style labels.
func Wrap() Option { return ui.Wrap() }

// Disabled disables interaction and dims the widget.
func Disabled() Option { return ui.Disabled() }

// Focusable marks a custom canvas as a keyboard focus target.
func Focusable() Option { return ui.Focusable() }

// Autofocus focuses a text input when no other widget owns focus.
func Autofocus() Option { return ui.Autofocus() }

// Pointer overrides the pointer cursor while hovering.
func Pointer(c Cursor) Option { return ui.Pointer(c) }

// OnSubmit is called when Enter is pressed in a single-line text input.
func OnSubmit(fn func(string)) Option { return ui.OnSubmit(fn) }

// Fit selects how an Image fills its box.
func Fit(mode ImageFit) Option { return ui.Fit(mode) }

// Tint multiplies an image's colors (white leaves it unchanged).
func Tint(c Color) Option { return ui.Tint(c) }

// NoScrollbar hides a Scrollable's scrollbar while keeping wheel/drag input.
func NoScrollbar() Option { return ui.NoScrollbar() }

// Step quantizes Slider values to a fixed increment.
func Step(v float32) Option { return ui.Step(v) }
