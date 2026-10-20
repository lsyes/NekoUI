// NekoUI input-method diagnostic: one auto-focused text field, a live caret
// position probe and the committed value. Run it on X11 or Wayland to verify
// candidate placement and CJK commits.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	neko "nekoui"
)

func main() {
	backend := flag.String("backend", "", "x11, wayland or headless")
	shot := flag.String("screenshot", "", "render a PNG and exit")
	flag.Parse()

	name := ""
	last := ""
	err := neko.Run(neko.Config{
		Title:      "NekoUI IME probe",
		Width:      620,
		Height:     300,
		MinWidth:   480,
		MinHeight:  260,
		Resizable:  true,
		Backend:    *backend,
		Screenshot: *shot,
		Theme:      neko.DefaultTheme(),
	}, func(c *neko.Ctx) {
		if name != last {
			last = name
			fmt.Fprintf(os.Stderr, "ime: value=%q\n", name)
		}
		c.Background(c.Theme.Bg)
		c.Column(func() {
			c.Label("输入法诊断 · IME probe", neko.TextSize(20), neko.TextColor(c.Theme.Text))
			c.Label(fmt.Sprintf("backend: %s   scale: %.2f   font: %s",
				c.Backend, c.Scale, c.FontInfo),
				neko.TextSize(10.5), neko.TextColor(c.Theme.TextFaint))

			c.InputText(&name,
				neko.Autofocus(),
				neko.ID("ime"),
				neko.Width(c.Available().W),
				neko.Height(52),
				neko.TextSize(17),
				neko.Placeholder("试试输入中文：你好，NekoUI…"),
			)
			lines := []string{"committed: " + name}
			if name == "" {
				lines = append(lines, "committed: (empty)")
			}
			c.Label(fmt.Sprintf("runes: %d", len([]rune(name))),
				neko.TextSize(11), neko.TextColor(c.Theme.TextMuted))
			for _, line := range lines {
				c.Label(line, neko.TextSize(13), neko.TextColor(c.Theme.Accent))
			}
			c.Spacer()
			c.Label("X11: XIM + XNSpotLocation   ·   Wayland: text-input-v3",
				neko.TextSize(10), neko.TextColor(c.Theme.TextFaint))
			c.Label("Ctrl+Q 退出", neko.TextSize(10), neko.TextColor(c.Theme.TextFaint))
			if c.Shortcut(neko.ModCtrl, neko.KeyQ) {
				c.Quit()
			}
		}, neko.Padding(24), neko.Gap(10), neko.FillWidth(), neko.FillHeight())
	})
	if err != nil {
		log.Fatal(err)
	}
}
