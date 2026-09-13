package graphics

import (
	"fmt"
	"image/color"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/i-am-the-slime/purescript-native-graphics/drawing"
)

// RenderEbiten executes a generic command buffer on the Ebiten render thread.
// Overlay preserves the screen contents, but still resolves the drawing's own
// layers independently before compositing the result over the screen.
func RenderEbiten(screen *ebiten.Image, frame *drawing.Drawing, overlay bool) {
	if frame == nil {
		return
	}
	s := acquireEbitenState(screen, frame)
	for _, cmd := range frame.Commands {
		a := cmd.Args
		switch cmd.Kind {
		case 1:
			ebitenFillPath(s.activeDst(), cmd.Path, s.cam, ebitenReadColor(a))
		case 2:
			ebitenStrokePath(s.activeDst(), cmd.Path, s.cam, ebitenReadColor(a), a[4], int(a[5]), int(a[6]))
		case 3:
			dst := s.activeDst()
			p := buildEbitenPath(cmd.Path, s.cam)
			vector.FillPath(dst, p, &vector.FillOptions{}, ebitenPathOptions(ebitenReadColor(a)))
			ebitenStroke(dst, p, s.cam, ebitenReadColor(a[4:]), a[8], int(a[9]), int(a[10]))
		case 4:
			ebitenDrawText(s.activeDst(), a[0], a[1], cmd.Text, cmd.Font, a[2], ebitenReadColor(a[6:]), int(a[3]), int(a[4]), a[5], s.cam)
		case 5:
			s.pushTransform(a[0], a[1], a[2], a[3])
		case 6:
			s.popTransform()
		case 7:
			s.pushClip(cmd.Path, a[0] != 0)
		case 8, 12, 19:
			s.popGroup()
		case 11:
			s.pushGroup(a[0], 0)
		case 13:
			s.layerStack = append(s.layerStack, int(a[0]))
		case 14:
			if len(s.layerStack) > 1 {
				s.layerStack = s.layerStack[:len(s.layerStack)-1]
			}
		case 15:
			s.cam = ebitenViewportAffine(float64(s.w), float64(s.h), a[0], a[1], a[2], a[3])
		case 16:
			s.activeDst().Fill(ebitenReadColor(a))
		case 17:
			ebitenCirclePattern(s.activeDst(), s.cam, a)
		case 18:
			s.pushGroup(1, a[0]*math.Abs(s.cam.a))
		case 20:
			ebitenLineLattice(s.activeDst(), s.cam, a)
		}
	}
	for len(s.groups) > 0 {
		s.popGroup()
	}
	resolved := s.acquireImage()
	s.resolveInto(resolved, &s.root)
	if !overlay {
		c := frame.Clear
		screen.Fill(color.NRGBA{R: uint8(c[0] * 255), G: uint8(c[1] * 255), B: uint8(c[2] * 255), A: uint8(c[3] * 255)})
	}
	screen.DrawImage(resolved, nil)
	s.releaseImage(resolved)
}

// Images remain pooled across frames. Global layers bypass open groups;
// whether a layer is global and how layers combine are caller-owned decisions.
type ebitenSurface struct {
	layers []*ebiten.Image
}

type ebitenGroup struct {
	surf        ebitenSurface
	alpha, blur float64
	clip        *ebiten.Image
}

type ebitenState struct {
	w, h       int
	cam        affine
	transforms []affine
	layerStack []int
	frame      *drawing.Drawing
	root       ebitenSurface
	groups     []ebitenGroup
	groupPool  []ebitenSurface
	pool       []*ebiten.Image
	zero       *ebiten.Image
}

var cachedEbitenState *ebitenState

func acquireEbitenState(screen *ebiten.Image, frame *drawing.Drawing) *ebitenState {
	bounds := screen.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	s := cachedEbitenState
	if s != nil {
		s.releaseSurface(&s.root)
		if s.w != w || s.h != h {
			for _, img := range s.pool {
				img.Deallocate()
			}
			if s.zero != nil {
				s.zero.Deallocate()
			}
			s = nil
		}
	}
	if s == nil {
		s = &ebitenState{w: w, h: h}
		cachedEbitenState = s
	}
	s.frame = frame
	s.cam = identityAffine()
	s.transforms = s.transforms[:0]
	s.layerStack = append(s.layerStack[:0], 0)
	s.prepareSurface(&s.root)
	return s
}

func (s *ebitenState) prepareSurface(g *ebitenSurface) {
	n := len(s.frame.Layers)
	if n == 0 {
		n = 1
	}
	if cap(g.layers) < n {
		g.layers = make([]*ebiten.Image, n)
	} else {
		g.layers = g.layers[:n]
		clear(g.layers)
	}
}

