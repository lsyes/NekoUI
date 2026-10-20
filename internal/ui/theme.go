package ui

import "nekoui/internal/core"

// Theme is the visual vocabulary shared by built-in widgets. Applications may
// keep a pointer to a Theme and mutate it, or swap it at runtime with Ctx.UseTheme.
type Theme struct {
	Bg           core.Color
	Panel        core.Color
	PanelAlt     core.Color
	PanelHover   core.Color
	Border       core.Color
	BorderStrong core.Color

	Text      core.Color
	TextMuted core.Color
	TextFaint core.Color

	Accent      core.Color
	AccentHover core.Color
	OnAccent    core.Color
	Danger      core.Color
	OnDanger    core.Color

	// Geometry and type scale, in logical pixels.
	Radius    float32
	FontSize  float32
	SmallSize float32
	TitleSize float32
	Pad       float32
}

// DefaultTheme is NekoUI's dark, low-contrast "ink and mint" theme.
func DefaultTheme() *Theme {
	t := &Theme{
		Bg:           core.Hex(0x0B0D13),
		Panel:        core.Hex(0x12151E),
		PanelAlt:     core.Hex(0x171B26),
		PanelHover:   core.Hex(0x1E2432),
		Border:       core.Hex(0x242B3A),
		BorderStrong: core.Hex(0x354055),
		Text:         core.Hex(0xE9EDF6),
		TextMuted:    core.Hex(0x99A3B7),
		TextFaint:    core.Hex(0x616C80),
		Accent:       core.Hex(0x7FA6FF),
		AccentHover:  core.Hex(0x9BBCFF),
		OnAccent:     core.Hex(0x0A1020),
		Danger:       core.Hex(0xFF7A8A),
		OnDanger:     core.Hex(0x1A0810),
		Radius:       14,
		FontSize:     14,
		SmallSize:    12,
		TitleSize:    24,
		Pad:          18,
	}
	return t
}

// LightTheme is the paper counterpart of DefaultTheme.
func LightTheme() *Theme {
	t := &Theme{
		Bg:           core.Hex(0xF3F5FA),
		Panel:        core.Hex(0xFFFFFF),
		PanelAlt:     core.Hex(0xF7F8FC),
		PanelHover:   core.Hex(0xEDF0F7),
		Border:       core.Hex(0xE0E5EF),
		BorderStrong: core.Hex(0xC8D0DF),
		Text:         core.Hex(0x141A26),
		TextMuted:    core.Hex(0x596377),
		TextFaint:    core.Hex(0x8A94A8),
		Accent:       core.Hex(0x3D6FE8),
		AccentHover:  core.Hex(0x2F5CD0),
		OnAccent:     core.Hex(0xFFFFFF),
		Danger:       core.Hex(0xD9485A),
		OnDanger:     core.Hex(0xFFFFFF),
		Radius:       14,
		FontSize:     14,
		SmallSize:    12,
		TitleSize:    24,
		Pad:          18,
	}
	return t
}

// Clone returns an independent copy of the theme.
func (t *Theme) Clone() *Theme {
	if t == nil {
		return nil
	}
	cp := *t
	return &cp
}

// SetAccent updates Accent and derives a readable hover/on-accent pair.
func (t *Theme) SetAccent(c core.Color) {
	t.Accent = c
	if luminance(c) > 0.62 {
		t.AccentHover = c.Mix(core.White, 0.18)
		t.OnAccent = core.Hex(0x11151E)
	} else {
		t.AccentHover = c.Mix(core.White, 0.22)
		t.OnAccent = core.White
	}
}

func luminance(c core.Color) float32 {
	return (0.2126*float32(c.R) + 0.7152*float32(c.G) + 0.0722*float32(c.B)) / 255
}

// Public type aliases. The implementation lives in internal/core so backends and
// the renderer can share it without importing the widget layer.

type Point = core.Point
type Size = core.Size
type Rect = core.Rect
type Insets = core.Insets
type Color = core.Color
type Key = core.Key
type Modifiers = core.Modifiers
type MouseButton = core.MouseButton
type Cursor = core.Cursor
type Align = core.Align

// MouseButtonCount is the size of fixed mouse state arrays.
const MouseButtonCount = core.MouseButtonCount

// Colors.
var (
	Transparent = core.Transparent
	White       = core.White
	Black       = core.Black
)

// RGBA constructs a color.
func RGBA(r, g, b, a uint8) Color { return core.RGBA(r, g, b, a) }

