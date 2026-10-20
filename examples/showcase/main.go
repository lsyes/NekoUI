// NekoUI showcase: a single-window playground for the immediate-mode API,
// custom painting, text input and shortcuts.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	neko "nekoui"
)

// accentPalette is drawn in the sidebar and used by all three shortcuts.
var accentPalette = []neko.Color{
	neko.Hex(0x7FA6FF), // periwinkle
	neko.Hex(0x6EE7B7), // mint
	neko.Hex(0xFF9EBC), // pink
	neko.Hex(0xFFC66D), // amber
}

type state struct {
	dark   bool
	anim   bool
	count  int
	name   string
	accent int

	pokes   int
	pokeAt  time.Duration
	last    string
	history []int
}

func main() {
	var (
		screenshot = flag.String("screenshot", "", "render a PNG and exit")
		scale      = flag.Float64("scale", 0, "UI scale override")
		backend    = flag.String("backend", "", "x11, wayland or headless")
		headless   = flag.Bool("headless", false, "use the surfaceless backend")
	)
	flag.Parse()

	dark := neko.DefaultTheme()
	light := neko.LightTheme()
	for _, th := range []*neko.Theme{dark, light} {
		th.SetAccent(accentPalette[0])
	}
	st := &state{
		dark: true, anim: true, accent: 0,
		count:   7,
		history: []int{1, 2, 2, 3, 3, 4, 3, 3, 4, 4, 5, 4, 5, 5, 6, 6, 5, 6, 7, 7, 6, 7, 7, 7},
	}

	cfg := neko.Config{
		Title:            "NekoUI — 把界面，写成 Go 函数",
		Width:            1180,
		Height:           840,
		MinWidth:         980,
		MinHeight:        700,
		Resizable:        true,
		Backend:          *backend,
		Scale:            float32(*scale),
		Theme:            dark,
		Headless:         *headless,
		Screenshot:       *screenshot,
		ScreenshotFrames: 3,
	}
	if *screenshot != "" && cfg.Scale == 0 {
		cfg.Scale = 2
	}

	err := neko.Run(cfg, func(c *neko.Ctx) {
		// Global shortcuts run before widgets so a theme/accent change is
		// visible in the same frame.
		applyShortcuts(c, st, dark, light)
		if st.dark {
			c.UseTheme(dark)
		} else {
			c.UseTheme(light)
		}
		if st.anim {
			c.Animate()
		}
		c.SetTitle("NekoUI — 把界面，写成 Go 函数")
		th := c.Theme

		c.Background(th.Bg)
		c.Backdrop(func(p *neko.Painter) {
			p.Circle(neko.Point{X: float32(c.W) * 0.83, Y: -30}, 330, c.Theme.Accent.WithAlpha(18))
			p.Circle(neko.Point{X: float32(c.W) * 0.10, Y: float32(c.H) * 1.02}, 260, c.Theme.Accent.WithAlpha(10))
		})

		c.Column(func() {
			header(c, st)
			c.Flex(1, func() {
				c.Row(func() {
					sidebar(c, st, dark, light)
					c.Flex(1, func() {
						mainContent(c, st)
					})
				}, neko.FillWidth(), neko.FillHeight(), neko.Gap(16))
			})
			footer(c)
		}, neko.Width(float32(c.W)), neko.Height(float32(c.H)), neko.Padding(22), neko.Gap(14))
	})
	if err != nil {
		log.Fatal(err)
	}
}

