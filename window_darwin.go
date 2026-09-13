//go:build darwin && !js

package graphics

import (
	"github.com/i-am-the-slime/purescript-native-graphics/metaldarwin"
	. "github.com/purescript-native/go-runtime"
	"math"
	"time"
)

func configureWindow(d Dict) error {
	cfg := metaldarwin.Config{AtlasPNG: d["atlasPNG"].([]byte), AtlasJSON: d["atlasJSON"].([]byte), IconPNG: d["iconPNG"].([]byte), QuitMenuTitle: d["quitMenuTitle"].(string), QuitKeyEquivalent: d["quitKeyEquivalent"].(string), SampleCount: integer(d["sampleCount"])}
	a := d["appearance"].(Dict)
	color := func(v Any) metaldarwin.Color {
		c := v.(Dict)
		return metaldarwin.Color{Red: number(c["red"]), Green: number(c["green"]), Blue: number(c["blue"]), Alpha: number(c["alpha"])}
	}
	cfg.Appearance = metaldarwin.Appearance{Name: a["name"].(string), Background: color(a["background"]), Tint: color(a["tint"]), Opaque: a["opaque"].(bool), TitlebarTransparent: a["titlebarTransparent"].(bool), TitleHidden: a["titleHidden"].(bool), FullSizeContentView: a["fullSizeContentView"].(bool), MovableByBackground: a["movableByBackground"].(bool), Backdrop: a["backdrop"].(bool), Glass: a["glass"].(bool), CornerRadius: number(a["cornerRadius"]), GlassStyle: integer(a["glassStyle"]), Material: integer(a["material"]), BlendingMode: integer(a["blendingMode"]), State: integer(a["state"])}
	for _, v := range d["fonts"].([]Any) {
		f := v.(Dict)
		cfg.Fonts = append(cfg.Fonts, metaldarwin.FontConfig{Data: f["data"].([]byte), AtlasVariant: integer(f["atlasVariant"]), UnicodeKeys: f["unicodeKeys"].(bool)})
	}
	return metaldarwin.Configure(cfg)
}
func runMetal(opts windowOptions, callback func(Dict) windowOutput) error {
	metaldarwin.Setup(opts.Width, opts.Height)
	metaldarwin.SetTitle(opts.Title)
	defer metaldarwin.Stop()
	var last time.Time
	var renderError error
	metaldarwin.SetTickFunc(func() {
		defer func() {
			if failure := recover(); failure != nil {
				renderError = callbackError(failure)
				metaldarwin.Stop()
			}
		}()
		now := time.Now()
		delta := 0.0
		if !last.IsZero() {
			delta = now.Sub(last).Seconds()
		}
		last = now
		vw, vh := metaldarwin.ViewportSize()
		mx, my, down := metaldarwin.MouseState()
		scale := math.Min(float64(vw)/float64(opts.FrameWidth), float64(vh)/float64(opts.FrameHeight))
		ox := (float64(vw) - float64(opts.FrameWidth)*scale) / 2
		oy := (float64(vh) - float64(opts.FrameHeight)*scale) / 2
		x, y := float64(mx), float64(my)
		if scale > 0 {
			x = (x - ox) / scale
			y = (y - oy) / scale
		}
		in := Dict{"deltaSeconds": delta, "mouseX": x, "mouseY": y, "mouseDown": down, "mouseInside": x >= 0 && y >= 0 && x < float64(opts.FrameWidth) && y < float64(opts.FrameHeight), "spaceDown": metaldarwin.KeyDown(metaldarwin.KeySpace), "leftDown": metaldarwin.KeyDown(metaldarwin.KeyLeft), "rightDown": metaldarwin.KeyDown(metaldarwin.KeyRight), "homeDown": metaldarwin.KeyDown(metaldarwin.KeyHome), "endDown": metaldarwin.KeyDown(metaldarwin.KeyEnd), "tDown": metaldarwin.KeyDown(metaldarwin.KeyT), "qDown": metaldarwin.KeyDown(metaldarwin.KeyQ), "escapeDown": metaldarwin.KeyDown(metaldarwin.KeyEscape), "closeRequested": metaldarwin.QuitRequested()}
		out := callback(in)
		if out.Quit {
			metaldarwin.Stop()
			return
		}
		opts.FrameWidth = out.FrameWidth
		opts.FrameHeight = out.FrameHeight
		if out.WindowWidth > 0 && out.WindowHeight > 0 && (out.WindowWidth != opts.Width || out.WindowHeight != opts.Height) {
			opts.Width = out.WindowWidth
			opts.Height = out.WindowHeight
			metaldarwin.Resize(opts.Width, opts.Height)
		}
		vw, vh = metaldarwin.ViewportSize()
		scale = math.Min(float64(vw)/float64(opts.FrameWidth), float64(vh)/float64(opts.FrameHeight))
		ox = (float64(vw) - float64(opts.FrameWidth)*scale) / 2
		oy = (float64(vh) - float64(opts.FrameHeight)*scale) / 2
		metaldarwin.SetDragExclusion(ox+out.DragX*scale, oy+out.DragY*scale, out.DragWidth*scale, out.DragHeight*scale)
		if err := metaldarwin.Prepare(out.Drawing); err != nil {
			renderError = err
			metaldarwin.Stop()
			return
		}
		clear := out.Drawing.Clear
		if !metaldarwin.FrameBegin(clear[0], clear[1], clear[2], clear[3]) {
			return
		}
		rw, rh := metaldarwin.ViewportSize()
		metaldarwin.Render(out.Drawing, float64(rw), float64(rh))
		metaldarwin.RenderOverlay(out.Overlay, float64(rw), float64(rh), float64(opts.FrameWidth), float64(opts.FrameHeight))
		metaldarwin.FrameEnd()
	})
	defer metaldarwin.SetTickFunc(nil)
	metaldarwin.StartTick(opts.FPS)
	metaldarwin.Run()
	return renderError
}
