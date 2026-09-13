//go:build darwin && !js

package metaldarwin

import "math"

type Mat3 struct {
	a, b, c, d, tx, ty float32
}

func MatIdentity() Mat3              { return Mat3{1, 0, 0, 1, 0, 0} }
func MatTranslate(x, y float32) Mat3 { return Mat3{1, 0, 0, 1, x, y} }
func MatScale(sx, sy float32) Mat3   { return Mat3{sx, 0, 0, sy, 0, 0} }

// MatNew builds a Mat3 from its six components.  apply(x,y) = (a*x+c*y+tx, b*x+d*y+ty).
func MatNew(a, b, c, d, tx, ty float32) Mat3 { return Mat3{a, b, c, d, tx, ty} }

func MatMul(m, n Mat3) Mat3 {
	return Mat3{
		a:  m.a*n.a + m.c*n.b,
		b:  m.b*n.a + m.d*n.b,
		c:  m.a*n.c + m.c*n.d,
		d:  m.b*n.c + m.d*n.d,
		tx: m.a*n.tx + m.c*n.ty + m.tx,
		ty: m.b*n.tx + m.d*n.ty + m.ty,
	}
}

func (m Mat3) apply(x, y float32) (float32, float32) {
	return m.a*x + m.c*y + m.tx, m.b*x + m.d*y + m.ty
}

const (
	BlendNormal     = 0
	BlendDifference = 1
)

type clipEntry struct {
	draws   [][]float32
	evenOdd bool
}

type Renderer struct {
	xformStack []Mat3
	alphaStack []float32
	blendStack []int
	clipStack  []clipEntry
	blurSigmas []float32
	batch      []float32
	rrectBatch []float32
}

func NewRenderer() *Renderer {
	return &Renderer{
		xformStack: []Mat3{MatIdentity()},
		alphaStack: []float32{1.0},
		blendStack: []int{BlendNormal},
	}
}

// Flush before changing blend state so queued geometry keeps its original mode.
func (r *Renderer) PushBlend(mode int) {
	r.Flush()
	r.blendStack = append(r.blendStack, mode)
}
func (r *Renderer) PopBlend() {
	r.Flush()
	if len(r.blendStack) > 1 {
		r.blendStack = r.blendStack[:len(r.blendStack)-1]
	}
}
func (r *Renderer) blend() int { return r.blendStack[len(r.blendStack)-1] }

// SelectLayer flushes pending draws, switches the active offscreen, then
// rebuilds the active clip stack in that layer's freshly-cleared stencil.
func (r *Renderer) SelectLayer(t int) {
	r.Flush()
	if !selectLayer(t) {
		return
	}
	for depth, clip := range r.clipStack {
		r.installClip(clip, depth+1, true)
	}
}

func (r *Renderer) installClip(clip clipEntry, depth int, pushing bool) {
	mode := clipModeSet
	if clip.evenOdd {
		mode = clipModeToggle
	} else if !pushing {
		mode = clipModeClear
	}
	if depth <= clipDepthLimit() {
		for _, verts := range clip.draws {
			drawClipPath(verts, depth, mode)
		}
	}
	if pushing {
		setClipDepth(depth)
	} else {
		setClipDepth(depth - 1)
	}
}

// PushClip assigns one stencil bit per nesting level. Non-zero clips set that
// bit; even-odd clips toggle it once per contour. Parent bits remain intact,
// so popping an inner clip restores the outer clip instead of punching a hole
// through it.
func (r *Renderer) PushClip(tokens []float32, evenOdd bool) {
	r.Flush()
	clip := clipEntry{draws: r.buildClipDraws(tokens, evenOdd), evenOdd: evenOdd}
	r.clipStack = append(r.clipStack, clip)
	r.installClip(clip, len(r.clipStack), true)
}

func (r *Renderer) PopClip() {
	r.Flush()
	if len(r.clipStack) == 0 {
		setClipDepth(0)
		return
	}
	depth := len(r.clipStack)
	clip := r.clipStack[depth-1]
	r.installClip(clip, depth, false)
	r.clipStack = r.clipStack[:depth-1]
}

func (r *Renderer) top() Mat3 { return r.xformStack[len(r.xformStack)-1] }
func (r *Renderer) alpha() float32 {
	return r.alphaStack[len(r.alphaStack)-1]
}

// CurrentScale returns the X-axis scale of the active affine transform.
func (r *Renderer) CurrentScale() float32 {
	m := r.top()
	return float32(math.Hypot(float64(m.a), float64(m.b)))
}

func (r *Renderer) PushTransform(m Mat3) {
	r.xformStack = append(r.xformStack, MatMul(r.top(), m))
}
func (r *Renderer) PopTransform() { r.xformStack = r.xformStack[:len(r.xformStack)-1] }

func (r *Renderer) PushAlpha(a float32) { r.alphaStack = append(r.alphaStack, r.alpha()*a) }
func (r *Renderer) PopAlpha()           { r.alphaStack = r.alphaStack[:len(r.alphaStack)-1] }

// PushBlur captures the outermost group and applies sigma in view points.
// Nested groups pair without opening another capture.
func (r *Renderer) PushBlur(sigma float32) {
	r.blurSigmas = append(r.blurSigmas, sigma)
	if len(r.blurSigmas) == 1 {
		r.Flush()
		blurBegin()
		for depth, clip := range r.clipStack {
			r.installClip(clip, depth+1, true)
		}
	}
}

func (r *Renderer) PopBlur() {
	if len(r.blurSigmas) == 0 {
		return
	}
	sigma := r.blurSigmas[0]
	r.blurSigmas = r.blurSigmas[:len(r.blurSigmas)-1]
	if len(r.blurSigmas) == 0 {
		r.Flush()
		blurEnd(sigma)
		for depth, clip := range r.clipStack {
			r.installClip(clip, depth+1, true)
		}
	}
}