func applyShortcuts(c *neko.Ctx, st *state, dark, light *neko.Theme) {
	if c.Shortcut(neko.ModCtrl, neko.KeyQ) {
		c.Quit()
	}
	if c.Shortcut(neko.ModCtrl, neko.KeyT) {
		st.dark = !st.dark
		st.last = "Ctrl+T 切换主题"
	}
	if c.Shortcut(0, neko.KeyF1) {
		st.anim = !st.anim
		if st.anim {
			st.last = "F1 恢复动效"
		} else {
			st.last = "F1 暂停动效"
		}
	}
	for i := range accentPalette {
		var key neko.Key
		switch i {
		case 0:
			key = neko.Key1
		case 1:
			key = neko.Key2
		case 2:
			key = neko.Key3
		default:
			key = neko.Key4
		}
		if c.Shortcut(neko.ModCtrl, key) {
			st.accent = i
			dark.SetAccent(accentPalette[i])
			light.SetAccent(accentPalette[i])
			st.last = fmt.Sprintf("Ctrl+%d 切换强调色", i+1)
		}
	}
}

func header(c *neko.Ctx, st *state) {
	th := c.Theme
	c.Row(func() {
		c.Canvas(38, 38, func(cv *neko.Canvas) {
			drawLogo(cv)
		})
		c.Column(func() {
			c.Label("NekoUI", neko.TextSize(17), neko.TextColor(th.Text))
			c.Label("frame-immediate gui · go", neko.TextSize(10.5), neko.TextColor(th.TextFaint))
		}, neko.Gap(1))
		c.Label("0.1 dev", neko.TextSize(10), neko.TextColor(th.TextMuted),
			neko.Background(th.PanelAlt), neko.Border(th.Border), neko.Radius(6),
			neko.PaddingXY(7, 3))
		c.Spacer()
		chip(c, c.Backend, true)
		chip(c, fmt.Sprintf("%.0f FPS", c.FPS), false)
		c.Label(st.lastHint(), neko.TextSize(10.5), neko.TextColor(th.TextFaint))
	}, neko.Height(48), neko.FillWidth(), neko.Gap(12), neko.AlignTo(neko.Center))
}

func (s *state) lastHint() string {
	if s.last == "" {
		return "Ctrl+T 主题 · F1 动效"
	}
	return s.last
}

func chip(c *neko.Ctx, text string, accent bool) {
	th := c.Theme
	c.Row(func() {
		fg := th.TextMuted
		if accent {
			fg = th.Accent
		}
		c.Canvas(7, 7, func(cv *neko.Canvas) {
			cv.Circle(neko.Point{X: 3.5, Y: 3.5}, 3.5, c.Theme.Accent)
		})
		c.Label(text, neko.TextSize(10.5), neko.TextColor(fg))
	}, neko.Background(th.PanelAlt), neko.Border(th.Border), neko.Radius(8),
		neko.PaddingXY(9, 5), neko.Gap(6), neko.AlignTo(neko.Center))
}

func footer(c *neko.Ctx) {
	th := c.Theme
	c.Row(func() {
		c.Label("NekoUI · 用 Go 描述，用 EGL/OpenGL 绘制", neko.TextSize(10.5), neko.TextColor(th.TextFaint))
		c.Spacer()
		c.Label("Ctrl+1..4 强调色   ·   Ctrl+Q 退出", neko.TextSize(10.5), neko.TextColor(th.TextFaint))
	}, neko.Height(18), neko.FillWidth(), neko.AlignTo(neko.Center))
}

