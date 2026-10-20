package ui

import "testing"

func TestPainterRoundRect(t *testing.T) {
	l := &layer{clip: Rect{W: 106.5, H: 38.5}}
	p := &Painter{layer: l, clip: l.clip}
	p.RoundRect(Rect{W: 106.5, H: 38.5}, 8, Hex(0x7FA6FF))
	if len(l.ops) != 1 {
		t.Fatalf("ops=%d", len(l.ops))
	}
	t.Logf("%+v", l.ops[0])
}
