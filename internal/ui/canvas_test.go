package ui

import (
	"testing"
)

func TestCanvasCapturedColor(t *testing.T) {
	c, _ := newTestCtx(t, 320, 240)
	c.begin()
	bg := Hex(0x123456)
	c.Canvas(40, 20, func(cv *Canvas) {
		cv.Rect(cv.Bounds, bg)
	})
	f := c.finish()
	for _, op := range f.Ops {
		if op.Kind == 0 && op.Color == bg {
			return
		}
	}
	t.Fatalf("captured color missing from ops: %+v", f.Ops)
}