func sidebar(c *neko.Ctx, st *state, dark, light *neko.Theme) {
	th := c.Theme
	c.Column(func() {
		c.Label("设计原则", neko.TextSize(10), neko.TextColor(th.TextFaint))
		feature(c, 0, "即时模式", "UI = f(state)，每帧重建")
		feature(c, 1, "帧内保留", "命中、焦点、编辑状态跨帧")
		feature(c, 2, "自研渲染", "EGL + GL 3.3 core，无 GUI 栈")
		feature(c, 3, "文本与输入法", "CJK 字体、XIM 提交、快捷键")

		c.Divider(neko.PaddingXY(0, 2))

		c.Column(func() {
			c.Label("UI = f(state)", neko.TextSize(12), neko.TextColor(th.Accent))
			c.Label("框架不保留控件对象，只保留上一帧的命中几何与输入状态。", neko.TextSize(10.5),
				neko.TextColor(th.TextMuted), neko.FillWidth(), neko.Wrap())
		}, neko.Background(th.PanelAlt), neko.Radius(10), neko.Padding(12), neko.Gap(4), neko.FillWidth())

		c.Label("快捷键", neko.TextSize(10), neko.TextColor(th.TextFaint))
		shortcutLine(c, "Ctrl+T", "切换主题")
		shortcutLine(c, "Ctrl+1..4", "切换强调色")
		shortcutLine(c, "F1", "暂停 / 恢复动效")
		shortcutLine(c, "Ctrl+Q", "退出")

		c.Spacer()

		c.Label("外观", neko.TextSize(10), neko.TextColor(th.TextFaint))
		if c.Toggle(&st.dark, "深色主题", neko.TextSize(12)) {
			st.last = "切换主题"
		}
		c.Row(func() {
			for i := range accentPalette {
				i := i
				c.Canvas(22, 22, func(cv *neko.Canvas) {
					col := accentPalette[i]
					cv.Circle(neko.Point{X: 11, Y: 11}, 7.5, col)
					if st.accent == i {
						cv.StrokeRoundRect(neko.Rect{X: 1, Y: 1, W: 20, H: 20}, 10, 1.5, col)
					}
					if cv.Hovered {
						cv.SetCursor(neko.CursorHand)
					}
					if cv.Clicked {
						st.accent = i
						dark.SetAccent(accentPalette[i])
						light.SetAccent(accentPalette[i])
						st.last = "切换强调色"
					}
				}, neko.ID(fmt.Sprintf("accent-%d", i)), neko.Pointer(neko.CursorHand))
				c.Spacer()
			}
		}, neko.FillWidth(), neko.AlignTo(neko.Center))
		c.Label("NekoUI · 0.1", neko.TextSize(10), neko.TextColor(th.TextFaint))
	}, neko.Width(232), neko.FillHeight(), neko.Background(th.Panel), neko.Border(th.Border),
		neko.Radius(th.Radius), neko.Padding(18), neko.Gap(9), neko.Clip())
}

func shortcutLine(c *neko.Ctx, keys, action string) {
	th := c.Theme
	c.Row(func() {
		c.Label(keys, neko.TextSize(9), neko.TextColor(th.TextMuted), neko.Background(th.PanelAlt),
			neko.Border(th.Border), neko.Radius(5), neko.PaddingXY(6, 2))
		c.Spacer()
		c.Label(action, neko.TextSize(10), neko.TextColor(th.TextFaint))
	}, neko.FillWidth(), neko.AlignTo(neko.Center))
}

func feature(c *neko.Ctx, kind int, title, desc string) {
	th := c.Theme
	c.Row(func() {
		c.Canvas(18, 18, func(cv *neko.Canvas) {
			drawFeatureIcon(cv, kind)
		})
		c.Column(func() {
			c.Label(title, neko.TextSize(11.5), neko.TextColor(th.Text))
			c.Label(desc, neko.TextSize(10), neko.TextColor(th.TextFaint))
		}, neko.Gap(0))
	}, neko.Gap(10), neko.AlignTo(neko.Center))
}

func mainContent(c *neko.Ctx, st *state) {
	c.Column(func() {
		hero(c)

		c.Row(func() {
			statTile(c, "1", "帧", "一次 View 调用")
			statTile(c, "1", "份状态", "用户持有唯一真相")
			statTile(c, "0", "GUI 依赖", "窗口/渲染/控件自研")
		}, neko.Height(80), neko.FillWidth(), neko.Gap(12))

		c.Row(func() {
			c.Flex(1.16, func() {
				stateCard(c, st)
			})
			c.Flex(0.84, func() {
				catCard(c, st)
			})
		}, neko.Height(262), neko.FillWidth(), neko.Gap(12))

		c.Row(func() {
			c.Flex(1, func() {
				inputCard(c, st)
			})
			renderCard(c, c.Theme)
		}, neko.Height(154), neko.FillWidth(), neko.Gap(12))
	}, neko.FillWidth(), neko.FillHeight(), neko.Gap(14))
}

