package graphics

import (
	"bytes"
	"encoding/base64"
	"github.com/i-am-the-slime/purescript-native-graphics/drawing"
	. "github.com/purescript-native/go-runtime"
	"image"
	"image/png"
	"runtime"
)

func number(v Any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		panic("native graphics: expected number")
	}
}
func integer(v Any) int { return int(number(v)) }
func numbers(v Any) []float64 {
	src := v.([]Any)
	out := make([]float64, len(src))
	for i, x := range src {
		out[i] = number(x)
	}
	return out
}
func commands(v Any) []drawing.Command {
	src := v.([]Any)
	out := make([]drawing.Command, len(src))
	for i, x := range src {
		out[i] = decodeCommand(x.(Dict))
	}
	return out
}
func decodeCommand(c Dict) drawing.Command {
	return drawing.Command{Kind: integer(c["kind"]), Args: numbers(c["args"]), Path: numbers(c["path"]), Text: c["text"].(string), Font: c["font"].(string), FontID: integer(c["fontID"])}
}
func decodeDrawing(v Any) *drawing.Drawing {
	d := v.(Dict)
	out := &drawing.Drawing{Commands: commands(d["commands"])}
	for _, x := range d["layers"].([]Any) {
		l := x.(Dict)
		out.Layers = append(out.Layers, drawing.Layer{Global: l["global"].(bool)})
	}
	for _, x := range d["composite"].([]Any) {
		p := x.(Dict)
		out.Composite = append(out.Composite, drawing.Composite{Source: integer(p["source"]), Mask: integer(p["mask"]), InvertMask: p["invertMask"].(bool), Blend: integer(p["blend"])})
	}
	for i, x := range d["clear"].([]Any) {
		if i >= 4 {
			break
		}
		out.Clear[i] = float32(number(x))
	}
	return out
}
func encodePNG(v Any) []byte {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, v.(image.Image)); err != nil {
		panic(err)
	}
	return buffer.Bytes()
}
func init() {
	g := Foreign("Native.Graphics")
	g["makeDrawing"] = func(v Any) Any { return decodeDrawing(v) }
	g["rasterize"] = curry3(func(w, h, d Any) Any {
		return func() Any { return Rasterize(integer(w), integer(h), d.(*drawing.Drawing)) }
	})
	g["capture"] = curry3(func(w, h, d Any) Any {
		return func() Any {
			img, err := Capture(integer(w), integer(h), d.(*drawing.Drawing))
			if err != nil {
				panic(err)
			}
			return img
		}
	})
	g["encodePngBytes"] = func(v Any) Any { return func() Any { return encodePNG(v) } }
	g["encodePng"] = func(v Any) Any {
		return func() Any {
			bytes := encodePNG(v)
			out := make([]Any, len(bytes))
			for i, b := range bytes {
				out[i] = int(b)
			}
			return out
		}
	}
	g["bytesFromBase64"] = func(v Any) Any {
		data, err := base64.StdEncoding.DecodeString(v.(string))
		if err != nil {
			panic(err)
		}
		return data
	}
	g["registerFont"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			if err := RegisterFont(d["name"].(string), d["data"].([]byte), d["features"].(string)); err != nil {
				panic(err)
			}
			return nil
		}
	}
	w := Foreign("Native.Window")
	w["platform"] = runtime.GOOS
	w["deviceScaleFactor"] = func() Any { return deviceScaleFactor() }
	w["configure"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			if err := configureResources(d); err != nil {
				panic(err)
			}
			if err := configureWindow(d); err != nil {
				panic(err)
			}
			return nil
		}
	}
	w["run"] = curry2(func(v, callback Any) Any {
		return func() Any {
			d := v.(Dict)
			opts := windowOptions{Backend: d["backend"].(string), Title: d["title"].(string), Width: integer(d["width"]), Height: integer(d["height"]), FrameWidth: integer(d["frameWidth"]), FrameHeight: integer(d["frameHeight"]), FPS: integer(d["fps"])}
			if err := runWindow(opts, func(in Dict) windowOutput { return decodeOutput(Run(Apply(callback, in)).(Dict)) }); err != nil {
				panic(err)
			}
			return nil
		}
	})
}
