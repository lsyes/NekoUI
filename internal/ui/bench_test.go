package ui

import (
	"fmt"
	"testing"
)

func newBenchCtx(b *testing.B, w, h int) (*Ctx, *Engine) {
	b.Helper()
	return newTestCtx(b, w, h)
}

func benchGallery(c *Ctx) {
	c.Background(c.Theme.Bg)
	c.Column(func() {
		c.Row(func() {
			c.Label("NekoUI gallery", TextSize(22))
			c.Label("Scrollable · Image · Checkbox · Radio · Slider", TextSize(11))
			c.Spacer()
			c.Label(fmt.Sprintf("%d FPS", int(c.FPS)), TextSize(10))
		}, FillWidth(), AlignTo(Center), Gap(12))
		c.Row(func() {
			c.Flex(0.9, func() {
				c.Card(func() {
					c.Label("控件", TextSize(15))
					c.Label("选择、拖动、输入", TextSize(10))
					c.Divider(PaddingXY(0, 4))
					on := false
					c.Checkbox(&on, "启用通知", TextSize(12))
					c.Label("主题", TextSize(10))
					choice := 0
					c.Radio(&choice, 0, "深色", TextSize(12))
					c.Radio(&choice, 1, "浅色", TextSize(12))
					c.Label("音量", TextSize(10))
					vol := float32(64)
					c.Slider(&vol, 0, 100, Width(c.Available().W), Step(1))
					c.InputText(new(string), Placeholder("输入法测试…"), Width(c.Available().W))
				}, FillWidth(), FillHeight())
			})
			c.Flex(1.25, func() {
				c.Card(func() {
					c.Label("Scrollable", TextSize(15))
					c.Flex(1, func() {
						c.Scrollable(func() {
							for i := 0; i < 30; i++ {
								c.Row(func() {
									c.Canvas(26, 26, func(cv *Canvas) {
										cv.Circle(Point{X: 13, Y: 13}, 9, c.Theme.Accent.WithAlpha(40))
									}, ID(fmt.Sprintf("dot-%d", i)))
									c.Label(fmt.Sprintf("滚动条目 %02d", i+1), TextSize(11.5))
									c.Spacer()
									c.Button("选择", Ghost(), TextSize(10), Height(26), PaddingXY(9, 3),
										ID(fmt.Sprintf("pick-%d", i)))
								}, FillWidth(), Gap(9), AlignTo(Center))
							}
						}, Gap(8), Padding(2))
					})
				}, FillWidth(), FillHeight())
			})
		}, Height(410), FillWidth(), Gap(16))
	}, Padding(22), Gap(14), Width(float32(c.W)), Height(float32(c.H)))
}

func BenchmarkGalleryFrame(b *testing.B) {
	c, eng := newBenchCtx(b, 980, 760)
	_ = eng
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.begin()
		c.Frame = i
		c.FPS = 60
		benchGallery(c)
		c.finish()
	}
}

func BenchmarkGalleryCold(b *testing.B) {
	c, _ := newBenchCtx(b, 980, 760)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clear(c.textRuns)
		c.textRunGlyphs = 0
		c.font.ClearCaches()
		c.begin()
		c.Frame = i
		c.FPS = 60
		benchGallery(c)
		c.finish()
	}
}
