package graphics

import (
	"bytes"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	. "github.com/purescript-native/go-runtime"
	"github.com/tdewolff/canvas"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type registeredFont struct {
	canvas *canvas.FontFamily
	source *text.GoTextFaceSource
}

var fontRegistry = struct {
	sync.RWMutex
	fonts map[string]*registeredFont
}{fonts: make(map[string]*registeredFont)}

func RegisterFont(name string, data []byte, features string) error {
	family := canvas.NewFontFamily(name)
	if err := family.LoadFont(data, 0, canvas.FontRegular); err != nil {
		return err
	}
	if features != "" {
		family.SetFeatures(features)
	}
	source, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		return err
	}
	fontRegistry.Lock()
	fontRegistry.fonts[name] = &registeredFont{family, source}
	fontRegistry.Unlock()
	return nil
}
func findFont(families string) *registeredFont {
	fontRegistry.RLock()
	defer fontRegistry.RUnlock()
	for _, name := range strings.Split(families, ",") {
		if f := fontRegistry.fonts[strings.Trim(strings.TrimSpace(name), "\"'")]; f != nil {
			return f
		}
	}
	return nil
}
func lookupFont(families string) *registeredFont {
	f := findFont(families)
	if f == nil {
		panic(fmt.Sprintf("native graphics: no registered face in %q", families))
	}
	return f
}
func PickFontSource(family string) *text.GoTextFaceSource { return lookupFont(family).source }

// Native Canvas contexts expose the same font shorthand and measured advance
// contract as CanvasRenderingContext2D; application fallback/cache policy is
// deliberately absent. Ink bounds unavailable from the shaper are NaN.
type measureCanvas struct {
	width, height float64
	context       measureContext
}
type measureContext struct {
	font, family string
	size         float64
}

var fontShorthand = regexp.MustCompile(`(?:^|\s)([0-9]+(?:\.[0-9]+)?)(px|pt)(?:/[^\s]+)?\s+(.+)$`)

func parseFont(font string) (float64, string, bool) {
	match := fontShorthand.FindStringSubmatch(font)
	if match == nil {
		return 0, "", false
	}
	size, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, "", false
	}
	if match[2] == "pt" {
		size *= 96.0 / 72.0
	}
	return size, match[3], true
}
func newMeasureCanvas(w, h float64) *measureCanvas { return &measureCanvas{width: w, height: h} }
func measureNative(ctx *measureContext, content string) float64 {
	face := &text.GoTextFace{Source: PickFontSource(ctx.family), Size: ctx.size}
	width, _ := text.Measure(content, face, 0)
	return width
}
func init() {
	c := Foreign("Graphics.Canvas")
	c["getContext2D"] = func(v Any) Any { return func() Any { return &v.(*measureCanvas).context } }
	c["setFont"] = curry2(func(v, f Any) Any {
		return func() Any {
			ctx := v.(*measureContext)
			font := f.(string)
			size, family, ok := parseFont(font)
			if ok {
				ctx.font = font
				ctx.size = size
				ctx.family = family
			}
			return nil
		}
	})
	c["font"] = func(v Any) Any { return func() Any { return v.(*measureContext).font } }
	c["measureText"] = curry2(func(v, s Any) Any {
		return func() Any { return Dict{"width": measureNative(v.(*measureContext), s.(string))} }
	})
	c["getCanvasWidth"] = func(v Any) Any { return func() Any { return v.(*measureCanvas).width } }
	c["getCanvasHeight"] = func(v Any) Any { return func() Any { return v.(*measureCanvas).height } }
	c["setCanvasWidth"] = curry2(func(v, w Any) Any { return func() Any { v.(*measureCanvas).width = number(w); return nil } })
	c["setCanvasHeight"] = curry2(func(v, h Any) Any { return func() Any { v.(*measureCanvas).height = number(h); return nil } })
	e := Foreign("Graphics.Canvas.Extra")
	e["supportsOffscreenCanvas"] = func() Any { return true }
	e["supportsCanvasElement"] = func() Any { return false }
	e["createOffscreenCanvasImpl"] = func(w, h Any) Any { return newMeasureCanvas(number(w), number(h)) }
	e["getContext2DNullable"] = func(v Any) Any { return func() Any { return &v.(*measureCanvas).context } }
	e["measureTextInkImpl"] = func(v, s Any) Any {
		return Dict{"width": measureNative(v.(*measureContext), s.(string)), "ascent": math.NaN(), "descent": math.NaN()}
	}
	f := Foreign("Web.Font.Loading")
	f["hasFontSet"] = func() Any { return false }
	f["supportsFontFace"] = func() Any { return false }
	f["checkFont"] = func(v Any) Any {
		return func() Any { _, family, ok := parseFont(v.(string)); return ok && findFont(family) != nil }
	}
}
