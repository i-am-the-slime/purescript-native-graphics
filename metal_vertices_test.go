package graphics

import (
	"math"
	"slices"
	"testing"

	. "github.com/purescript-native/go-runtime"
)

func TestMetalVerticesConversionOwnsStorage(t *testing.T) {
	v := Foreign("Native.Graphics.Metal.Vertices")
	input := []Any{0.1, -3, math.Copysign(0, -1), math.Inf(1), math.NaN()}
	vertices := Apply(v["fromArray"], input).(metalVertices)
	readback := Apply(v["toArray"], vertices).([]Any)
	for i, value := range input {
		want := float32(number(value))
		got := float32(readback[i].(float64))
		if math.Float32bits(got) != math.Float32bits(want) {
			t.Fatalf("float %d: got %08x, want %08x", i, math.Float32bits(got), math.Float32bits(want))
		}
	}
	input[0] = 99.0
	readback[1] = 88.0
	if vertices[0] != float32(0.1) || vertices[1] != -3 {
		t.Fatalf("array mutation changed immutable vertices: %v", vertices)
	}
}

func TestMetalVerticesFreezeDetachesAndConcatPreservesOrder(t *testing.T) {
	v := Foreign("Native.Graphics.Metal.Vertices")
	builder := Apply(v["new"], 192).(func() Any)().(*metalVertexBuilder)
	quad := []Any{1.0, 2.0, 3.0, 4.0, 0.1, 0.2, 0.3, 0.4, 0.6, 0.7, 0.8, 0.25}
	appendQuad := func() { Apply(v["appendGlyphQuad"], builder, quad).(func() Any)() }
	freeze := func() metalVertices { return Apply(v["freeze"], builder).(func() Any)().(metalVertices) }
	appendQuad()
	first := freeze()
	if len(first) != 48 || first[0] != 1 {
		t.Fatalf("first glyph quad did not pack its input: %v", first)
	}
	wantFirst := slices.Clone(first)
	if got := freeze(); len(got) != 0 {
		t.Fatalf("second freeze returned already detached data: %v", got)
	}
	empty := v["empty"].(metalVertices)
	withEmpty := Apply(v["concat"], []Any{empty, first, empty}).(metalVertices)
	quad[0], quad[11] = -10.0, 0.0
	appendQuad()
	second := freeze()
	want := append(slices.Clone(wantFirst), second...)
	if len(second) != 48 || second[0] != -10 {
		t.Fatalf("reused builder did not pack its new input: %v", second)
	}
	combined := Apply(v["concat"], []Any{empty, first, empty, second, empty}).(metalVertices)
	quad[0] = 100.0
	appendQuad()
	if !slices.Equal(first, wantFirst) || !slices.Equal(withEmpty, wantFirst) {
		t.Fatal("builder reuse mutated a frozen buffer or singleton concatenation")
	}
	if !slices.Equal(combined, want) {
		t.Fatalf("concatenation changed order or reused mutable storage: got %v, want %v", combined, want)
	}
	if got := Apply(v["length"], combined).(int); got != len(want) {
		t.Fatalf("length returned %d, want %d floats", got, len(want))
	}
	if got := Apply(v["concat"], []Any{empty, empty}).(metalVertices); len(got) != 0 {
		t.Fatalf("concatenating empty buffers produced vertices: %v", got)
	}
}