func hero(c *neko.Ctx) {
	th := c.Theme
	c.Card(func() {
		c.Row(func() {
			c.Canvas(3, 70, func(cv *neko.Canvas) {
				cv.RoundRect(cv.Bounds, 2, c.Theme.Accent)
			})
			c.Column(func() {
				c.Label("FRAME-IMMEDIATE · 0.1", neko.TextSize(9.5), neko.TextColor(th.Accent))
				c.Row(func() {
					c.Label("把界面，", neko.TextSize(29), neko.TextColor(th.Text))
					c.Label("写成 Go 函数。", neko.TextSize(29), neko.TextColor(th.Accent))
				}, neko.Gap(0))
				c.Label("没有控件对象，没有 diff。View 是普通 Go 函数，帧结束后只留下命中几何、焦点和输入法状态。",
					neko.TextSize(12), neko.TextColor(th.TextMuted))
			}, neko.Gap(3))
			c.Spacer()
			c.Canvas(172, 84, func(cv *neko.Canvas) {
				drawHeroArt(cv)
			})
		}, neko.Gap(16), neko.AlignTo(neko.Center))
	}, neko.Height(128), neko.Gap(0))
}

func statTile(c *neko.Ctx, number, unit, desc string) {
	th := c.Theme
	c.Flex(1, func() {
		c.Card(func() {
			c.Row(func() {
				c.Label(number, neko.TextSize(23), neko.TextColor(th.Text))
				c.Label(unit, neko.TextSize(11), neko.TextColor(th.Accent))
			}, neko.Gap(6), neko.AlignTo(neko.End))
			c.Label(desc, neko.TextSize(10), neko.TextColor(th.TextFaint))
		}, neko.Height(80), neko.Padding(14), neko.Gap(1))
	})
}

func stateCard(c *neko.Ctx, st *state) {
	th := c.Theme
	c.Card(func() {
		c.Row(func() {
			c.Label("状态实验", neko.TextSize(15), neko.TextColor(th.Text))
			c.Spacer()
			if c.Button("归零", neko.Ghost(), neko.TextSize(11), neko.Height(26), neko.PaddingXY(10, 3)) {
				st.bump(-st.count)
				st.last = "计数器归零"
			}
		}, neko.FillWidth(), neko.AlignTo(neko.Center))

		c.Row(func() {
			c.Label(fmt.Sprint(st.count), neko.TextSize(44), neko.TextColor(th.Accent))
			c.Column(func() {
				c.Label("次交互", neko.TextSize(11), neko.TextColor(th.TextMuted))
				c.Label("点击 − / + 修改状态", neko.TextSize(10), neko.TextColor(th.TextFaint))
			}, neko.Gap(0))
		}, neko.Gap(12), neko.AlignTo(neko.End))

		c.Row(func() {
			roundControl(c, false, st)
			roundControl(c, true, st)
			c.Spacer()
			c.Label("状态只存在于你的 struct 里", neko.TextSize(10.5), neko.TextColor(th.TextFaint))
		}, neko.Gap(10), neko.AlignTo(neko.Center))

		c.Column(func() {
			c.Row(func() {
				c.Label("状态轨迹", neko.TextSize(9.5), neko.TextColor(th.TextFaint))
				c.Spacer()
				c.Label("最近 24 次变化", neko.TextSize(9), neko.TextColor(th.TextFaint))
			}, neko.FillWidth(), neko.AlignTo(neko.Center))
			w := c.Available().W
			c.Canvas(w, 36, func(cv *neko.Canvas) {
				drawHistory(cv, st.history)
			}, neko.ID("history"))
		}, neko.Gap(4))

	}, neko.Height(262))
}

func (s *state) bump(delta int) {
	s.count += delta
	if len(s.history) == 0 {
		s.history = make([]int, 24)
	}
	s.history = append(s.history[1:], s.count)
}

