package graphics

import (
	"github.com/hajimehoshi/ebiten/v2"
	. "github.com/purescript-native/go-runtime"
)

type windowOptions struct {
	Title                                       string
	Width, Height, FrameWidth, FrameHeight, FPS int
}
type windowOutput struct {
	Drawing, Overlay                                   Any
	Quit                                               bool
	FrameWidth, FrameHeight, WindowWidth, WindowHeight int
	DragExclusion                                      Dict
}

func decodeOutput(d Dict) windowOutput {
	r := d["dragExclusion"].(Dict)
	return windowOutput{Drawing: d["drawing"], Overlay: d["overlay"], Quit: d["quit"].(bool), FrameWidth: integer(d["frameWidth"]), FrameHeight: integer(d["frameHeight"]), WindowWidth: integer(d["windowWidth"]), WindowHeight: integer(d["windowHeight"]), DragExclusion: r}
}

type windowRenderer func(*ebiten.Image, Any, Any)
type metalRenderer struct {
	render       func(Any, Any, float64, float64, int, int)
	viewportRect func(float64, float64, int, int, Dict) Dict
}

func decodeOptions(d Dict) windowOptions {
	return windowOptions{Title: d["title"].(string), Width: integer(d["width"]), Height: integer(d["height"]), FrameWidth: integer(d["frameWidth"]), FrameHeight: integer(d["frameHeight"]), FPS: integer(d["fps"])}
}

func decodeCallback(callback Any) func(Dict) windowOutput {
	return func(in Dict) windowOutput { return decodeOutput(Run(Apply(callback, in)).(Dict)) }
}

type nativeGame struct {
	options   windowOptions
	callback  func(Dict) windowOutput
	render    windowRenderer
	output    windowOutput
	hasOutput bool
	failure   error
}

func runEbiten(opts windowOptions, callback func(Dict) windowOutput, render windowRenderer) error {
	g := &nativeGame{options: opts, callback: callback, render: render}
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
	in := Dict{"deltaSeconds": delta, "mouseX": float64(x), "mouseY": float64(y), "mouseDown": ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), "frameWidth": g.options.FrameWidth, "frameHeight": g.options.FrameHeight, "viewportWidth": float64(g.options.FrameWidth), "viewportHeight": float64(g.options.FrameHeight), "spaceDown": ebiten.IsKeyPressed(ebiten.KeySpace), "leftDown": ebiten.IsKeyPressed(ebiten.KeyArrowLeft), "rightDown": ebiten.IsKeyPressed(ebiten.KeyArrowRight), "homeDown": ebiten.IsKeyPressed(ebiten.KeyHome), "endDown": ebiten.IsKeyPressed(ebiten.KeyEnd), "tDown": ebiten.IsKeyPressed(ebiten.KeyT), "qDown": ebiten.IsKeyPressed(ebiten.KeyQ), "escapeDown": ebiten.IsKeyPressed(ebiten.KeyEscape), "closeRequested": ebiten.IsWindowBeingClosed()}
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
	g.render(screen, g.output.Drawing, g.output.Overlay)
}
func (g *nativeGame) Layout(int, int) (int, int) { return g.options.FrameWidth, g.options.FrameHeight }
