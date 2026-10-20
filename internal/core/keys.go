package core

import (
	"strconv"
	"strings"
)

// Key is a layout-independent key identity. Printable characters are delivered
// through text events; Key exists for shortcuts and editing commands.
type Key uint16

const (
	KeyNone Key = iota

	KeyEnter
	KeyEscape
	KeyTab
	KeyBackspace
	KeyDelete
	KeyInsert
	KeySpace
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyLeft
	KeyRight
	KeyUp
	KeyDown

	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ

	Key0
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9

	KeyMinus
	KeyEqual
	KeyLeftBracket
	KeyRightBracket
	KeyBackslash
	KeySemicolon
	KeyApostrophe
	KeyComma
	KeyPeriod
	KeySlash
	KeyGrave

	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12

	KeyShift
	KeyCtrl
	KeyAlt
	KeySuper
	KeyCapsLock
)

var keyNames = map[Key]string{
	KeyEnter: "Enter", KeyEscape: "Esc", KeyTab: "Tab", KeyBackspace: "Backspace",
	KeyDelete: "Delete", KeyInsert: "Insert", KeySpace: "Space", KeyHome: "Home",
	KeyEnd: "End", KeyPageUp: "PgUp", KeyPageDown: "PgDn", KeyLeft: "←",
	KeyRight: "→", KeyUp: "↑", KeyDown: "↓",
	KeyMinus: "-", KeyEqual: "=", KeyLeftBracket: "[", KeyRightBracket: "]",
	KeyBackslash: "\\", KeySemicolon: ";", KeyApostrophe: "'", KeyComma: ",",
	KeyPeriod: ".", KeySlash: "/", KeyGrave: "`",
	KeyShift: "Shift", KeyCtrl: "Ctrl", KeyAlt: "Alt", KeySuper: "Super",
	KeyCapsLock: "Caps",
}

// String returns a short display name for the key.
func (k Key) String() string {
	if s, ok := keyNames[k]; ok {
		return s
	}
	switch {
	case k >= KeyA && k <= KeyZ:
		return string(rune('A' + (k - KeyA)))
	case k >= Key0 && k <= Key9:
		return string(rune('0' + (k - Key0)))
	case k >= KeyF1 && k <= KeyF12:
		return "F" + strconv.Itoa(1+int(k-KeyF1))
	}
	return "?"
}

// Modifiers is a bitmask of held modifier keys.
type Modifiers uint8

const (
	ModShift Modifiers = 1 << iota
	ModCtrl
	ModAlt
	ModSuper
)

// Has reports whether all bits in m are set.
func (m Modifiers) Has(want Modifiers) bool { return m&want == want }

// String renders modifiers in the conventional Ctrl+Shift+Alt order.
func (m Modifiers) String() string {
	var parts []string
	if m&ModCtrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if m&ModAlt != 0 {
		parts = append(parts, "Alt")
	}
	if m&ModShift != 0 {
		parts = append(parts, "Shift")
	}
	if m&ModSuper != 0 {
		parts = append(parts, "Super")
	}
	return strings.Join(parts, "+")
}

// ShortcutString renders modifiers and a key.
func ShortcutString(mods Modifiers, key Key) string {
	if mods == 0 {
		return key.String()
	}
	if key == KeyNone {
		return mods.String()
	}
	return mods.String() + "+" + key.String()
}

// MouseButton identifies a pointer button. Values match the common X11
// numbering (1 left, 2 middle, 3 right) to keep platform adapters trivial.
type MouseButton uint8

const (
	MouseNone   MouseButton = 0
	MouseLeft   MouseButton = 1
	MouseMiddle MouseButton = 2
	MouseRight  MouseButton = 3
	MouseX1     MouseButton = 8
	MouseX2     MouseButton = 9
)

// MouseButtonCount is the size of fixed button state arrays.
const MouseButtonCount = 10

// Cursor is a platform cursor shape requested by the UI.
type Cursor uint8

const (
	CursorArrow Cursor = iota
	CursorIBeam
	CursorHand
	CursorCrosshair
	CursorResizeH
	CursorResizeV
	CursorNotAllowed
)
