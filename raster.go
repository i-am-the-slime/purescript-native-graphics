package graphics

import (
	"github.com/i-am-the-slime/purescript-native-graphics/drawing"
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
	"image"
	"image/color"
	"math"
	"sync"
)

// canvas's path intersection code uses process-global scratch state.
// Keep independent raster operations isolated even when callers render concurrently.
var rasterMutex sync.Mutex

type rasterLayer struct {
	canvas  *canvas.Canvas
	context *canvas.Context
}
type rasterState struct {
	width, height float64
	layers        []rasterLayer
	stack         []int
	recipe        []drawing.Composite
}

func newRaster(w, h float64, d *drawing.Drawing) *rasterState {
	s := &rasterState{width: w, height: h, stack: []int{0}, recipe: d.Composite, layers: make([]rasterLayer, len(d.Layers))}
	for i := range s.layers {
		c := canvas.New(w, h)
		s.layers[i] = rasterLayer{c, newCtx(c)}
	}
	return s
}
func newCtx(c *canvas.Canvas) *canvas.Context {
	ctx := canvas.NewContext(c)
	ctx.SetCoordSystem(canvas.CartesianIV)
	return ctx
}
func (s *rasterState) active() *canvas.Context { return s.layers[s.stack[len(s.stack)-1]].context }
func (s *rasterState) compose() *image.RGBA {
	images := make([]*image.RGBA, len(s.layers))
	for i, l := range s.layers {
		images[i] = rasterizer.Draw(l.canvas, canvas.DPMM(1), canvas.DefaultColorSpace)
	}
	out := image.NewRGBA(image.Rect(0, 0, int(s.width), int(s.height)))
	for _, p := range s.recipe {
		var mask *image.RGBA
		if p.Mask >= 0 {
			mask = images[p.Mask]
		}
		composite(out, images[p.Source], mask, p.InvertMask, p.Blend)
	}
	return out
}
func Rasterize(w, h int, d *drawing.Drawing) *image.RGBA {
	rasterMutex.Lock()
	defer rasterMutex.Unlock()
	s := newRaster(float64(w), float64(h), d)
	s.run(d.Commands, d)
	return s.compose()
}
func rgba(a []float64) color.NRGBA {
	return color.NRGBA{uint8(a[0]), uint8(a[1]), uint8(a[2]), uint8(a[3])}
}
func matching(commands []drawing.Command, start, push, pop int) int {
	depth := 0
	for i := start; i < len(commands); i++ {
		switch commands[i].Kind {
		case push:
			depth++
		case pop:
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return len(commands)
}
func transformBalance(commands []drawing.Command) int {
	n := 0
	for _, c := range commands {
		if c.Kind == 5 {
			n++
		}
		if c.Kind == 6 {
			n--
		}
	}
	return n
}
func (s *rasterState) run(commands []drawing.Command, d *drawing.Drawing) {
	suppressed := 0
	for i := 0; i < len(commands); i++ {
		c := commands[i]
		a := c.Args
		if suppressed > 0 {
			if c.Kind == 5 {
				suppressed++
			}
			if c.Kind == 6 {
				suppressed--
			}
			continue
		}
		ctx := s.active()
		switch c.Kind {
		case 1:
			ctx.SetFillColor(rgba(a))
			ctx.SetStrokeColor(color.Transparent)
			ctx.DrawPath(0, 0, buildPath(c.Path))
		case 2:
			ctx.SetFillColor(color.Transparent)
			ctx.SetStrokeColor(rgba(a))
			ctx.SetStrokeWidth(a[4])
			ctx.SetStrokeJoiner(joinFromTag(int(a[5])))
			ctx.SetStrokeCapper(capFromTag(int(a[6])))
			ctx.DrawPath(0, 0, buildPath(c.Path))
		case 3:
			ctx.SetFillColor(rgba(a))
			ctx.SetStrokeColor(rgba(a[4:]))
			ctx.SetStrokeWidth(a[8])
			ctx.SetStrokeJoiner(joinFromTag(int(a[9])))
			ctx.SetStrokeCapper(capFromTag(int(a[10])))
			ctx.DrawPath(0, 0, buildPath(c.Path))
		case 4:
			drawTextCanvas(ctx, a[0], a[1], c.Text, c.Font, a[2], rgba(a[6:]), int(a[3]), int(a[4]), a[5])
		case 5:
			if a[2] == 0 || a[3] == 0 {
				suppressed = 1
				continue
			}
			for _, l := range s.layers {
				l.context.Push()
				l.context.Translate(a[0], a[1])
				l.context.Scale(a[2], a[3])
			}
		case 6:
			for _, l := range s.layers {
				l.context.Pop()
			}
		case 7, 18, 11:
			pop := 8
			if c.Kind == 18 {
				pop = 19
			}
			if c.Kind == 11 {
				pop = 12
			}
			end := matching(commands, i+1, c.Kind, pop)
			body := commands[i+1 : end]
			// A clipping span that closes an enclosing transform cannot be captured
			// independently. Its path remains available to nested balanced captures.
			if c.Kind == 7 && transformBalance(body) < 0 {
				continue
			}
			sub := newRaster(s.width, s.height, d)
			for k, l := range sub.layers {
				l.context.SetView(s.layers[k].context.View())
			}
			sub.run(body, d)
			img := sub.compose()
			if c.Kind == 18 {
				if sigma := a[0] * viewScale(ctx.View()); sigma >= 0.5 {
					blurRGBA(img, sigma)
				}
			}
			if c.Kind == 11 {
				for k := range img.Pix {
					img.Pix[k] = uint8(float64(img.Pix[k]) * a[0])
				}
			}
			if c.Kind == 7 {
				mc := canvas.New(s.width, s.height)
				m := newCtx(mc)
				m.SetView(ctx.View())
				m.SetFillColor(color.White)
				m.SetStrokeColor(color.Transparent)
				if a[0] != 0 {
					m.SetFillRule(canvas.EvenOdd)
				}
				m.DrawPath(0, 0, buildPath(c.Path))
				applyAlphaMask(img, rasterizer.Draw(mc, canvas.DPMM(1), canvas.DefaultColorSpace))
			}
			ctx.Push()
			ctx.ResetView()
			ctx.DrawImage(0, 0, img, canvas.DPMM(1))
			ctx.Pop()
			i = end
		case 8, 12, 19:
		case 13:
			s.stack = append(s.stack, int(a[0]))
		case 14:
			if len(s.stack) > 1 {
				s.stack = s.stack[:len(s.stack)-1]
			}
		case 15:
			scale := math.Min(s.width/a[2], s.height/a[3])
			tx := (s.width-a[2]*scale)/2 - a[0]*scale
			ty := (s.height-a[3]*scale)/2 - a[1]*scale
			for _, l := range s.layers {
				l.context.ResetView()
				l.context.Translate(tx, ty)
				l.context.Scale(scale, scale)
			}
		case 16:
			ctx.Push()
			ctx.ResetView()
			ctx.SetFillColor(rgba(a))
			ctx.SetStrokeColor(color.Transparent)
			ctx.DrawPath(0, 0, canvas.Rectangle(s.width, s.height))
			ctx.Pop()
		case 17:
			drawCirclePattern(ctx, a[0], a[1], a[2], a[3], rgba(a[4:]), rgba(a[8:]), a[12], a[13], a[14], a[15])
		case 20:
			drawLineLatticeCanvas(ctx, s.width, s.height, a)
		default:
			panic("native graphics: unknown drawing primitive")
		}
	}
}

// composite executes one generic masked source-over or alpha inversion pass.
func composite(dst, src, mask *image.RGBA, inverted bool, blend int) {
	for i := 0; i < len(dst.Pix); i += 4 {
		coverage := uint32(255)
		if mask != nil {
			coverage = uint32(mask.Pix[i+3])
			if inverted {
				coverage = 255 - coverage
			}
		}
		alpha := uint32(src.Pix[i+3]) * coverage / 255
		if alpha == 0 {
			continue
		}
		for c := 0; c < 3; c++ {
			d := uint32(dst.Pix[i+c])
			if blend == 1 {
				dst.Pix[i+c] = uint8((d*(255-alpha) + (255-d)*alpha) / 255)
			} else {
				dst.Pix[i+c] = clamp255(uint32(src.Pix[i+c])*coverage/255 + d*(255-alpha)/255)
			}
		}
		dst.Pix[i+3] = clamp255(alpha + uint32(dst.Pix[i+3])*(255-alpha)/255)
	}
}
