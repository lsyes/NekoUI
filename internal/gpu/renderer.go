//go:build linux && cgo

// Package gpu implements NekoUI's OpenGL 3.3 renderer and the EGL context
// plumbing lives in the egl subpackage. Nothing in this package knows about
// widgets, layout or windowing: it consumes a render.Frame display list.
package gpu

/*
#cgo LDFLAGS: -l:libGL.so.1
#include "gl_api.h"
*/
import "C"

import (
	"fmt"
	"math"
	"time"
	"unsafe"

	"nekoui/internal/core"
	"nekoui/internal/render"
)

const (
	vertexRect = iota
	vertexText
	vertexSolid
	vertexImage
)

// vertex is the single interleaved format used for every primitive. Keeping one
// format lets rectangles, text and custom art share one draw call and preserve
// painter's order.
type vertex struct {
	X, Y       float32
	U, V       float32
	R, G, B, A float32
	CX, CY     float32
	HX, HY     float32
	Radius     float32
	Mode       float32
	Stroke     float32
}

// Renderer is a GL 3.3 core renderer. It must be used with the backend's EGL
// context current, on the UI thread.
type Renderer struct {
	prog   C.GLuint
	vao    C.GLuint
	vbo    C.GLuint
	tex    C.GLuint
	texW   int
	texH   int
	uView  C.GLint
	uAtlas C.GLint
	uImage C.GLint
	verts  []vertex
	images map[*render.Image]*gpuImage
	used   map[*render.Image]struct{}
	stats  render.Stats
	info   string
}

type gpuImage struct {
	tex     C.GLuint
	version uint64
}

var (
	rectVS = `#version 330 core
layout(location=0) in vec2 aPos;
layout(location=1) in vec2 aUV;
layout(location=2) in vec4 aColor;
layout(location=3) in vec2 aCenter;
layout(location=4) in vec2 aHalf;
layout(location=5) in float aRadius;
layout(location=6) in float aMode;
layout(location=7) in float aStroke;
uniform vec2 uViewport;
out vec2 vLocal;
out vec2 vHalf;
out float vRadius;
out float vStroke;
out vec4 vColor;
out vec2 vUV;
flat out int vMode;
void main() {
	vLocal = aPos - aCenter;
	vHalf = aHalf;
	vRadius = aRadius;
	vStroke = aStroke;
	vColor = aColor;
	vUV = aUV;
	vMode = int(aMode + 0.5);
	vec2 ndc = (aPos / uViewport) * 2.0 - 1.0;
	gl_Position = vec4(ndc.x, -ndc.y, 0.0, 1.0);
}
`

	rectFS = `#version 330 core
in vec2 vLocal;
in vec2 vHalf;
in float vRadius;
in float vStroke;
in vec4 vColor;
in vec2 vUV;
flat in int vMode;
uniform sampler2D uAtlas;
uniform sampler2D uImage;
out vec4 FragColor;

float sdRoundBox(vec2 p, vec2 b, float r) {
	vec2 q = abs(p) - b + r;
	return min(max(q.x, q.y), 0.0) + length(max(q, 0.0)) - r;
}

void main() {
	float alpha;
	if (vMode == 1) {
		alpha = texture(uAtlas, vUV).r;
	} else if (vMode == 2) {
		alpha = 1.0;
	} else if (vMode == 3) {
		float rr = clamp(vRadius, 0.0, min(vHalf.x, vHalf.y));
		float d = sdRoundBox(vLocal, vHalf, rr);
		float aa = max(fwidth(d), 0.0001);
		float mask = 1.0 - smoothstep(-aa, aa, d);
		vec4 tint = vColor;
		tint.rgb *= tint.a;
		FragColor = texture(uImage, vUV) * tint * mask;
		return;
	} else {
		float rr = clamp(vRadius, 0.0, min(vHalf.x, vHalf.y));
		float d = sdRoundBox(vLocal, vHalf, rr);
		if (vStroke > 0.01) {
			d = abs(d) - vStroke * 0.5;
		}
		float aa = max(fwidth(d), 0.0001);
		alpha = 1.0 - smoothstep(-aa, aa, d);
	}
	alpha *= vColor.a;
	FragColor = vec4(vColor.rgb * alpha, alpha);
}
`
)

