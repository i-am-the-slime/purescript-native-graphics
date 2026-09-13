//go:build darwin && !js

package metaldarwin

import "testing"

func rectPath(x, y, w, h float32) []float32 {
	return []float32{
		PathOpMove, x, y,
		PathOpLine, x + w, y,
		PathOpLine, x + w, y + h,
		PathOpLine, x, y + h,
		PathOpClose,
	}
}

func drawContains(draw []float32, x, y float32) bool {
	cross := func(ax, ay, bx, by, px, py float32) float32 {
		return (bx-ax)*(py-ay) - (by-ay)*(px-ax)
	}
	for i := 0; i+17 < len(draw); i += 18 {
		ax, ay := draw[i], draw[i+1]
		bx, by := draw[i+6], draw[i+7]
		cx, cy := draw[i+12], draw[i+13]
		ab := cross(ax, ay, bx, by, x, y)
		bc := cross(bx, by, cx, cy, x, y)
		ca := cross(cx, cy, ax, ay, x, y)
		if (ab >= 0 && bc >= 0 && ca >= 0) || (ab <= 0 && bc <= 0 && ca <= 0) {
			return true
		}
	}
	return false
}

func parityAt(draws [][]float32, x, y float32) int {
	parity := 0
	for _, draw := range draws {
		if drawContains(draw, x, y) {
			parity ^= 1
		}
	}
	return parity
}

func TestEvenOddClipKeepsContoursSeparateForStencilToggle(t *testing.T) {
	r := NewRenderer()
	ring := append(rectPath(0, 0, 100, 100), rectPath(25, 25, 50, 50)...)
	draws := r.buildClipDraws(ring, true)

	if got := parityAt(draws, 10, 10); got != 1 {
		t.Fatalf("outer ring parity = %d, want included", got)
	}
	if got := parityAt(draws, 50, 50); got != 0 {
		t.Fatalf("ring hole parity = %d, want excluded", got)
	}
}