func (s *ebitenState) acquireImage() *ebiten.Image {
	var img *ebiten.Image
	if n := len(s.pool); n > 0 {
		img = s.pool[n-1]
		s.pool = s.pool[:n-1]
	} else {
		img = ebiten.NewImage(s.w, s.h)
	}
	img.Clear()
	return img
}

func (s *ebitenState) releaseImage(img *ebiten.Image) {
	if img != nil {
		s.pool = append(s.pool, img)
	}
}

func (s *ebitenState) releaseSurface(g *ebitenSurface) {
	for i, img := range g.layers {
		s.releaseImage(img)
		g.layers[i] = nil
	}
}

func (s *ebitenState) zeroImage() *ebiten.Image {
	if s.zero == nil {
		s.zero = ebiten.NewImage(s.w, s.h)
	}
	return s.zero
}

func (s *ebitenState) activeDst() *ebiten.Image {
	index := s.layerStack[len(s.layerStack)-1]
	g := &s.root
	global := index >= 0 && index < len(s.frame.Layers) && s.frame.Layers[index].Global
	if !global && len(s.groups) > 0 {
		g = &s.groups[len(s.groups)-1].surf
	}
	if index < 0 || index >= len(g.layers) {
		panic("graphics: Ebiten layer index outside drawing configuration")
	}
	if g.layers[index] == nil {
		g.layers[index] = s.acquireImage()
	}
	return g.layers[index]
}

func (s *ebitenState) pushGroup(alpha, blur float64) {
	var surf ebitenSurface
	if n := len(s.groupPool); n > 0 {
		surf = s.groupPool[n-1]
		s.groupPool = s.groupPool[:n-1]
	}
	s.prepareSurface(&surf)
	s.groups = append(s.groups, ebitenGroup{surf: surf, alpha: alpha, blur: blur})
}

func (s *ebitenState) pushClip(path []float64, evenOdd bool) {
	mask := s.acquireImage()
	opts := &vector.FillOptions{}
	if evenOdd {
		opts.FillRule = vector.FillRuleEvenOdd
	}
	vector.FillPath(mask, buildEbitenPath(path, s.cam), opts, ebitenPathOptions(color.RGBA{255, 255, 255, 255}))
	s.pushGroup(1, 0)
	s.groups[len(s.groups)-1].clip = mask
}

func (s *ebitenState) popGroup() {
	if len(s.groups) == 0 {
		return
	}
	top := s.groups[len(s.groups)-1]
	s.groups = s.groups[:len(s.groups)-1]
	resolved := s.acquireImage()
	s.resolveInto(resolved, &top.surf)
	if top.blur > 0 {
		tmp := s.acquireImage()
		s.blurInto(tmp, resolved, top.blur, 1, 0)
		resolved.Clear()
		s.blurInto(resolved, tmp, top.blur, 0, 1)
		s.releaseImage(tmp)
	}
	if top.clip != nil {
		opts := &ebiten.DrawRectShaderOptions{}
		opts.Images[0], opts.Images[1] = resolved, top.clip
		opts.Uniforms = map[string]any{"Alpha": float32(top.alpha)}
		s.activeDst().DrawRectShader(s.w, s.h, ebitenClipShader.load(), opts)
	} else {
		opts := &ebiten.DrawImageOptions{}
		opts.ColorScale.ScaleAlpha(float32(top.alpha))
		s.activeDst().DrawImage(resolved, opts)
	}
	s.releaseImage(resolved)
	s.releaseImage(top.clip)
	s.releaseSurface(&top.surf)
	s.groupPool = append(s.groupPool, top.surf)
}

func (s *ebitenState) pushTransform(tx, ty, sx, sy float64) {
	s.transforms = append(s.transforms, s.cam)
	cam := s.cam
	s.cam = affine{
		a: cam.a * sx, b: cam.b * sy, c: cam.c * sx, d: cam.d * sy,
		tx: cam.a*tx + cam.b*ty + cam.tx,
		ty: cam.c*tx + cam.d*ty + cam.ty,
	}
}

func (s *ebitenState) popTransform() {
	if n := len(s.transforms); n > 0 {
		s.cam = s.transforms[n-1]
		s.transforms = s.transforms[:n-1]
	}
}

func ebitenLayer(g *ebitenSurface, index int) *ebiten.Image {
	if index >= 0 && index < len(g.layers) {
		return g.layers[index]
	}
	return nil
}

