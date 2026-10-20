package core

// KeyFromKeysym maps an X11/XKB keysym to a layout-independent Key. Printable
// characters are delivered as text events; this mapping only exists for editing
// commands and shortcuts.
func KeyFromKeysym(v uint32) Key {
	if v >= 'a' && v <= 'z' {
		return KeyA + Key(v-'a')
	}
	if v >= 'A' && v <= 'Z' {
		return KeyA + Key(v-'A')
	}
	if v >= '0' && v <= '9' {
		return Key0 + Key(v-'0')
	}
	switch v {
	case 0xff0d, 0xff8d: // Return, KP_Enter
		return KeyEnter
	case 0xff1b:
		return KeyEscape
	case 0xff09, 0xfe20: // Tab, ISO_Left_Tab
		return KeyTab
	case 0xff08:
		return KeyBackspace
	case 0xffff, 0xff9f: // Delete, KP_Delete
		return KeyDelete
	case 0xff63:
		return KeyInsert
	case 0xff50, 0xff95: // Home, KP_Home
		return KeyHome
	case 0xff57, 0xff9c: // End, KP_End
		return KeyEnd
	case 0xff55, 0xff9a: // Page_Up, KP_Page_Up
		return KeyPageUp
	case 0xff56, 0xff9b: // Page_Down, KP_Page_Down
		return KeyPageDown
	case 0xff51, 0xff96: // Left, KP_Left
		return KeyLeft
	case 0xff53, 0xff98: // Right, KP_Right
		return KeyRight
	case 0xff52, 0xff97: // Up, KP_Up
		return KeyUp
	case 0xff54, 0xff99: // Down, KP_Down
		return KeyDown
	case ' ':
		return KeySpace
	case '-', 0xffad: // minus, KP_Subtract
		return KeyMinus
	case '=', 0xffbd: // equal, KP_Equal
		return KeyEqual
	case '[':
		return KeyLeftBracket
	case ']':
		return KeyRightBracket
	case '\\':
		return KeyBackslash
	case ';':
		return KeySemicolon
	case '\'':
		return KeyApostrophe
	case ',':
		return KeyComma
	case '.':
		return KeyPeriod
	case '/':
		return KeySlash
	case '`':
		return KeyGrave
	case 0xffe1, 0xffe2: // Shift_L, Shift_R
		return KeyShift
	case 0xffe3, 0xffe4: // Control_L, Control_R
		return KeyCtrl
	case 0xffe9, 0xffea: // Alt_L, Alt_R
		return KeyAlt
	case 0xffeb, 0xffec: // Super_L, Super_R
		return KeySuper
	case 0xffe5:
		return KeyCapsLock
	}
	if v >= 0xffbe && v <= 0xffc9 { // F1..F12
		return KeyF1 + Key(v-0xffbe)
	}
	if v >= 0xffb0 && v <= 0xffb9 { // KP_0..KP_9
		return Key0 + Key(v-0xffb0)
	}
	return KeyNone
}
