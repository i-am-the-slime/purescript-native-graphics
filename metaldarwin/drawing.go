//go:build darwin && !js

package metaldarwin

import (
	"math"

	"github.com/i-am-the-slime/purescript-native-graphics/drawing"
)

// Render consumes only generic drawing primitives. Prepare must precede FrameBegin;
// multiple Render calls may contribute to that frame before FrameEnd composes it.
func Render(frame *drawing.Drawing, dstW, dstH float64) {
	render(frame, dstW, dstH, MatIdentity(), false)
}

// RenderOverlay aspect-fits frame coordinates without copying the command stream.
func RenderOverlay(frame *drawing.Drawing, dstW, dstH, frameW, frameH float64) {
	scale := math.Min(dstW/frameW, dstH/frameH)
	transform := MatNew(float32(scale), 0, 0, float32(scale),
		float32((dstW-frameW*scale)/2), float32((dstH-frameH*scale)/2))
	render(frame, dstW, dstH, transform, true)
}

func render(frame *drawing.Drawing, dstW, dstH float64, initial Mat3, viewportApplied bool) {
	r := NewRenderer()
	if viewportApplied {
		r.PushTransform(initial)
	}
	r.SelectLayer(0)
	layers := []int{0}
	var path []float32
	for _, command := range frame.Commands {
		a := command.Args
		path = path[:0]
		if len(command.Path) > 0 {
			for _, value := range command.Path {
				path = append(path, float32(value))
			}
		}
		switch command.Kind {
		case 1:
			cr, cg, cb, ca := drawingColor(a)
			r.FillPath(path, cr, cg, cb, ca)
		case 2:
			cr, cg, cb, ca := drawingColor(a)
			r.StrokePath(path, float32(a[4])*r.CurrentScale(), int(a[5]), int(a[6]), cr, cg, cb, ca)
		case 3:
			fr, fg, fb, fa := drawingColor(a)
			sr, sg, sb, sa := drawingColor(a[4:])
			r.FillPath(path, fr, fg, fb, fa)
			r.StrokePath(path, float32(a[8])*r.CurrentScale(), int(a[9]), int(a[10]), sr, sg, sb, sa)
		case 4:
			cr, cg, cb, ca := drawingColor(a[6:])
			r.DrawText(float32(a[0]), float32(a[1]), command.Text, float32(a[2])*r.CurrentScale(),
				int(a[3]), int(a[4]), command.FontID, float32(a[5]), cr, cg, cb, ca)
		case 5:
			r.PushTransform(MatMul(MatTranslate(float32(a[0]), float32(a[1])), MatScale(float32(a[2]), float32(a[3]))))
		case 6:
			r.PopTransform()
		case 7:
			r.PushClip(path, a[0] != 0)
		case 8:
			r.PopClip()
		case 11:
			r.PushAlpha(float32(a[0]))
		case 12:
			r.PopAlpha()
		case 13:
			layers = append(layers, int(a[0]))
			r.SelectLayer(layers[len(layers)-1])
		case 14:
			if len(layers) > 1 {
				layers = layers[:len(layers)-1]
				r.SelectLayer(layers[len(layers)-1])
			}
		case 15:
			scale := math.Min(dstW/a[2], dstH/a[3])
			offX, offY := (dstW-a[2]*scale)/2, (dstH-a[3]*scale)/2
			if viewportApplied {
				r.PopTransform()
			}
			r.PushTransform(MatNew(float32(scale), 0, 0, float32(scale), float32(offX-a[0]*scale), float32(offY-a[1]*scale)))
			viewportApplied = true
		case 16:
			// Clear is carried in Drawing.Clear and applied by FrameBegin.
		case 17:
			br, bg, bb, ba := drawingColor(a[4:])
			ir, ig, ib, ia := drawingColor(a[8:])
			r.BackgroundDots(float32(a[0]), float32(a[1]), float32(a[2]), float32(a[3]), float32(a[12]), float32(a[13]), float32(a[14]), float32(a[15]), br, bg, bb, ba, ir, ig, ib, ia)
		case 18:
			r.PushBlur(float32(a[0]) * r.CurrentScale())
		case 19:
			r.PopBlur()
		case 20:
			r.LineLattice(a, dstW, dstH)
		}
	}
	r.Flush()
}

func drawingColor(values []float64) (float32, float32, float32, float32) {
	return float32(values[0]) / 255, float32(values[1]) / 255, float32(values[2]) / 255, float32(values[3]) / 255
}

// LineLattice draws a screen-anchored lattice inside the transformed rectangle.
// All pitch, weight, frequency, color, and anchor choices belong to the caller.
func (r *Renderer) LineLattice(args []float64, dstW, dstH float64) {
	r.Flush()
	m := r.top()
	x, y, w, h := float32(args[0]), float32(args[1]), float32(args[2]), float32(args[3])
	x0, y0 := m.apply(x, y)
	x1, y1 := m.apply(x+w, y)
	x2, y2 := m.apply(x+w, y+h)
	x3, y3 := m.apply(x, y+h)
	vertices := []float32{x0, y0, x0, y0, x1, y1, x1, y1, x2, y2, x2, y2, x0, y0, x0, y0, x2, y2, x2, y2, x3, y3, x3, y3}
	pitch := args[12] * math.Min(dstW, dstH)
	major := pitch * args[13]
	if pitch <= 0 || major <= 0 {
		return
	}
	ox, oy := math.Mod(args[16]*dstW, major), math.Mod(args[17]*dstH, major)
	br, bg, bb, ba := drawingColor(args[4:])
	ir, ig, ib, ia := drawingColor(args[8:])
	uniform := []float32{float32(pitch), float32(args[13]), float32(args[14]), float32(args[15]), float32(ox), float32(oy), 0, 0, br, bg, bb, ba * r.alpha(), ir, ig, ib, ia * r.alpha()}
	drawLattice(vertices, uniform)
}
