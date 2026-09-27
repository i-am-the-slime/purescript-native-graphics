package graphics

import (
	"image"
	"image/color"
	"sync"

	. "github.com/purescript-native/go-runtime"
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
	"github.com/tdewolff/font"
)

// Canvas path intersection uses process-global scratch storage. The caller owns
// the complete transaction, including callbacks from the hatch rasterizer.
var rasterMutex sync.Mutex

type rasterSurface struct {
	canvas  *canvas.Canvas
	context *canvas.Context
}

func rasterColor(v Any) color.NRGBA {
	d := v.(Dict)
	return color.NRGBA{uint8(number(d["red"]) * 255), uint8(number(d["green"]) * 255), uint8(number(d["blue"]) * 255), uint8(number(d["alpha"]) * 255)}
}
func rasterMatrix(v Any) canvas.Matrix {
	d := v.(Dict)
	return canvas.Matrix{{number(d["a"]), number(d["c"]), number(d["tx"])}, {number(d["b"]), number(d["d"]), number(d["ty"])}}
}
func rasterEffect(f func(Dict) Any) Any {
	return func(v Any) Any { return func() Any { return f(v.(Dict)) } }
}
func init() {
	g := Foreign("Native.Graphics.Raster.Native")
	g["critical"] = func(v Any) Any { return func() Any { rasterMutex.Lock(); defer rasterMutex.Unlock(); return Run(v) } }
	g["newSurface"] = rasterEffect(func(d Dict) Any {
		c := canvas.New(float64(integer(d["width"])), float64(integer(d["height"])))
		ctx := canvas.NewContext(c)
		ctx.SetCoordSystem(canvas.CartesianIV)
		return &rasterSurface{c, ctx}
	})
	g["renderSurface"] = func(v Any) Any {
		return func() Any {
			return rasterizer.Draw(v.(*rasterSurface).canvas, canvas.DPMM(1), canvas.DefaultColorSpace)
		}
	}
	g["newImage"] = rasterEffect(func(d Dict) Any { return image.NewRGBA(image.Rect(0, 0, integer(d["width"]), integer(d["height"]))) })
	g["imageLength"] = func(v Any) Any { return len(v.(*image.RGBA).Pix) }
	g["readByte"] = curry2(func(v, i Any) Any { return func() Any { return int(v.(*image.RGBA).Pix[integer(i)]) } })
	g["writeByte"] = curry3(func(v, i, b Any) Any {
		return func() Any { v.(*image.RGBA).Pix[integer(i)] = uint8(integer(b)); return nil }
	})
	g["newPath"] = func() Any { return &canvas.Path{} }
	g["moveTo"] = rasterEffect(func(d Dict) Any { d["path"].(*canvas.Path).MoveTo(number(d["x"]), number(d["y"])); return nil })
	g["lineTo"] = rasterEffect(func(d Dict) Any { d["path"].(*canvas.Path).LineTo(number(d["x"]), number(d["y"])); return nil })
	g["quadTo"] = rasterEffect(func(d Dict) Any {
		d["path"].(*canvas.Path).QuadTo(number(d["cx"]), number(d["cy"]), number(d["x"]), number(d["y"]))
		return nil
	})
	g["cubicTo"] = rasterEffect(func(d Dict) Any {
		d["path"].(*canvas.Path).CubeTo(number(d["cx1"]), number(d["cy1"]), number(d["cx2"]), number(d["cy2"]), number(d["x"]), number(d["y"]))
		return nil
	})
	g["closePath"] = func(v Any) Any { return func() Any { v.(*canvas.Path).Close(); return nil } }
	g["appendCircle"] = rasterEffect(func(d Dict) Any {
		p := d["path"].(*canvas.Path)
		*p = *p.Append(canvas.Circle(number(d["radius"])).Translate(number(d["x"]), number(d["y"])))
		return nil
	})
	g["paint"] = rasterEffect(func(d Dict) Any {
		ctx := d["surface"].(*rasterSurface).context
		ctx.SetView(rasterMatrix(d["transform"]))
		ctx.SetFillColor(rasterColor(d["fill"]))
		ctx.SetStrokeColor(rasterColor(d["stroke"]))
		ctx.SetStrokeWidth(number(d["width"]))
		ctx.SetStrokeJoiner([]canvas.Joiner{canvas.RoundJoin, canvas.BevelJoin, canvas.MiterJoin}[integer(d["join"])])
		ctx.SetStrokeCapper([]canvas.Capper{canvas.ButtCap, canvas.RoundCap, canvas.SquareCap}[integer(d["cap"])])
		rule := canvas.NonZero
		if d["evenOdd"].(bool) {
			rule = canvas.EvenOdd
		}
		ctx.SetFillRule(rule)
		ctx.DrawPath(0, 0, d["path"].(*canvas.Path))
		return nil
	})
	g["paintImage"] = rasterEffect(func(d Dict) Any {
		ctx := d["surface"].(*rasterSurface).context
		ctx.ResetView()
		ctx.DrawImage(0, 0, d["image"].(image.Image), canvas.DPMM(1))
		return nil
	})
	g["makeText"] = rasterEffect(func(d Dict) Any {
		face := lookupFont(d["font"].(string)).canvas.Face(number(d["size"]), rasterColor(d["color"]), canvas.FontRegular, canvas.FontNormal, font.NoHinting)
		text := d["text"].(string)
		metrics := face.Metrics()
		return Dict{"line": canvas.NewTextLine(face, text, canvas.Left), "width": face.TextWidth(text), "capHeight": metrics.CapHeight, "ascent": metrics.Ascent, "descent": metrics.Descent}
	})
	g["paintText"] = rasterEffect(func(d Dict) Any {
		ctx := d["surface"].(*rasterSurface).context
		ctx.SetView(rasterMatrix(d["transform"]))
		ctx.DrawText(number(d["x"]), number(d["y"]), d["line"].(*canvas.Text))
		return nil
	})
	g["paintHatch"] = rasterEffect(func(d Dict) Any {
		ctx := d["surface"].(*rasterSurface).context
		ctx.SetView(rasterMatrix(d["transform"]))
		ctx.SetStrokeColor(color.Transparent)
		ctx.SetFillRule(canvas.NonZero)
		hatch := func(x0, y0, x1, y1 float64) *canvas.Path {
			return Run(Apply(d["hatch"], Dict{"x0": x0, "y0": y0, "x1": x1, "y1": y1})).(*canvas.Path)
		}
		ctx.SetFillPattern(canvas.NewHatchPattern(rasterColor(d["color"]), 0, rasterMatrix(d["cell"]), hatch))
		ctx.DrawPath(0, 0, d["path"].(*canvas.Path))
		return nil
	})
}