// Each recipe step reads the previously composed destination. A pooled scratch
// image avoids reading and writing the same GPU texture during masked blending.
func (s *ebitenState) resolveInto(dst *ebiten.Image, g *ebitenSurface) {
	var scratch *ebiten.Image
	for _, step := range s.frame.Composite {
		source := ebitenLayer(g, step.Source)
		if source == nil {
			continue
		}
		if step.Mask < 0 && step.Blend == 0 {
			dst.DrawImage(source, nil)
			continue
		}
		mask := ebitenLayer(g, step.Mask)
		if mask == nil {
			mask = s.zeroImage()
		}
		if scratch == nil {
			scratch = s.acquireImage()
		} else {
			scratch.Clear()
		}
		opts := &ebiten.DrawRectShaderOptions{}
		opts.Images[0], opts.Images[1], opts.Images[2] = dst, source, mask
		masked, inverted := float32(0), float32(0)
		if step.Mask >= 0 {
			masked = 1
		}
		if step.InvertMask {
			inverted = 1
		}
		opts.Uniforms = map[string]any{"Masked": masked, "InvertMask": inverted, "Blend": float32(step.Blend)}
		scratch.DrawRectShader(s.w, s.h, ebitenCompositeShader.load(), opts)
		dst.Clear()
		dst.DrawImage(scratch, nil)
	}
	s.releaseImage(scratch)
}

type ebitenShaderCache struct {
	once   sync.Once
	source string
	shader *ebiten.Shader
}

func (s *ebitenShaderCache) load() *ebiten.Shader {
	s.once.Do(func() {
		var err error
		s.shader, err = ebiten.NewShader([]byte(s.source))
		if err != nil {
			panic(err)
		}
	})
	return s.shader
}

var ebitenCompositeShader = ebitenShaderCache{source: `
//kage:unit pixels
package main
var Masked float
var InvertMask float
var Blend float
func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	base := imageSrc0UnsafeAt(srcPos)
	fg := imageSrc1UnsafeAt(srcPos)
	mask := imageSrc2UnsafeAt(srcPos).a
	mask = mix(mask, 1.0-mask, InvertMask)
	fg *= mix(1.0, mask, Masked)
	if Blend == 1.0 {
		return vec4(mix(base.rgb, vec3(1.0)-base.rgb, fg.a), fg.a+base.a*(1.0-fg.a))
	}
	return fg + base*(1.0-fg.a)
}
`}

var ebitenClipShader = ebitenShaderCache{source: `
//kage:unit pixels
package main
var Alpha float
func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	return imageSrc0UnsafeAt(srcPos)*imageSrc1UnsafeAt(srcPos).a*Alpha
}
`}

func (s *ebitenState) blurInto(dst, src *ebiten.Image, radius, dx, dy float64) {
	opts := &ebiten.DrawRectShaderOptions{}
	opts.Images[0] = src
	opts.Uniforms = map[string]any{
		"Sigma":     float32(radius),
		"Direction": []float32{float32(dx), float32(dy)},
	}
	dst.DrawRectShader(s.w, s.h, ebitenLoadBlurShader(int(math.Ceil(radius*3))), opts)
}

var ebitenBlurShaders = make(map[int]*ebiten.Shader)

// Kage requires constant loop bounds. Cache the compiled support size while
// keeping sigma uniform, so changing a filter radius retains its full kernel.
func ebitenLoadBlurShader(extent int) *ebiten.Shader {
	if shader := ebitenBlurShaders[extent]; shader != nil {
		return shader
	}
	shader, err := ebiten.NewShader([]byte(fmt.Sprintf(ebitenBlurSource, extent, extent)))
	if err != nil {
		panic(err)
	}
	ebitenBlurShaders[extent] = shader
	return shader
}

const ebitenBlurSource = `
//kage:unit pixels
package main
var Sigma float
var Direction vec2
func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	sum := vec4(0.0)
	weight := 0.0
	for i := -%d; i <= %d; i++ {
		x := float(i)
		w := exp(-0.5*x*x/(Sigma*Sigma))
		sum += imageSrc0At(srcPos+Direction*x)*w
		weight += w
	}
	return sum/weight
}
`

func ebitenReadColor(a []float64) color.RGBA {
	r, g, b, alpha := uint8(a[0]), uint8(a[1]), uint8(a[2]), uint8(a[3])
	if alpha < 255 {
		r = uint8(uint32(r) * uint32(alpha) / 255)
		g = uint8(uint32(g) * uint32(alpha) / 255)
		b = uint8(uint32(b) * uint32(alpha) / 255)
	}
	return color.RGBA{R: r, G: g, B: b, A: alpha}
}