func roundControl(c *neko.Ctx, plus bool, st *state) {
	c.Canvas(44, 44, func(cv *neko.Canvas) {
		th := c.Theme
		bg := th.PanelAlt
		if cv.Hovered {
			bg = th.PanelHover
		}
		if cv.Pressed {
			bg = th.PanelHover.Mix(neko.Black, 0.12)
		}
		cv.Circle(neko.Point{X: 22, Y: 22}, 21, bg)
		cv.StrokeRoundRect(neko.Rect{X: 0.75, Y: 0.75, W: 42.5, H: 42.5}, 21, 1, th.Border)
		col := th.Text
		if cv.Hovered {
			col = th.Accent
		}
		cv.Line(neko.Point{X: 14, Y: 22}, neko.Point{X: 30, Y: 22}, 2, col)
		if plus {
			cv.Line(neko.Point{X: 22, Y: 14}, neko.Point{X: 22, Y: 30}, 2, col)
		}
		cv.SetCursor(neko.CursorHand)
		if cv.Clicked {
			if plus {
				st.bump(1)
				st.last = "点击 +1"
			} else {
				st.bump(-1)
				st.last = "点击 −1"
			}
		}
	}, neko.ID(fmt.Sprintf("round-%v", plus)), neko.Pointer(neko.CursorHand))
}

func catCard(c *neko.Ctx, st *state) {
	th := c.Theme
	c.Card(func() {
		c.Row(func() {
			c.Label("自绘猫", neko.TextSize(15), neko.TextColor(th.Text))
			c.Spacer()
			c.Label(fmt.Sprintf("poke %d", st.pokes), neko.TextSize(9.5), neko.TextColor(th.TextFaint))
		}, neko.FillWidth(), neko.AlignTo(neko.Center))
		w := c.Available().W
		elapsed := c.Elapsed
		c.Canvas(w, 150, func(cv *neko.Canvas) {
			drawCat(c, cv, st, st.anim, elapsed)
		}, neko.ID("cat"))
		c.Row(func() {
			c.Toggle(&st.anim, "呼吸与眨眼", neko.TextSize(10.5))
			c.Spacer()
			c.Label("点击它", neko.TextSize(10), neko.TextColor(th.TextFaint))
		}, neko.FillWidth(), neko.AlignTo(neko.Center))
	}, neko.Height(262))
}

func inputCard(c *neko.Ctx, st *state) {
	th := c.Theme
	c.Card(func() {
		c.Row(func() {
			c.Label("输入法实验场", neko.TextSize(15), neko.TextColor(th.Text))
			c.Spacer()
			c.Label("XIM / text-input", neko.TextSize(9.5), neko.TextColor(th.Accent))
		}, neko.FillWidth(), neko.AlignTo(neko.Center))
		w := c.Available().W
		changed := c.InputText(&st.name, neko.Placeholder("试试输入中文：你好，NekoUI…"),
			neko.Width(w), neko.Height(46), neko.ID("name"),
			neko.OnSubmit(func(s string) { st.last = "已提交输入" }))
		if changed {
			st.last = fmt.Sprintf("正在输入 · %d 字符", len([]rune(st.name)))
		}
		c.Row(func() {
			keyChip(c, "Enter 提交")
			keyChip(c, "Ctrl+C/V")
			c.Spacer()
			c.Label("中文由系统输入法提交到光标", neko.TextSize(10), neko.TextColor(th.TextFaint))
		}, neko.FillWidth(), neko.Gap(6), neko.AlignTo(neko.Center))
	}, neko.Height(154))
}

