package graphics

import (
	"bytes"
	"github.com/i-am-the-slime/purescript-native-graphics/drawing"
	"image/color"
	"sync"
	"testing"
)

func rasterDrawing(commands ...drawing.Command) *drawing.Drawing {
	return &drawing.Drawing{Commands: commands, Layers: []drawing.Layer{{}}, Composite: []drawing.Composite{{Source: 0, Mask: -1}}}
}
func TestClearIgnoresViewportTransform(t *testing.T) {
	frame := Rasterize(100, 100, rasterDrawing(drawing.Command{Kind: 15, Args: []float64{0, 0, 50, 100}}, drawing.Command{Kind: 16, Args: []float64{255, 255, 255, 255}}))
	for _, point := range [][2]int{{0, 0}, {99, 0}, {0, 99}, {99, 99}} {
		if got := frame.RGBAAt(point[0], point[1]); got != (color.RGBA{255, 255, 255, 255}) {
			t.Fatalf("background corner %v = %v, want opaque white", point, got)
		}
	}
}
func TestViewportReplacesPriorAbsoluteCamera(t *testing.T) {
	frame := Rasterize(100, 100, rasterDrawing(
		drawing.Command{Kind: 15, Args: []float64{0, 0, 50, 50}},
		drawing.Command{Kind: 15, Args: []float64{0, 0, 100, 100}},
		drawing.Command{Kind: 1, Args: []float64{255, 0, 0, 255}, Path: []float64{1, 75, 75, 2, 85, 75, 2, 85, 85, 2, 75, 85, 5}},
	))
	if got := frame.RGBAAt(80, 80); got != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("pixel under replacement viewport = %v, want opaque red", got)
	}
}
func TestClipMasksCapturedPixels(t *testing.T) {
	frame := Rasterize(100, 100, rasterDrawing(
		drawing.Command{Kind: 16, Args: []float64{255, 255, 255, 255}},
		drawing.Command{Kind: 7, Args: []float64{0}, Path: []float64{1, 25, 25, 2, 75, 25, 2, 75, 75, 2, 25, 75, 5}},
		drawing.Command{Kind: 1, Args: []float64{255, 0, 0, 255}, Path: []float64{1, 0, 0, 2, 100, 0, 2, 100, 100, 2, 0, 100, 5}},
		drawing.Command{Kind: 8},
	))
	if got := frame.RGBAAt(10, 10); got != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("pixel outside clip = %v, want opaque white", got)
	}
	if got := frame.RGBAAt(50, 50); got != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("pixel inside clip = %v, want opaque red", got)
	}
}

func TestConcurrentRasterizeMatchesSerialPixels(t *testing.T) {
	scene := rasterDrawing(
		drawing.Command{Kind: 16, Args: []float64{255, 255, 255, 255}},
		drawing.Command{Kind: 2, Args: []float64{255, 0, 0, 255, 4, 0, 0}, Path: []float64{1, 10, 10, 2, 50, 50, 2, 10, 50, 2, 50, 10}},
	)
	expected := Rasterize(64, 64, scene)
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			actual := Rasterize(64, 64, scene)
			if !bytes.Equal(actual.Pix, expected.Pix) {
				t.Error("concurrent rasterization differs from the serial image")
			}
		}()
	}
	workers.Wait()
}
