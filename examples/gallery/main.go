// NekoUI gallery: Scrollable, Image, Checkbox, Radio, Slider and ProgressBar.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"

	neko "nekoui"
)

type state struct {
	dark    bool
	notify  bool
	quality int
	volume  float32
	name    string
	last    string
}

func main() {
	backend := flag.String("backend", "", "x11, wayland or headless")
	shot := flag.String("screenshot", "", "render a PNG and exit")
	flag.Parse()

	st := &state{dark: true, notify: true, quality: 0, volume: 64, last: "就绪"}
	art := neko.NewImage(galleryArt())
	err := neko.Run(neko.Config{
		Title:      "NekoUI gallery — advanced widgets",
		Width:      980,
		Height:     760,
		MinWidth:   820,
		MinHeight:  640,
		Resizable:  true,
		Backend:    *backend,
		Screenshot: *shot,
		Theme:      neko.DefaultTheme(),
	}, func(c *neko.Ctx) {
		if st.dark {
			c.UseTheme(neko.DefaultTheme())
		} else {
			c.UseTheme(neko.LightTheme())
		}
		th := c.Theme
		c.Background(th.Bg)

		c.Column(func() {
			c.Row(func() {
				c.Label("NekoUI gallery", neko.TextSize(22), neko.TextColor(th.Text))
				c.Label("Scrollable · Image · Checkbox · Radio · Slider · Progress",
					neko.TextSize(11), neko.TextColor(th.TextFaint))
				c.Spacer()
				c.Label(st.last, neko.TextSize(10.5), neko.TextColor(th.Accent),
					neko.Background(th.PanelAlt), neko.Border(th.Border),
					neko.Radius(7), neko.PaddingXY(9, 4))
			}, neko.FillWidth(), neko.AlignTo(neko.Center), neko.Gap(12))

			c.Row(func() {
				c.Flex(0.9, func() { controls(c, st) })
				c.Flex(1.25, func() { scrollDemo(c, st) })
			}, neko.Height(410), neko.FillWidth(), neko.Gap(16))

			c.Row(func() {
				imageDemo(c, art)
				c.Flex(1, func() { progressDemo(c, st) })
			}, neko.Height(210), neko.FillWidth(), neko.Gap(16))

			c.Row(func() {
				c.Label("用 Go 描述，用 EGL/OpenGL 绘制 · Wheel / drag 测试 Scrollable",
					neko.TextSize(10.5), neko.TextColor(th.TextFaint))
				c.Spacer()
				c.Label(fmt.Sprintf("%.0f FPS · %s", c.FPS, c.Backend),
					neko.TextSize(10.5), neko.TextColor(th.TextFaint))
			}, neko.FillWidth(), neko.AlignTo(neko.Center))
		}, neko.Padding(22), neko.Gap(14), neko.FillWidth(), neko.FillHeight())
	})
	if err != nil {
		log.Fatal(err)
	}
}

func controls(c *neko.Ctx, st *state) {
	th := c.Theme
	c.Card(func() {
		c.Label("控件", neko.TextSize(15), neko.TextColor(th.Text))
		c.Label("选择、拖动、输入", neko.TextSize(10), neko.TextColor(th.TextFaint))
		c.Divider(neko.PaddingXY(0, 4))
		if c.Checkbox(&st.notify, "启用通知", neko.TextSize(12)) {
			st.last = "通知状态已切换"
		}
		c.Label("主题", neko.TextSize(10), neko.TextColor(th.TextFaint))
		if c.Radio(&st.quality, 0, "深色", neko.TextSize(12)) {
			st.dark = true
			st.last = "深色主题"
		}
		if c.Radio(&st.quality, 1, "浅色", neko.TextSize(12)) {
			st.dark = false
			st.last = "浅色主题"
		}
		c.Label("音量", neko.TextSize(10), neko.TextColor(th.TextFaint))
		if c.Slider(&st.volume, 0, 100, neko.Width(c.Available().W), neko.Step(1)) {
			st.last = fmt.Sprintf("音量 %.0f", st.volume)
		}
		c.Row(func() {
			c.Label(fmt.Sprintf("%.0f", st.volume), neko.TextSize(11), neko.TextColor(th.Accent))
			c.Spacer()
			c.Label("Slider", neko.TextSize(9.5), neko.TextColor(th.TextFaint))
		}, neko.FillWidth())
		c.Divider(neko.PaddingXY(0, 3))
		c.InputText(&st.name, neko.Placeholder("输入法测试…"), neko.Width(c.Available().W))
	}, neko.FillWidth(), neko.FillHeight())
}