func renderCard(c *neko.Ctx, th *neko.Theme) {
	c.Card(func() {
		c.Row(func() {
			c.Label("渲染管线", neko.TextSize(13), neko.TextColor(th.Text))
			c.Spacer()
			c.Label("live", neko.TextSize(9.5), neko.TextColor(th.Accent))
		}, neko.FillWidth(), neko.AlignTo(neko.Center))
		infoRow(c, "Backend", c.Backend)
		infoRow(c, "Frame", fmt.Sprintf("%.2f ms", float64(c.Stats.CPUTime)/1e6))
		infoRow(c, "Draw calls", fmt.Sprint(c.Stats.DrawCalls))
		infoRow(c, "Vertices", fmt.Sprint(c.Stats.Vertices))
	}, neko.Width(322), neko.Height(154), neko.Padding(15), neko.Gap(5))
}

func infoRow(c *neko.Ctx, k, v string) {
	th := c.Theme
	c.Row(func() {
		c.Label(k, neko.TextSize(9.5), neko.TextColor(th.TextFaint))
		c.Spacer()
		c.Label(v, neko.TextSize(9.5), neko.TextColor(th.TextMuted))
	}, neko.FillWidth(), neko.AlignTo(neko.Center))
}

func keyChip(c *neko.Ctx, s string) {
	th := c.Theme
	c.Label(s, neko.TextSize(9), neko.TextColor(th.TextMuted), neko.Background(th.PanelAlt),
		neko.Border(th.Border), neko.Radius(6), neko.PaddingXY(7, 3))
}

func drawLogo(cv *neko.Canvas) {
	th := cv.Theme
	cv.RoundRect(cv.Bounds, 11, th.Accent)
	face := th.Bg.Mix(th.Accent, 0.12)
	cv.Circle(neko.Point{X: 19, Y: 21}, 11, face)
	// ears
	cv.Polygon([]neko.Point{{X: 8, Y: 14}, {X: 11, Y: 3}, {X: 18, Y: 10}}, face)
	cv.Polygon([]neko.Point{{X: 30, Y: 14}, {X: 27, Y: 3}, {X: 20, Y: 10}}, face)
	// eyes
	cv.Circle(neko.Point{X: 14.5, Y: 20}, 1.6, th.Bg)
	cv.Circle(neko.Point{X: 23.5, Y: 20}, 1.6, th.Bg)
	cv.Line(neko.Point{X: 17, Y: 26}, neko.Point{X: 19, Y: 27}, 1, th.Bg)
	cv.Line(neko.Point{X: 19, Y: 27}, neko.Point{X: 21, Y: 26}, 1, th.Bg)
}

func drawHeroArt(cv *neko.Canvas) {
	th := cv.Theme
	r := cv.Bounds
	cv.RoundRect(r, 13, th.PanelAlt)
	cv.StrokeRoundRect(r.InsetAll(0.75), 13, 1, th.Border)
	for i, col := range []neko.Color{neko.Hex(0xFF7A8A), neko.Hex(0xFFC66D), neko.Hex(0x6EE7B7)} {
		cv.Circle(neko.Point{X: 15 + float32(i)*10, Y: 15}, 3, col.WithAlpha(180))
	}
	lines := []struct {
		y, w float32
		col  neko.Color
	}{
		{32, 86, th.TextFaint},
		{47, 120, th.TextMuted},
		{62, 56, th.Accent},
	}
	for _, l := range lines {
		cv.RoundRect(neko.Rect{X: 15, Y: l.y, W: l.w, H: 4}, 2, l.col.WithAlpha(130))
	}
	cv.RoundRect(neko.Rect{X: 138, Y: 60, W: 3, H: 10}, 1, th.Accent)
}

