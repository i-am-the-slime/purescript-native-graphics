package graphics

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/i-am-the-slime/purescript-native-graphics/drawing"
	. "github.com/purescript-native/go-runtime"
)

type windowOptions struct {
	Backend, Title                              string
	Width, Height, FrameWidth, FrameHeight, FPS int
}
type windowOutput struct {
	Drawing, Overlay                                   *drawing.Drawing
	Quit                                               bool
	FrameWidth, FrameHeight, WindowWidth, WindowHeight int
	DragX, DragY, DragWidth, DragHeight                float64
}

func decodeOutput(d Dict) windowOutput {
	r := d["dragExclusion"].(Dict)
	return windowOutput{Drawing: d["drawing"].(*drawing.Drawing), Overlay: d["overlay"].(*drawing.Drawing), Quit: d["quit"].(bool), FrameWidth: integer(d["frameWidth"]), FrameHeight: integer(d["frameHeight"]), WindowWidth: integer(d["windowWidth"]), WindowHeight: integer(d["windowHeight"]), DragX: number(r["x"]), DragY: number(r["y"]), DragWidth: number(r["width"]), DragHeight: number(r["height"])}
}
func runWindow(opts windowOptions, callback func(Dict) windowOutput) error {
	if opts.Backend == "metal" {
		return runMetal(opts, callback)
	}
	return runEbiten(opts, callback)
}

type nativeGame struct {
	options   windowOptions
	callback  func(Dict) windowOutput
	output    windowOutput
	hasOutput bool
	failure   error
}

func runEbiten(opts windowOptions, callback func(Dict) windowOutput) error {
	g := &nativeGame{options: opts, callback: callback}
	ebiten.SetWindowSize(opts.Width, opts.Height)
	ebiten.SetWindowTitle(opts.Title)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowIcon(windowIcons)
	ebiten.SetWindowClosingHandled(true)
	ebiten.SetTPS(opts.FPS)
	return ebiten.RunGame(g)
}
func (g *nativeGame) Update() (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = callbackError(failure)
		}
	}()
	if g.failure != nil {
		return g.failure
	}
	// Update is a logical tick: Ebiten drops excess elapsed wall time after stalls.
	delta := 1.0 / float64(ebiten.TPS())
	x, y := ebiten.CursorPosition()
	in := Dict{"deltaSeconds": delta, "mouseX": float64(x), "mouseY": float64(y), "mouseDown": ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), "mouseInside": x >= 0 && y >= 0 && x < g.options.FrameWidth && y < g.options.FrameHeight, "spaceDown": ebiten.IsKeyPressed(ebiten.KeySpace), "leftDown": ebiten.IsKeyPressed(ebiten.KeyArrowLeft), "rightDown": ebiten.IsKeyPressed(ebiten.KeyArrowRight), "homeDown": ebiten.IsKeyPressed(ebiten.KeyHome), "endDown": ebiten.IsKeyPressed(ebiten.KeyEnd), "tDown": ebiten.IsKeyPressed(ebiten.KeyT), "qDown": ebiten.IsKeyPressed(ebiten.KeyQ), "escapeDown": ebiten.IsKeyPressed(ebiten.KeyEscape), "closeRequested": ebiten.IsWindowBeingClosed()}
	out := g.callback(in)
	g.output = out
	g.hasOutput = true
	if out.Quit {
		return ebiten.Termination
	}
	g.options.FrameWidth = out.FrameWidth
	g.options.FrameHeight = out.FrameHeight
	if out.WindowWidth > 0 && out.WindowHeight > 0 && (out.WindowWidth != g.options.Width || out.WindowHeight != g.options.Height) {
		g.options.Width = out.WindowWidth
		g.options.Height = out.WindowHeight
		ebiten.SetWindowSize(out.WindowWidth, out.WindowHeight)
	}
	return nil
}
func (g *nativeGame) Draw(screen *ebiten.Image) {
	defer func() {
		if failure := recover(); failure != nil {
			g.failure = callbackError(failure)
		}
	}()
	if !g.hasOutput {
		return
	}
	if g.options.Backend == "cpu" {
		image := ebiten.NewImageFromImage(Rasterize(g.options.FrameWidth, g.options.FrameHeight, g.output.Drawing))
		screen.Clear()
		screen.DrawImage(image, nil)
		image.Dispose()
		overlay := ebiten.NewImageFromImage(Rasterize(g.options.FrameWidth, g.options.FrameHeight, g.output.Overlay))
		screen.DrawImage(overlay, nil)
		overlay.Dispose()
	} else {
		RenderEbiten(screen, g.output.Drawing, false)
		RenderEbiten(screen, g.output.Overlay, true)
	}
}
func (g *nativeGame) Layout(int, int) (int, int) { return g.options.FrameWidth, g.options.FrameHeight }
