package graphics

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
	. "github.com/purescript-native/go-runtime"
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
func encodePNG(v Any) []byte {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, v.(image.Image)); err != nil {
		panic(err)
	}
	return buffer.Bytes()
}
func init() {
	g := Foreign("Native.Graphics")
	g["captureImpl"] = curry3(func(w, h, render Any) Any {
		return func() Any {
			img, err := Capture(integer(w), integer(h), func(target *ebiten.Image) { Run(Apply(render, target)) })
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
	w := Foreign("Native.Window")
	w["platform"] = runtime.GOOS
	w["deviceScaleFactor"] = func() Any { return deviceScaleFactor() }
	w["maximumFramesPerSecond"] = func() Any { return maximumFramesPerSecond() }
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
	w["runMetalImpl"] = curry3(func(v, callback, rendering Any) Any {
		return func() Any {
			r := rendering.(Dict)
			renderer := metalRenderer{
				render: func(frame, overlay Any, vw, vh float64, fw, fh int) {
					Run(Apply(r["render"], frame, overlay, vw, vh, fw, fh))
				},
				viewportRect: func(vw, vh float64, fw, fh int, rect Dict) Dict {
					return Apply(r["viewportRect"], vw, vh, fw, fh, rect).(Dict)
				},
			}
			if err := runMetal(decodeOptions(v.(Dict)), decodeCallback(callback), renderer); err != nil {
				panic(err)
			}
			return nil
		}
	})
	w["runEbitenImpl"] = curry3(func(v, callback, render Any) Any {
		return func() Any {
			renderer := func(target *ebiten.Image, drawing, overlay Any) {
				Run(Apply(render, target, drawing, overlay))
			}
			if err := runEbiten(decodeOptions(v.(Dict)), decodeCallback(callback), renderer); err != nil {
				panic(err)
			}
			return nil
		}
	})
}
