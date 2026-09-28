package graphics

import . "github.com/purescript-native/go-runtime"

func init() {
	Foreign("Native.Graphics.Metal.Vertices")["appendGlyphQuad"] = curry2(func(builder, quad Any) Any {
		return func() Any {
			buffer := builder.(*metalVertexBuilder)
			values := quad.([]Any)
			x0, y0 := float32(number(values[0])), float32(number(values[1]))
			x1, y1 := float32(number(values[2])), float32(number(values[3]))
			u0, v0 := float32(number(values[4])), float32(number(values[5]))
			u1, v1 := float32(number(values[6])), float32(number(values[7]))
			r, g := float32(number(values[8])), float32(number(values[9]))
			b, a := float32(number(values[10])), float32(number(values[11]))
			buffer.values = append(buffer.values,
				x0, y0, u0, v0, r, g, b, a,
				x1, y0, u1, v0, r, g, b, a,
				x1, y1, u1, v1, r, g, b, a,
				x0, y0, u0, v0, r, g, b, a,
				x1, y1, u1, v1, r, g, b, a,
				x0, y1, u0, v1, r, g, b, a,
			)
			return nil
		}
	})
}