// NewRenderer builds GL objects. The caller must have made an EGL context
// current already.
func NewRenderer() (*Renderer, error) {
	r := &Renderer{
		verts:  make([]vertex, 0, 4096),
		images: make(map[*render.Image]*gpuImage),
		used:   make(map[*render.Image]struct{}, 8),
	}
	if err := r.init(); err != nil {
		r.Destroy()
		return nil, err
	}
	return r, nil
}

// Info describes the GL implementation for diagnostics and the demo HUD.
func (r *Renderer) Info() string { return r.info }

// Stats returns counters for the last rendered frame.
func (r *Renderer) Stats() render.Stats { return r.stats }

// Destroy releases GL objects. The context must still be current.
func (r *Renderer) Destroy() {
	if r.vbo != 0 {
		C.glDeleteBuffers(1, &r.vbo)
		r.vbo = 0
	}
	if r.vao != 0 {
		C.glDeleteVertexArrays(1, &r.vao)
		r.vao = 0
	}
	if r.tex != 0 {
		C.glDeleteTextures(1, &r.tex)
		r.tex = 0
	}
	if r.prog != 0 {
		C.glDeleteProgram(r.prog)
		r.prog = 0
	}
	for _, img := range r.images {
		if img.tex != 0 {
			C.glDeleteTextures(1, &img.tex)
		}
	}
	r.images = nil
}

func (r *Renderer) init() error {
	version := goString((*C.char)(unsafe.Pointer(C.glGetString(C.GL_VERSION))))
	renderer := goString((*C.char)(unsafe.Pointer(C.glGetString(C.GL_RENDERER))))
	sl := goString((*C.char)(unsafe.Pointer(C.glGetString(C.GL_SHADING_LANGUAGE_VERSION))))
	r.info = fmt.Sprintf("%s · %s · GLSL %s", version, renderer, sl)
	if version == "" {
		return fmt.Errorf("gpu: no current OpenGL context")
	}

	vs, err := compileShader(C.GL_VERTEX_SHADER, rectVS)
	if err != nil {
		return err
	}
	defer C.glDeleteShader(vs)
	fs, err := compileShader(C.GL_FRAGMENT_SHADER, rectFS)
	if err != nil {
		return err
	}
	defer C.glDeleteShader(fs)

	var status C.GLint
	log := make([]byte, 4096)
	prog := C.ng_link_program(vs, fs, &status, (*C.GLchar)(unsafe.Pointer(&log[0])), C.GLsizei(len(log)))
	if status == C.GL_FALSE {
		C.glDeleteProgram(prog)
		return fmt.Errorf("gpu: link program: %s", trimLog(log))
	}
	r.prog = prog
	uv := C.CString("uViewport")
	r.uView = C.glGetUniformLocation(r.prog, uv)
	C.free(unsafe.Pointer(uv))
	ua := C.CString("uAtlas")
	r.uAtlas = C.glGetUniformLocation(r.prog, ua)
	C.free(unsafe.Pointer(ua))
	ui := C.CString("uImage")
	r.uImage = C.glGetUniformLocation(r.prog, ui)
	C.free(unsafe.Pointer(ui))

	C.glGenVertexArrays(1, &r.vao)
	C.glGenBuffers(1, &r.vbo)
	C.glBindVertexArray(r.vao)
	C.glBindBuffer(C.GL_ARRAY_BUFFER, r.vbo)

	stride := C.GLsizei(unsafe.Sizeof(vertex{}))
	// Locations must match the shader's layout declarations.
	r.attr(0, 2, unsafe.Offsetof(r.verts[0].X), stride)
	r.attr(1, 2, unsafe.Offsetof(r.verts[0].U), stride)
	r.attr(2, 4, unsafe.Offsetof(r.verts[0].R), stride)
	r.attr(3, 2, unsafe.Offsetof(r.verts[0].CX), stride)
	r.attr(4, 2, unsafe.Offsetof(r.verts[0].HX), stride)
	r.attr(5, 1, unsafe.Offsetof(r.verts[0].Radius), stride)
	r.attr(6, 1, unsafe.Offsetof(r.verts[0].Mode), stride)
	r.attr(7, 1, unsafe.Offsetof(r.verts[0].Stroke), stride)

	C.glBindVertexArray(0)
	C.glEnable(C.GL_BLEND)
	C.glDisable(C.GL_DEPTH_TEST)
	C.glDisable(C.GL_CULL_FACE)
	C.glBlendFunc(C.GL_ONE, C.GL_ONE_MINUS_SRC_ALPHA)
	C.glPixelStorei(C.GL_UNPACK_ALIGNMENT, 1)

	r.makeDummyTexture()
	return nil
}