func ebitenColorVector(c color.RGBA) []float32 {
	return []float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
}

func ebitenPatternRect(dst *ebiten.Image, cam affine, a []float64, shader *ebiten.Shader, uniforms map[string]any) {
	x0, y0 := cam.apply(a[0], a[1])
	x1, y1 := cam.apply(a[0]+a[2], a[1]+a[3])
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	w, h := int(math.Ceil(x1-x0)), int(math.Ceil(y1-y0))
	if w <= 0 || h <= 0 {
		return
	}
	opts := &ebiten.DrawRectShaderOptions{Uniforms: uniforms}
	opts.GeoM.Translate(x0, y0)
	dst.DrawRectShader(w, h, shader, opts)
}

// The caller supplies all lattice density, thickness, and anchoring choices.
func ebitenLineLattice(dst *ebiten.Image, cam affine, a []float64) {
	bounds := dst.Bounds()
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	pitch := a[12] * math.Min(w, h)
	majorPitch := pitch * a[13]
	if pitch <= 0 || majorPitch <= 0 {
		return
	}
	ebitenPatternRect(dst, cam, a, ebitenLatticeShader.load(), map[string]any{
		"TilePx": float32(pitch), "MajorEvery": float32(a[13]),
		"MajorHalf": float32(a[14]), "MinorHalf": float32(a[15]),
		"Origin":   []float32{float32(math.Mod(a[16]*w, majorPitch)), float32(math.Mod(a[17]*h, majorPitch))},
		"BgColor":  ebitenColorVector(ebitenReadColor(a[4:])),
		"InkColor": ebitenColorVector(ebitenReadColor(a[8:])),
	})
}

var ebitenLatticeShader = ebitenShaderCache{source: `
//kage:unit pixels
package main
var TilePx float
var MajorEvery float
var MajorHalf float
var MinorHalf float
var Origin vec2
var BgColor vec4
var InkColor vec4
// Derivatives are evaluated before fract, keeping coverage continuous as a
// line crosses device pixels and the minor and major lattices in phase.
func lineCoverage(c vec2, halfPx float) float {
	fw := fwidth(c)
	d := abs(fract(c+vec2(0.5))-vec2(0.5))
	dpx := d/fw
	covX := clamp(halfPx-dpx.x+0.5, 0.0, 1.0)
	covY := clamp(halfPx-dpx.y+0.5, 0.0, 1.0)
	return max(covX, covY)
}
func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	uv := dstPos.xy-Origin
	minor := lineCoverage(uv/TilePx, MinorHalf)
	major := lineCoverage(uv/(TilePx*MajorEvery), MajorHalf)
	return mix(BgColor, InkColor, max(minor, major))
}
`}

func ebitenCirclePattern(dst *ebiten.Image, cam affine, a []float64) {
	if a[12] <= 0 || cam.a == 0 || cam.d == 0 {
		return
	}
	ebitenPatternRect(dst, cam, a, ebitenCircleShader.load(), map[string]any{
		"Scale":       []float32{float32(cam.a), float32(cam.d)},
		"Translation": []float32{float32(cam.tx), float32(cam.ty)},
		"Origin":      []float32{float32(a[14]), float32(a[15])},
		"Tile":        float32(a[12]), "Radius": float32(a[13]),
		"BgColor":  ebitenColorVector(ebitenReadColor(a[4:])),
		"InkColor": ebitenColorVector(ebitenReadColor(a[8:])),
	})
}

var ebitenCircleShader = ebitenShaderCache{source: `
//kage:unit pixels
package main
var Scale vec2
var Translation vec2
var Origin vec2
var Tile float
var Radius float
var BgColor vec4
var InkColor vec4
func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	world := (dstPos.xy-Translation)/Scale-Origin
	cell := (fract(world/Tile+vec2(0.5))-vec2(0.5))*Tile
	distance := length(cell)-Radius
	pixel := max(length(fwidth(world)), 0.000001)
	coverage := clamp(0.5-distance/pixel, 0.0, 1.0)
	return mix(BgColor, InkColor, coverage)
}
`}

func ebitenViewportAffine(dstW, dstH, vx, vy, vw, vh float64) affine {
	scale := math.Min(dstW/vw, dstH/vh)
	return affine{a: scale, d: scale, tx: (dstW-vw*scale)/2 - vx*scale, ty: (dstH-vh*scale)/2 - vy*scale}
}

func ebitenApply(cam affine, x, y float64) (float32, float32) {
	x, y = cam.apply(x, y)
	return float32(x), float32(y)
}

