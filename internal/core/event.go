package core

// EventType is the normalized platform event kind.
type EventType uint8

const (
	EventNone EventType = iota
	EventKeyDown
	EventKeyUp
	EventText
	EventPreedit
	EventMouseMove
	EventMouseDown
	EventMouseUp
	EventScroll
	EventMouseEnter
	EventMouseLeave
	EventResize
	EventFocus
	EventExpose
	EventClose
	// EventDeleteSurrounding asks the focused editor to delete text around the
	// caret. Before/After are byte counts, as used by text-input-v3.
	EventDeleteSurrounding
)

// Event is a platform-neutral input/lifecycle event. It is intentionally a
// value type with a superset of fields: backends can append events without
// allocating and the UI layer can apply them in order.
type Event struct {
	Type EventType

	// Keyboard.
	Key    Key
	Mods   Modifiers
	Repeat bool
	Text   string // UTF-8 committed text (EventText) or preedit string
	Cursor int    // rune offset inside Text for EventPreedit
	Before int    // EventDeleteSurrounding: bytes before the caret
	After  int    // EventDeleteSurrounding: bytes after the caret

	// Pointer.
	Button MouseButton
	X, Y   float32 // logical window coordinates
	DX, DY float32 // scroll deltas

	// Resize. Width/Height are logical; PixelWidth/PixelHeight are physical.
	Width, Height           int
	PixelWidth, PixelHeight int
	Scale                   float32

	// Window focus.
	Focused bool
}

// Hit is a rectangle registered by a widget for input dispatch on the next
// frame. Rect is in the root coordinate space.
type Hit struct {
	ID        uint64
	Rect      Rect
	Clip      Rect
	Cursor    Cursor
	Focusable bool
	Disabled  bool
	// ScrollID, when non-zero, names the scroll container that should receive
	// wheel input for this hit. Scrollable parents annotate their descendants.
	ScrollID uint64
}

// TextInput is the IME state published by the UI for the current frame. Caret
// is in logical window coordinates; backends use it to place candidate windows
// and inline preedit. An empty Enabled means the focused widget does not accept
// text and the backend must release the input method.
type TextInput struct {
	Enabled bool
	Caret   Rect
	// Cursor and Anchor are rune offsets inside the editor text. They are
	// reserved for surrounding-text aware input methods.
	Cursor, Anchor int
}
