package ui

import (
	"testing"

	"nekoui/internal/render"
)

func TestBuildButtonNoGL(t *testing.T) {
	c, _ := newTestCtx(t, 640, 400)
	c.begin()
	c.Background(Hex(0x0B0D13))
	c.Column(func() {
		c.Button("按钮 Button", Primary())
	}, FillWidth(), FillHeight(), Padding(32), Gap(16))
	f := c.finish()
	found := false
	for _, op := range f.Ops {
		if op.Kind == 0 && op.Color == Hex(0x7FA6FF) && op.Rect.W > 100 {
			found = true
		}
	}
	if !found {
		t.Fatalf("primary button background missing from frame: %+v", f.Ops)
	}
}

// TestWidgetColorsIntact checks that themed fills survive the paint closures.
// On Go 1.27/loong64 with -race, small Color aggregates passed through inlined
// paint calls have been observed to lose channels; this is a regression guard
// for the Button/Radio workarounds (see docs/DESIGN.md).
func TestWidgetColorsIntact(t *testing.T) {
	th := DefaultTheme()
	tests := []struct {
		name  string
		build func(c *Ctx)
		want  []Color
	}{
		{
			name: "checkbox",
			build: func(c *Ctx) {
				v := true
				c.Checkbox(&v, "check")
			},
			want: []Color{th.Accent, th.OnAccent},
		},
		{
			name: "radio",
			build: func(c *Ctx) {
				n := 0
				c.Radio(&n, 0, "radio")
			},
			want: []Color{th.PanelAlt, th.Accent},
		},
		{
			name:  "slider",
			build: func(c *Ctx) { v := float32(50); c.Slider(&v, 0, 100, Width(200)) },
			want:  []Color{th.BorderStrong, th.Accent},
		},
		{
			name:  "progress",
			build: func(c *Ctx) { c.ProgressBar(0.5) },
			want:  []Color{th.PanelAlt, th.Accent},
		},
		{
			name:  "toggle",
			build: func(c *Ctx) { v := true; c.Toggle(&v, "toggle") },
			want:  []Color{th.Accent, th.Panel},
		},
		{
			name:  "input",
			build: func(c *Ctx) { c.InputText(new(string)) },
			want:  []Color{th.PanelAlt},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestCtx(t, 300, 200)
			c.begin()
			tc.build(c)
			f := c.finish()
			seen := map[Color]bool{}
			for _, op := range f.Ops {
				if op.Kind == render.RectFill || op.Kind == render.RectStroke {
					seen[op.Color] = true
				}
				for _, tr := range op.Tris {
					seen[tr.A.C] = true
					seen[tr.B.C] = true
					seen[tr.C.C] = true
				}
			}
			for _, want := range tc.want {
				if !seen[want] {
					t.Fatalf("%s: expected color %+v missing from ops", tc.name, want)
				}
			}
		})
	}
}
