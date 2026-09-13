package graphics

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	. "github.com/purescript-native/go-runtime"
)

// This boundary marshals already-decided GPU operations. Drawing commands,
// transforms, renderer state, caches and resource-pool policy live in PureScript.
func ebitenNativeColor(v Any) color.Color {
	c := v.(Dict)
	return color.RGBA{R: uint8(integer(c["red"])), G: uint8(integer(c["green"])), B: uint8(integer(c["blue"])), A: uint8(integer(c["alpha"]))}
}
func ebitenNativePathOptions(v Any) *vector.DrawPathOptions {
	var scale ebiten.ColorScale
	scale.ScaleWithColor(ebitenNativeColor(v))
	return &vector.DrawPathOptions{AntiAlias: true, ColorScale: scale}
}
func ebitenNativePoint(v Any) (float32, float32) {
	p := v.(Dict)
	return float32(number(p["x"])), float32(number(p["y"]))
}
func init() {
	g := Foreign("Native.Graphics.Ebiten.Primitives")
	g["compositeSource"] = ebitenCompositeSource
	g["clipSource"] = ebitenClipSource
	g["circleSource"] = ebitenCircleSource
	g["latticeSource"] = ebitenLatticeSource
	g["blurSource"] = func(v Any) Any { return fmt.Sprintf(ebitenBlurSource, integer(v), integer(v)) }
	g["dimensions"] = func(v Any) Any {
		return func() Any {
			b := v.(*ebiten.Image).Bounds()
			return Dict{"width": b.Dx(), "height": b.Dy()}
		}
	}
	g["newImage"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			return ebiten.NewImage(integer(d["width"]), integer(d["height"]))
		}
	}
	g["uploadImage"] = func(v Any) Any { return func() Any { return ebiten.NewImageFromImage(v.(image.Image)) } }
	g["clearImage"] = func(v Any) Any { return func() Any { v.(*ebiten.Image).Clear(); return nil } }
	g["disposeImage"] = func(v Any) Any { return func() Any { v.(*ebiten.Image).Deallocate(); return nil } }
	g["fillImage"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			d["target"].(*ebiten.Image).Fill(ebitenNativeColor(d["color"]))
			return nil
		}
	}
	g["fillBackground"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			d["target"].(*ebiten.Image).Fill(rasterColor(d["color"]))
			return nil
		}
	}
	g["drawImage"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			opts := &ebiten.DrawImageOptions{}
			opts.ColorScale.ScaleAlpha(float32(number(d["alpha"])))
			d["target"].(*ebiten.Image).DrawImage(d["source"].(*ebiten.Image), opts)
			return nil
		}
	}
	g["compileShader"] = func(v Any) Any {
		return func() Any {
			shader, err := ebiten.NewShader([]byte(v.(string)))
			if err != nil {
				panic(err)
			}
			return shader
		}
	}
	g["drawShader"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			opts := &ebiten.DrawRectShaderOptions{}
			for i, img := range d["images"].([]Any) {
				opts.Images[i] = img.(*ebiten.Image)
			}
			opts.Uniforms = make(map[string]any)
			for _, value := range d["uniforms"].([]Any) {
				u := value.(Dict)
				values := u["values"].([]Any)
				if len(values) == 1 {
					opts.Uniforms[u["name"].(string)] = float32(number(values[0]))
				} else {
					floats := make([]float32, len(values))
					for i, value := range values {
						floats[i] = float32(number(value))
					}
					opts.Uniforms[u["name"].(string)] = floats
				}
			}
			opts.GeoM.Translate(number(d["x"]), number(d["y"]))
			d["target"].(*ebiten.Image).DrawRectShader(integer(d["width"]), integer(d["height"]), d["shader"].(*ebiten.Shader), opts)
			return nil
		}
	}
	g["newPath"] = func() Any { return &vector.Path{} }
	g["moveTo"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			x, y := ebitenNativePoint(d["point"])
			d["path"].(*vector.Path).MoveTo(x, y)
			return nil
		}
	}
	g["lineTo"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			x, y := ebitenNativePoint(d["point"])
			d["path"].(*vector.Path).LineTo(x, y)
			return nil
		}
	}
	g["quadTo"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			x, y := ebitenNativePoint(d["point"])
			cx, cy := ebitenNativePoint(d["control"])
			d["path"].(*vector.Path).QuadTo(cx, cy, x, y)
			return nil
		}
	}
	g["cubicTo"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			x, y := ebitenNativePoint(d["point"])
			ax, ay := ebitenNativePoint(d["control1"])
			bx, by := ebitenNativePoint(d["control2"])
			d["path"].(*vector.Path).CubicTo(ax, ay, bx, by, x, y)
			return nil
		}
	}
	g["closePath"] = func(v Any) Any { return func() Any { v.(*vector.Path).Close(); return nil } }
	g["fillPath"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			opts := &vector.FillOptions{}
			if d["evenOdd"].(bool) {
				opts.FillRule = vector.FillRuleEvenOdd
			}
			vector.FillPath(d["target"].(*ebiten.Image), d["path"].(*vector.Path), opts, ebitenNativePathOptions(d["color"]))
			return nil
		}
	}
	g["strokePath"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			joins := [...]vector.LineJoin{vector.LineJoinRound, vector.LineJoinBevel, vector.LineJoinMiter}
			caps := [...]vector.LineCap{vector.LineCapButt, vector.LineCapRound, vector.LineCapSquare}
			opts := &vector.StrokeOptions{Width: float32(number(d["width"])), LineJoin: joins[integer(d["join"])], LineCap: caps[integer(d["cap"])]}
			vector.StrokePath(d["target"].(*ebiten.Image), d["path"].(*vector.Path), opts, ebitenNativePathOptions(d["color"]))
			return nil
		}
	}
	g["faceSource"] = func(v Any) Any { return func() Any { return PickFontSource(v.(string)) } }
	g["sameFaceSource"] = curry2(func(a, b Any) Any { return a.(*text.GoTextFaceSource) == b.(*text.GoTextFaceSource) })
	g["newFace"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			return &text.GoTextFace{Source: d["source"].(*text.GoTextFaceSource), Size: number(d["size"])}
		}
	}
	g["faceMetrics"] = func(v Any) Any {
		return func() Any {
			metrics := v.(*text.GoTextFace).Metrics()
			return Dict{"ascent": metrics.HAscent, "descent": metrics.HDescent, "capHeight": metrics.CapHeight}
		}
	}
	g["measureText"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			face := d["face"].(*text.GoTextFace)
			width, _ := text.Measure(d["text"].(string), face, 0)
			return width
		}
	}
	g["drawText"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			opts := &text.DrawOptions{}
			opts.GeoM.Translate(number(d["x"]), number(d["y"]))
			opts.ColorScale.ScaleWithColor(ebitenNativeColor(d["color"]))
			opts.LayoutOptions.PrimaryAlign = text.Align(integer(d["align"]))
			text.Draw(d["target"].(*ebiten.Image), d["text"].(string), d["face"].(*text.GoTextFace), opts)
			return nil
		}
	}
}