func buildEbitenPath(buf []float64, cam affine) *vector.Path {
	p := &vector.Path{}
	for i := 0; i < len(buf); {
		switch buf[i] {
		case 1:
			x, y := ebitenApply(cam, buf[i+1], buf[i+2])
			p.MoveTo(x, y)
			i += 3
		case 2:
			x, y := ebitenApply(cam, buf[i+1], buf[i+2])
			p.LineTo(x, y)
			i += 3
		case 3:
			cx, cy := ebitenApply(cam, buf[i+1], buf[i+2])
			x, y := ebitenApply(cam, buf[i+3], buf[i+4])
			p.QuadTo(cx, cy, x, y)
			i += 5
		case 4:
			c1x, c1y := ebitenApply(cam, buf[i+1], buf[i+2])
			c2x, c2y := ebitenApply(cam, buf[i+3], buf[i+4])
			x, y := ebitenApply(cam, buf[i+5], buf[i+6])
			p.CubicTo(c1x, c1y, c2x, c2y, x, y)
			i += 7
		case 5:
			p.Close()
			i++
		default:
			return p
		}
	}
	return p
}

func ebitenFillPath(dst *ebiten.Image, buf []float64, cam affine, c color.RGBA) {
	vector.FillPath(dst, buildEbitenPath(buf, cam), &vector.FillOptions{}, ebitenPathOptions(c))
}

func ebitenStrokePath(dst *ebiten.Image, buf []float64, cam affine, c color.RGBA, width float64, join, cap int) {
	if width > 0 {
		ebitenStroke(dst, buildEbitenPath(buf, cam), cam, c, width, join, cap)
	}
}

func ebitenStroke(dst *ebiten.Image, p *vector.Path, cam affine, c color.RGBA, width float64, join, cap int) {
	if width <= 0 {
		return
	}
	vector.StrokePath(dst, p, &vector.StrokeOptions{
		Width:    float32(width * math.Abs(cam.a)),
		LineJoin: ebitenJoinFromTag(join), LineCap: ebitenCapFromTag(cap),
	}, ebitenPathOptions(c))
}

func ebitenPathOptions(c color.RGBA) *vector.DrawPathOptions {
	var cs ebiten.ColorScale
	cs.ScaleWithColor(c)
	return &vector.DrawPathOptions{AntiAlias: true, ColorScale: cs}
}

func ebitenJoinFromTag(tag int) vector.LineJoin {
	switch tag {
	case 0:
		return vector.LineJoinRound
	case 1:
		return vector.LineJoinBevel
	default:
		return vector.LineJoinMiter
	}
}

func ebitenCapFromTag(tag int) vector.LineCap {
	switch tag {
	case 1:
		return vector.LineCapRound
	case 2:
		return vector.LineCapSquare
	default:
		return vector.LineCapButt
	}
}

func ebitenDrawText(dst *ebiten.Image, x, y float64, content, family string, size float64, c color.RGBA, align, baseline int, boldOffset float64, cam affine) {
	if content == "" || size <= 0 {
		return
	}
	source := PickFontSource(family)
	if source == nil {
		return
	}
	pxSize := size * math.Abs(cam.a)
	face := &text.GoTextFace{Source: source, Size: pxSize}
	metrics := face.Metrics()
	xPx, yPx := cam.apply(x, y)
	switch baseline {
	case 1:
		yPx += metrics.CapHeight / 2
	case 2:
		yPx += metrics.HAscent
	case 3:
		yPx -= metrics.HDescent
	}
	yPx -= metrics.HAscent
	if boldOffset == 0 {
		opts := &text.DrawOptions{}
		opts.GeoM.Translate(xPx, yPx)
		opts.ColorScale.ScaleWithColor(c)
		opts.LayoutOptions.PrimaryAlign = ebitenAlignFromTag(align)
		text.Draw(dst, content, face, opts)
		return
	}
	offset := pxSize * boldOffset
	width, _ := text.Measure(content, face, 0)
	width += offset
	switch align {
	case 0:
	case 2:
		xPx -= width
	default:
		xPx -= width / 2
	}
	opts := &text.DrawOptions{}
	opts.GeoM.Translate(xPx, yPx)
	opts.ColorScale.ScaleWithColor(c)
	opts.LayoutOptions.PrimaryAlign = text.AlignStart
	text.Draw(dst, content, face, opts)
	opts.GeoM.Translate(offset, 0)
	text.Draw(dst, content, face, opts)
}

func ebitenAlignFromTag(tag int) text.Align {
	switch tag {
	case 0:
		return text.AlignStart
	case 2:
		return text.AlignEnd
	default:
		return text.AlignCenter
	}
}