func drawFeatureIcon(cv *neko.Canvas, kind int) {
	th := cv.Theme
	col := th.Accent
	switch kind {
	case 0:
		cv.StrokeRoundRect(neko.Rect{X: 1, Y: 1, W: 16, H: 16}, 5, 1.2, col.WithAlpha(150))
		cv.Circle(neko.Point{X: 9, Y: 9}, 3, col)
	case 1:
		cv.RoundRect(neko.Rect{X: 2, Y: 3, W: 12, H: 12}, 3, col.WithAlpha(55))
		cv.StrokeRoundRect(neko.Rect{X: 4, Y: 1.5, W: 12, H: 12}, 3, 1.1, col)
	case 2:
		cv.Polygon([]neko.Point{{X: 9, Y: 1.5}, {X: 17, Y: 16}, {X: 1, Y: 16}}, col.WithAlpha(190))
		cv.Polygon([]neko.Point{{X: 9, Y: 6}, {X: 13.5, Y: 14}, {X: 4.5, Y: 14}}, th.Panel)
	case 3:
		cv.StrokeRoundRect(neko.Rect{X: 1, Y: 3, W: 16, H: 12}, 3, 1.1, col.WithAlpha(150))
		cv.Rect(neko.Rect{X: 4, Y: 6, W: 4, H: 1.5}, col)
		cv.Rect(neko.Rect{X: 4, Y: 9, W: 8, H: 1.5}, col.WithAlpha(100))
	}
}

