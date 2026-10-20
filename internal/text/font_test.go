package text

import (
	"testing"

	"nekoui/internal/core"
)

func TestShapeLatinAndCJK(t *testing.T) {
	e, err := Load(LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	glyphs := e.Shape("NekoUI 你好", 32, 0, 0, core.White)
	if len(glyphs) != 8 {
		t.Fatalf("got %d glyphs, want 8", len(glyphs))
	}
	for i, g := range glyphs {
		if g.W <= 0 || g.H <= 0 || g.W > 200 || g.H > 200 {
			t.Fatalf("glyph %d has implausible size %+v", i, g)
		}
		if g.U1 <= g.U0 || g.V1 <= g.V0 {
			t.Fatalf("glyph %d has invalid UVs %+v", i, g)
		}
	}
	if len(e.Atlas().Dirty()) == 0 {
		t.Fatal("atlas has no dirty rects after shaping")
	}
	if e.Measure("你好", 32).W <= 0 {
		t.Fatal("measure returned non-positive width")
	}
}
