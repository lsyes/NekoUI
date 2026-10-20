package ui

// Input is the normalized, read-only view of this frame's input. Key/text events
// are consumed internally by text fields and shortcuts; the summary below is
// primarily for application shortcuts and custom canvases.
type Input struct {
	Mouse       Point
	MousePrev   Point
	MouseInside bool
	Wheel       Point
	Mods        Modifiers

	keys      map[Key]bool
	buttons   [MouseButtonCount]bool
	downKeys  []Key
	upKeys    []Key
	downMouse [MouseButtonCount]bool
	upMouse   [MouseButtonCount]bool
}

// Down reports whether a key is currently held.
func (in *Input) Down(k Key) bool {
	if in == nil || in.keys == nil {
		return false
	}
	return in.keys[k]
}

// Pressed reports whether a key was pressed during this frame.
func (in *Input) Pressed(k Key) bool {
	if in == nil {
		return false
	}
	for _, v := range in.downKeys {
		if v == k {
			return true
		}
	}
	return false
}

// Released reports whether a key was released during this frame.
func (in *Input) Released(k Key) bool {
	if in == nil {
		return false
	}
	for _, v := range in.upKeys {
		if v == k {
			return true
		}
	}
	return false
}

// MouseDown reports whether a mouse button is held.
func (in *Input) MouseDown(b MouseButton) bool {
	if in == nil || int(b) >= len(in.buttons) {
		return false
	}
	return in.buttons[b]
}

// MousePressed reports whether a mouse button went down this frame.
func (in *Input) MousePressed(b MouseButton) bool {
	if in == nil || int(b) >= len(in.downMouse) {
		return false
	}
	return in.downMouse[b]
}

// MouseReleased reports whether a mouse button went up this frame.
func (in *Input) MouseReleased(b MouseButton) bool {
	if in == nil || int(b) >= len(in.upMouse) {
		return false
	}
	return in.upMouse[b]
}

func (in *Input) beginFrame() {
	in.Wheel = Point{}
	in.downKeys = in.downKeys[:0]
	in.upKeys = in.upKeys[:0]
	in.downMouse = [MouseButtonCount]bool{}
	in.upMouse = [MouseButtonCount]bool{}
	in.MousePrev = in.Mouse
}

func (in *Input) clearAll() {
	in.keys = make(map[Key]bool)
	in.buttons = [MouseButtonCount]bool{}
	in.MouseInside = false
	in.Mods = 0
}
