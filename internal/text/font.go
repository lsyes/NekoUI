// Package text turns UTF-8 strings into glyph quads backed by a CPU atlas.
//
// NekoUI deliberately keeps font parsing out of its core: the only third-party
// import is golang.org/x/image, used for sfnt/CFF/TTC parsing and rasterization.
// Shaping is one glyph per rune for now (correct for Latin/CJK, not for complex
// scripts); a HarfBuzz-backed shaper is on the roadmap.
package text

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"nekoui/internal/core"
	"nekoui/internal/render"
)

const defaultAtlas = 2048

// Engine owns one font family, a cache of per-pixel-size faces, and a glyph
// atlas. It is not safe for concurrent use; NekoUI builds frames on one thread.
type Engine struct {
	font   *opentype.Font
	family string
	path   string
	atlas  *render.Atlas

	mu      sync.Mutex // atlas access is single-threaded today; keeps future async raster safe
	faces   map[int]font.Face
	glyphs  map[glyphKey]glyph
	nextX   int
	nextY   int
	rowH    int
	pad     int
	metrics map[int]faceMetrics

	// Measure and advance caches remove repeated sfnt lookups for the same
	// label and rune. measureCache is cleared when it grows past a bound so
	// dynamic text (counters, timers) cannot leak memory.
	measureCache map[measureKey]core.Size
	advances     map[glyphKey]fixed.Int26_6
	generation   uint64
}

type measureKey struct {
	px int
	s  string
}

type glyphKey struct {
	px   int
	rune rune
}

type glyph struct {
	x, y    int
	w, h    int
	dr      image.Rectangle
	advance fixed.Int26_6
	valid   bool
}

type faceMetrics struct {
	ascent, descent, height float32
}

// LoadOptions selects a font. Path wins over the built-in search list; Family
// selects a face inside a TTC/OTC collection.
type LoadOptions struct {
	Path   string
	Family string
}