func (r *Renderer) attr(loc C.GLuint, n C.GLint, off uintptr, stride C.GLsizei) {
	C.glEnableVertexAttribArray(loc)
	C.ng_vertex_attrib_pointer(loc, n, C.GL_FLOAT, C.GL_FALSE, stride, C.uintptr_t(off))
}

func compileShader(kind C.GLenum, src string) (C.GLuint, error) {
	csrc := C.CString(src)
	defer C.free(unsafe.Pointer(csrc))
	var status C.GLint
	log := make([]byte, 4096)
	s := C.ng_compile_shader(kind, (*C.GLchar)(unsafe.Pointer(csrc)), &status, (*C.GLchar)(unsafe.Pointer(&log[0])), C.GLsizei(len(log)))
	if status == C.GL_FALSE {
		C.glDeleteShader(s)
		return 0, fmt.Errorf("gpu: compile shader: %s", trimLog(log))
	}
	return s, nil
}

func (r *Renderer) makeDummyTexture() {
	C.glGenTextures(1, &r.tex)
	C.glBindTexture(C.GL_TEXTURE_2D, r.tex)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
	white := []byte{255, 255, 255, 255}
	C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GL_R8, 1, 1, 0, C.GL_RED, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&white[0]))
	r.texW, r.texH = 1, 1
}

// Render draws one frame. The EGL context must be current.
func (r *Renderer) Render(f *render.Frame) error {
	if f == nil || f.PixelWidth <= 0 || f.PixelHeight <= 0 {
		return nil
	}
	start := time.Now()
	r.stats.DrawCalls = 0
	r.stats.Vertices = 0

	C.glViewport(0, 0, C.GLsizei(f.PixelWidth), C.GLsizei(f.PixelHeight))
	C.glDisable(C.GL_DEPTH_TEST)
	C.glDisable(C.GL_CULL_FACE)
	C.glEnable(C.GL_BLEND)
	C.glEnable(C.GL_SCISSOR_TEST)
	C.glBlendFunc(C.GL_ONE, C.GL_ONE_MINUS_SRC_ALPHA)
	C.glClearColor(C.GLfloat(f.Clear.R)/255, C.GLfloat(f.Clear.G)/255, C.GLfloat(f.Clear.B)/255, 1)
	C.glClear(C.GL_COLOR_BUFFER_BIT)

	C.glUseProgram(r.prog)
	C.glUniform2f(r.uView, C.GLfloat(f.PixelWidth), C.GLfloat(f.PixelHeight))
	C.glUniform1i(r.uAtlas, 0)
	C.glUniform1i(r.uImage, 1)
	C.glActiveTexture(C.GL_TEXTURE0)
	C.glBindTexture(C.GL_TEXTURE_2D, r.syncAtlas(f.Atlas))

	C.glBindVertexArray(r.vao)
	C.glBindBuffer(C.GL_ARRAY_BUFFER, r.vbo)

	scale := f.Scale
	if scale <= 0 {
		scale = 1
	}
	full := scissorRect{0, 0, int32(f.PixelWidth), int32(f.PixelHeight)}
	cur := full
	C.glScissor(0, 0, C.GLsizei(f.PixelWidth), C.GLsizei(f.PixelHeight))
	clear(r.used)
	for _, img := range f.Images {
		r.used[img] = struct{}{}
	}

	for i := range f.Ops {
		op := &f.Ops[i]
		if op.Clip.IsEmpty() {
			continue
		}
		sc := toScissor(op.Clip, scale, f.PixelWidth, f.PixelHeight)
		if sc.empty() {
			continue
		}
		if sc != cur {
			r.flush()
			cur = sc
			C.glScissor(C.GLint(sc.x), C.GLint(sc.y), C.GLsizei(sc.w), C.GLsizei(sc.h))
		}
		switch op.Kind {
		case render.RectFill, render.RectStroke:
			r.addRect(op, scale)
		case render.Text:
			r.addGlyphs(op, scale)
		case render.Triangles:
			r.addTriangles(op, scale)
		case render.ImageQuad:
			if op.Image == nil {
				continue
			}
			tex := r.syncImage(op.Image)
			if tex == 0 {
				continue
			}
			r.flush()
			C.glActiveTexture(C.GL_TEXTURE1)
			C.glBindTexture(C.GL_TEXTURE_2D, tex)
			C.glActiveTexture(C.GL_TEXTURE0)
			r.addImage(op, scale)
		}
	}
	r.flush()
	C.glBindVertexArray(0)
	C.glDisable(C.GL_SCISSOR_TEST)
	C.glActiveTexture(C.GL_TEXTURE1)
	C.glBindTexture(C.GL_TEXTURE_2D, 0)
	C.glActiveTexture(C.GL_TEXTURE0)
	// Evict textures that did not participate in this frame.
	for img, gi := range r.images {
		if _, ok := r.used[img]; ok {
			continue
		}
		if gi.tex != 0 {
			C.glDeleteTextures(1, &gi.tex)
		}
		delete(r.images, img)
	}
	if f.Atlas != nil {
		f.Atlas.ClearDirty()
	}
	r.stats.CPUTime = int64(time.Since(start))
	return nil
}