func (r *Renderer) FillRect(x, y, w, h, cr, cg, cb, ca float32) {
	if len(r.rrectBatch) > 0 {
		r.Flush()
	}
	m := r.top()
	a := ca * r.alpha()
	x0, y0 := m.apply(x, y)
	x1, y1 := m.apply(x+w, y)
	x2, y2 := m.apply(x+w, y+h)
	x3, y3 := m.apply(x, y+h)
	r.batch = append(r.batch,
		x0, y0, cr, cg, cb, a,
		x1, y1, cr, cg, cb, a,
		x2, y2, cr, cg, cb, a,
		x0, y0, cr, cg, cb, a,
		x2, y2, cr, cg, cb, a,
		x3, y3, cr, cg, cb, a,
	)
}

func (r *Renderer) Flush() {
	if len(r.batch) > 0 {
		if r.blend() == BlendDifference {
			drawTrianglesDiff(r.batch)
		} else {
			drawTriangles(r.batch)
		}
		r.batch = r.batch[:0]
	}
	if len(r.rrectBatch) > 0 {
		drawRRect(r.rrectBatch)
		r.rrectBatch = r.rrectBatch[:0]
	}
}

// BackgroundDots paints the rect (vx,vy,vw,vh) — in world space — with the
// bg colour and a halftone dot grid (tile spacing, dotR radius). Goes through
// the bgdots pipeline directly; not batched with other geometry.
func (r *Renderer) BackgroundDots(vx, vy, vw, vh, tile, dotR, ox, oy, bgR, bgG, bgB, bgA, dotRr, dotG, dotB, dotA float32) {
	r.Flush()
	m := r.top()
	x0, y0 := m.apply(vx, vy)
	x1, y1 := m.apply(vx+vw, vy)
	x2, y2 := m.apply(vx+vw, vy+vh)
	x3, y3 := m.apply(vx, vy+vh)
	a := r.alpha()
	verts := []float32{
		x0, y0, vx, vy,
		x1, y1, vx + vw, vy,
		x2, y2, vx + vw, vy + vh,
		x0, y0, vx, vy,
		x2, y2, vx + vw, vy + vh,
		x3, y3, vx, vy + vh,
	}
	uni := []float32{
		tile, dotR, ox, oy,
		bgR, bgG, bgB, bgA * a,
		dotRr, dotG, dotB, dotA * a,
	}
	drawBgDots(verts, uni)
}

func (r *Renderer) FillRoundedRect(x, y, w, h, radius, softness, cr, cg, cb, ca float32) {
	r.rrect(x, y, w, h, radius, softness, 0, cr, cg, cb, ca)
}

func (r *Renderer) StrokeRoundedRect(x, y, w, h, radius, thickness, cr, cg, cb, ca float32) {
	r.rrect(x, y, w, h, radius, 0, thickness, cr, cg, cb, ca)
}

func (r *Renderer) rrect(x, y, w, h, radius, softness, thickness, cr, cg, cb, ca float32) {
	if len(r.batch) > 0 {
		r.Flush()
	}
	m := r.top()
	a := ca * r.alpha()
	pad := softness + thickness
	x0, y0 := x-pad, y-pad
	x1, y1 := x+w+pad, y+h+pad
	hx, hy := w*0.5, h*0.5
	lx0, ly0 := -hx-pad, -hy-pad
	lx1, ly1 := hx+pad, hy+pad

	add := func(sx, sy, lx, ly float32) {
		tx, ty := m.apply(sx, sy)
		r.rrectBatch = append(r.rrectBatch,
			tx, ty, lx, ly, hx, hy, radius, softness, thickness, cr, cg, cb, a)
	}
	add(x0, y0, lx0, ly0)
	add(x1, y0, lx1, ly0)
	add(x1, y1, lx1, ly1)
	add(x0, y0, lx0, ly0)
	add(x1, y1, lx1, ly1)
	add(x0, y1, lx0, ly1)
}

func (r *Renderer) FillCapsule(ax, ay, bx, by, radius, softness, cr, cg, cb, ca float32) {
	r.capsule(ax, ay, bx, by, radius, softness, 0, cr, cg, cb, ca)
}

func (r *Renderer) StrokeCapsule(ax, ay, bx, by, radius, thickness, cr, cg, cb, ca float32) {
	r.capsule(ax, ay, bx, by, radius, 0, thickness, cr, cg, cb, ca)
}

func (r *Renderer) capsule(ax, ay, bx, by, radius, softness, thickness, cr, cg, cb, ca float32) {
	if len(r.batch) > 0 {
		r.Flush()
	}
	m := r.top()
	alpha := ca * r.alpha()

	dx, dy := bx-ax, by-ay
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	var ux, uy float32 = 1, 0
	if length > 1e-6 {
		ux, uy = dx/length, dy/length
	}
	nx, ny := -uy, ux

	cx, cy := (ax+bx)*0.5, (ay+by)*0.5
	hx := length*0.5 + radius
	hy := radius
	pad := softness + thickness
	lhx := hx + pad
	lhy := hy + pad

	add := func(lx, ly float32) {
		wx := cx + ux*lx + nx*ly
		wy := cy + uy*lx + ny*ly
		sx, sy := m.apply(wx, wy)
		r.rrectBatch = append(r.rrectBatch,
			sx, sy, lx, ly, hx, hy, radius, softness, thickness, cr, cg, cb, alpha)
	}

	add(-lhx, -lhy)
	add(lhx, -lhy)
	add(lhx, lhy)
	add(-lhx, -lhy)
	add(lhx, lhy)
	add(-lhx, lhy)
}