// Hex constructs an opaque color from 0xRRGGBB.
func Hex(v uint32) Color { return core.Hex(v) }

// HexA constructs a color from 0xRRGGBBAA.
func HexA(v uint32) Color { return core.HexA(v) }

// Insets helpers.
func All(v float32) Insets   { return core.All(v) }
func XY(h, v float32) Insets { return core.XY(h, v) }

// Alignment constants.
const (
	Start   = core.AlignStart
	Center  = core.AlignCenter
	End     = core.AlignEnd
	Stretch = core.AlignStretch
)

// Cursor constants.
const (
	CursorArrow      = core.CursorArrow
	CursorIBeam      = core.CursorIBeam
	CursorHand       = core.CursorHand
	CursorCrosshair  = core.CursorCrosshair
	CursorResizeH    = core.CursorResizeH
	CursorResizeV    = core.CursorResizeV
	CursorNotAllowed = core.CursorNotAllowed
)

// Mouse buttons.
const (
	MouseNone   = core.MouseNone
	MouseLeft   = core.MouseLeft
	MouseMiddle = core.MouseMiddle
	MouseRight  = core.MouseRight
)

// Modifier bits.
const (
	ModShift = core.ModShift
	ModCtrl  = core.ModCtrl
	ModAlt   = core.ModAlt
	ModSuper = core.ModSuper
)

// Keys. These aliases keep the public API descriptive without forcing users to
// import an internal package.
const (
	KeyNone         = core.KeyNone
	KeyEnter        = core.KeyEnter
	KeyEscape       = core.KeyEscape
	KeyTab          = core.KeyTab
	KeyBackspace    = core.KeyBackspace
	KeyDelete       = core.KeyDelete
	KeyInsert       = core.KeyInsert
	KeySpace        = core.KeySpace
	KeyHome         = core.KeyHome
	KeyEnd          = core.KeyEnd
	KeyPageUp       = core.KeyPageUp
	KeyPageDown     = core.KeyPageDown
	KeyLeft         = core.KeyLeft
	KeyRight        = core.KeyRight
	KeyUp           = core.KeyUp
	KeyDown         = core.KeyDown
	KeyMinus        = core.KeyMinus
	KeyEqual        = core.KeyEqual
	KeyLeftBracket  = core.KeyLeftBracket
	KeyRightBracket = core.KeyRightBracket
	KeyBackslash    = core.KeyBackslash
	KeySemicolon    = core.KeySemicolon
	KeyApostrophe   = core.KeyApostrophe
	KeyComma        = core.KeyComma
	KeyPeriod       = core.KeyPeriod
	KeySlash        = core.KeySlash
	KeyGrave        = core.KeyGrave
)

// Letters and digits are declared explicitly for discoverability.
const (
	KeyA = core.KeyA
	KeyB = core.KeyB
	KeyC = core.KeyC
	KeyD = core.KeyD
	KeyE = core.KeyE
	KeyF = core.KeyF
	KeyG = core.KeyG
	KeyH = core.KeyH
	KeyI = core.KeyI
	KeyJ = core.KeyJ
	KeyK = core.KeyK
	KeyL = core.KeyL
	KeyM = core.KeyM
	KeyN = core.KeyN
	KeyO = core.KeyO
	KeyP = core.KeyP
	KeyQ = core.KeyQ
	KeyR = core.KeyR
	KeyS = core.KeyS
	KeyT = core.KeyT
	KeyU = core.KeyU
	KeyV = core.KeyV
	KeyW = core.KeyW
	KeyX = core.KeyX
	KeyY = core.KeyY
	KeyZ = core.KeyZ

	Key0 = core.Key0
	Key1 = core.Key1
	Key2 = core.Key2
	Key3 = core.Key3
	Key4 = core.Key4
	Key5 = core.Key5
	Key6 = core.Key6
	Key7 = core.Key7
	Key8 = core.Key8
	Key9 = core.Key9
)

// Function keys.
const (
	KeyF1  = core.KeyF1
	KeyF2  = core.KeyF2
	KeyF3  = core.KeyF3
	KeyF4  = core.KeyF4
	KeyF5  = core.KeyF5
	KeyF6  = core.KeyF6
	KeyF7  = core.KeyF7
	KeyF8  = core.KeyF8
	KeyF9  = core.KeyF9
	KeyF10 = core.KeyF10
	KeyF11 = core.KeyF11
	KeyF12 = core.KeyF12
)