// syncAtlas uploads newly rasterized glyphs and returns the texture to bind.
func (r *Renderer) syncAtlas(a *render.Atlas) C.GLuint {
	if a == nil {
		return r.tex
	}
	recreate := r.tex == 0 || r.texW != a.Width || r.texH != a.Height
	if recreate {
		if r.tex != 0 {
			C.glDeleteTextures(1, &r.tex)
		}
		C.glGenTextures(1, &r.tex)
		C.glBindTexture(C.GL_TEXTURE_2D, r.tex)
		C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_LINEAR)
		C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_LINEAR)
		C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
		C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
		C.glPixelStorei(C.GL_UNPACK_ALIGNMENT, 1)
		C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GL_R8, C.GLsizei(a.Width), C.GLsizei(a.Height), 0, C.GL_RED, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&a.Pix[0]))
		r.texW, r.texH = a.Width, a.Height
		return r.tex
	}
	C.glBindTexture(C.GL_TEXTURE_2D, r.tex)
	// Atlas sub-rects are read from a row-major buffer whose stride is the full
	// atlas width. Without this, GL packs each dirty row at width*4 bytes and
	// every glyph uploaded after the first full upload is skewed.
	C.glPixelStorei(C.GL_UNPACK_ROW_LENGTH, C.GLint(a.Width))
	for _, d := range a.Dirty() {
		x := int(math.Floor(float64(d.X)))
		y := int(math.Floor(float64(d.Y)))
		w := int(math.Ceil(float64(d.W)))
		h := int(math.Ceil(float64(d.H)))
		if w <= 0 || h <= 0 {
			continue
		}
		if x < 0 {
			w += x
			x = 0
		}
		if y < 0 {
			h += y
			y = 0
		}
		if x+w > a.Width {
			w = a.Width - x
		}
		if y+h > a.Height {
			h = a.Height - y
		}
		if w <= 0 || h <= 0 {
			continue
		}
		idx := y*a.Width + x
		C.glTexSubImage2D(C.GL_TEXTURE_2D, 0, C.GLint(x), C.GLint(y), C.GLsizei(w), C.GLsizei(h), C.GL_RED, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&a.Pix[idx]))
	}
	C.glPixelStorei(C.GL_UNPACK_ROW_LENGTH, 0)
	return r.tex
}

func (r *Renderer) addRect(op *render.Op, scale float32) {
	rc := op.Rect
	if rc.W <= 0 || rc.H <= 0 {
		return
	}
	mode := float32(vertexRect)
	stroke := op.Stroke * scale
	if op.Kind == render.RectStroke && stroke <= 0 {
		stroke = scale
	}
	pad := float32(1.5)
	x0 := rc.X*scale - pad
	y0 := rc.Y*scale - pad
	x1 := rc.Right()*scale + pad
	y1 := rc.Bottom()*scale + pad
	cx := (rc.X + rc.W/2) * scale
	cy := (rc.Y + rc.H/2) * scale
	hx := rc.W * scale / 2
	hy := rc.H * scale / 2
	rad := op.Radius * scale
	col := color(op.Color)

	r.quad(x0, y0, x1, y1, 0, 0, 0, 0, col, cx, cy, hx, hy, rad, mode, stroke)
}

