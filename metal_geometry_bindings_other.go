//go:build !darwin || js

package graphics

import . "github.com/purescript-native/go-runtime"

func init() {
	g := Foreign("Native.Graphics.Metal.Geometry")
	unsupported := func() Any { panic("native graphics: Metal geometry requires Darwin") }
	g["newPath"] = unsupported
	for _, name := range []string{"moveTo", "lineTo", "quadTo", "cubicTo", "closePath", "tessellateFill", "tessellateStroke"} {
		g[name] = func(Any) Any { return unsupported }
	}
}