func drawCat(c *neko.Ctx, cv *neko.Canvas, st *state, anim bool, elapsed time.Duration) {
	th := c.Theme
	t := float32(elapsed.Seconds())
	w, h := cv.Bounds.W, cv.Bounds.H
	cx, cy := w/2, h*0.56

	// subtle grid
	for x := float32(10); x < w; x += 22 {
		for y := float32(8); y < h; y += 22 {
			cv.Circle(neko.Point{X: x, Y: y}, 0.7, th.Border.WithAlpha(80))
		}
	}
	cv.Circle(neko.Point{X: cx, Y: cy}, 74, th.Accent.WithAlpha(10))

	breath := float32(1)
	if anim {
		breath = 1 + 0.012*float32(math.Sin(float64(t*1.9)))
	}
	blink := anim && math.Mod(float64(t)+2.0, 4.1) < 0.14
	pokeAge := float32(1)
	if st.pokeAt > 0 {
		pokeAge = float32((elapsed - st.pokeAt).Seconds())
	}
	mouthOpen := pokeAge >= 0 && pokeAge < 0.28

	face := neko.Hex(0xF6D8C9)
	ink := neko.Hex(0x2A2433)
	pink := neko.Hex(0xFFADBE)
	accent := th.Accent

	// ears
	earL := []neko.Point{{X: cx - 47, Y: cy - 35}, {X: cx - 38, Y: cy - 72}, {X: cx - 12, Y: cy - 50}}
	earR := []neko.Point{{X: cx + 47, Y: cy - 35}, {X: cx + 38, Y: cy - 72}, {X: cx + 12, Y: cy - 50}}
	cv.Polygon(earL, face)
	cv.Polygon(earR, face)
	cv.Polygon([]neko.Point{{X: cx - 41, Y: cy - 43}, {X: cx - 36, Y: cy - 62}, {X: cx - 20, Y: cy - 50}}, pink.WithAlpha(170))
	cv.Polygon([]neko.Point{{X: cx + 41, Y: cy - 43}, {X: cx + 36, Y: cy - 62}, {X: cx + 20, Y: cy - 50}}, pink.WithAlpha(170))

	// head
	cv.Circle(neko.Point{X: cx, Y: cy}, 56*breath, face)

	// mouse-following pupils
	mx, my := cx, cy-6
	if cv.Hovered {
		mx = cv.Mouse.X
		my = cv.Mouse.Y
	}
	dx := clampF((mx-cx)/w*16, -4, 4)
	dy := clampF((my-cy)/h*12, -3, 3)

	eyeY := cy - 9
	if blink {
		cv.Line(neko.Point{X: cx - 30, Y: eyeY}, neko.Point{X: cx - 14, Y: eyeY}, 2.4, ink)
		cv.Line(neko.Point{X: cx + 14, Y: eyeY}, neko.Point{X: cx + 30, Y: eyeY}, 2.4, ink)
	} else {
		cv.Circle(neko.Point{X: cx - 22 + dx, Y: eyeY + dy}, 9, ink)
		cv.Circle(neko.Point{X: cx + 22 + dx, Y: eyeY + dy}, 9, ink)
		cv.Circle(neko.Point{X: cx - 19 + dx, Y: eyeY - 3 + dy}, 3, neko.White)
		cv.Circle(neko.Point{X: cx + 25 + dx, Y: eyeY - 3 + dy}, 3, neko.White)
	}

	// blush
	cv.Circle(neko.Point{X: cx - 37, Y: cy + 13}, 9, pink.WithAlpha(80))
	cv.Circle(neko.Point{X: cx + 37, Y: cy + 13}, 9, pink.WithAlpha(80))

	// nose + mouth
	cv.Polygon([]neko.Point{{X: cx - 4, Y: cy + 9}, {X: cx + 4, Y: cy + 9}, {X: cx, Y: cy + 14}}, pink)
	if mouthOpen {
		cv.Circle(neko.Point{X: cx, Y: cy + 23}, 6, ink)
		cv.Circle(neko.Point{X: cx, Y: cy + 27}, 3, pink)
	} else {
		cv.Line(neko.Point{X: cx, Y: cy + 14}, neko.Point{X: cx, Y: cy + 18}, 1.6, ink)
		cv.Line(neko.Point{X: cx, Y: cy + 18}, neko.Point{X: cx - 7, Y: cy + 22}, 1.6, ink)
		cv.Line(neko.Point{X: cx, Y: cy + 18}, neko.Point{X: cx + 7, Y: cy + 22}, 1.6, ink)
	}

	// whiskers
	wh := ink.WithAlpha(150)
	cv.Line(neko.Point{X: cx - 18, Y: cy + 18}, neko.Point{X: cx - 54, Y: cy + 12}, 1.1, wh)
	cv.Line(neko.Point{X: cx - 18, Y: cy + 23}, neko.Point{X: cx - 55, Y: cy + 24}, 1.1, wh)
	cv.Line(neko.Point{X: cx + 18, Y: cy + 18}, neko.Point{X: cx + 54, Y: cy + 12}, 1.1, wh)
	cv.Line(neko.Point{X: cx + 18, Y: cy + 23}, neko.Point{X: cx + 55, Y: cy + 24}, 1.1, wh)

	// collar + bell
	cv.Line(neko.Point{X: cx - 28, Y: cy + 46}, neko.Point{X: cx + 28, Y: cy + 46}, 3, accent)
	cv.Circle(neko.Point{X: cx, Y: cy + 51}, 5, accent)
	cv.Circle(neko.Point{X: cx, Y: cy + 51}, 2, th.Bg.WithAlpha(120))

	if cv.Hovered {
		cv.SetCursor(neko.CursorHand)
	}
	if cv.Clicked {
		st.pokes++
		st.pokeAt = elapsed
		st.last = "撸了一下猫"
	}
}

func drawHistory(cv *neko.Canvas, history []int) {
	th := cv.Theme
	w, h := cv.Bounds.W, cv.Bounds.H
	cv.Line(neko.Point{X: 0, Y: h - 6}, neko.Point{X: w, Y: h - 6}, 1, th.Border)
	if len(history) == 0 {
		return
	}
	maxAbs := 1
	for _, v := range history {
		if v < 0 {
			v = -v
		}
		if v > maxAbs {
			maxAbs = v
		}
	}
	n := len(history)
	gap := float32(2)
	barW := (w - gap*float32(n-1)) / float32(n)
	base := h - 6
	for i, v := range history {
		bh := float32(v) / float32(maxAbs) * (h - 12)
		if bh < 0 {
			bh = -bh
		}
		if bh < 1.5 {
			bh = 1.5
		}
		col := th.Accent.WithAlpha(70)
		if i == n-1 {
			col = th.Accent
		}
		x := float32(i) * (barW + gap)
		cv.RoundRect(neko.Rect{X: x, Y: base - bh, W: barW, H: bh}, 2, col)
	}
}

func clampF(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
