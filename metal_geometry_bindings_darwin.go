//go:build darwin && !js

package graphics

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/i-am-the-slime/purescript-native-graphics/metaldarwin"
	. "github.com/purescript-native/go-runtime"
)

func metalGeometryPoint(value Any) (float32, float32) {
	point := value.(Dict)
	return float32(number(point["x"])), float32(number(point["y"]))
}

func metalGeometryIndexed(vertices []ebiten.Vertex, indices []uint16) Any {
	points := make([]Any, len(vertices))
	for i, vertex := range vertices {
		points[i] = Dict{"x": float64(vertex.DstX), "y": float64(vertex.DstY)}
	}
	indexed := make([]Any, len(indices))
	for i, index := range indices {
		indexed[i] = int(index)
	}
	return Dict{"vertices": points, "indices": indexed}
}

func init() {
	g := Foreign("Native.Graphics.Metal.Geometry")
	g["newPath"] = func() Any { return metaldarwin.NewPath() }
	g["moveTo"] = func(value Any) Any {
		return func() Any {
			args := value.(Dict)
			x, y := metalGeometryPoint(args["point"])
			args["path"].(*metaldarwin.NativePath).MoveTo(x, y)
			return nil
		}
	}
	g["lineTo"] = func(value Any) Any {
		return func() Any {
			args := value.(Dict)
			x, y := metalGeometryPoint(args["point"])
			args["path"].(*metaldarwin.NativePath).LineTo(x, y)
			return nil
		}
	}
	g["quadTo"] = func(value Any) Any {
		return func() Any {
			args := value.(Dict)
			cx, cy := metalGeometryPoint(args["control"])
			x, y := metalGeometryPoint(args["point"])
			args["path"].(*metaldarwin.NativePath).QuadTo(cx, cy, x, y)
			return nil
		}
	}
	g["cubicTo"] = func(value Any) Any {
		return func() Any {
			args := value.(Dict)
			c1x, c1y := metalGeometryPoint(args["control1"])
			c2x, c2y := metalGeometryPoint(args["control2"])
			x, y := metalGeometryPoint(args["point"])
			args["path"].(*metaldarwin.NativePath).CubicTo(c1x, c1y, c2x, c2y, x, y)
			return nil
		}
	}
	g["closePath"] = func(value Any) Any {
		return func() Any {
			value.(*metaldarwin.NativePath).Close()
			return nil
		}
	}
	g["tessellateFill"] = func(value Any) Any {
		return func() Any {
			vertices, indices := value.(*metaldarwin.NativePath).Fill()
			return metalGeometryIndexed(vertices, indices)
		}
	}
	g["tessellateStroke"] = func(value Any) Any {
		return func() Any {
			args := value.(Dict)
			vertices, indices := args["path"].(*metaldarwin.NativePath).Stroke(
				float32(number(args["width"])), integer(args["join"]), integer(args["cap"]),
			)
			return metalGeometryIndexed(vertices, indices)
		}
	}
}