func (r *Renderer) addGlyphs(op *render.Op, scale float32) {
	for _, g := range op.Glyphs {
		if g.W <= 0 || g.H <= 0 {
			continue
		}
		col := color(g.Color)
		r.quad(g.X*scale, g.Y*scale, (g.X+g.W)*scale, (g.Y+g.H)*scale,
			g.U0, g.V0, g.U1, g.V1, col, 0, 0, 0, 0, 0, vertexText, 0)
	}
}

// syncImage uploads an image texture on first use or when its version changes.
func (r *Renderer) syncImage(img *render.Image) C.GLuint {
	if img == nil || img.Width <= 0 || img.Height <= 0 || len(img.Pix) < img.Width*img.Height*4 {
		return 0
	}
	gi := r.images[img]
	if gi == nil {
		gi = &gpuImage{}
		r.images[img] = gi
	}
	if gi.tex != 0 && gi.version == img.Version {
		return gi.tex
	}
	if gi.tex == 0 {
		C.glGenTextures(1, &gi.tex)
	}
	C.glActiveTexture(C.GL_TEXTURE1)
	C.glBindTexture(C.GL_TEXTURE_2D, gi.tex)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
	C.glPixelStorei(C.GL_UNPACK_ALIGNMENT, 1)
	C.glPixelStorei(C.GL_UNPACK_ROW_LENGTH, 0)
	C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GL_RGBA,
		C.GLsizei(img.Width), C.GLsizei(img.Height), 0,
		C.GL_RGBA, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&img.Pix[0]))
	C.glActiveTexture(C.GL_TEXTURE0)
	gi.version = img.Version
	return gi.tex
}

func (r *Renderer) addImage(op *render.Op, scale float32) {
	rc := op.Rect
	if rc.W <= 0 || rc.H <= 0 {
		return
	}
	pad := float32(1.0)
	x0 := rc.X*scale - pad
	y0 := rc.Y*scale - pad
	x1 := rc.Right()*scale + pad
	y1 := rc.Bottom()*scale + pad
	cx := (rc.X + rc.W/2) * scale
	cy := (rc.Y + rc.H/2) * scale
	hx := rc.W * scale / 2
	hy := rc.H * scale / 2
	rad := op.Radius * scale
	col := color(op.Color)
	u0, v0 := op.UV.X, op.UV.Y
	u1, v1 := op.UV.X+op.UV.W, op.UV.Y+op.UV.H
	r.quad(x0, y0, x1, y1, u0, v0, u1, v1, col, cx, cy, hx, hy, rad, vertexImage, 0)
}

func (r *Renderer) addTriangles(op *render.Op, scale float32) {
	// Append vertices directly: constructing a []render.Vertex per triangle
	// allocated once per triangle in custom art.
	for _, t := range op.Tris {
		ca := color(t.A.C)
		r.verts = append(r.verts, vertex{
			X: t.A.P.X * scale, Y: t.A.P.Y * scale,
			R: ca[0], G: ca[1], B: ca[2], A: ca[3], Mode: vertexSolid,
		})
		cb := color(t.B.C)
		r.verts = append(r.verts, vertex{
			X: t.B.P.X * scale, Y: t.B.P.Y * scale,
			R: cb[0], G: cb[1], B: cb[2], A: cb[3], Mode: vertexSolid,
		})
		cc := color(t.C.C)
		r.verts = append(r.verts, vertex{
			X: t.C.P.X * scale, Y: t.C.P.Y * scale,
			R: cc[0], G: cc[1], B: cc[2], A: cc[3], Mode: vertexSolid,
		})
	}
}

func (r *Renderer) quad(x0, y0, x1, y1, u0, v0, u1, v1 float32, col [4]float32, cx, cy, hx, hy, radius, mode, stroke float32) {
	r.verts = append(r.verts,
		vertex{X: x0, Y: y0, U: u0, V: v0, R: col[0], G: col[1], B: col[2], A: col[3], CX: cx, CY: cy, HX: hx, HY: hy, Radius: radius, Mode: mode, Stroke: stroke},
		vertex{X: x1, Y: y0, U: u1, V: v0, R: col[0], G: col[1], B: col[2], A: col[3], CX: cx, CY: cy, HX: hx, HY: hy, Radius: radius, Mode: mode, Stroke: stroke},
		vertex{X: x1, Y: y1, U: u1, V: v1, R: col[0], G: col[1], B: col[2], A: col[3], CX: cx, CY: cy, HX: hx, HY: hy, Radius: radius, Mode: mode, Stroke: stroke},
		vertex{X: x0, Y: y0, U: u0, V: v0, R: col[0], G: col[1], B: col[2], A: col[3], CX: cx, CY: cy, HX: hx, HY: hy, Radius: radius, Mode: mode, Stroke: stroke},
		vertex{X: x1, Y: y1, U: u1, V: v1, R: col[0], G: col[1], B: col[2], A: col[3], CX: cx, CY: cy, HX: hx, HY: hy, Radius: radius, Mode: mode, Stroke: stroke},
		vertex{X: x0, Y: y1, U: u0, V: v1, R: col[0], G: col[1], B: col[2], A: col[3], CX: cx, CY: cy, HX: hx, HY: hy, Radius: radius, Mode: mode, Stroke: stroke},
	)
}

