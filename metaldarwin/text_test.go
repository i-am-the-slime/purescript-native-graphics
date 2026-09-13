//go:build darwin && !js

package metaldarwin

import (
	"bytes"
	"math"
	"testing"

	"github.com/go-text/typesetting/font"
	"golang.org/x/image/font/gofont/goregular"
)

func TestMSDFShapingDoesNotRequireEbitenWindow(t *testing.T) {
	face, err := font.ParseTTF(bytes.NewReader(goregular.TTF))
	if err != nil {
		t.Fatalf("parse font: %v", err)
	}
	configured := configuredFont{face: face, unicodeKeys: true, glyphs: map[uint32]msdfEntry{
		'P': {py0: -0.75, py1: 0.25, hasGlyph: true},
	}}
	const baseline = 40.0
	const pxSize = 16.0
	var vertices []float32
	appendRunVerts(&vertices, "P", &configured, 0, baseline, pxSize, 0, 0, 0, 1)
	if len(vertices) != 48 {
		t.Fatalf("vertex buffer has %d floats; want one six-vertex glyph quad", len(vertices))
	}
	wantTop := baseline - 0.75*pxSize
	if got := float64(vertices[1]); math.Abs(got-wantTop) > 1e-6 {
		t.Fatalf("glyph top = %.2f, want %.2f from supplied baseline", got, wantTop)
	}
}
