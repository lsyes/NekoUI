// Smoke is a tiny headless/visible sanity check for the renderer and widgets.
package main

import (
	"flag"
	"log"

	neko "nekoui"
)

func main() {
	shot := flag.String("screenshot", "/tmp/nekoui-smoke.png", "write a PNG and exit")
	visible := flag.Bool("visible", false, "open an X11 window instead of headless")
	flag.Parse()

	cfg := neko.Config{
		Title:            "NekoUI smoke",
		Width:            640,
		Height:           400,
		Resizable:        true,
		Scale:            2,
		Theme:            neko.DefaultTheme(),
		Screenshot:       *shot,
		ScreenshotFrames: 2,
	}
	if *visible {
		cfg.Backend = "x11"
		cfg.Scale = 1
		cfg.Screenshot = ""
	}

	err := neko.Run(cfg, func(c *neko.Ctx) {
		c.Background(neko.Hex(0x0B0D13))
		c.Column(func() {
			c.Label("NekoUI", neko.TextSize(32), neko.TextColor(neko.White))
			c.Label("把界面，写成 Go 函数。", neko.TextSize(16), neko.TextColor(neko.Hex(0x99A3B7)))
			c.Button("按钮 Button", neko.Primary())
			c.Toggle(new(bool), "即时模式")
			c.InputText(new(string), neko.Placeholder("输入法测试…"), neko.Width(320))
			c.Canvas(200, 100, func(cv *neko.Canvas) {
				cv.RoundRect(neko.Rect{W: 200, H: 100}, 16, neko.Hex(0x1E2432))
				cv.Circle(neko.Point{X: 100, Y: 50}, 24, neko.Hex(0x7FA6FF))
				cv.Text("Canvas", neko.Point{X: 62, Y: 70}, 16, neko.White)
			})
		}, neko.FillWidth(), neko.FillHeight(), neko.Padding(32), neko.Gap(16))
	})
	if err != nil {
		log.Fatal(err)
	}
}
