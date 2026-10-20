// Package nekoui is a frame-immediate GUI toolkit for Linux. A view is a plain
// Go function run once per frame; the framework retains hit geometry, focus,
// editors and scroll offsets between frames.
//
// The public API lives in this package. The frame engine and built-in widgets
// are implemented in internal/ui so the package root stays a thin, stable
// facade.
package nekoui

import (
	"image"

	"nekoui/internal/core"
	"nekoui/internal/ui"
)

// Ctx is the per-frame UI context passed to Run's view callback.
type Ctx = ui.Ctx

// App is the retained frame engine. Most programs only see it through Ctx.
type App = ui.Engine

// Theme is the visual vocabulary shared by built-in widgets.
type Theme = ui.Theme

// Option customizes a widget or container.
type Option = ui.Option

// Image is a decoded, GPU-ready texture.
type Image = ui.Image

// ImageFit controls how a source image fills its widget box.
type ImageFit = ui.ImageFit

// Canvas is a self-drawn interactive rectangle.
type Canvas = ui.Canvas

// Painter records primitives into the current display list.
type Painter = ui.Painter

// Response is the interaction state of a Canvas for the current frame.
type Response = ui.Response

// Input is the normalized, read-only view of the current frame's input.
type Input = ui.Input

// Core geometry and value types. The implementation lives in internal/core so
// backends and the renderer can share it without importing the widget layer.
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

// Image fit modes.
const (
	// ImageStretch fills the box, ignoring the source aspect ratio.
	ImageStretch = ui.ImageStretch
	// ImageContain letterboxes the source inside the box.
	ImageContain = ui.ImageContain
	// ImageCover fills the box and crops the overflow.
	ImageCover = ui.ImageCover
	// ImageNone draws the source at its natural size, centered and cropped.
	ImageNone = ui.ImageNone
)

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

// DefaultTheme is NekoUI's dark, low-contrast "ink and mint" theme.
func DefaultTheme() *Theme { return ui.DefaultTheme() }

// LightTheme is the paper counterpart of DefaultTheme.
func LightTheme() *Theme { return ui.LightTheme() }

// NewImage converts any image.Image into a reusable NekoUI texture.
func NewImage(src image.Image) *Image { return ui.NewImage(src) }

// LoadImage decodes a PNG, JPEG or GIF file into a reusable texture.
func LoadImage(path string) (*Image, error) { return ui.LoadImage(path) }

// ShortcutString formats a chord for labels.
func ShortcutString(mods Modifiers, key Key) string {
	return core.ShortcutString(mods, key)
}

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
