//go:build darwin && !js

package graphics

import (
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/i-am-the-slime/purescript-native-graphics/metaldarwin"
	. "github.com/purescript-native/go-runtime"
)

func TestMetalGeometryVerticesPreservesIndicesAndTransparentColor(t *testing.T) {
	vertices := []ebiten.Vertex{
		{DstX: 1, DstY: 2},
		{DstX: 3, DstY: 4},
		{DstX: 5, DstY: 6},
	}
	color := Dict{"red": 0.1, "green": 0.2, "blue": 0.3, "alpha": 0.0}
	got := metalGeometryVertices(vertices, []uint16{2, 0, 9, 1, 0, 2, 1}, color)
	want := metalVertices{
		5, 6, 0.1, 0.2, 0.3, 0,
		1, 2, 0.1, 0.2, 0.3, 0,
		3, 4, 0.1, 0.2, 0.3, 0,
		1, 2, 0.1, 0.2, 0.3, 0,
		5, 6, 0.1, 0.2, 0.3, 0,
		3, 4, 0.1, 0.2, 0.3, 0,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("packed triangles: got %v, want %v", got, want)
	}
}

func TestMetalTessellationMatchesFormerUpload(t *testing.T) {
	path := metaldarwin.NewPath()
	path.MoveTo(0.1, 0.2)
	path.LineTo(21.4, 3.8)
	path.LineTo(17.2, 19.3)
	path.LineTo(-2.6, 13.7)
	path.Close()
	color := Dict{"red": 0.13, "green": 0.27, "blue": 0.39, "alpha": 0.41 * 0.53}
	g := Foreign("Native.Graphics.Metal.Geometry")
	for _, stroke := range []bool{false, true} {
		name := "tessellateFill"
		vertices, indices := path.Fill()
		args := Dict{"path": path, "color": color}
		if stroke {
			name = "tessellateStroke"
			vertices, indices = path.Stroke(2.75, 1, 2)
			args["width"], args["join"], args["cap"] = 2.75, 1, 2
		}
		// Former indexed readback and PureScript pack widened positions to
		// Number and left RGBA in Number until Metal upload narrowed each float.
		want := make(metalVertices, 0, len(indices)*6)
		for _, index := range indices {
			vertex := vertices[index]
			for _, value := range []float64{
				float64(vertex.DstX), float64(vertex.DstY),
				number(color["red"]), number(color["green"]), number(color["blue"]), number(color["alpha"]),
			} {
				want = append(want, float32(value))
			}
		}
		got := Apply(g[name], args).(func() Any)().(metalVertices)
		if !slices.Equal(got, want) {
			t.Fatalf("%s upload differs: got %v, want %v", name, got, want)
		}
	}
}