func scrollDemo(c *neko.Ctx, st *state) {
	th := c.Theme
	c.Card(func() {
		c.Row(func() {
			c.Label("Scrollable", neko.TextSize(15), neko.TextColor(th.Text))
			c.Spacer()
			c.Label("wheel / drag", neko.TextSize(9.5), neko.TextColor(th.TextFaint))
		}, neko.FillWidth(), neko.AlignTo(neko.Center))
		c.Flex(1, func() {
			c.Scrollable(func() {
				for i := 0; i < 30; i++ {
					i := i
					c.Row(func() {
						c.Canvas(26, 26, func(cv *neko.Canvas) {
							col := th.Accent
							if i%3 == 1 {
								col = neko.Hex(0x6EE7B7)
							} else if i%3 == 2 {
								col = neko.Hex(0xFFC66D)
							}
							cv.Circle(neko.Point{X: 13, Y: 13}, 9, col.WithAlpha(40))
							cv.Circle(neko.Point{X: 13, Y: 13}, 3.5, col)
						}, neko.ID(fmt.Sprintf("dot-%d", i)))
						c.Column(func() {
							c.Label(fmt.Sprintf("滚动条目 %02d", i+1), neko.TextSize(11.5),
								neko.TextColor(th.Text))
							c.Label("固定行高，内容超出视口后裁剪",
								neko.TextSize(9.5), neko.TextColor(th.TextFaint))
						}, neko.Gap(0))
						c.Spacer()
						if c.Button("选择", neko.Ghost(), neko.TextSize(10),
							neko.Height(26), neko.PaddingXY(9, 3),
							neko.ID(fmt.Sprintf("pick-%d", i))) {
							st.last = fmt.Sprintf("选择条目 %02d", i+1)
						}
					}, neko.FillWidth(), neko.Gap(9), neko.AlignTo(neko.Center))
				}
			}, neko.Gap(8), neko.Padding(2))
		})
	}, neko.FillWidth(), neko.FillHeight())
}

func imageDemo(c *neko.Ctx, art *neko.Image) {
	th := c.Theme
	c.Card(func() {
		c.Row(func() {
			c.Label("Image", neko.TextSize(15), neko.TextColor(th.Text))
			c.Spacer()
			c.Label("RGBA · cover · radius", neko.TextSize(9.5), neko.TextColor(th.TextFaint))
		}, neko.FillWidth(), neko.AlignTo(neko.Center))
		c.Row(func() {
			c.Column(func() {
				c.Image(art, neko.Width(210), neko.Height(112),
					neko.Fit(neko.ImageCover), neko.Radius(10))
				c.Label("NewImage(generated)", neko.TextSize(9), neko.TextColor(th.TextFaint))
			}, neko.Gap(4))
			c.Column(func() {
				c.ImageFile("docs/assets/showcase.png", neko.Width(210), neko.Height(112),
					neko.Fit(neko.ImageCover), neko.Radius(10))
				c.Label("ImageFile(showcase.png)", neko.TextSize(9), neko.TextColor(th.TextFaint))
			}, neko.Gap(4))
		}, neko.Gap(12))
	}, neko.Width(540), neko.FillHeight())
}

func progressDemo(c *neko.Ctx, st *state) {
	th := c.Theme
	c.Card(func() {
		c.Label("ProgressBar", neko.TextSize(15), neko.TextColor(th.Text))
		c.Label("进度与状态", neko.TextSize(10), neko.TextColor(th.TextFaint))
		c.ProgressBar(st.volume/100, neko.Width(c.Available().W), neko.Height(12))
		c.Row(func() {
			c.Label(fmt.Sprintf("%.0f%%", st.volume), neko.TextSize(11), neko.TextColor(th.Accent))
			c.Spacer()
			c.Label("Checkbox / Radio / Slider 状态都留在 app struct 中",
				neko.TextSize(9.5), neko.TextColor(th.TextFaint))
		}, neko.FillWidth(), neko.AlignTo(neko.Center))
		c.Spacer()
		c.Label(fmt.Sprintf("notify=%v  theme=%d  volume=%.0f",
			st.notify, st.quality, st.volume),
			neko.TextSize(10), neko.TextColor(th.TextMuted))
	}, neko.FillWidth(), neko.FillHeight())
}

// galleryArt builds a small in-memory image so the example does not depend on
// external assets for the NewImage path.
func galleryArt() image.Image {
	const w, h = 420, 224
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t := float64(x) / float64(w)
			u := float64(y) / float64(h)
			r := uint8(30 + 90*t)
			g := uint8(40 + 130*u)
			b := uint8(90 + 150*(1-t*u))
			if (x-300)*(x-300)+(y-70)*(y-70) < 46*46 {
				r, g, b = 0x7F, 0xA6, 0xFF
			}
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	// A few translucent accent circles exercise blending.
	for i := 0; i < 3; i++ {
		cx := 90.0 + float64(i)*70
		cy := 150.0 + 20*math.Sin(float64(i))
		rad := 30.0
		for y := int(cy - rad); y < int(cy+rad); y++ {
			for x := int(cx - rad); x < int(cx+rad); x++ {
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				dx, dy := float64(x)-cx, float64(y)-cy
				if dx*dx+dy*dy <= rad*rad {
					old := img.RGBAAt(x, y)
					img.SetRGBA(x, y, color.RGBA{
						R: uint8((int(old.R) + 0xFF) / 2),
						G: uint8((int(old.G) + 0xFF) / 2),
						B: uint8((int(old.B) + 0xFF) / 2),
						A: 255,
					})
				}
			}
		}
	}
	return img
}