func (r *Renderer) flush() {
	if len(r.verts) == 0 {
		return
	}
	C.glBufferData(C.GL_ARRAY_BUFFER,
		C.GLsizeiptr(len(r.verts))*C.GLsizeiptr(unsafe.Sizeof(vertex{})),
		unsafe.Pointer(&r.verts[0]), C.GL_STREAM_DRAW)
	C.glDrawArrays(C.GL_TRIANGLES, 0, C.GLsizei(len(r.verts)))
	r.stats.DrawCalls++
	r.stats.Vertices += len(r.verts)
	r.verts = r.verts[:0]
}

// Capture reads the current framebuffer as top-down RGBA. Call it after Render
// and before SwapBuffers.
func (r *Renderer) Capture(w, h int) ([]byte, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("gpu: invalid capture size %dx%d", w, h)
	}
	C.glFinish()
	// Clear any stale error, then prefer the back buffer. Pbuffers can be
	// single-buffered, in which case GL_FRONT is the readable buffer.
	for C.glGetError() != C.GL_NO_ERROR {
	}
	buf := make([]byte, w*h*4)
	C.glPixelStorei(C.GL_PACK_ALIGNMENT, 1)
	C.glReadBuffer(C.GL_BACK)
	C.glReadPixels(0, 0, C.GLsizei(w), C.GLsizei(h), C.GL_RGBA, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&buf[0]))
	if C.glGetError() != C.GL_NO_ERROR {
		for C.glGetError() != C.GL_NO_ERROR {
		}
		C.glReadBuffer(C.GL_FRONT)
		C.glReadPixels(0, 0, C.GLsizei(w), C.GLsizei(h), C.GL_RGBA, C.GL_UNSIGNED_BYTE, unsafe.Pointer(&buf[0]))
		if err := C.glGetError(); err != C.GL_NO_ERROR {
			return nil, fmt.Errorf("gpu: read pixels failed (0x%x)", uint32(err))
		}
	}
	// GL rows are bottom-up; images and screenshots are top-down.
	stride := w * 4
	tmp := make([]byte, stride)
	for y := 0; y < h/2; y++ {
		a := y * stride
		b := (h - 1 - y) * stride
		copy(tmp, buf[a:a+stride])
		copy(buf[a:a+stride], buf[b:b+stride])
		copy(buf[b:b+stride], tmp)
	}
	return buf, nil
}

type scissorRect struct {
	x, y, w, h int32
}

func (s scissorRect) empty() bool { return s.w <= 0 || s.h <= 0 }

func toScissor(r core.Rect, scale float32, pw, ph int) scissorRect {
	// Expand by one pixel so antialiased geometry is not clipped by its own
	// widget bounds.
	x0 := int32(math.Floor(float64(r.X*scale))) - 1
	y0 := int32(math.Floor(float64(r.Y*scale))) - 1
	x1 := int32(math.Ceil(float64(r.Right()*scale))) + 1
	y1 := int32(math.Ceil(float64(r.Bottom()*scale))) + 1
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > int32(pw) {
		x1 = int32(pw)
	}
	if y1 > int32(ph) {
		y1 = int32(ph)
	}
	if x1 <= x0 || y1 <= y0 {
		return scissorRect{}
	}
	return scissorRect{x: x0, y: int32(ph) - y1, w: x1 - x0, h: y1 - y0}
}

func color(c core.Color) [4]float32 {
	return [4]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
}

func goString(p *C.char) string {
	if p == nil {
		return ""
	}
	return C.GoString(p)
}

func trimLog(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
