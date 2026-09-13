package graphics

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/i-am-the-slime/purescript-native-graphics/drawing"
	"image"
)

// Capture runs the actual GPU drawing backend and reads its framebuffer. It
// needs a native window because Ebiten creates the device inside RunGame.
func Capture(w, h int, d *drawing.Drawing) (image.Image, error) {
	g := &captureGame{width: w, height: h, drawing: d}
	ebiten.SetWindowSize(w, h)
	ebiten.SetWindowTitle("")
	ebiten.SetWindowIcon(windowIcons)
	if err := ebiten.RunGame(g); err != nil && err != ebiten.Termination {
		return nil, err
	}
	return g.image, g.failure
}

type captureGame struct {
	width, height int
	drawing       *drawing.Drawing
	image         *image.RGBA
	done          bool
	failure       error
}

func (g *captureGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *captureGame) Draw(*ebiten.Image) {
	defer func() {
		if failure := recover(); failure != nil {
			g.failure = callbackError(failure)
			g.done = true
		}
	}()
	if g.done {
		return
	}
	g.done = true
	target := ebiten.NewImage(g.width, g.height)
	defer target.Dispose()
	RenderEbiten(target, g.drawing, false)
	buffer := make([]byte, 4*g.width*g.height)
	target.ReadPixels(buffer)
	g.image = &image.RGBA{Pix: buffer, Stride: 4 * g.width, Rect: image.Rect(0, 0, g.width, g.height)}
}
func (g *captureGame) Layout(int, int) (int, int) { return g.width, g.height }
