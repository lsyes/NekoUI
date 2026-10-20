package ui

// Option customizes a widget or a container. Options are applied in order, so
// later options win (for example Width(200) after FillWidth()).
type Option func(*options)

type variant uint8

const (
	variantSecondary variant = iota
	variantPrimary
	variantGhost
	variantDanger
)

type options struct {
	id string

	width, height float32
	fillW, fillH  bool

	pad       Insets
	padSet    bool
	gap       float32
	align     Align
	clip      bool
	radius    float32
	bg        Color
	bgSet     bool
	border    Color
	borderSet bool
	borderW   float32

	fg       Color
	fgSet    bool
	fontSize float32
	variant  variant

	placeholder string
	disabled    bool
	wrap        bool
	cursor      Cursor
	cursorSet   bool
	focusable   bool
	autofocus   bool

	fit     ImageFit
	fitSet  bool
	tint    Color
	tintSet bool

	hideScrollbar bool

	step float32

	onSubmit func(string)
}

func defaultOptions(o *options) {
	*o = options{
		align:   Start,
		radius:  -1,
		variant: variantSecondary,
		borderW: 1,
	}
}

// takeOptions hands out a per-frame options slot. The slab is reset in begin,
// and every slot is used by exactly one widget call, so options never
// allocate on the heap and closures can safely capture o for the duration of
// the synchronous paint callback.
func (c *Ctx) takeOptions() *options {
	if c.optN == len(c.optSlab) {
		c.optSlab = append(c.optSlab, options{})
	}
	o := &c.optSlab[c.optN]
	c.optN++
	defaultOptions(o)
	return o
}

func (c *Ctx) options(opts []Option) *options { return c.applyOptions(c.takeOptions(), opts) }

func (c *Ctx) applyOptions(o *options, opts []Option) *options {
	for _, f := range opts {
		if f != nil {
			f(o)
		}
	}
	return o
}

// ID gives a widget a stable identity independent of its source location. Use it
// for widgets rendered in loops or helper functions.
func ID(name string) Option { return func(o *options) { o.id = name } }

// Width fixes the outer width of a widget or container.
func Width(w float32) Option { return func(o *options) { o.width = w } }

// Height fixes the outer height of a widget or container.
func Height(h float32) Option { return func(o *options) { o.height = h } }

// Size fixes both dimensions.
func SizeOf(w, h float32) Option { return func(o *options) { o.width, o.height = w, h } }

// FillWidth makes a container expand to the available width.
func FillWidth() Option { return func(o *options) { o.fillW = true } }

// FillHeight makes a container expand to the available height.
func FillHeight() Option { return func(o *options) { o.fillH = true } }

// Fill expands in both axes.
func Fill() Option { return func(o *options) { o.fillW, o.fillH = true, true } }

// Padding sets inner spacing on all sides.
func Padding(v float32) Option { return func(o *options) { o.pad = All(v); o.padSet = true } }

// PaddingXY sets horizontal and vertical padding.
func PaddingXY(h, v float32) Option { return func(o *options) { o.pad = XY(h, v); o.padSet = true } }

// PaddingInsets sets each side independently.
func PaddingInsets(v Insets) Option { return func(o *options) { o.pad = v; o.padSet = true } }

// Gap sets spacing between a container's children.
func Gap(v float32) Option { return func(o *options) { o.gap = v } }

// Align sets cross-axis alignment inside a container.
func AlignTo(a Align) Option { return func(o *options) { o.align = a } }

// Clip clips children to the container bounds.
func Clip() Option { return func(o *options) { o.clip = true } }

// Radius overrides the corner radius.
func Radius(v float32) Option { return func(o *options) { o.radius = v } }

// Background sets a fill color.
func Background(c Color) Option { return func(o *options) { o.bg = c; o.bgSet = true } }

// Border sets a border color.
func Border(c Color) Option { return func(o *options) { o.border = c; o.borderSet = true } }

// BorderWidth sets the border thickness.
func BorderWidth(w float32) Option { return func(o *options) { o.borderW = w } }

// TextColor overrides the foreground color.
func TextColor(c Color) Option { return func(o *options) { o.fg = c; o.fgSet = true } }

// TextSize overrides the font size in logical pixels.
func TextSize(px float32) Option { return func(o *options) { o.fontSize = px } }

// Placeholder sets single-line input placeholder text.
func Placeholder(s string) Option { return func(o *options) { o.placeholder = s } }

// Primary selects the accent-filled button style.
func Primary() Option { return func(o *options) { o.variant = variantPrimary } }

// Ghost selects the borderless button style.
func Ghost() Option { return func(o *options) { o.variant = variantGhost } }

// Danger selects the destructive button style.
func Danger() Option { return func(o *options) { o.variant = variantDanger } }

// Wrap breaks text at the available width instead of clipping it. Combined with
// FillWidth it produces paragraph-style labels.
func Wrap() Option { return func(o *options) { o.wrap = true } }

// Disabled disables interaction and dims the widget.
func Disabled() Option { return func(o *options) { o.disabled = true } }

// Focusable marks a custom canvas as a keyboard focus target.
func Focusable() Option { return func(o *options) { o.focusable = true } }

// Autofocus focuses a text input when no other widget owns focus. It is useful
// for single-input tools and input-method diagnostics.
func Autofocus() Option { return func(o *options) { o.autofocus = true } }

// Pointer overrides the pointer cursor while hovering.
func Pointer(c Cursor) Option { return func(o *options) { o.cursor = c; o.cursorSet = true } }

// OnSubmit is called when Enter is pressed in a single-line text input.
func OnSubmit(fn func(string)) Option { return func(o *options) { o.onSubmit = fn } }

// Fit selects how an Image fills its box.
func Fit(mode ImageFit) Option { return func(o *options) { o.fit = mode; o.fitSet = true } }

// Tint multiplies an image's colors (white leaves it unchanged).
func Tint(c Color) Option { return func(o *options) { o.tint = c; o.tintSet = true } }

// NoScrollbar hides a Scrollable's scrollbar while keeping wheel/drag input.
func NoScrollbar() Option { return func(o *options) { o.hideScrollbar = true } }

// Step quantizes Slider values to a fixed increment.
func Step(v float32) Option { return func(o *options) { o.step = v } }
