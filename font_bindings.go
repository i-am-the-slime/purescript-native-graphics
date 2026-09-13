package graphics

import (
	"math"
	"sync"

	. "github.com/purescript-native/go-runtime"
)

var fontCallbacksOnce sync.Once
var fontCallbacks Dict

func init() {
	g := Foreign("Native.Graphics.Font")
	g["installCallbacks"] = func(v Any) Any {
		return func() Any {
			fontCallbacksOnce.Do(func() { fontCallbacks = v.(Dict) })
			return nil
		}
	}
	g["registerResource"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			if err := RegisterFont(d["name"].(string), d["data"].([]byte), d["features"].(string)); err != nil {
				panic(err)
			}
			return nil
		}
	}
	g["isRegistered"] = func(v Any) Any {
		return func() Any { return findFont(v.(string)) != nil }
	}
	g["measureNative"] = func(v Any) Any {
		return func() Any {
			d := v.(Dict)
			width := measureFont(d["name"].(string), number(d["size"]), d["text"].(string))
			return Dict{"width": width, "ascent": math.NaN(), "descent": math.NaN()}
		}
	}

	// Foreign exports exist during module initialization; their effects dispatch
	// only after Native.Graphics.registerFont has installed the PS callbacks.
	c := Foreign("Graphics.Canvas")
	for _, name := range []string{"getContext2D", "font", "getCanvasWidth", "getCanvasHeight"} {
		c[name] = func(v Any) Any {
			return func() Any { return Run(Apply(fontCallbacks[name], v)) }
		}
	}
	for _, name := range []string{"setFont", "measureText", "setCanvasWidth", "setCanvasHeight"} {
		c[name] = curry2(func(v, arg Any) Any {
			return func() Any { return Run(Apply(fontCallbacks[name], v, arg)) }
		})
	}
	e := Foreign("Graphics.Canvas.Extra")
	e["supportsOffscreenCanvas"] = func() Any { return true }
	e["supportsCanvasElement"] = func() Any { return false }
	e["createOffscreenCanvasImpl"] = func(w, h Any) Any {
		return Run(Apply(fontCallbacks["createCanvas"], number(w), number(h)))
	}
	e["getContext2DNullable"] = func(v Any) Any {
		return func() Any { return Run(Apply(fontCallbacks["getContext2D"], v)) }
	}
	e["measureTextInkImpl"] = func(v, s Any) Any {
		return Run(Apply(fontCallbacks["measureTextInk"], v, s))
	}
	f := Foreign("Web.Font.Loading")
	f["hasFontSet"] = func() Any { return false }
	f["supportsFontFace"] = func() Any { return false }
	f["checkFont"] = func(v Any) Any {
		return func() Any { return Run(Apply(fontCallbacks["checkFont"], v)) }
	}
}