// Load finds and parses a usable UI font. It prefers the system Noto Sans CJK SC
// collection because it covers Latin and CJK with consistent metrics.
func Load(opts LoadOptions) (*Engine, error) {
	path := opts.Path
	family := opts.Family
	if path == "" {
		path = os.Getenv("NEKOUI_FONT")
	}
	if path == "" {
		family = "Noto Sans CJK SC"
		candidates := []string{
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/opentype/noto/NotoSansCJKsc-Regular.otf",
			"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
			"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}
	if path == "" {
		return nil, fmt.Errorf("text: no system font found; set NEKOUI_FONT")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("text: read font %s: %w", path, err)
	}

	var f *opentype.Font
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".ttc" || ext == ".otc" {
		coll, err := opentype.ParseCollection(data)
		if err != nil {
			return nil, fmt.Errorf("text: parse font collection %s: %w", path, err)
		}
		if coll.NumFonts() == 0 {
			return nil, fmt.Errorf("text: empty font collection %s", path)
		}
		chosen := -1
		if family != "" {
			for i := 0; i < coll.NumFonts(); i++ {
				cand, err := coll.Font(i)
				if err != nil {
					continue
				}
				name, _ := cand.Name(nil, sfnt.NameIDFamily)
				if strings.EqualFold(name, family) {
					f, chosen = cand, i
					break
				}
			}
		}
		if f == nil {
			f, err = coll.Font(0)
			if err != nil {
				return nil, fmt.Errorf("text: load first face of %s: %w", path, err)
			}
			chosen = 0
		}
		_ = chosen
	} else {
		f, err = opentype.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("text: parse font %s: %w", path, err)
		}
	}

	name, _ := f.Name(nil, sfnt.NameIDFamily)
	e := &Engine{
		font:         f,
		family:       name,
		path:         path,
		atlas:        render.NewAtlas(defaultAtlas, defaultAtlas),
		faces:        make(map[int]font.Face),
		glyphs:       make(map[glyphKey]glyph),
		pad:          2,
		metrics:      make(map[int]faceMetrics),
		measureCache: make(map[measureKey]core.Size, 256),
		advances:     make(map[glyphKey]fixed.Int26_6, 512),
	}
	// Warm the common size and fail fast if the face cannot be created.
	if _, err := e.Face(28); err != nil {
		return nil, err
	}
	return e, nil
}

// Family is the selected font family name.
func (e *Engine) Family() string { return e.family }

// Path is the selected font file.
func (e *Engine) Path() string { return e.path }

// Atlas returns the shared glyph atlas.
func (e *Engine) Atlas() *render.Atlas { return e.atlas }

// ClearCaches drops measurement/advance memoization. It is useful after
// loading a new font or in cold-start benchmarks; shaped-run caches keyed on
// Generation remain valid.
func (e *Engine) ClearCaches() {
	clear(e.measureCache)
	clear(e.advances)
}

// Generation changes whenever the atlas is reset and previously shaped UV
// coordinates become invalid. Layout caches keyed on glyph geometry must
// include it.
func (e *Engine) Generation() uint64 { return e.generation }

// Face returns (and caches) a face rasterized at px physical pixels.
func (e *Engine) Face(px int) (font.Face, error) {
	if px < 4 {
		px = 4
	}
	if f, ok := e.faces[px]; ok {
		return f, nil
	}
	f, err := opentype.NewFace(e.font, &opentype.FaceOptions{
		Size:    float64(px),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("text: create %dpx face: %w", px, err)
	}
	e.faces[px] = f
	return f, nil
}

func (e *Engine) faceMetrics(px int) faceMetrics {
	if m, ok := e.metrics[px]; ok {
		return m
	}
	f, err := e.Face(px)
	if err != nil {
		return faceMetrics{ascent: float32(px) * 0.8, descent: float32(px) * 0.2, height: float32(px) * 1.2}
	}
	m := f.Metrics()
	fm := faceMetrics{
		ascent:  float32(m.Ascent) / 64,
		descent: float32(m.Descent) / 64,
		height:  float32(m.Height) / 64,
	}
	e.metrics[px] = fm
	return fm
}

// Ascent returns the baseline offset for px, in physical pixels.
func (e *Engine) Ascent(px int) float32 { return e.faceMetrics(px).ascent }

// LineHeight returns the natural line height for px, in physical pixels.
func (e *Engine) LineHeight(px int) float32 { return e.faceMetrics(px).height }

// Measure returns the advance width and line height of s at px physical pixels.
func (e *Engine) Measure(s string, px int) core.Size {
	key := measureKey{px: px, s: s}
	if m, ok := e.measureCache[key]; ok {
		return m
	}
	f, err := e.Face(px)
	if err != nil {
		return core.Size{W: float32(len(s)) * float32(px) * 0.55, H: e.LineHeight(px)}
	}
	var w fixed.Int26_6
	for _, r := range s {
		w += e.advanceFor(f, px, r)
	}
	m := core.Size{W: float32(w) / 64, H: e.LineHeight(px)}
	if len(e.measureCache) > 4096 {
		clear(e.measureCache)
	}
	e.measureCache[key] = m
	return m
}

func (e *Engine) advanceFor(f font.Face, px int, r rune) fixed.Int26_6 {
	key := glyphKey{px: px, rune: r}
	if adv, ok := e.advances[key]; ok {
		return adv
	}
	adv, ok := f.GlyphAdvance(r)
	if !ok {
		adv = fixed.Int26_6(px * 64 / 2)
	}
	e.advances[key] = adv
	return adv
}

// Shape converts s into glyph quads. penX and top are physical pixels; top is
// the top of the line box, not the baseline. The returned quads are in physical
// pixels and share the engine's atlas.
func (e *Engine) Shape(s string, px int, penX, top float32, color core.Color) []render.Glyph {
	if s == "" {
		return nil
	}
	f, err := e.Face(px)
	if err != nil {
		return nil
	}
	ascent := e.Ascent(px)
	baseline := top + ascent
	out := make([]render.Glyph, 0, len(s))
	x := penX

	for _, r := range s {
		g := e.glyphFor(f, px, r)
		if r != '\n' && g.valid && g.w > 0 && g.h > 0 {
			out = append(out, render.Glyph{
				X:     x + float32(g.dr.Min.X),
				Y:     baseline + float32(g.dr.Min.Y),
				W:     float32(g.w),
				H:     float32(g.h),
				U0:    float32(g.x) / float32(e.atlas.Width),
				V0:    float32(g.y) / float32(e.atlas.Height),
				U1:    float32(g.x+g.w) / float32(e.atlas.Width),
				V1:    float32(g.y+g.h) / float32(e.atlas.Height),
				Color: color,
			})
		}
		x += float32(g.advance) / 64
	}
	return out
}

// glyphFor rasterizes and packs a rune on first use.
func (e *Engine) glyphFor(f font.Face, px int, r rune) glyph {
	key := glyphKey{px, r}
	if g, ok := e.glyphs[key]; ok {
		return g
	}

	dr, mask, maskp, advance, ok := f.Glyph(fixed.Point26_6{}, r)
	if !ok {
		g := glyph{advance: e.advanceFor(f, px, r)}
		e.glyphs[key] = g
		return g
	}
	e.advances[key] = advance
	if dr.Dx() <= 0 || dr.Dy() <= 0 {
		g := glyph{advance: advance, dr: dr, valid: true}
		e.glyphs[key] = g
		return g
	}

	w, h := dr.Dx(), dr.Dy()
	if w > e.atlas.Width || h > e.atlas.Height {
		g := glyph{advance: advance}
		e.glyphs[key] = g
		return g
	}
	if e.nextX+w+e.pad > e.atlas.Width {
		e.nextX = 0
		e.nextY += e.rowH + e.pad
		e.rowH = 0
	}
	if e.nextY+h+e.pad > e.atlas.Height {
		// The atlas is full. Rather than silently dropping all future text,
		// reset it; existing glyph quads from previous frames will be stale for
		// one frame, which is preferable to a permanently broken UI.
		for i := range e.atlas.Pix {
			e.atlas.Pix[i] = 0
		}
		e.atlas.MarkDirty(core.Rect{W: float32(e.atlas.Width), H: float32(e.atlas.Height)})
		e.glyphs = make(map[glyphKey]glyph)
		e.advances = make(map[glyphKey]fixed.Int26_6, len(e.advances))
		e.generation++
		e.nextX, e.nextY, e.rowH = 0, 0, 0
	}

	gx, gy := e.nextX, e.nextY
	if mask != nil {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				_, _, _, a := mask.At(maskp.X+x, maskp.Y+y).RGBA()
				e.atlas.Pix[(gy+y)*e.atlas.Width+gx+x] = uint8(a >> 8)
			}
		}
	}
	e.atlas.MarkDirty(core.Rect{X: float32(gx), Y: float32(gy), W: float32(w), H: float32(h)})
	g := glyph{x: gx, y: gy, w: w, h: h, dr: dr, advance: advance, valid: true}
	e.glyphs[key] = g
	e.nextX += w + e.pad
	if h > e.rowH {
		e.rowH = h
	}
	return g
}
