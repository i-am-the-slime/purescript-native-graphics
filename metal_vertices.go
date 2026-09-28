package graphics

import . "github.com/purescript-native/go-runtime"

type metalVertices []float32

type metalVertexBuilder struct {
	values []float32
}

func metalVerticesFromArray(values []Any) metalVertices {
	out := make(metalVertices, len(values))
	for i, value := range values {
		out[i] = float32(number(value))
	}
	return out
}

func metalVerticesConcat(values []Any) metalVertices {
	var single metalVertices
	total, count := 0, 0
	for _, value := range values {
		vertices := value.(metalVertices)
		if len(vertices) != 0 {
			single = vertices
			total += len(vertices)
			count++
		}
	}
	if count <= 1 {
		return single
	}
	out := make(metalVertices, total)
	offset := 0
	for _, value := range values {
		offset += copy(out[offset:], value.(metalVertices))
	}
	return out
}

func init() {
	v := Foreign("Native.Graphics.Metal.Vertices")
	v["empty"] = metalVertices(nil)
	v["fromArray"] = func(value Any) Any { return metalVerticesFromArray(value.([]Any)) }
	v["toArray"] = func(value Any) Any {
		vertices := value.(metalVertices)
		out := make([]Any, len(vertices))
		for i, number := range vertices {
			out[i] = float64(number)
		}
		return out
	}
	v["length"] = func(value Any) Any { return len(value.(metalVertices)) }
	v["concat"] = func(value Any) Any { return metalVerticesConcat(value.([]Any)) }
	v["new"] = func(value Any) Any {
		return func() Any {
			return &metalVertexBuilder{values: make([]float32, 0, max(0, integer(value)))}
		}
	}
	v["freeze"] = func(value Any) Any {
		return func() Any {
			builder := value.(*metalVertexBuilder)
			vertices := metalVertices(builder.values[:len(builder.values):len(builder.values)])
			builder.values = nil
			return vertices
		}
	}
}
